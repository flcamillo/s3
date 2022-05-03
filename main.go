package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// define as variaveis globais
var (
	// configurações gerais do programa
	myConfig *Config
	// define um client para o serviço s3 da aws
	s3client *s3.Client
	// define o gerador de numeros aleatórios
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
)

const (
	// define os tipos de váriaveis para renomeio do arquivo
	renameVars = `#DY = year 4 digits
#YY = year 2 digits
#DM = month number
#DD = day of month
#DJ = day of year
#TH = hour 2 digits 00-23h
#TM = minute 2 digits 00-59
#TS = second 2 digits 00-59
#TU = miliseconds 3 digits 000-999
#SP = timestamp format yyyymmddhhMMssnnnnnnn
#FN = file name without extension
#FE = file extension with dot
#R1 = random number 1 digit 0-9
#R2 = random number 2 digits 00-99
#R4 = random number 4 digits 0000-9999`
)

func main() {
	// define o help do comando
	help := "Usage:\n"
	help += " s3 get -?\n"
	help += " s3 put -?\n"
	help += " s3 config local -?\n"
	help += " s3 config s3 -?\n"
	help += " s3 config vault -?\n"
	// se não há parametros exibe a ajuda
	if len(os.Args) < 2 {
		fmt.Print(help)
		os.Exit(1)
	}
	// define a configuração padrão
	myConfig = DefaultConfig()
	// identifica o diretório do arquivo de configuração
	// prioridade do diretório:
	// 1) variavel de ambiente S3_CONFIG
	// 2) diretório padrão do usuário
	// 3) diretório da aplicação
	configDir := os.Getenv("S3_CONFIG")
	if configDir == "" {
		dir, err1 := os.UserHomeDir()
		if err1 != nil {
			dir, err2 := filepath.Abs(filepath.Dir(os.Args[0]))
			if err2 != nil {
				log.Fatal("unable to identify configuration directory")
			}
			configDir = dir
		} else {
			configDir = dir
		}
	}
	// carrega o arquivo de configuração
	if configDir != "" {
		err := myConfig.Load(filepath.Join(configDir, "s3.json"))
		if err != nil {
			log.Printf("unable to load configuration file, %s", err)
		}
	}
	// se definido carrega o access key, secret key e access token
	// das variaveis de ambiente, estes valores são prioridades
	// ao invés do que esta configurado
	myConfig.AccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	myConfig.SecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	myConfig.AccessToken = os.Getenv("AWS_SESSION_TOKEN")
	// identifica a operação
	switch os.Args[1] {
	case "get":
		processGet(os.Args[2:])
	case "put":
		processPut(os.Args[2:])
	case "config":
		args := os.Args[2:]
		if len(args) == 0 {
			fmt.Print(help)
			os.Exit(1)
		}
		switch args[0] {
		case "s3":
			processConfigS3(args[1:])
		case "vault":
			processConfigVault(args[1:])
		case "local":
			processConfigLocal(args[1:])
		default:
			fmt.Print(help)
			os.Exit(1)
		}
	default:
		fmt.Print(help)
		os.Exit(1)
	}
}

