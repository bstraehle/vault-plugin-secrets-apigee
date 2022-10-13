package secretsengine

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

func TestRoles(t *testing.T) {
	b, s := getTestBackend(t)

	t.Run("CreateRole", func(t *testing.T) {
		resp, err := testRoleCreate(t, b, s, map[string]interface{}{
			"org_name":        os.Getenv(envVarApigeeOrgName),
			"developer_email": os.Getenv(envVarApigeeDeveloperEmail),
			"app_name":        os.Getenv(envVarApigeeAppName),
			"api_products":    os.Getenv(envVarApigeeApiProducts),
			"ttl":             "24h",
		})

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("ReadRole", func(t *testing.T) {
		resp, err := testRoleRead(t, b, s)

		require.Nil(t, err)
		require.Nil(t, resp.Error())

		require.NotNil(t, resp)
		require.NotNil(t, resp.Data)

		require.Contains(t, resp.Data["org_name"], os.Getenv(envVarApigeeOrgName))
		require.Contains(t, resp.Data["developer_email"], os.Getenv(envVarApigeeDeveloperEmail))
		require.Contains(t, resp.Data["app_name"], os.Getenv(envVarApigeeAppName))
		require.Contains(t, resp.Data["api_products"], os.Getenv(envVarApigeeApiProducts))

		require.NotEmpty(t, resp.Data["ttl"])
	})

	t.Run("DeleteRole", func(t *testing.T) {
		resp, err := testRoleDelete(t, b, s)

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})
}

func testRoleCreate(t *testing.T, b *apigeeBackend, s logical.Storage, d map[string]interface{}) (*logical.Response, error) {
	t.Helper()

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/test",
		Data:      d,
		Storage:   s,
	})

	if err != nil {
		return nil, err
	}

	if resp != nil && resp.IsError() {
		t.Fatal(resp.Error())
	}

	return resp, nil
}

func testRoleRead(t *testing.T, b *apigeeBackend, s logical.Storage) (*logical.Response, error) {
	t.Helper()

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "roles/test",
		Storage:   s,
	})

	if err != nil {
		return nil, err
	}

	if resp != nil && resp.IsError() {
		t.Fatal(resp.Error())
	}

	return resp, nil
}

func testRoleDelete(t *testing.T, b *apigeeBackend, s logical.Storage) (*logical.Response, error) {
	t.Helper()

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "roles/test",
		Storage:   s,
	})

	if err != nil {
		return nil, err
	}

	if resp != nil && resp.IsError() {
		t.Fatal(resp.Error())
	}

	return resp, nil
}
