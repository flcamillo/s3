package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// Define as formas de autenticação no vault
const (
	VaultAuthByToken   = "token"
	VaultAuthByAppRole = "approle"
)

// Define todas as configurações que podem ser definidas como padrão,
// caso não sejam passadas por linha de comando
type Config struct {
	Metadata             map[string]string `json:"bucket_metadata,omitempty"`
	Bucket               string            `json:"bucket_name,omitempty"`
	Region               string            `json:"bucket_region,omitempty"`
	PartSize             int               `json:"bucket_part_size,omitempty"`
	EndPoint             string            `json:"bucket_endpoint_address,omitempty"`
	AccessKey            string            `json:"bucket_access_key,omitempty"`
	SecretKey            string            `json:"bucket_secret_key,omitempty"`
	AccessToken          string            `json:"bucket_token_session,omitempty"`
	VaultAuthMethod      string            `json:"vault_auth_method,omitempty"`
	VaultAuthToken       string            `json:"vault_auth_token,omitempty"`
	VaultAuthRoleId      string            `json:"vault_auth_role_id,omitempty"`
	VaultAuthSecretId    string            `json:"vault_auth_secret_id,omitempty"`
	VaultAuthAppRolePath string            `json:"vault_auth_approle_path,omitempty"`
	VaultAddress         string            `json:"vault_address,omitempty"`
	VaultEnginePath      string            `json:"vault_token_engine_pah,omitempty"`
	LocalFolder          string            `json:"local_folder,omitempty"`
}

// Retorna a configuração padrão
func DefaultConfig() *Config {
	// cria uma nova configuração
	config := &Config{
		Region: "sa-east-1",
	}
	// identifica o diretório da aplicação
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	config.LocalFolder = dir
	return config
}

// Carrega a configuração do arquivo
func (p *Config) Load(config string) error {
	// abre o arquivo
	f, err := os.OpenFile(config, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	// decodifica o json
	err = json.NewDecoder(f).Decode(p)
	if err != nil {
		return err
	}
	return nil
}

// Grava a configuração no arquivo
func (p *Config) Save(config string) error {
	// abre o arquivo
	f, err := os.OpenFile(config, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	// decodifica o json
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(p)
	if err != nil {
		return err
	}
	return nil
}