// Carrega as credenciais para acesso ao bucket
func loadCredentials(role string) error {
	// verifica se deve buscar as credenciais no vault
	if myConfig.VaultAddress == "" {
		return fmt.Errorf("vault address not provided")
	}
	// valida se possui role para gerar as credenciais
	if role == "" {
		return fmt.Errorf("vault role not provided")
	}
	// valida o caminho da engine, se não foi informado
	// assume o default
	if myConfig.VaultEnginePath == "" {
		myConfig.VaultEnginePath = "aws"
	} else {
		myConfig.VaultEnginePath = strings.TrimPrefix(myConfig.VaultEnginePath, "/")
		myConfig.VaultEnginePath = strings.TrimSuffix(myConfig.VaultEnginePath, "/")
	}
	// cria um novo client para o vault
	vault := NewVault(myConfig.VaultAddress, myConfig.VaultAuthToken)
	// identifica o metodo de autenticação a ser usado
	switch strings.ToLower(myConfig.VaultAuthMethod) {
	case VaultAuthByCertificate:
		if myConfig.VaultAuthCert == "" {
			return fmt.Errorf("vault autentication certificate not provided")
		}
		if myConfig.VaultAuthCertKey == "" {
			return fmt.Errorf("vault autentication certificate key not provided")
		}
		if myConfig.VaultAuthCertCA == "" {
			return fmt.Errorf("vault autentication CA certificate not provided")
		}
		err := vault.AuthByCertificate(myConfig.VaultAuthCertPath, myConfig.VaultAuthCert, myConfig.VaultAuthCertKey, myConfig.VaultAuthCertCA, myConfig.VaultAuthCertRole)
		if err != nil {
			return err
		}
	case VaultAuthByAppRole:
		if myConfig.VaultAuthRoleId == "" {
			return fmt.Errorf("vault autentication role id not provided")
		}
		if myConfig.VaultAuthSecretId == "" {
			return fmt.Errorf("vault autentication secret id not provided")
		}
		err := vault.AuthByAppRole(myConfig.VaultAuthAppRolePath, myConfig.VaultAuthRoleId, myConfig.VaultAuthSecretId)
		if err != nil {
			return err
		}
	default:
		if myConfig.VaultAuthToken == "" {
			return fmt.Errorf("vault token not provided")
		}
	}
	// solicita a credencial ao vault
	secret, err := vault.Secrets(fmt.Sprintf("%s/%s", myConfig.VaultEnginePath, "sts"), role, "", "1")
	if err != nil {
		return err
	}
	// configura as credenciais
	myConfig.AccessKey = secret["access_key"]
	myConfig.SecretKey = secret["secret_key"]
	myConfig.AccessToken = secret["security_token"]
	return nil
}

// Salva as configurações
func saveConfig() error {
	// define o caminho do arquivo de configuração
	configPath, _ := os.UserHomeDir()
	if configPath == "" {
		configPath, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	}
	configPath = filepath.Join(configPath, "s3.json")
	// salva as configurações
	return myConfig.Save(configPath)
}

// processa o comando de configuração
func processConfigS3(args []string) {
	// identifica os flags informados
	cmdConfig := flag.NewFlagSet("s3", flag.ExitOnError)
	// define os parametros para utilização
	pBucket := cmdConfig.String("bucket", "", "bucket name")
	pRegion := cmdConfig.String("region", "", "bucket region")
	pPartSize := cmdConfig.Int("partsize", 0, "size of each part of the file uploaded to the bucket (use 0 to automatic calculate)")
	pMetaData := cmdConfig.String("metadata", "", "metadata that will be stored in the file uploaded to the bucket (sintax key1=value1;key2=value2...)")
	pEndPoint := cmdConfig.String("endpoint", "", "url of bucket end point (sintax https://my-s3-url.com)")
	pAccessKey := cmdConfig.String("accesskey", "", "bucket access key (will be asked to vault if not provided)")
	pSecretKey := cmdConfig.String("secretkey", "", "bucket secret key (will be asked to vault if not provided)")
	pAccessToken := cmdConfig.String("accesstoken", "", "token session")
	// processa os parametros
	err := cmdConfig.Parse(args)
	if err != nil || len(args) == 0 {
		cmdConfig.Usage()
		os.Exit(1)
	}
	// configura o bucket
	if *pBucket != "" {
		myConfig.Bucket = *pBucket
	}
	// configura a região do bucket
	if *pRegion != "" {
		myConfig.Region = *pRegion
	} else {
		if myConfig.Region == "" {
			myConfig.Region = "sa-east-1"
		}
	}
	// configura o tamanho das partes para o envio de arquivo multipart
	if *pPartSize < 5*1024*1024 {
		myConfig.PartSize = 0
	} else {
		myConfig.PartSize = *pPartSize
	}
	// configura os metadados que serão gravados por padrão em todos os
	// arquivos que forem enviados para o bucket
	if *pMetaData != "" {
		// inicializa o mapa de metadados
		myConfig.Metadata = make(map[string]string)
		// extrai e configura os metados informados
		// os metadados seguem o padrão: key1=valor1;key2=valor2
		values := strings.Split(*pMetaData, ";")
		for k, v := range values {
			keyvalue := strings.Split(v, "=")
			if len(keyvalue) < 2 {
				log.Fatalf("[%d] metadata {%s} is invalid", k, v)
			}
			myConfig.Metadata[strings.TrimSpace(keyvalue[0])] = strings.TrimSpace(keyvalue[1])
		}
	}
	// configura o endereço http do endpoint para acesso ao bucket
	if *pEndPoint != "" {
		myConfig.EndPoint = *pEndPoint
	}
	// configura as credenciais para acesso ao bucket
	if *pAccessKey != "" {
		myConfig.AccessKey = *pAccessKey
	}
	if *pSecretKey != "" {
		myConfig.SecretKey = *pSecretKey
	}
	if *pAccessToken != "" {
		myConfig.AccessToken = *pAccessToken
	}
	// grava as configurações
	err = saveConfig()
	if err != nil {
		log.Fatal(err)
	}
}

