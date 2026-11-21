package secretsengine

import (
	"context"
	"fmt"

	apigee "github.com/bstraehle/apigee-client-go/v2"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

// secretCredentialsType must not change: existing leases reference it.
const secretCredentialsType = "apigee_secret"

func secretCredentials(b *apigeeBackend) *framework.Secret {
	return &framework.Secret{
		Type: secretCredentialsType,
		Fields: map[string]*framework.FieldSchema{
			"org_name":        {Type: framework.TypeString, Description: "Apigee organization name."},
			"developer_email": {Type: framework.TypeString, Description: "Email of the Apigee developer."},
			"app_name":        {Type: framework.TypeString, Description: "Name of the Apigee developer app."},
			"api_products":    {Type: framework.TypeCommaStringSlice, Description: "API products associated with the credentials."},
			"key":             {Type: framework.TypeString, Description: "Consumer key."},
			"secret":          {Type: framework.TypeString, Description: "Consumer secret."},
			"credentials":     {Type: framework.TypeString, Description: "base64(key:secret) for HTTP basic authentication."},
			"expires_at":      {Type: framework.TypeString, Description: "Expiry of the credentials in Apigee (RFC 3339)."},
		},
		Revoke: b.secretCredentialsRevoke,
	}
}

func (b *apigeeBackend) secretCredentialsRevoke(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	var ids [4]string
	for i, field := range []string{"org_name", "developer_email", "app_name", "key"} {
		v, ok := req.Secret.InternalData[field].(string)
		if !ok || v == "" {
			return nil, fmt.Errorf("lease is missing %s in internal data", field)
		}
		ids[i] = v
	}

	client, err := b.getClient(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	// A 404 is only proof of deletion from the Apigee that issued the key.
	// Leases from before v1.2.0 don't record the host.
	if host, ok := req.Secret.InternalData["host"].(string); ok && host != client.Host() {
		return nil, fmt.Errorf("lease was issued by %s but the mount is now configured for %s; restore the host to revoke, or delete key %s in Apigee and force-revoke the lease", host, client.Host(), ids[3])
	}

	err = client.DeleteCredentials(ctx, ids[0], ids[1], ids[2], ids[3])
	if apigee.IsNotFound(err) {
		// Already deleted, or the app is gone: nothing left to revoke.
		b.Logger().Debug("credentials already deleted in Apigee", "org", ids[0], "app", ids[2])
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error deleting credentials: %w", err)
	}
	return nil, nil
}
