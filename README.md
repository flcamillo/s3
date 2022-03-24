## Conteúdo
* [Informações Gerais](#Informações-Gerais)
* [Tecnologias](#Tecnologias)
* [Configuração](#Configuração)
* [Bucket Policy](#Bucket-Policy)
* [Renomeio de arquivos](#Renomeio-de-arquivos)
* [Forma de Uso](#Forma-de-uso)


## Informações Gerais
Este programa fornece recursos para realizar envio ou recepção de arquivos para um bucket usando a API S3.
Abaixo seguem alguns recursos:
* Envio e recepção de multiplos arquivos
* Renomeio de arquivos através de variáveis
* Exclusão automática dos arquivos enviados ou recebidos
* Inclusão de metadados nos arquivos enviados para o bucket
* Autenticação usando credenciais `ACCESS_KEY` e `SECRET_KEY` estáticas
* Autenticação usando credenciais dinâmica geradas via [Vault](https://www.hashicorp.com/products/vault)
	

## Tecnologias
Este projeto foi criado com:
* Golang: 1.17
* AWS API: aws-sdk-go-v2	


## Configuração
Antes de começar a utilizar o `s3` é necessário identificar qual o método para as credenciais será utilizado.


### Credenciais Estáticas
Para configurar as credenciais que fornecem acesso ao seu bucket siga o processo abaixo:
```
$ s3 config s3 -accesskey=ACCESS_KEY -secretkey=SECRET_KEY
```

### Credenciais Dinâmicas
As credenciais dinâmicas são geradas pelo Vault através da API STS e desta forma será necessário configurar a URL do Vault para gerar as credênciais e o método de autenticação para executar suas API's.

A autenticação pode ser realizada de duas formas:
* Token
* AppRole

A forma mais simples é utilizando `Token` onde é necessário configurar apenas um parametro de autenticação. Para o `AppRole` será necessário configurar o `role_id` e o `secret_id` para se autenticar.

```
$ s3 config vault -endpoint=https://my-vault-url.com -token=TOKEN -enginepath=aws
```

O `s3` deverá receber a `role` que será passada para o Vault gerar as credenciais para acessar o bucket.

Abaixo segue um resumo dos passos para o Vault poder gerar as credenciais dinâmicas:
* Criar um usuário na conta da AWS para o Vault
* Criar uma política que fornecerá o acesso necessário ao bucket
* Criar uma função e associar a esta função:
  * a política criada para acesso ao bucket
  * a relação de confiança para o usuário do Vault realizar o `sts:AssumeRole`
* Configurar a engine `aws` no Vault
* Configurar na engine `aws` o `ACCESS_KEY` e `SECRET_KEY` do usuário do Vault que foi criado
* Configurar uma role usando o tipo de credencial `Assumed Role` e informar o `Arn` da função criada na AWS

**Observação:** o usuário criado para o Vault não precisa ter nenhum acesso associado à ele

### Configurações gerais
Para simplificar os parametros para envio e recepção dos arquivos, pode-se definir uma pasta padrão onde estarão os arquivos a serem enviados ou onde deverão ser recebidos os arquivos do bucket.
```
$ s3 config local -folder=/myfolder/mysubfolder
```

### Endpoint para o bucket
O `s3` também permite que seja configurado um endpoint específico para o bucket, este recurso é muito útil em uma rede privada sem exposição para Internet. 
```
$ s3 config s3 -endpoint=https://my-s3-url.com
```

Todas as configurações realizadas serão gravadas no arquivo `config.json`. Este arquivo por padrão é armazenado no diretório `$HOME` (em ambiente Linux) ou `%USERPROFILE%` (em ambiente Windows). 
Caso não seja possível identificar este diretório então o arquivo será gravado na mesma pasta onde o `s3` se encontra.


## Bucket Policy
Abaixo seguem exemplos de políticas que podem ser definidas para restringir o acesso ao bucket.

### Acesso apenas de leitura
```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket"
            ],
            "Resource": "arn:aws:s3:::MY-BUCKET"
        },
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject"
            ],
            "Resource": "arn:aws:s3:::MY-BUCKET/*"
        }
    ]
}
```

### Acesso de leitura e gravação
```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "s3:ListBucket",
            "Resource": "arn:aws:s3:::MY-BUCKET"
        },
        {
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:GetObject",
                "s3:DeleteObject"
            ],
            "Resource": "arn:aws:s3:::MY-BUCKET/*"
        }
    ]
}
```

## Renomeio de arquivos

O parametro `-c` oferece diversas opções para customizar o nome do arquivo que será enviado ou recebido do bucket S3. Abaixo seguem as variáveis que podem ser usadas para gerar nomes dinâmicos.

```
#DY = year 4 digits
#YY = year 2 digits
#DM = month number
#DD = day of month
#DJ = day of year
#TH = hour 2 digits 00-23h
#TM = minute 2 digits 00-59
#TS = second 2 digits 00-59
#TU = miliseconds 3 digits 000-999
#SP = timestamp format yyyymmddhhMMssnnnnnnnnn
#FN = file name without extension
#FE = file extension with dot
#R1 = random number 1 digit 0-9
#R2 = random number 2 digits 00-99
#R4 = random number 4 digits 0000-9999`
```

Para deixar mais claro vamos supor que o nome de um arquivo seja `teste.txt` e que seja utilizado o parametro `-c=#DY#DM#DD_#FN_#R1#FE` o nome gerado seguiria esse padrão: `20220317_teste_1.txt`.


## Forma de uso

### Envio para o bucket
#### Um único arquivo

```
s3 put -b=MY-BUCKET -r=MY-ROLE -f=FILE.TXT
```

#### Multiplos arquivos

```
s3 put -b=MY-BUCKET -r=MY-ROLE -f=*.TXT
```

#### Multiplos arquivos com renomeio

```
s3 put -b=MY-BUCKET -r=MY-ROLE -f=*.TXT -c=FILE_#SP#FE
```

#### Usando subpasta no bucket
```
s3 put -b=MY-BUCKET -r=MY-ROLE -f=*.TXT -bp=SUB-FOLDER
```
**Observação:** A pasta será criada caso não exista.

#### Com metadados
```
s3 put -b=MY-BUCKET -r=MY-ROLE -f=*.TXT -m="metada1=value1;metada2=value2"
```
**Observação:** Todos os arquivos que forem gravados no bucket terão os metadados informados.

#### Removendo os arquivos após copiar

```
s3 put -b=MY-BUCKET -r=MY-ROLE -f=*.TXT -rm
```

### Recepção do bucket
#### Um único arquivo

```
s3 get -b=MY-BUCKET -r=MY-ROLE -f=FILE.TXT
```

#### Multiplos arquivos

```
s3 get -b=MY-BUCKET -r=MY-ROLE -f=*.TXT
```

#### Multiplos arquivos com renomeio

```
s3 get -b=MY-BUCKET -r=MY-ROLE -f=*.TXT -c=#DY#DM#DD_#FN#FE
```

#### Usando subpasta no bucket
```
s3 get -b=MY-BUCKET -r=MY-ROLE -f=*.TXT -bp=SUB-FOLDER
```
**Observação:** Se o bucket possuir muitos objetos é altamente recomendável que agrupe os arquivos em sub pastas para evitar a leitura de todos os objetos usando o filtro.

#### Removendo os arquivos após copiar

```
s3 get -b=MY-BUCKET -r=MY-ROLE -f=*.TXT -rm
```