// processa o comando de configuração
func processConfigLocal(args []string) {
	// identifica os flags informados
	cmdConfig := flag.NewFlagSet("local", flag.ExitOnError)
	// define os parametros para utilização
	pFolder := cmdConfig.String("folder", "", "default folder of file to upload or download")
	// processa os parametros
	err := cmdConfig.Parse(args)
	if err != nil || len(args) == 0 {
		cmdConfig.Usage()
		os.Exit(1)
	}
	// configura a pasta dos arquivos
	if *pFolder != "" {
		myConfig.LocalFolder = *pFolder
	}
	// grava as configurações
	err = saveConfig()
	if err != nil {
		log.Fatal(err)
	}
}

// processa o comando de configuração
func processConfigVault(args []string) {
	// identifica os flags informados
	cmdConfig := flag.NewFlagSet("vault", flag.ExitOnError)
	// define os parametros para utilização
	pVaultAddress := cmdConfig.String("endpoint", "", "url of vault api (sintax https://my-vault-url.com)")
	pVaultAuthToken := cmdConfig.String("token", "", "vault authentication token")
	pVaultAuthMethod := cmdConfig.String("auth", "", "vault authentication method (token, approle, cert)")
	pVaultEnginePath := cmdConfig.String("enginepath", "", "vault engine path to ask for credentials")
	// parametros para autenticação via app role
	pVaultAuthRoleId := cmdConfig.String("authrole", "", "vault authentication role id")
	pVaultAuthSecretId := cmdConfig.String("authsecret", "", "vault authentication secret id")
	pVaultAppRolePath := cmdConfig.String("approlepath", "", "vault approle authentication path")
	// parametros para autenticação via certificado
	pVaultAuthCert := cmdConfig.String("authcert", "", "vault authentication certificate")
	pVaultAuthCertKey := cmdConfig.String("authcertkey", "", "vault authentication certificate key")
	pVaultAuthCertCA := cmdConfig.String("authcertca", "", "vault authentication certificate CA")
	pVaultAuthCertRole := cmdConfig.String("authcertrole", "", "vault authentication certificate role name")
	pVaultAuthCertPath := cmdConfig.String("authcertpath", "", "vault certificate autentication path")
	// processa os parametros
	err := cmdConfig.Parse(args)
	if err != nil || len(args) == 0 {
		cmdConfig.Usage()
		os.Exit(1)
	}
	// configura o endereço http para as APIs do vault
	if *pVaultAddress != "" {
		myConfig.VaultAddress = *pVaultAddress
	}
	// configura o token de acesso ao vault
	if *pVaultAuthToken != "" {
		myConfig.VaultAuthToken = *pVaultAuthToken
	}
	// configura o metodo de autenticação do vault
	method := strings.ToLower(*pVaultAuthMethod)
	if method != "" {
		if method != VaultAuthByAppRole && method != VaultAuthByToken && method != VaultAuthByCertificate {
			log.Fatalf("vault authentication method {%s} is invalid", method)
		}
		myConfig.VaultAuthMethod = method
	} else {
		if myConfig.VaultAuthMethod == "" {
			myConfig.VaultAuthMethod = "token"
		}
	}
	// configura o approle role id
	if *pVaultAuthRoleId != "" {
		myConfig.VaultAuthRoleId = *pVaultAuthRoleId
	}
	// configura o approle secret id
	if *pVaultAuthSecretId != "" {
		myConfig.VaultAuthSecretId = *pVaultAuthSecretId
	}
	// configura o approle path
	if *pVaultAppRolePath != "" {
		myConfig.VaultAuthAppRolePath = *pVaultAppRolePath
	}
	// configura o caminho do arquivo de certificado
	if *pVaultAuthCert != "" {
		myConfig.VaultAuthCert = *pVaultAuthCert
	}
	// configura o caminho do arquivo contendo a chave do certificado
	if *pVaultAuthCertKey != "" {
		myConfig.VaultAuthCertKey = *pVaultAuthCertKey
	}
	// configura o caminho do arquivo de certificado raiz (CA)
	if *pVaultAuthCertCA != "" {
		myConfig.VaultAuthCertCA = *pVaultAuthCertCA
	}
	// configura o nome da role de certificado
	if *pVaultAuthCertRole != "" {
		myConfig.VaultAuthCertRole = *pVaultAuthCertRole
	}
	// configura o caminho da url para autenticar via certificado
	if *pVaultAuthCertPath != "" {
		myConfig.VaultAuthCertPath = *pVaultAuthCertPath
	}
	// configura o caminho da engine para solicitar credenciais
	if *pVaultEnginePath != "" {
		myConfig.VaultEnginePath = *pVaultEnginePath
	} else {
		if myConfig.VaultEnginePath == "" {
			myConfig.VaultEnginePath = "aws"
		}
	}
	// grava as configurações
	err = saveConfig()
	if err != nil {
		log.Fatal(err)
	}
}

