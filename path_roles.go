package secretsengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const roleStoragePrefix = "roles/"

type apigeeRole struct {
	OrgName        string        `json:"org_name"`
	DeveloperEmail string        `json:"developer_email"`
	AppName        string        `json:"app_name"`
	APIProducts    apiProducts   `json:"api_products"`
	TTL            time.Duration `json:"ttl"`
}

func (r *apigeeRole) validate() error {
	var missing []string
	for _, f := range []struct {
		name  string
		unset bool
	}{
		{"org_name", r.OrgName == ""},
		{"developer_email", r.DeveloperEmail == ""},
		{"app_name", r.AppName == ""},
		{"api_products", len(r.APIProducts) == 0},
	} {
		if f.unset {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}
	if r.TTL < 0 {
		return errors.New("ttl must not be negative")
	}
	return nil
}

func (r *apigeeRole) toResponseData() map[string]interface{} {
	return map[string]interface{}{
		"org_name":        r.OrgName,
		"developer_email": r.DeveloperEmail,
		"app_name":        r.AppName,
		"api_products":    []string(r.APIProducts),
		"ttl":             int64(r.TTL.Seconds()),
	}
}

// apiProducts is a list of Apigee API product names. Versions before v1.2.0
// stored it as a JSON array encoded in a string (e.g. `["a","b"]`); both forms
// are accepted.
type apiProducts []string

// UnmarshalJSON implements json.Unmarshaler.
func (p *apiProducts) UnmarshalJSON(b []byte) error {
	var list []string
	if err := json.Unmarshal(b, &list); err == nil {
		*p = list
		return nil
	}
	var legacy string
	if err := json.Unmarshal(b, &legacy); err != nil {
		return fmt.Errorf("invalid api_products: %w", err)
	}
	list, err := parseAPIProducts(strings.Split(legacy, ","))
	if err != nil {
		return err
	}
	*p = list
	return nil
}

// parseAPIProducts normalizes the api_products field. It accepts a list, a
// comma-separated string (already split by Vault), or a JSON array string
// such as `["a","b"]`, which Vault's comma splitting breaks apart.
func parseAPIProducts(in []string) ([]string, error) {
	if len(in) > 0 && strings.HasPrefix(strings.TrimSpace(in[0]), "[") {
		var list []string
		if err := json.Unmarshal([]byte(strings.Join(in, ",")), &list); err != nil {
			return nil, fmt.Errorf("invalid api_products: %w", err)
		}
		in = list
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func pathRoles(b *apigeeBackend) []*framework.Path {
	return []*framework.Path{
		{
			Pattern: roleStoragePrefix + framework.GenericNameRegex("name"),
			Fields: map[string]*framework.FieldSchema{
				"name": {
					Type:        framework.TypeString,
					Description: "Name of the role.",
					Required:    true,
				},
				"org_name": {
					Type:        framework.TypeString,
					Description: "Apigee organization name.",
				},
				"developer_email": {
					Type:        framework.TypeString,
					Description: "Email of the Apigee developer that owns the app.",
				},
				"app_name": {
					Type:        framework.TypeString,
					Description: "Name of the Apigee developer app to generate credentials for.",
				},
				"api_products": {
					Type:        framework.TypeCommaStringSlice,
					Description: `API products to associate with generated credentials, as a list, a comma-separated string, or a JSON array string (e.g. ["a","b"]).`,
				},
				"ttl": {
					Type:        framework.TypeDurationSecond,
					Description: "Lease duration and Apigee expiry of generated credentials. Defaults to the mount's default lease TTL; capped at the mount's max lease TTL.",
				},
			},
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.ReadOperation:   &framework.PathOperation{Callback: b.pathRolesRead},
				logical.CreateOperation: &framework.PathOperation{Callback: b.pathRolesWrite},
				logical.UpdateOperation: &framework.PathOperation{Callback: b.pathRolesWrite},
				logical.DeleteOperation: &framework.PathOperation{Callback: b.pathRolesDelete},
			},
			ExistenceCheck:  b.pathRolesExistenceCheck,
			HelpSynopsis:    pathRoleHelpSynopsis,
			HelpDescription: pathRoleHelpDescription,
		},
		{
			Pattern: roleStoragePrefix + "?$",
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.ListOperation: &framework.PathOperation{Callback: b.pathRolesList},
			},
			HelpSynopsis:    pathRoleListHelpSynopsis,
			HelpDescription: pathRoleListHelpDescription,
		},
	}
}

func (b *apigeeBackend) pathRolesExistenceCheck(ctx context.Context, req *logical.Request, d *framework.FieldData) (bool, error) {
	role, err := getRole(ctx, req.Storage, d.Get("name").(string))
	if err != nil {
		return false, err
	}
	return role != nil, nil
}

func (b *apigeeBackend) pathRolesRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	role, err := getRole(ctx, req.Storage, d.Get("name").(string))
	if err != nil || role == nil {
		return nil, err
	}
	return &logical.Response{Data: role.toResponseData()}, nil
}

