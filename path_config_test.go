package secretsengine

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"

	apigee "github.com/bstraehle/apigee-client-go/v2"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	b, s := getTestBackend(t)

	t.Run("read before write", func(t *testing.T) {
		require.Nil(t, request(t, b, s, logical.ReadOperation, "config", nil))
	})

	t.Run("requires an auth method", func(t *testing.T) {
		requireErrorResponse(t, request(t, b, s, logical.CreateOperation, "config", map[string]interface{}{}), "one of oauth_token")
	})

	t.Run("rejects multiple auth methods", func(t *testing.T) {
		resp := request(t, b, s, logical.CreateOperation, "config", map[string]interface{}{
			"oauth_token": "t", "username": "u", "password": "p",
		})
		requireErrorResponse(t, resp, "only one of")
	})

	t.Run("rejects username without password", func(t *testing.T) {
		requireErrorResponse(t, request(t, b, s, logical.CreateOperation, "config", map[string]interface{}{"username": "u"}), "set together")
	})

	t.Run("rejects invalid host", func(t *testing.T) {
		resp := request(t, b, s, logical.CreateOperation, "config", map[string]interface{}{"oauth_token": "t", "host": "not-a-url"})
		requireErrorResponse(t, resp, "invalid host")
	})

	t.Run("rejects invalid service account credentials", func(t *testing.T) {
		resp := request(t, b, s, logical.CreateOperation, "config", map[string]interface{}{"credentials": `{"type":"authorized_user"}`})
		requireErrorResponse(t, resp, "invalid service account credentials")
	})

	t.Run("oauth token defaults host and hides secrets", func(t *testing.T) {
		requireOK(t, request(t, b, s, logical.CreateOperation, "config", map[string]interface{}{"oauth_token": "secret-token"}))

		resp := request(t, b, s, logical.ReadOperation, "config", nil)
		require.Equal(t, map[string]interface{}{"host": apigee.DefaultHost, "auth_type": authTypeOAuthToken}, resp.Data)
	})

	t.Run("switching to basic auth clears the token", func(t *testing.T) {
		requireOK(t, request(t, b, s, logical.UpdateOperation, "config", map[string]interface{}{
			"host": "https://api.enterprise.apigee.com", "username": "u", "password": "p",
		}))
		config, err := getConfig(t.Context(), s)
		require.NoError(t, err)
		require.Equal(t, &apigeeConfig{Host: "https://api.enterprise.apigee.com", Username: "u", Password: "p"}, config)
	})

	t.Run("partial update keeps other fields", func(t *testing.T) {
		requireOK(t, request(t, b, s, logical.UpdateOperation, "config", map[string]interface{}{"password": "p2"}))
		config, err := getConfig(t.Context(), s)
		require.NoError(t, err)
		require.Equal(t, &apigeeConfig{Host: "https://api.enterprise.apigee.com", Username: "u", Password: "p2"}, config)

		resp := request(t, b, s, logical.ReadOperation, "config", nil)
		require.Equal(t, authTypeBasic, resp.Data["auth_type"])
		require.NotContains(t, resp.Data, "password")
		require.NotContains(t, resp.Data, "username")
	})

	t.Run("legacy config without host uses default", func(t *testing.T) {
		entry, err := logical.StorageEntryJSON(configStoragePath, map[string]string{"oauth_token": "t"})
		require.NoError(t, err)
		require.NoError(t, s.Put(t.Context(), entry))
		config, err := getConfig(t.Context(), s)
		require.NoError(t, err)
		require.Equal(t, apigee.DefaultHost, config.Host)
	})

	t.Run("delete", func(t *testing.T) {
		requireOK(t, request(t, b, s, logical.DeleteOperation, "config", nil))
		require.Nil(t, request(t, b, s, logical.ReadOperation, "config", nil))
	})
}

func TestServiceAccountCredentials(t *testing.T) {
	b, s := getTestBackend(t)
	f := newFakeApigee(t)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	sa, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "test",
		"private_key_id": "1",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: must(x509.MarshalPKCS8PrivateKey(key))})),
		"client_email":   "vault@test.iam.gserviceaccount.com",
		"token_uri":      f.URL + "/token",
	})
	require.NoError(t, err)

	requireOK(t, request(t, b, s, logical.CreateOperation, "config", map[string]interface{}{
		"host":        f.URL,
		"credentials": string(sa),
	}))
	require.Equal(t, authTypeServiceAccount, request(t, b, s, logical.ReadOperation, "config", nil).Data["auth_type"])

	writeRole(t, b, s, "test", nil)
	request(t, b, s, logical.ReadOperation, "creds/test", nil)
	request(t, b, s, logical.ReadOperation, "creds/test", nil)

	require.Equal(t, 1, f.tokenIssued, "access token should be cached and reused")
	for _, a := range f.auth {
		require.Equal(t, "Bearer sa-token", a)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