// processa o comando de download de arquivos
func processGet(args []string) {
	// identifica os flags informados
	cmdGet := flag.NewFlagSet("get", flag.ExitOnError)
	// define os parametros para sobrescrever o padrão configurado
	pBucket := cmdGet.String("b", "", "bucket name")
	pRegion := cmdGet.String("br", "", "bucket region")
	pPartSize := cmdGet.Int("ps", 0, "size of each part of the file uploaded to the bucket (use 0 to automatic calculate)")
	pEndPoint := cmdGet.String("ep", "", "url of bucket end point (sintax https://my-s3-url.com)")
	pFolder := cmdGet.String("df", "", "default folder for files")
	// define os parametros para utilização específicos para este método
	pFilter := cmdGet.String("f", "", "filter to select files")
	pRemove := cmdGet.Bool("rm", false, "remove files after transfer")
	pRename := cmdGet.String("c", "", fmt.Sprintf("change the name of target file\n%s", renameVars))
	pErrorNoFiles := cmdGet.Bool("enf", false, "terminate with exit code 1 if no files found")
	pRole := cmdGet.String("r", "", "vault role name to access bucket")
	// parametros adicionais
	pBucketPrefix := cmdGet.String("bp", "", "bucket prefix (sub folder)")
	// processa os parametros
	err := cmdGet.Parse(args)
	if err != nil || len(args) == 0 {
		cmdGet.Usage()
		os.Exit(1)
	}
	// configura o bucket
	if *pBucket != "" {
		myConfig.Bucket = *pBucket
	}
	// configura a região do bucket
	if *pRegion != "" {
		myConfig.Region = *pRegion
	}
	// configura o tamanho das partes para o envio de arquivo multipart
	if *pPartSize < 5*1024*1024 {
		myConfig.PartSize = 0
	} else {
		myConfig.PartSize = *pPartSize
	}
	// configura o endereço http do endpoint para acesso ao bucket
	if *pEndPoint != "" {
		myConfig.EndPoint = *pEndPoint
	}
	// configura a pasta padrão onde estão ou serão gravados os arquivos
	if *pFolder != "" {
		myConfig.LocalFolder = *pFolder
	}
	// valida o filtro
	if *pFilter == "" {
		log.Fatalf("file name filter not provided")
	}
	// valida o rename
	if *pRename == "" {
		*pRename = "#FN#FE"
	}
	// ajusta o prefixo do bucket (sub pasta)
	if *pBucketPrefix != "" {
		*pBucketPrefix = strings.TrimPrefix(*pBucketPrefix, "/")
		if !strings.HasSuffix(*pBucketPrefix, "/") {
			*pBucketPrefix = *pBucketPrefix + "/"
		}
	}
	// carrega das credenciais do vault se necessário
	if myConfig.AccessKey == "" || myConfig.SecretKey == "" {
		err = loadCredentials(*pRole)
		if err != nil {
			log.Fatal(err)
		}
	}
	// valida se há parametros suficientes
	if *pFilter == "" || myConfig.Bucket == "" {
		cmdGet.Usage()
		os.Exit(1)
	}
	// inicializa o serviço da aws
	err = configureAWSClient()
	if err != nil {
		log.Fatal(err)
	}
	// executa as recepções
	err = receiveFiles(*pFilter, *pBucketPrefix, myConfig.LocalFolder, *pRename, *pRemove, *pErrorNoFiles)
	if err != nil {
		log.Fatal(err)
	}
}

