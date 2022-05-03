package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

/**************************
 Exemplo de Policy do VAULT
 **************************
	path "sys/mounts" {
	capabilities = ["read"]
	}
	path "sys/mounts" {
	capabilities = ["read"]
	}
*/

type Vault struct {
	// endereco do vault
	Address string
	// token de acesso ao vault
	Token string
	// define o client http para as requisicoes
	httpClient *http.Client
}

// Retorna um novo Vault
func NewVault(address string, token string) *Vault {
	return &Vault{
		Address: strings.TrimSuffix(address, "/"),
		Token:   token,
		httpClient: &http.Client{
			Timeout: time.Duration(10 * time.Second),
			Transport: &http.Transport{
				IdleConnTimeout:       time.Duration(60 * time.Second),
				ResponseHeaderTimeout: time.Duration(30 * time.Second),
				TLSHandshakeTimeout:   time.Duration(10 * time.Second),
				ExpectContinueTimeout: time.Duration(45 * time.Second),
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}
}

// Realiza a autenticação usando role e secret
func (p *Vault) AuthByAppRole(path string, roleId string, secretId string) error {
	// se não foi passado o path então usa o padrão
	if path == "" {
		path = "v1/auth/approle/login"
	}
	path = strings.TrimSuffix(path, "/")
	// define a url para realizar a autenticação
	apiURL := fmt.Sprintf("%s/%s", p.Address, path)
	// formata o corpo da mensagem para a requisição
	body := bytes.NewBufferString(fmt.Sprintf(`{"role_id": "%s", "secret_id": "%s"}`, roleId, secretId))
	// define a requisição para realizar a autenticação
	req, err := http.NewRequest(http.MethodPost, apiURL, body)
	if err != nil {
		return fmt.Errorf("unable to create http request, %s", err)
	}
	// configura os cabecalhos da requisição
	req.Header.Set("Content-Type", "application/json")
	// executa a autenticação
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to execute http request to {%s}, %s", req.URL, err)
	}
	defer resp.Body.Close()
	// loga a falha se a requisição não foi processada com sucesso
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid response receive from {%s}, %s", req.URL, resp.Status)
	}
	// le o retorno
	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return fmt.Errorf("failed to decode vault response, %s", err)
	}
	// identifica o token a ser usado
	auth := data["auth"].(map[string]interface{})
	p.Token = auth["client_token"].(string)
	return nil
}

// Realiza a autenticação usando role e secret
func (p *Vault) AuthByCertificate(path string, cert string, certkey string, cacert string, certrole string) error {
	// se não foi passado o path então usa o padrão
	if path == "" {
		path = "v1/auth/cert/login"
	}
	path = strings.TrimSuffix(path, "/")
	// define a url para realizar a autenticação
	apiURL := fmt.Sprintf("%s/%s", p.Address, path)
	// formata o corpo da mensagem para a requisição
	body := bytes.NewBufferString(fmt.Sprintf(`{"name": "%s"}`, certrole))
	// define a requisição para realizar a autenticação
	req, err := http.NewRequest(http.MethodPost, apiURL, body)
	if err != nil {
		return fmt.Errorf("unable to create http request, %s", err)
	}
	// carrega os certificados do cliente
	cadata, err := ioutil.ReadFile(cacert)
	if err != nil {
		return fmt.Errorf("unable to load CA certificate, %s", err)
	}
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(cadata)
	if !ok {
		return fmt.Errorf("CA PEM certificate not found, %s", err)
	}
	x509cert, err := tls.LoadX509KeyPair(cert, certkey)
	if err != nil {
		return fmt.Errorf("unable to load client certificate, %s", err)
	}
	// cria um novo transport para o client http com os certificados lidos
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:            caCertPool,
			Certificates:       []tls.Certificate{x509cert},
			InsecureSkipVerify: true,
		},
	}
	p.httpClient.Transport = tr
	// configura os cabecalhos da requisição
	req.Header.Set("Content-Type", "application/json")
	// executa a autenticação
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to execute http request to {%s}, %s", req.URL, err)
	}
	defer resp.Body.Close()
	// loga a falha se a requisição não foi processada com sucesso
	if resp.StatusCode != http.StatusOK {
		respData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("invalid response receive from {%s}, %s", req.URL, resp.Status)
		}
		return fmt.Errorf("invalid response receive from {%s}, %s, %s", req.URL, resp.Status, string(respData))
	}
	// le o retorno
	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return fmt.Errorf("failed to decode vault response, %s", err)
	}
	// identifica o token a ser usado
	auth := data["auth"].(map[string]interface{})
	p.Token = auth["client_token"].(string)
	return nil
}

