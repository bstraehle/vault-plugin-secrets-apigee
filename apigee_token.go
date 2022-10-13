package secretsengine

import (
	"context"
	"fmt"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	apigeeSecretType = "apigee_secret"
)

type apigeeToken struct {
	OrgName        string `json:"org_name"`
	DeveloperEmail string `json:"developer_email"`
	AppName        string `json:"app_name"`
	ApiProducts    string `json:"api_products"`

	Key         string `json:"key"`
	Secret      string `json:"secret"`
	Credentials string `json:"credentials"`
}

func (b *apigeeBackend) apigeeToken() *framework.Secret {
	return &framework.Secret{
		Type: apigeeSecretType,
		Fields: map[string]*framework.FieldSchema{
			"org_name": {
				Type:        framework.TypeString,
				Description: "Apigee OrgName",
			},
			"developer_email": {
				Type:        framework.TypeString,
				Description: "Apigee DeveloperEmail",
			},
			"app_name": {
				Type:        framework.TypeString,
				Description: "Apigee AppName",
			},
			"api_products": {
				Type:        framework.TypeString,
				Description: "Apigee ApiProducts",
			},
			"key": {
				Type:        framework.TypeString,
				Description: "Apigee Key",
			},
			"secret": {
				Type:        framework.TypeString,
				Description: "Apigee Secret",
			},
			"credentials": {
				Type:        framework.TypeString,
				Description: "Apigee Credentials",
			},
		},
		Revoke: b.credentialsRevoke,
	}
}

func (b *apigeeBackend) credentialsRevoke(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	client, err := b.getClient(ctx, req.Storage)

	if err != nil {
		return nil, fmt.Errorf("error getting client: %w", err)
	}

	orgName := ""
	developerEmail := ""
	appName := ""
	key := ""

	orgNameRaw, ok := req.Secret.InternalData["org_name"]

	if ok {
		orgName, ok = orgNameRaw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value for org_name in secret internal data")
		}
	}

	developerEmailRaw, ok := req.Secret.InternalData["developer_email"]

	if ok {
		developerEmail, ok = developerEmailRaw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value for developer_email in secret internal data")
		}
	}

	appNameRaw, ok := req.Secret.InternalData["app_name"]

	if ok {
		appName, ok = appNameRaw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value for app_name in secret internal data")
		}
	}

	keyRaw, ok := req.Secret.InternalData["key"]

	if ok {
		key, ok = keyRaw.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value for key in secret internal data")
		}
	}

	err = deleteCredentials(ctx, client, orgName, developerEmail, appName, key)

	if err != nil {
		return nil, fmt.Errorf("error deleting credentials: %w", err)
	}

	return nil, nil
}

func createCredentials(ctx context.Context, c *apigeeClient, orgName string, developerEmail string, appName string, apiProducts string, ttl int) (*apigeeToken, error) {
	response, err := c.CreateCredentials(orgName, developerEmail, appName, apiProducts, ttl)

	if err != nil {
		return nil, fmt.Errorf("error creating credentials: %w", err)
	}

	return &apigeeToken{
		OrgName:        orgName,
		DeveloperEmail: developerEmail,
		AppName:        appName,
		ApiProducts:    apiProducts,
		Key:            response.Key,
		Secret:         response.Secret,
		Credentials:    response.Credentials,
	}, nil
}

func deleteCredentials(ctx context.Context, c *apigeeClient, orgName string, developerEmail string, appName string, key string) error {
	err := c.DeleteCredentials(orgName, developerEmail, appName, key)

	if err != nil {
		return err
	}

	return nil
}