// processa o comando de upload de arquivos
func processPut(args []string) {
	// identifica os flags informados
	cmdPut := flag.NewFlagSet("put", flag.ExitOnError)
	// define os parametros para sobrescrever o padrão configurado
	pBucket := cmdPut.String("b", "", "bucket name")
	pRegion := cmdPut.String("br", "", "bucket region")
	pPartSize := cmdPut.Int("ps", 0, "size of each part of the file uploaded to the bucket (use 0 to automatic calculate)")
	pMetaData := cmdPut.String("m", "", "metadata that will be stored in the file uploaded to the bucket (sintax key1=value1;key2=value2...)")
	pEndPoint := cmdPut.String("ep", "", "url of bucket end point (sintax https://my-s3-url.com)")
	pFolder := cmdPut.String("df", "", "default folder for files")
	// define os parametros para utilização específicos para este método
	pFilter := cmdPut.String("f", "", "filter to select files")
	pRemove := cmdPut.Bool("rm", false, "remove files after transfer")
	pRename := cmdPut.String("c", "", fmt.Sprintf("change the name of target file\n%s", renameVars))
	pErrorNoFiles := cmdPut.Bool("enf", false, "terminate with exit code 1 if no files found")
	pRole := cmdPut.String("r", "", "vault role name to access bucket")
	// parametros adicionais
	pBucketPrefix := cmdPut.String("bp", "", "bucket prefix (sub folder)")
	// processa os parametros
	err := cmdPut.Parse(args)
	if err != nil || len(args) == 0 {
		cmdPut.Usage()
		os.Exit(1)
	}
	// configura o bucket
	if *pBucket != "" {
		myConfig.Bucket = *pBucket
	}
	// configura a região do bucket
	if *pRegion != "" {
		myConfig.Region = *pRegion
	}
	// configura o tamanho das partes para o envio de arquivo multipart
	if *pPartSize < 5*1024*1024 {
		myConfig.PartSize = 0
	} else {
		myConfig.PartSize = *pPartSize
	}
	// configura os metadados que serão gravados por padrão em todos os
	// arquivos que forem enviados para o bucket
	if *pMetaData != "" {
		// inicializa o mapa de metadados
		myConfig.Metadata = make(map[string]string)
		// extrai e configura os metados informados
		// os metadados seguem o padrão: key1=valor1;key2=valor2
		values := strings.Split(*pMetaData, ";")
		for k, v := range values {
			keyvalue := strings.Split(v, "=")
			if len(keyvalue) < 2 {
				log.Fatalf("[%d] metadata {%s} is invalid", k, v)
			}
			myConfig.Metadata[strings.TrimSpace(keyvalue[0])] = strings.TrimSpace(keyvalue[1])
		}
	}
	// configura o endereço http do endpoint para acesso ao bucket
	if *pEndPoint != "" {
		myConfig.EndPoint = *pEndPoint
	}
	// configura a pasta padrão onde estão ou serão gravados os arquivos
	if *pFolder != "" {
		myConfig.LocalFolder = *pFolder
	}
	// valida o filtro
	if *pFilter == "" {
		log.Fatalf("file name filter not provided")
	}
	// valida o rename
	if *pRename == "" {
		*pRename = "#FN#FE"
	}
	// ajusta o prefixo do bucket (sub pasta)
	if *pBucketPrefix != "" {
		*pBucketPrefix = strings.TrimPrefix(*pBucketPrefix, "/")
		if !strings.HasSuffix(*pBucketPrefix, "/") {
			*pBucketPrefix = *pBucketPrefix + "/"
		}
	}
	// carrega das credenciais do vault se necessário
	if myConfig.AccessKey == "" || myConfig.SecretKey == "" {
		err = loadCredentials(*pRole)
		if err != nil {
			log.Fatal(err)
		}
	}
	// valida se há parametros suficientes
	if *pFilter == "" || myConfig.Bucket == "" {
		cmdPut.Usage()
		os.Exit(1)
	}
	// inicializa o serviço da aws
	err = configureAWSClient()
	if err != nil {
		log.Fatal(err)
	}
	// executa as recepções
	err = sendFiles(*pFilter, *pBucketPrefix, myConfig.LocalFolder, *pRename, *pRemove, myConfig.Metadata, *pErrorNoFiles)
	if err != nil {
		log.Fatal(err)
	}
}

