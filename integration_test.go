//go:build integration

package secretsengine

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

// TestIntegration generates and revokes credentials against a real Apigee
// organization. Run with: go test -tags integration -run Integration -v
//
// Apigee X / hybrid: APIGEE_OAUTH_TOKEN or APIGEE_CREDENTIALS (service account
// key JSON), optionally APIGEE_HOST. Apigee Edge: APIGEE_HOST, APIGEE_USERNAME,
// APIGEE_PASSWORD. Both: APIGEE_ORG_NAME, APIGEE_DEVELOPER_EMAIL,
// APIGEE_APP_NAME, APIGEE_API_PRODUCTS (comma-separated).
func TestIntegration(t *testing.T) {
	env := func(k string) string {
		v := os.Getenv(k)
		if v == "" {
			t.Skipf("%s not set", k)
		}
		return v
	}

	config := map[string]interface{}{}
	if v := os.Getenv("APIGEE_HOST"); v != "" {
		config["host"] = v
	}
	switch {
	case os.Getenv("APIGEE_OAUTH_TOKEN") != "":
		config["oauth_token"] = os.Getenv("APIGEE_OAUTH_TOKEN")
	case os.Getenv("APIGEE_CREDENTIALS") != "":
		config["credentials"] = os.Getenv("APIGEE_CREDENTIALS")
	default:
		config["username"] = env("APIGEE_USERNAME")
		config["password"] = env("APIGEE_PASSWORD")
	}
	role := map[string]interface{}{
		"org_name":        env("APIGEE_ORG_NAME"),
		"developer_email": env("APIGEE_DEVELOPER_EMAIL"),
		"app_name":        env("APIGEE_APP_NAME"),
		"api_products":    env("APIGEE_API_PRODUCTS"),
		"ttl":             "10m",
	}

	b, s := getTestBackend(t)
	requireOK(t, request(t, b, s, logical.CreateOperation, "config", config))
	requireOK(t, request(t, b, s, logical.CreateOperation, "roles/test", role))

	var secrets []*logical.Secret
	for range 3 {
		resp := request(t, b, s, logical.ReadOperation, "creds/test", nil)
		requireOK(t, resp)
		require.NotEmpty(t, resp.Data["key"])
		require.NotEmpty(t, resp.Data["secret"])
		secrets = append(secrets, resp.Secret)
	}

	for _, secret := range secrets {
		_, err := b.HandleRequest(context.Background(), &logical.Request{Operation: logical.RevokeOperation, Secret: secret, Storage: s})
		require.NoError(t, err)
	}
}
