package secretsengine

import (
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

func TestRoles(t *testing.T) {
	b, s := getTestBackend(t)

	t.Run("create requires fields", func(t *testing.T) {
		resp := request(t, b, s, logical.CreateOperation, "roles/test", map[string]interface{}{"org_name": testOrg})
		requireErrorResponse(t, resp, "missing required fields: developer_email, app_name, api_products")
	})

	t.Run("create with legacy JSON array api_products", func(t *testing.T) {
		requireOK(t, request(t, b, s, logical.CreateOperation, "roles/test", map[string]interface{}{
			"org_name":        testOrg,
			"developer_email": testDev,
			"app_name":        testApp,
			"api_products":    `["product-a","product-b"]`,
			"ttl":             "30m",
		}))

		resp := request(t, b, s, logical.ReadOperation, "roles/test", nil)
		require.Equal(t, map[string]interface{}{
			"org_name":        testOrg,
			"developer_email": testDev,
			"app_name":        testApp,
			"api_products":    []string{"product-a", "product-b"},
			"ttl":             int64(1800),
		}, resp.Data)
	})

	t.Run("partial update keeps other fields", func(t *testing.T) {
		requireOK(t, request(t, b, s, logical.UpdateOperation, "roles/test", map[string]interface{}{"api_products": "product-c, product-d"}))

		resp := request(t, b, s, logical.ReadOperation, "roles/test", nil)
		require.Equal(t, []string{"product-c", "product-d"}, resp.Data["api_products"])
		require.Equal(t, testApp, resp.Data["app_name"])
		require.Equal(t, int64(1800), resp.Data["ttl"])
	})

	t.Run("invalid JSON api_products", func(t *testing.T) {
		resp := request(t, b, s, logical.UpdateOperation, "roles/test", map[string]interface{}{"api_products": `["unterminated`})
		requireErrorResponse(t, resp, "invalid api_products")
	})

	t.Run("ttl above mount max warns", func(t *testing.T) {
		resp := request(t, b, s, logical.UpdateOperation, "roles/test", map[string]interface{}{"ttl": "48h"})
		require.NotNil(t, resp)
		require.Len(t, resp.Warnings, 1)
		require.Contains(t, resp.Warnings[0], "capped")
	})

	t.Run("list", func(t *testing.T) {
		writeRole(t, b, s, "other", nil)
		resp := request(t, b, s, logical.ListOperation, "roles/", nil)
		require.Equal(t, []string{"other", "test"}, resp.Data["keys"])
	})

	t.Run("delete", func(t *testing.T) {
		requireOK(t, request(t, b, s, logical.DeleteOperation, "roles/test", nil))
		require.Nil(t, request(t, b, s, logical.ReadOperation, "roles/test", nil))
	})
}

func TestLegacyStoredRole(t *testing.T) {
	b, s := getTestBackend(t)

	// Roles written by v1.1.x stored api_products as a JSON array string.
	entry := &logical.StorageEntry{
		Key:   "roles/legacy",
		Value: []byte(`{"org_name":"org","developer_email":"dev@example.com","app_name":"app","api_products":"[\"product-a\",\"product-b\"]","ttl":86400000000000}`),
	}
	require.NoError(t, s.Put(t.Context(), entry))

	resp := request(t, b, s, logical.ReadOperation, "roles/legacy", nil)
	require.Equal(t, []string{"product-a", "product-b"}, resp.Data["api_products"])
	require.Equal(t, int64(86400), resp.Data["ttl"])
}

func TestParseAPIProducts(t *testing.T) {
	for _, tt := range []struct {
		in   []string
		want []string
	}{
		{[]string{"a"}, []string{"a"}},
		{[]string{"a", " b ", ""}, []string{"a", "b"}},
		{[]string{`["a"`, `"b"]`}, []string{"a", "b"}},
		{[]string{`["a,b"]`}, []string{"a,b"}},
		{[]string{`[]`}, []string{}},
	} {
		got, err := parseAPIProducts(tt.in)
		require.NoError(t, err)
		require.Equal(t, tt.want, got, "input %q", tt.in)
	}
}
