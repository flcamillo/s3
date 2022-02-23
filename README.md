## Table of contents
* [Informações Gerais](#Informações-Gerais)
* [Tecnologias](#Tecnologias)
* [Configuração](#Configuração)
* [Bucket Policy](#Bucket-Policy)
* [Forma de Uso](#Forma-de-uso)


## Informações Gerais
Este programa fornece recursos para realizar transmissões de arquivos para um bucket usando a API S3.
Abaixo seguem alguns recursos:
* Envio e recepção multiplos arquivos
* Renomeio de arquivos através de variáveis
* Inclusão de metadados nos arquivos gravados no bucket
* Autenticação usando credenciais `ACCESS_KEY` e `SECRET_KEY` estáticas
* Autenticação usando credenciais dinâmica geradas via [Vault](https://www.hashicorp.com/products/vault)
	

## Tecnologias
Este projeto foi criado com:
* Golang: 1.17
	

## Configuração
Antes de começar a utilizar o `s3` é necessário identificar qual o método para as credenciais será utilizado.


### Credenciais Estáticas
Para configurar as credenciais que fornecem acesso ao seu bucket siga o processo abaixo:
```
$ s3 config -accesskey=ACCESS_KEY -secretkey=SECRET_KEY
```


### Credenciais Dinâmicas
As credenciais dinâmicas são geradas pelo Vault através da API STS e desta forma será necessário configurar a URL do Vault para gerar as credênciais assim como o token de acesso para executar a API.
```
$ s3 config -vault=https://my-vault-url.com -vaulttoken=TOKEN
```

O `s3` deverá receber a `role` que será passada para o Vault gerar as credenciais para acessar o bucket.


### Configurações gerais
Para simplificar os parametros para envio e recepção dos arquivos, pode-se definir uma pasta padrão onde estarão os arquivos a serem enviados ou onde deverão ser recebidos os arquivos do bucket.
```
$ s3 config -folder=/myfolder/mysubfolder
```


### Endpoint para o bucket
O `s3` também permite que seja configurado um endpoint específico para o bucket, este recurso é muito útil em uma rede privada sem exposição para Internet. 
```
$ s3 config -endpoint=https://my-s3-url.com
```

## Bucket Policy
### Acesso apenas de leitura
```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Acesso para listar os objetos no bucket",
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket"
            ],
            "Resource": "arn:aws:s3:::MY-BUCKET"
        },
        {
            "Sid": "Acesso para copiar os objetos do bucket",
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

#### Usando subpasta no bucket
```
s3 get -b=MY-BUCKET -r=MY-ROLE -f=*.TXT -bp=SUB-FOLDER
```
**Observação:** Se o bucket possuir muitos objetos é altamente recomendável que agrupe os arquivos em sub pastas para evitar a leitura de todos os objetos usando o filtro.

#### Removendo os arquivos após copiar

```
s3 get -b=MY-BUCKET -r=MY-ROLE -f=*.TXT -rm
```