// inicializa o serviço da aws com base nas configurações
func configureAWSClient() (err error) {
	// configura o transport do client http
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	// cria o client http
	httpClient := &http.Client{
		Transport: tr,
	}
	// define o resolver para o endpoint da aws
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if myConfig.EndPoint != "" {
			return aws.Endpoint{
				PartitionID:       "aws",
				URL:               myConfig.EndPoint,
				SigningRegion:     myConfig.Region,
				HostnameImmutable: true,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})
	// define a configuração da aws
	var awsConfig aws.Config
	// define o provedor de credenciais da aws
	provider := credentials.NewStaticCredentialsProvider(myConfig.AccessKey, myConfig.SecretKey, myConfig.AccessToken)
	// configura as credenciais
	awsConfig, err = config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(provider), config.WithEndpointResolverWithOptions(customResolver), config.WithHTTPClient(httpClient))
	if err != nil {
		return err
	}
	// configura a region
	if myConfig.Region != "" {
		awsConfig.Region = myConfig.Region
	}
	// configura o serviço
	s3client = s3.NewFromConfig(awsConfig)
	return nil
}

// Realiza o envio dos arquivos para o bucket com o filtro especificado
func sendFiles(filter string, prefix string, folder string, rename string, remove bool, metaData map[string]string, errornofiles bool) error {
	// loga o endpoint e o bucket que será usado
	if myConfig.EndPoint != "" {
		log.Printf("using custom endpoint {%s} for bucket {%s}...", myConfig.EndPoint, myConfig.Bucket)
	} else {
		log.Printf("using default AWS endpoint for bucket {%s}...", myConfig.Bucket)
	}
	// lista os arquivos que batem com o filtro
	matches, err := filepath.Glob(filepath.Join(folder, filter))
	if err != nil {
		return fmt.Errorf("unable to list files with filter {%s}, %s", filter, err)
	}
	// se não encontrou arquivo retorna
	if len(matches) == 0 {
		log.Printf("no files found in folder {%s} with filter {%s}", folder, filter)
		if errornofiles {
			os.Exit(1)
		}
		return nil
	}
	// lista os arquivos
	for k, v := range matches {
		log.Printf("[%d] selected to upload: %s", k, v)
	}
	// realiza o envio
	for k, v := range matches {
		// captura o horário de início da transmissão
		start := time.Now()
		// define o nome do arquivo que sera gravado no bucket
		fileName := prefix + parseName(v, rename)
		// realiza o envio
		log.Printf("[%d] starting upload of file {%s}...", k, v)
		n, result, err := send(v, fileName, metaData)
		if err != nil {
			log.Fatalf("[%d] failed to upload file {%s}, %s", k, v, err)
		}
		// calcula a taxa de envio do arquivo
		elapsed := time.Since(start).Seconds()
		var rate float64
		if elapsed <= 0 {
			rate = float64(n)
		} else {
			rate = float64(n) / float64(elapsed)
		}
		rate /= 1024 * 1024
		log.Printf("[%d] upload completed, size: %d elapsed: %.2fs rate: %.2fMB/s url: %s", k, n, elapsed, rate, result.Location)
		// verifica se deve remover o arquivo
		if remove {
			err = os.Remove(v)
			if err != nil {
				log.Printf("[%d] unable to remove file {%s}, %s", k, v, err)
			} else {
				log.Printf("[%d] file {%s} removed successfully", k, v)
			}
		}
	}
	return nil
}

