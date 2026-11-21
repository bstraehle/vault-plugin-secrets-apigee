package secretsengine

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	t.Run("legacy config with token and stale basic auth uses the token", func(t *testing.T) {
		entry, err := logical.StorageEntryJSON(configStoragePath, map[string]string{
			"host": "https://apigee.googleapis.com", "oauth_token": "t", "username": "stale", "password": "stale",
		})
		require.NoError(t, err)
		require.NoError(t, s.Put(t.Context(), entry))

		config, err := getConfig(t.Context(), s)
		require.NoError(t, err)
		require.Equal(t, &apigeeConfig{Host: "https://apigee.googleapis.com", OAuthToken: "t"}, config)
		_, err = config.newClient()
		require.NoError(t, err)
		require.Equal(t, authTypeOAuthToken, request(t, b, s, logical.ReadOperation, "config", nil).Data["auth_type"])
	})

	t.Run("delete", func(t *testing.T) {
		requireOK(t, request(t, b, s, logical.DeleteOperation, "config", nil))
		require.Nil(t, request(t, b, s, logical.ReadOperation, "config", nil))
	})
}

func serviceAccountJSON(t *testing.T, tokenURI string, privateKey string) string {
	t.Helper()
	if privateKey == "" {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		privateKey = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: must(x509.MarshalPKCS8PrivateKey(key))}))
	}
	sa, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "test",
		"private_key_id": "1",
		"private_key":    privateKey,
		"client_email":   "vault@test.iam.gserviceaccount.com",
		"token_uri":      tokenURI,
	})
	require.NoError(t, err)
	return string(sa)
}

func TestServiceAccountInvalidKeyKeepsConfig(t *testing.T) {
	b, s := getTestBackend(t)
	f := newFakeApigee(t)
	configure(t, b, s, f)

	badKey := "-----BEGIN PRIVATE KEY-----\nZGFtYWdlZA==\n-----END PRIVATE KEY-----\n"
	resp := request(t, b, s, logical.UpdateOperation, "config", map[string]interface{}{"credentials": serviceAccountJSON(t, f.URL+"/token", badKey)})
	requireErrorResponse(t, resp, "private_key")

	resp = request(t, b, s, logical.UpdateOperation, "config", map[string]interface{}{"credentials": `{"type":"service_account","private_key":"x"}`})
	requireErrorResponse(t, resp, "client_email and private_key are required")

	require.Equal(t, authTypeOAuthToken, request(t, b, s, logical.ReadOperation, "config", nil).Data["auth_type"], "rejected update must keep the working config")
}

func TestServiceAccountTokenTimeout(t *testing.T) {
	old := tokenHTTPTimeout
	tokenHTTPTimeout = 200 * time.Millisecond
	t.Cleanup(func() { tokenHTTPTimeout = old })

	release := make(chan struct{})
	stalled := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); stalled.Close() })

	b, s := getTestBackend(t)
	f := newFakeApigee(t)
	requireOK(t, request(t, b, s, logical.CreateOperation, "config", map[string]interface{}{
		"host":        f.URL,
		"credentials": serviceAccountJSON(t, stalled.URL, ""),
	}))
	writeRole(t, b, s, "test", nil)

	start := time.Now()
	_, err := b.HandleRequest(context.Background(), &logical.Request{Operation: logical.ReadOperation, Path: "creds/test", Storage: s})
	require.Error(t, err)
	require.Less(t, time.Since(start), 5*time.Second, "token exchange must be bounded")
}

func TestServiceAccountCredentials(t *testing.T) {
	b, s := getTestBackend(t)
	f := newFakeApigee(t)
	sa := serviceAccountJSON(t, f.URL+"/token", "")

	requireOK(t, request(t, b, s, logical.CreateOperation, "config", map[string]interface{}{
		"host":        f.URL,
		"credentials": sa,
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