func (b *apigeeBackend) pathRolesWrite(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	name := d.Get("name").(string)

	role, err := getRole(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}
	if role == nil {
		role = &apigeeRole{}
	}

	for field, dst := range map[string]*string{"org_name": &role.OrgName, "developer_email": &role.DeveloperEmail, "app_name": &role.AppName} {
		if v, ok := d.GetOk(field); ok {
			*dst = v.(string)
		}
	}
	if v, ok := d.GetOk("api_products"); ok {
		if role.APIProducts, err = parseAPIProducts(v.([]string)); err != nil {
			return logical.ErrorResponse(err.Error()), nil
		}
	}
	if v, ok := d.GetOk("ttl"); ok {
		role.TTL = time.Duration(v.(int)) * time.Second
	}

	if err := role.validate(); err != nil {
		return logical.ErrorResponse(err.Error()), nil
	}

	entry, err := logical.StorageEntryJSON(roleStoragePrefix+name, role)
	if err != nil {
		return nil, err
	}
	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	if maxTTL := b.System().MaxLeaseTTL(); maxTTL > 0 && role.TTL > maxTTL {
		resp := &logical.Response{}
		resp.AddWarning(fmt.Sprintf("ttl %s exceeds the mount's max lease TTL; credentials will be capped at %s", role.TTL, maxTTL))
		return resp, nil
	}
	return nil, nil
}

func (b *apigeeBackend) pathRolesDelete(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	if err := req.Storage.Delete(ctx, roleStoragePrefix+d.Get("name").(string)); err != nil {
		return nil, fmt.Errorf("error deleting role: %w", err)
	}
	return nil, nil
}

func (b *apigeeBackend) pathRolesList(ctx context.Context, req *logical.Request, _ *framework.FieldData) (*logical.Response, error) {
	entries, err := req.Storage.List(ctx, roleStoragePrefix)
	if err != nil {
		return nil, err
	}
	return logical.ListResponse(entries), nil
}

func getRole(ctx context.Context, s logical.Storage, name string) (*apigeeRole, error) {
	if name == "" {
		return nil, errors.New("missing role name")
	}

	entry, err := s.Get(ctx, roleStoragePrefix+name)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	role := &apigeeRole{}
	if err := entry.DecodeJSON(role); err != nil {
		return nil, fmt.Errorf("error reading role %q: %w", name, err)
	}
	return role, nil
}

const (
	pathRoleHelpSynopsis    = `Manage roles that generate Apigee credentials.`
	pathRoleHelpDescription = `
A role identifies the Apigee developer app (org_name, developer_email,
app_name), the API products to associate with generated credentials, and the
credential TTL. On create, org_name, developer_email, app_name, and
api_products are required; on update, omitted fields keep their values.
`

	pathRoleListHelpSynopsis    = `List roles that generate Apigee credentials.`
	pathRoleListHelpDescription = `Lists the names of all configured roles.`
)