// realiza o envio dos arquivos com o filtro especificado para o bucket
func send(file string, key string, metaData map[string]string) (n int64, result *manager.UploadOutput, err error) {
	// abre o arquivo para realizar o envio
	f, err := os.OpenFile(file, os.O_RDONLY, 0774)
	if err != nil {
		return 0, nil, fmt.Errorf("unable to open file {%s}, %s", file, err)
	}
	defer f.Close()
	// extrai as propriedades do arquivo
	stat, err := f.Stat()
	if err != nil {
		return 0, nil, fmt.Errorf("unable to read properties of file {%s}, %s", file, err)
	}
	// calcula o tamanho da parte se necessário
	partSize := myConfig.PartSize
	if partSize < 1024*1024*5 {
		if stat.Size() < 1024*1024*1024*10 {
			partSize = 1024 * 1024 * 64
		} else if stat.Size() < 1024*1024*1024*100 {
			partSize = 1024 * 1024 * 100
		} else {
			partSize = 1024 * 1024 * 250
		}
	}
	// configura o uploader
	uploader := manager.NewUploader(s3client, func(u *manager.Uploader) {
		u.PartSize = int64(partSize)
	})
	// realiza o envio
	result, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket:   aws.String(myConfig.Bucket),
		Key:      aws.String(key),
		Body:     f,
		Metadata: metaData,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("transfer failed, %s", err)
	}
	return stat.Size(), result, nil
}

// Recebe todos os arquivos que atendem ao filtro especificado
func receiveFiles(filter string, prefix string, folder string, rename string, remove bool, errornofiles bool) error {
	// define uma váriavel para usar para armazenar os arquivos que serão baixados
	var matches []types.Object
	// define um contador para exibir quantos objetos foram verificados no bucket
	var count int64
	// loga o endpoint e o bucket que será usado
	if myConfig.EndPoint != "" {
		log.Printf("using custom endpoint {%s} for bucket {%s}...", myConfig.EndPoint, myConfig.Bucket)
	} else {
		log.Printf("using default AWS endpoint for bucket {%s}...", myConfig.Bucket)
	}
	// se foi passado wildcard no filto então lista o bucket para selecionar os arquivos
	if strings.Contains(filter, "*") {
		// define os parametros de listagem
		params := &s3.ListObjectsV2Input{
			Bucket: aws.String(myConfig.Bucket),
			Prefix: aws.String(prefix),
		}
		// define o paginador
		paginator := s3.NewListObjectsV2Paginator(s3client, params, func(o *s3.ListObjectsV2PaginatorOptions) {
			o.Limit = 1000
		})
		// define a expressão regular para realizar a pesquisa
		// caso seja passado o prefixo do bucket o mesmo deve
		// ser considerado na validação
		pattern := wildCardToRegexp(filter)
		if prefix != "" {
			pattern = regexp.QuoteMeta(prefix) + pattern
		}
		// processa a listagem das páginas
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(context.TODO())
			if err != nil {
				return fmt.Errorf("unable to list bucket, %s", err)
			}
			for _, value := range output.Contents {
				count++
				match, err := regexp.MatchString(pattern, *value.Key)
				if err != nil {
					return fmt.Errorf("unable filter files, %s", err)
				}
				if match {
					matches = append(matches, value)
				}
			}
		}
		// exibe a quantidade de objetos lidos do bucket
		log.Printf("total of keys verified in bucket {%s}: %d", myConfig.Bucket, count)
		// verifica se foi selecionado algum arquivo
		if len(matches) == 0 {
			log.Printf("no files found in bucket {%s} with filter {%s}", myConfig.Bucket, filter)
			if errornofiles {
				os.Exit(1)
			}
			return nil
		}
	} else {
		// adiciona o proprio filtro para buscar no bucket
		matches = append(matches, types.Object{
			Key: aws.String(prefix + filter),
		})
	}
	// lista os arquivos
	for k, v := range matches {
		log.Printf("[%d] selected to download: %s", k, *v.Key)
	}
	// realiza a recepção
	for k, v := range matches {
		// captura o horário de início da transmissão
		start := time.Now()
		// define o nome do arquivo que sera recebido
		filePath := filepath.Join(folder, parseName(*v.Key, rename))
		// realiza a recepção
		log.Printf("[%d] starting download of file {%s}...", k, *v.Key)
		n, err := receive(*v.Key, filePath)
		if err != nil {
			log.Fatalf("[%d] failed to download file {%s}, %s", k, *v.Key, err)
		}
		// calcula a taxa de recepção do arquivo
		elapsed := time.Since(start).Seconds()
		var rate float64
		if elapsed <= 0 {
			rate = float64(n)
		} else {
			rate = float64(n) / float64(elapsed)
		}
		rate /= 1024 * 1024
		log.Printf("[%d] download completed, size: %dbytes elapsed: %.2fs rate: %.2fMB/s path: %s", k, n, elapsed, rate, filePath)
		// verifica se deve remover o arquivo
		if remove {
			_, err := s3client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
				Bucket: aws.String(myConfig.Bucket),
				Key:    aws.String(*v.Key),
			})
			if err != nil {
				log.Printf("[%d] unable to remove file {%s}, %s", k, *v.Key, err)
			} else {
				log.Printf("[%d] file {%s} removed successfully", k, *v.Key)
			}
		}
	}
	return nil
}

