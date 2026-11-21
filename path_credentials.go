package secretsengine

import (
	"context"
	"fmt"
	"time"

	apigee "github.com/bstraehle/apigee-client-go/v2"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/helper/locksutil"
	"github.com/hashicorp/vault/sdk/logical"
)

func pathCredentials(b *apigeeBackend) *framework.Path {
	return &framework.Path{
		Pattern: "creds/" + framework.GenericNameRegex("name"),
		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the role.",
				Required:    true,
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation:   &framework.PathOperation{Callback: b.pathCredentialsRead},
			logical.UpdateOperation: &framework.PathOperation{Callback: b.pathCredentialsRead},
		},
		HelpSynopsis:    pathCredentialsHelpSynopsis,
		HelpDescription: pathCredentialsHelpDescription,
	}
}

func (b *apigeeBackend) pathCredentialsRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	name := d.Get("name").(string)

	role, err := getRole(ctx, req.Storage, name)
	if err != nil {
		return nil, fmt.Errorf("error retrieving role: %w", err)
	}
	if role == nil {
		return logical.ErrorResponse("unknown role: %s", name), nil
	}

	client, err := b.getClient(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	ttl := b.leaseTTL(role)

	lock := locksutil.LockForKey(b.appLocks, role.OrgName+"/"+role.DeveloperEmail+"/"+role.AppName)
	lock.Lock()
	creds, err := client.CreateCredentials(ctx, apigee.CreateCredentialsInput{
		Org:            role.OrgName,
		DeveloperEmail: role.DeveloperEmail,
		App:            role.AppName,
		APIProducts:    role.APIProducts,
		ExpiresIn:      ttl,
	})
	lock.Unlock()
	if err != nil {
		return nil, fmt.Errorf("error creating credentials: %w", err)
	}

	data := map[string]interface{}{
		"org_name":        role.OrgName,
		"developer_email": role.DeveloperEmail,
		"app_name":        role.AppName,
		"api_products":    []string(role.APIProducts),
		"key":             creds.Key,
		"secret":          creds.Secret,
		"credentials":     creds.BasicAuth(),
	}
	if !creds.ExpiresAt.IsZero() {
		data["expires_at"] = creds.ExpiresAt.UTC().Format(time.RFC3339)
	}

	// Internal data is persisted with the lease; it holds only what revocation
	// needs and never the consumer secret.
	resp := b.Secret(secretCredentialsType).Response(data, map[string]interface{}{
		"role":            name,
		"org_name":        role.OrgName,
		"developer_email": role.DeveloperEmail,
		"app_name":        role.AppName,
		"key":             creds.Key,
	})
	resp.Secret.TTL = ttl
	resp.Secret.MaxTTL = ttl
	return resp, nil
}

// leaseTTL returns the role's TTL, defaulting to and capped by the mount's lease TTLs.
func (b *apigeeBackend) leaseTTL(role *apigeeRole) time.Duration {
	ttl := role.TTL
	if ttl <= 0 {
		ttl = b.System().DefaultLeaseTTL()
	}
	if maxTTL := b.System().MaxLeaseTTL(); maxTTL > 0 && ttl > maxTTL {
		ttl = maxTTL
	}
	return ttl
}

const pathCredentialsHelpSynopsis = `Generate Apigee credentials for a role.`

const pathCredentialsHelpDescription = `
Generates a new consumer key and secret on the role's Apigee developer app.
The credentials expire in Apigee after the lease TTL and are deleted from
Apigee when the lease is revoked. They are not renewable; read a new set
before the lease expires.
`
