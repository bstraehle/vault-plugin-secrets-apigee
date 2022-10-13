package secretsengine

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

type testEnv struct {
	Host       string
	OAuthToken string
	Username   string
	Password   string

	OrgName        string
	DeveloperEmail string
	AppName        string
	ApiProducts    string

	Keys []string

	Backend logical.Backend
	Context context.Context
	Storage logical.Storage
}

func getTestBackend(tb testing.TB) (*apigeeBackend, logical.Storage) {
	tb.Helper()

	config := logical.TestBackendConfig()
	config.StorageView = new(logical.InmemStorage)
	config.Logger = hclog.NewNullLogger()
	config.System = logical.TestSystemView()

	b, err := Factory(context.Background(), config)

	if err != nil {
		tb.Fatal(err)
	}

	return b.(*apigeeBackend), config.StorageView
}

func (e *testEnv) CreateConfig(t *testing.T) {
	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"host":        e.Host,
			"oauth_token": e.OAuthToken,
			"username":    e.Username,
			"password":    e.Password,
		},
	}

	resp, err := e.Backend.HandleRequest(e.Context, req)

	require.Nil(t, err)
	require.Nil(t, resp)
}

func (e *testEnv) CreateRole(t *testing.T) {
	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/test",
		Storage:   e.Storage,
		Data: map[string]interface{}{
			"org_name":        e.OrgName,
			"developer_email": e.DeveloperEmail,
			"app_name":        e.AppName,
			"api_products":    e.ApiProducts,

			"ttl": 86400,
		},
	}

	resp, err := e.Backend.HandleRequest(e.Context, req)

	require.Nil(t, err)
	require.Nil(t, resp)
}

func (e *testEnv) ReadCred(t *testing.T) {
	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "creds/test",
		Storage:   e.Storage,
	}

	resp, err := e.Backend.HandleRequest(e.Context, req)

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Data)

	require.Contains(t, resp.Data["org_name"], os.Getenv(envVarApigeeOrgName))
	require.Contains(t, resp.Data["developer_email"], os.Getenv(envVarApigeeDeveloperEmail))
	require.Contains(t, resp.Data["app_name"], os.Getenv(envVarApigeeAppName))
	require.Contains(t, resp.Data["api_products"], os.Getenv(envVarApigeeApiProducts))

	require.NotEmpty(t, resp.Data["key"])
	require.NotEmpty(t, resp.Data["secret"])
	require.NotEmpty(t, resp.Data["credentials"])

	k, ok := resp.Data["key"]

	if ok {
		e.Keys = append(e.Keys, k.(string))
	}
}

func (e *testEnv) DeleteCreds(t *testing.T) {
	if len(e.Keys) == 0 {
		t.Fatalf("expected 3 keys, got: %d", len(e.Keys))
	}

	for _, key := range e.Keys {
		b := e.Backend.(*apigeeBackend)
		client, err := b.getClient(e.Context, e.Storage)

		if err != nil {
			t.Fatal("error getting client")
		}

		err = client.DeleteCredentials(e.OrgName, e.DeveloperEmail, e.AppName, key)

		if err != nil {
			t.Fatalf("error deleting credentials: %s", err)
		}
	}
}