// Retorna a versão do mount point
func (p *Vault) MountPointVersion(mount string) (version string, err error) {
	// define a url para ler informacoes do mountpoint
	apiURL := fmt.Sprintf("%s/v1/sys/mounts/%s/tune", p.Address, mount)
	// configura a requisição do vault
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("unable to create http request, %s", err)
	}
	// configura os cabecalhos da requisição
	req.Header.Set("X-Vault-Token", p.Token)
	// solicita o segredo
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to execute http request to {%s}, %s", req.URL, err)
	}
	defer resp.Body.Close()
	// loga a falha se a requisição não foi processada com sucesso
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("invalid response receive from {%s}, %s", req.URL, resp.Status)
	}
	// le o retorno
	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return "", fmt.Errorf("failed to decode vault response, %s", err)
	}
	// identifica a versão
	options := data["options"].(map[string]interface{})
	if version, ok := options["version"]; ok {
		return fmt.Sprintf("%v", version), nil
	}
	return "", fmt.Errorf("version not found")
}

// retorna os segredos
func (p *Vault) Secrets(mount string, secret string, nameSpace string, version string) (secrets map[string]string, err error) {
	// valida se suporta a versão
	if version != "1" && version != "2" {
		if version == "" {
			version = "1"
		} else {
			return nil, fmt.Errorf("version {%s} is not supported", version)
		}
	}
	// define a url para a versão 1
	apiURL := fmt.Sprintf("%s/v1/%s/%s", p.Address, mount, secret)
	// ajusta a url caso seja versão 2
	if version == "2" {
		apiURL = fmt.Sprintf("%s/v1/%s/data/%s", p.Address, mount, secret)
	}
	// formata o corpo da mensagem para a requisição
	body := bytes.NewBufferString(fmt.Sprintf(`{"ttl": "%s"}`, "3600s"))
	// configura a requisição do vault
	req, err := http.NewRequest(http.MethodPost, apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("unable to create http request, %s", err)
	}
	// configura os cabecalhos da requisição
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vault-Token", p.Token)
	if nameSpace != "" {
		req.Header.Set("X-Vault-Namespace", nameSpace)
	}
	// solicita o segredo
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to execute http request to {%s}, %s", req.URL, err)
	}
	defer resp.Body.Close()
	// loga a falha se a requisição não foi processada com sucesso
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response receive from {%s}, %s", req.URL, resp.Status)
	}
	// le o retorno
	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode vault response, %s", err)
	}
	// identifica os segredos
	var keys map[string]interface{}
	if version == "1" {
		keys = data["data"].(map[string]interface{})
	} else if version == "2" {
		keys = data["data"].(map[string]interface{})["data"].(map[string]interface{})
	}
	// le todas as chaves
	secrets = make(map[string]string)
	for k, v := range keys {
		value := fmt.Sprintf("%v", v)
		if value == "<nil>" {
			value = ""
		}
		secrets[k] = value
	}
	return
}

// retorna os segredos
func (p *Vault) RawAPI(apiURL string) (data map[string]interface{}, err error) {
	// configura a requisição do vault
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/%s", p.Address, apiURL), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create http request, %s", err)
	}
	// configura os cabecalhos da requisição
	req.Header.Set("X-Vault-Token", p.Token)
	// executa a api
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to execute http request to {%s}, %s", req.URL, err)
	}
	defer resp.Body.Close()
	// loga a falha se a requisição não foi processada com sucesso
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response receive from {%s}, %s", req.URL, resp.Status)
	}
	// le o retorno
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode vault response, %s", err)
	}
	return
}
