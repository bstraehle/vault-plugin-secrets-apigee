package secretsengine

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

func pathCredentials(b *apigeeBackend) *framework.Path {
	return &framework.Path{
		Pattern: "creds/" + framework.GenericNameRegex("name"),
		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeLowerCaseString,
				Description: "Name of the role",
				Required:    true,
			},
		},
		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.ReadOperation:   b.pathCredentialsRead,
			logical.UpdateOperation: b.pathCredentialsRead,
		},
		HelpSynopsis:    pathCredentialsHelpSyn,
		HelpDescription: pathCredentialsHelpDesc,
	}
}

func (b *apigeeBackend) pathCredentialsRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roleName := d.Get("name").(string)

	roleEntry, err := b.getRole(ctx, req.Storage, roleName)

	if err != nil {
		return nil, fmt.Errorf("error retrieving role: %w", err)
	}

	if roleEntry == nil {
		return nil, errors.New("error retrieving role: role is nil")
	}

	return b.createCreds(ctx, req, roleEntry)
}

func (b *apigeeBackend) createCreds(ctx context.Context, req *logical.Request, role *apigeeRole) (*logical.Response, error) {
	token, err := b.createCredentials(ctx, req.Storage, role)

	if err != nil {
		return nil, err
	}

	resp := b.Secret(apigeeSecretType).Response(map[string]interface{}{
		"org_name":        token.OrgName,
		"developer_email": token.DeveloperEmail,
		"app_name":        token.AppName,
		"api_products":    token.ApiProducts,
		"key":             token.Key,
		"secret":          token.Secret,
		"credentials":     token.Credentials,
	}, map[string]interface{}{
		"org_name":        token.OrgName,
		"developer_email": token.DeveloperEmail,
		"app_name":        token.AppName,
		"api_products":    token.ApiProducts,
		"key":             token.Key,
		"secret":          token.Secret,
		"credentials":     token.Credentials,
	})

	if role.TTL > 0 {
		resp.Secret.TTL = role.TTL
	}

	return resp, nil
}

func (b *apigeeBackend) createCredentials(ctx context.Context, s logical.Storage, role *apigeeRole) (*apigeeToken, error) {
	client, err := b.getClient(ctx, s)

	if err != nil {
		return nil, err
	}

	var token *apigeeToken

	token, err = createCredentials(ctx, client, role.OrgName, role.DeveloperEmail, role.AppName, role.ApiProducts, int(role.TTL.Seconds()))

	if err != nil {
		return nil, fmt.Errorf("error creating credentials: %w", err)
	}

	if token == nil {
		return nil, errors.New("error creating credentials")
	}

	return token, nil
}

const pathCredentialsHelpSyn = `Generate Apigee credentials from Vault role.`

const pathCredentialsHelpDesc = `Generate Apigee credentials from Vault role.`
