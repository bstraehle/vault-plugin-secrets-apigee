package secretsengine

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	pathRoleHelpSynopsis    = `Manages roles for generating Apigee credentials.`
	pathRoleHelpDescription = `This path manages Vault roles for generating Apigee credentials.`

	pathRoleListHelpSynopsis    = `Lists roles for generating Apigee credentials.`
	pathRoleListHelpDescription = `This path lists Vault roles for generating Apigee credentials.`
)

type apigeeRole struct {
	OrgName        string        `json:"org_name"`
	DeveloperEmail string        `json:"developer_email"`
	AppName        string        `json:"app_name"`
	ApiProducts    string        `json:"api_products"`
	TTL            time.Duration `json:"ttl"`
}

func pathRoles(b *apigeeBackend) []*framework.Path {
	return []*framework.Path{
		{
			Pattern: "roles/" + framework.GenericNameRegex("name"),
			Fields: map[string]*framework.FieldSchema{
				"name": {
					Type:        framework.TypeString,
					Description: "The role name",
					Required:    true,
				},
				"org_name": {
					Type:        framework.TypeString,
					Description: "The org_name for the Apigee Management API",
					Required:    true,
				},
				"developer_email": {
					Type:        framework.TypeString,
					Description: "The developer_email for the Apigee Management API",
					Required:    true,
				},
				"app_name": {
					Type:        framework.TypeString,
					Description: "The app_name for the Apigee Management API",
					Required:    true,
				},
				"api_products": {
					Type:        framework.TypeString,
					Description: "The api_products for the Apigee Management API",
					Required:    true,
				},
				"ttl": {
					Type:        framework.TypeDurationSecond,
					Description: "Lease for credentials",
				},
			},
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.ReadOperation: &framework.PathOperation{
					Callback: b.pathRolesRead,
				},
				logical.CreateOperation: &framework.PathOperation{
					Callback: b.pathRolesWrite,
				},
				logical.UpdateOperation: &framework.PathOperation{
					Callback: b.pathRolesWrite,
				},
				logical.DeleteOperation: &framework.PathOperation{
					Callback: b.pathRolesDelete,
				},
			},
			HelpSynopsis:    pathRoleHelpSynopsis,
			HelpDescription: pathRoleHelpDescription,
		},
		{
			Pattern: "roles/?$",
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.ListOperation: &framework.PathOperation{
					Callback: b.pathRolesList,
				},
			},
			HelpSynopsis:    pathRoleListHelpSynopsis,
			HelpDescription: pathRoleListHelpDescription,
		},
	}
}

func (b *apigeeBackend) pathRolesWrite(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	name, ok := d.GetOk("name")

	if !ok {
		return logical.ErrorResponse("missing role name"), nil
	}

	role, err := b.getRole(ctx, req.Storage, name.(string))

	if err != nil {
		return nil, err
	}

	if role == nil {
		role = &apigeeRole{}
	}

	if org_name, ok := d.GetOk("org_name"); ok {
		role.OrgName = org_name.(string)
	} else if !ok {
		return nil, fmt.Errorf("missing org_name in role")
	}

	if developer_email, ok := d.GetOk("developer_email"); ok {
		role.DeveloperEmail = developer_email.(string)
	} else if !ok {
		return nil, fmt.Errorf("missing developer_email in role")
	}

	if app_name, ok := d.GetOk("app_name"); ok {
		role.AppName = app_name.(string)
	} else if !ok {
		return nil, fmt.Errorf("missing app_name in role")
	}

	if api_products, ok := d.GetOk("api_products"); ok {
		role.ApiProducts = api_products.(string)
	} else if !ok {
		return nil, fmt.Errorf("missing api_products in role")
	}

	if ttl, ok := d.GetOk("ttl"); ok {
		role.TTL = time.Duration(ttl.(int)) * time.Second
	} else if !ok {
		return nil, fmt.Errorf("missing ttl in role")
	}

	if err := setRole(ctx, req.Storage, name.(string), role); err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *apigeeBackend) pathRolesRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	role, err := b.getRole(ctx, req.Storage, d.Get("name").(string))

	if err != nil {
		return nil, err
	}

	if role == nil {
		return nil, nil
	}

	return &logical.Response{
		Data: role.toResponseData(),
	}, nil
}

func (b *apigeeBackend) pathRolesDelete(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	err := req.Storage.Delete(ctx, "roles/"+d.Get("name").(string))

	if err != nil {
		return nil, fmt.Errorf("error deleting role: %w", err)
	}

	return nil, nil
}

func (b *apigeeBackend) pathRolesList(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	entries, err := req.Storage.List(ctx, "roles/")

	if err != nil {
		return nil, err
	}

	return logical.ListResponse(entries), nil
}

func setRole(ctx context.Context, s logical.Storage, name string, apigeeRole *apigeeRole) error {
	role, err := logical.StorageEntryJSON("roles/"+name, apigeeRole)

	if err != nil {
		return err
	}

	if role == nil {
		return fmt.Errorf("failed to create storage entry for role")
	}

	if err := s.Put(ctx, role); err != nil {
		return err
	}

	return nil
}

func (b *apigeeBackend) getRole(ctx context.Context, s logical.Storage, name string) (*apigeeRole, error) {
	if name == "" {
		return nil, fmt.Errorf("missing role name")
	}

	role, err := s.Get(ctx, "roles/"+name)

	if err != nil {
		return nil, err
	}

	if role == nil {
		return nil, nil
	}

	var decodedRole apigeeRole

	if err := role.DecodeJSON(&decodedRole); err != nil {
		return nil, err
	}

	return &decodedRole, nil
}

func (r *apigeeRole) toResponseData() map[string]interface{} {
	respData := map[string]interface{}{
		"org_name":        r.OrgName,
		"developer_email": r.DeveloperEmail,
		"app_name":        r.AppName,
		"api_products":    r.ApiProducts,
		"ttl":             r.TTL.Seconds(),
	}

	return respData
}