// realiza a recepção do arquivo
func receive(key string, filePath string) (n int64, err error) {
	// cria o arquivo em disco
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0774)
	if err != nil {
		return 0, fmt.Errorf("unable to create file, %s", err)
	}
	defer f.Close()
	// configura o downloader
	downloader := manager.NewDownloader(s3client, func(d *manager.Downloader) {
		d.PartSize = 64 * 1024 * 1024
	})
	// inicia o download
	n, err = downloader.Download(context.TODO(), f, &s3.GetObjectInput{
		Bucket: aws.String(myConfig.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("unable to download file, %s", err)
	}
	return
}

// converte uma expressão wildcard para regex
func wildCardToRegexp(pattern string) string {
	var result strings.Builder
	var lastRune rune
	for _, v := range pattern {
		if v == '*' {
			if lastRune != v {
				lastRune = v
				result.WriteString(".*")
			}
			continue
		}
		result.WriteRune(v)
		lastRune = v
	}
	return result.String()
}

// converte um nome em outro usando a máscara informada
func parseName(name string, mask string) string {
	// define os valores das datas
	date := time.Now()
	year := date.Format("2006")
	month := date.Format("01")
	day := date.Format("02")
	dayOfYear := date.YearDay()
	hour := date.Format("15")
	minute := date.Format("04")
	second := date.Format("05")
	milisecond := date.Format("000")
	timestamp := strings.ReplaceAll(date.Format("20060102150405.999999999"), ".", "")
	random1 := random.Intn(9)
	random2 := random.Intn(99)
	random4 := random.Intn(9999)
	// extrai apenas o nome do arquivo
	_, name = filepath.Split(name)
	// extrai o nome e a extenção do arquivo
	ext := ""
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			ext = name[i:]
			name = name[:i]
			break
		}
	}
	// converte a máscara
	result := strings.ReplaceAll(mask, "#DY", year)
	result = strings.ReplaceAll(result, "#YY", year[2:])
	result = strings.ReplaceAll(result, "#DM", month)
	result = strings.ReplaceAll(result, "#DD", day)
	result = strings.ReplaceAll(result, "#DJ", fmt.Sprintf("%d", dayOfYear))
	result = strings.ReplaceAll(result, "#TH", hour)
	result = strings.ReplaceAll(result, "#TM", minute)
	result = strings.ReplaceAll(result, "#TS", second)
	result = strings.ReplaceAll(result, "#TU", milisecond)
	result = strings.ReplaceAll(result, "#SP", timestamp)
	result = strings.ReplaceAll(result, "#FN", name)
	result = strings.ReplaceAll(result, "#FE", ext)
	result = strings.ReplaceAll(result, "#R1", fmt.Sprintf("%01d", random1))
	result = strings.ReplaceAll(result, "#R2", fmt.Sprintf("%02d", random2))
	result = strings.ReplaceAll(result, "#R4", fmt.Sprintf("%04d", random4))
	return result
}
