package secretsengine

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	configStoragePath = "config"
)

type apigeeConfig struct {
	Host       string `json:"host"`
	OAuthToken string `json:"oauth_token"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

func pathConfig(b *apigeeBackend) *framework.Path {
	return &framework.Path{
		Pattern: "config",
		Fields: map[string]*framework.FieldSchema{
			"host": {
				Type:        framework.TypeString,
				Description: "The host for the Apigee Management API",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "host",
					Sensitive: false,
				},
			},
			"oauth_token": {
				Type:        framework.TypeString,
				Description: "The oauth_token for the Apigee Management API",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "oauth_token",
					Sensitive: true,
				},
			},
			"username": {
				Type:        framework.TypeString,
				Description: "The username for the Apigee Management API",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "username",
					Sensitive: true,
				},
			},
			"password": {
				Type:        framework.TypeString,
				Description: "The password for the Apigee Management API",
				Required:    false,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "password",
					Sensitive: true,
				},
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathConfigRead,
			},
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathConfigWrite,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigWrite,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathConfigDelete,
			},
		},
		ExistenceCheck:  b.pathConfigExistenceCheck,
		HelpSynopsis:    pathConfigHelpSynopsis,
		HelpDescription: pathConfigHelpDescription,
	}
}

func (b *apigeeBackend) pathConfigExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	out, err := req.Storage.Get(ctx, req.Path)

	if err != nil {
		return false, fmt.Errorf("existence check failed: %w", err)
	}

	return out != nil, nil
}

func (b *apigeeBackend) pathConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := getConfig(ctx, req.Storage)

	if err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"host": config.Host,
		},
	}, nil
}

func (b *apigeeBackend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := getConfig(ctx, req.Storage)

	if err != nil {
		return nil, err
	}

	createOperation := (req.Operation == logical.CreateOperation)

	if config == nil {
		if !createOperation {
			return nil, errors.New("config not found during update operation")
		}
		config = new(apigeeConfig)
	}

	if host, ok := data.GetOk("host"); ok {
		config.Host = host.(string)
	} else if !ok && createOperation {
		return nil, fmt.Errorf("missing host in configuration")
	}

	if oauth_token, ok := data.GetOk("oauth_token"); ok {
		config.OAuthToken = oauth_token.(string)
	}

	if username, ok := data.GetOk("username"); ok {
		config.Username = username.(string)
	}

	if password, ok := data.GetOk("password"); ok {
		config.Password = password.(string)
	}

	entry, err := logical.StorageEntryJSON(configStoragePath, config)

	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	b.reset()

	return nil, nil
}

func (b *apigeeBackend) pathConfigDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	err := req.Storage.Delete(ctx, configStoragePath)

	if err == nil {
		b.reset()
	}

	return nil, err
}

func getConfig(ctx context.Context, s logical.Storage) (*apigeeConfig, error) {
	entry, err := s.Get(ctx, configStoragePath)

	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	config := new(apigeeConfig)

	if err := entry.DecodeJSON(&config); err != nil {
		return nil, fmt.Errorf("error reading root configuration: %w", err)
	}

	return config, nil
}

const pathConfigHelpSynopsis = `Configure the Apigee backend.`

const pathConfigHelpDescription = `The Apigee backend requires host and oauth_token for the Apigee Management API.`
