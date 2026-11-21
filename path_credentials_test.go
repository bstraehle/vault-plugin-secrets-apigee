package secretsengine

import (
	"context"
	"encoding/base64"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

func TestCredentialsNotConfigured(t *testing.T) {
	b, s := getTestBackend(t)
	writeRole(t, b, s, "test", nil)

	_, err := b.HandleRequest(context.Background(), &logical.Request{Operation: logical.ReadOperation, Path: "creds/test", Storage: s})
	require.ErrorIs(t, err, errNotConfigured)
}

func TestCredentialsUnknownRole(t *testing.T) {
	b, s := getTestBackend(t)
	configure(t, b, s, newFakeApigee(t))
	requireErrorResponse(t, request(t, b, s, logical.ReadOperation, "creds/missing", nil), "unknown role")
}

func TestCredentialsLifecycle(t *testing.T) {
	b, s := getTestBackend(t)
	f := newFakeApigee(t)
	configure(t, b, s, f)
	writeRole(t, b, s, "test", map[string]interface{}{"ttl": "30m"})

	resp := request(t, b, s, logical.ReadOperation, "creds/test", nil)
	requireOK(t, resp)

	key := resp.Data["key"].(string)
	require.Equal(t, "key-1", key)
	require.Equal(t, "secret-of-key-1", resp.Data["secret"])
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("key-1:secret-of-key-1")), resp.Data["credentials"])
	require.Equal(t, []string{"product-a", "product-b"}, resp.Data["api_products"])
	expiresAt, err := time.Parse(time.RFC3339, resp.Data["expires_at"].(string))
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(30*time.Minute), expiresAt, time.Minute)

	require.Equal(t, 30*time.Minute, resp.Secret.TTL)
	require.False(t, resp.Secret.Renewable)
	require.Equal(t, "1800000", f.lastPost()["keyExpiresIn"])
	require.Equal(t, "Bearer "+testToken, f.auth[0])

	require.NotContains(t, resp.Secret.InternalData, "secret", "consumer secret must not be persisted with the lease")
	require.NotContains(t, resp.Secret.InternalData, "credentials")
	require.Equal(t, key, resp.Secret.InternalData["key"])

	revoke := func() (*logical.Response, error) {
		return b.HandleRequest(context.Background(), &logical.Request{
			Operation: logical.RevokeOperation,
			Secret:    resp.Secret,
			Storage:   s,
		})
	}

	_, err = revoke()
	require.NoError(t, err)
	require.False(t, f.hasKey(key), "key should be deleted from Apigee")
	require.True(t, f.hasKey("preexisting"), "other keys must be untouched")

	_, err = revoke()
	require.NoError(t, err, "revoking an already deleted key should succeed")
}

func TestCredentialsTTL(t *testing.T) {
	b, s := getTestBackend(t)
	configure(t, b, s, newFakeApigee(t))

	writeRole(t, b, s, "default", nil)
	require.Equal(t, testDefaultTTL, request(t, b, s, logical.ReadOperation, "creds/default", nil).Secret.TTL)

	writeRole(t, b, s, "long", map[string]interface{}{"ttl": "72h"})
	require.Equal(t, testMaxTTL, request(t, b, s, logical.ReadOperation, "creds/long", nil).Secret.TTL)
}

func TestCredentialsConcurrent(t *testing.T) {
	b, s := getTestBackend(t)
	configure(t, b, s, newFakeApigee(t))
	writeRole(t, b, s, "test", nil)

	const n = 10
	keys := make(chan string, n)
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			resp, err := b.HandleRequest(context.Background(), &logical.Request{Operation: logical.ReadOperation, Path: "creds/test", Storage: s})
			if err == nil && resp != nil && !resp.IsError() {
				keys <- resp.Data["key"].(string)
			}
		})
	}
	wg.Wait()
	close(keys)

	seen := map[string]bool{}
	for k := range keys {
		require.False(t, seen[k], "key %s handed out twice", k)
		seen[k] = true
	}
	require.Len(t, seen, n)
}

func TestRevokeMissingInternalData(t *testing.T) {
	b, s := getTestBackend(t)
	_, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.RevokeOperation,
		Storage:   s,
		Secret: &logical.Secret{InternalData: map[string]interface{}{
			"secret_type": secretCredentialsType,
			"org_name":    testOrg,
		}},
	})
	require.ErrorContains(t, err, "missing developer_email")
}
