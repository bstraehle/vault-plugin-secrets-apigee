package secretsengine

import (
	"context"
	"errors"
	"fmt"

	apigee "github.com/bstraehle/apigee-client-go/v2"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	configStoragePath = "config"

	authTypeOAuthToken     = "oauth_token"
	authTypeBasic          = "basic"
	authTypeServiceAccount = "service_account"
)

// authFields maps each authentication field to its authentication method.
var authFields = map[string]string{
	"oauth_token": authTypeOAuthToken,
	"username":    authTypeBasic,
	"password":    authTypeBasic,
	"credentials": authTypeServiceAccount,
}

type apigeeConfig struct {
	Host        string `json:"host"`
	OAuthToken  string `json:"oauth_token,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Credentials string `json:"credentials,omitempty"`
}

func (c *apigeeConfig) authType() string {
	switch {
	case c.OAuthToken != "":
		return authTypeOAuthToken
	case c.Username != "" || c.Password != "":
		return authTypeBasic
	case c.Credentials != "":
		return authTypeServiceAccount
	}
	return ""
}

func (c *apigeeConfig) validate() error {
	methods := 0
	for _, set := range []bool{c.OAuthToken != "", c.Username != "" || c.Password != "", c.Credentials != ""} {
		if set {
			methods++
		}
	}
	switch {
	case methods == 0:
		return errors.New("one of oauth_token, username and password, or credentials is required")
	case methods > 1:
		return errors.New("only one of oauth_token, username and password, or credentials may be set")
	case (c.Username == "") != (c.Password == ""):
		return errors.New("username and password must be set together")
	}
	return nil
}

// set applies an authentication field, clearing the fields of other methods
// so that switching methods doesn't leave stale secrets behind.
func (c *apigeeConfig) set(field, value string) {
	if c.authType() != "" && c.authType() != authFields[field] {
		c.OAuthToken, c.Username, c.Password, c.Credentials = "", "", "", ""
	}
	switch field {
	case "oauth_token":
		c.OAuthToken = value
	case "username":
		c.Username = value
	case "password":
		c.Password = value
	case "credentials":
		c.Credentials = value
	}
}

func pathConfig(b *apigeeBackend) *framework.Path {
	return &framework.Path{
		Pattern: configStoragePath,
		Fields: map[string]*framework.FieldSchema{
			"host": {
				Type:        framework.TypeString,
				Description: fmt.Sprintf("Base URL of the Apigee management API. Defaults to %s (Apigee X / hybrid); for Apigee Edge use e.g. https://api.enterprise.apigee.com.", apigee.DefaultHost),
				Default:     apigee.DefaultHost,
			},
			"oauth_token": {
				Type:         framework.TypeString,
				Description:  "Static OAuth 2.0 access token (Apigee X / hybrid). Tokens from gcloud expire after one hour; prefer credentials.",
				DisplayAttrs: &framework.DisplayAttributes{Name: "OAuth Token", Sensitive: true},
			},
			"credentials": {
				Type:         framework.TypeString,
				Description:  "Google service account key JSON used to obtain and refresh access tokens (Apigee X / hybrid).",
				DisplayAttrs: &framework.DisplayAttributes{Name: "Service Account Credentials", Sensitive: true},
			},
			"username": {
				Type:         framework.TypeString,
				Description:  "Username for basic authentication (Apigee Edge).",
				DisplayAttrs: &framework.DisplayAttributes{Name: "Username", Sensitive: true},
			},
			"password": {
				Type:         framework.TypeString,
				Description:  "Password for basic authentication (Apigee Edge).",
				DisplayAttrs: &framework.DisplayAttributes{Name: "Password", Sensitive: true},
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation:   &framework.PathOperation{Callback: b.pathConfigRead},
			logical.CreateOperation: &framework.PathOperation{Callback: b.pathConfigWrite},
			logical.UpdateOperation: &framework.PathOperation{Callback: b.pathConfigWrite},
			logical.DeleteOperation: &framework.PathOperation{Callback: b.pathConfigDelete},
		},
		ExistenceCheck:  b.pathConfigExistenceCheck,
		HelpSynopsis:    pathConfigHelpSynopsis,
		HelpDescription: pathConfigHelpDescription,
	}
}

func (b *apigeeBackend) pathConfigExistenceCheck(ctx context.Context, req *logical.Request, _ *framework.FieldData) (bool, error) {
	config, err := getConfig(ctx, req.Storage)
	if err != nil {
		return false, err
	}
	return config != nil, nil
}

func (b *apigeeBackend) pathConfigRead(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	config, err := getConfig(ctx, req.Storage)
	if err != nil || config == nil {
		return nil, err
	}

	return &logical.Response{Data: map[string]interface{}{
		"host":      config.Host,
		"auth_type": config.authType(),
	}}, nil
}

func (b *apigeeBackend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	config, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if config == nil {
		config = &apigeeConfig{}
	}

	if host, ok := data.GetOk("host"); ok {
		config.Host = host.(string)
	}
	if config.Host == "" {
		config.Host = apigee.DefaultHost
	}
	methods := map[string]bool{}
	for _, field := range []string{"oauth_token", "username", "password", "credentials"} {
		if v, ok := data.GetOk(field); ok {
			methods[authFields[field]] = true
			config.set(field, v.(string))
		}
	}
	if len(methods) > 1 {
		return logical.ErrorResponse("only one of oauth_token, username and password, or credentials may be set"), nil
	}

	if _, err := config.newClient(); err != nil {
		return logical.ErrorResponse(err.Error()), nil
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

func (b *apigeeBackend) pathConfigDelete(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	if err := req.Storage.Delete(ctx, configStoragePath); err != nil {
		return nil, err
	}
	b.reset()
	return nil, nil
}

func getConfig(ctx context.Context, s logical.Storage) (*apigeeConfig, error) {
	entry, err := s.Get(ctx, configStoragePath)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	config := &apigeeConfig{}
	if err := entry.DecodeJSON(config); err != nil {
		return nil, fmt.Errorf("error reading configuration: %w", err)
	}
	if config.Host == "" {
		config.Host = apigee.DefaultHost
	}
	// Versions before v1.2.0 kept omitted fields on update, so a config could
	// hold both an OAuth token and stale basic auth fields; the token took
	// precedence.
	if config.OAuthToken != "" {
		config.Username, config.Password = "", ""
	}
	return config, nil
}

const pathConfigHelpSynopsis = `Configure access to the Apigee management API.`

const pathConfigHelpDescription = `
Configures the host and exactly one authentication method for the Apigee
management API:

  - credentials: Google service account key JSON (Apigee X / hybrid, recommended)
  - oauth_token: static OAuth 2.0 access token (Apigee X / hybrid)
  - username and password: basic authentication (Apigee Edge)

host defaults to the Apigee X / hybrid endpoint. Writing a field of a different
authentication method replaces the previously configured method. Secrets are
never returned when reading the configuration.
`
