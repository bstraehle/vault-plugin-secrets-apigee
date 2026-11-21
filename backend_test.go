package secretsengine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

const (
	testDefaultTTL = time.Hour
	testMaxTTL     = 24 * time.Hour

	testOrg   = "org"
	testDev   = "dev@example.com"
	testApp   = "app"
	testToken = "static-token"
)

func getTestBackend(t *testing.T) (*apigeeBackend, logical.Storage) {
	t.Helper()

	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}
	config.Logger = hclog.NewNullLogger()
	config.System = &logical.StaticSystemView{
		DefaultLeaseTTLVal: testDefaultTTL,
		MaxLeaseTTLVal:     testMaxTTL,
	}

	b, err := Factory(context.Background(), config)
	require.NoError(t, err)
	return b.(*apigeeBackend), config.StorageView
}

func request(t *testing.T, b logical.Backend, s logical.Storage, op logical.Operation, path string, data map[string]interface{}) *logical.Response {
	t.Helper()
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: op,
		Path:      path,
		Data:      data,
		Storage:   s,
	})
	require.NoError(t, err)
	return resp
}

func requireOK(t *testing.T, resp *logical.Response) {
	t.Helper()
	if resp != nil && resp.IsError() {
		t.Fatalf("unexpected error response: %v", resp.Error())
	}
}

func requireErrorResponse(t *testing.T, resp *logical.Response, contains string) {
	t.Helper()
	require.NotNil(t, resp)
	require.True(t, resp.IsError(), "expected error response, got %#v", resp)
	require.Contains(t, resp.Error().Error(), contains)
}

// fakeApigee is an in-memory Apigee management API serving a single developer
// app, plus an OAuth 2.0 token endpoint for service account tests.
type fakeApigee struct {
	*httptest.Server

	mu          sync.Mutex
	keys        map[string]int64 // consumer key -> expiresAt (ms)
	next        int
	auth        []string
	posts       []map[string]interface{}
	tokenIssued int
}

func newFakeApigee(t *testing.T) *fakeApigee {
	t.Helper()
	f := &fakeApigee{keys: map[string]int64{"preexisting": -1}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeApigee) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if r.URL.Path == "/token" {
		f.tokenIssued++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"sa-token","token_type":"Bearer","expires_in":3600}`))
		return
	}

	f.auth = append(f.auth, r.Header.Get("Authorization"))
	appPath := "/v1/organizations/" + testOrg + "/developers/" + testDev + "/apps/" + testApp

	switch {
	case r.URL.Path == appPath && r.Method == http.MethodGet:
	case r.URL.Path == appPath && r.Method == http.MethodPost:
		var body map[string]interface{}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		f.posts = append(f.posts, body)
		ms, _ := strconv.ParseInt(body["keyExpiresIn"].(string), 10, 64)
		f.next++
		f.keys["key-"+strconv.Itoa(f.next)] = time.Now().UnixMilli() + ms
	case strings.HasPrefix(r.URL.Path, appPath+"/keys/") && r.Method == http.MethodDelete:
		key := strings.TrimPrefix(r.URL.Path, appPath+"/keys/")
		if _, ok := f.keys[key]; !ok {
			http.Error(w, `{"error":{"code":404}}`, http.StatusNotFound)
			return
		}
		delete(f.keys, key)
		_, _ = w.Write([]byte(`{}`))
		return
	default:
		http.Error(w, `{"error":{"code":404}}`, http.StatusNotFound)
		return
	}

	creds := []map[string]string{}
	for k, exp := range f.keys {
		creds = append(creds, map[string]string{
			"consumerKey":    k,
			"consumerSecret": "secret-of-" + k,
			"issuedAt":       strconv.FormatInt(time.Now().UnixMilli(), 10),
			"expiresAt":      strconv.FormatInt(exp, 10),
		})
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"name": testApp, "credentials": creds})
}

func (f *fakeApigee) hasKey(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.keys[key]
	return ok
}

func (f *fakeApigee) lastPost() map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.posts[len(f.posts)-1]
}

func configure(t *testing.T, b logical.Backend, s logical.Storage, f *fakeApigee) {
	t.Helper()
	requireOK(t, request(t, b, s, logical.CreateOperation, "config", map[string]interface{}{
		"host":        f.URL,
		"oauth_token": testToken,
	}))
}

func writeRole(t *testing.T, b logical.Backend, s logical.Storage, name string, extra map[string]interface{}) {
	t.Helper()
	data := map[string]interface{}{
		"org_name":        testOrg,
		"developer_email": testDev,
		"app_name":        testApp,
		"api_products":    "product-a,product-b",
	}
	for k, v := range extra {
		data[k] = v
	}
	requireOK(t, request(t, b, s, logical.CreateOperation, "roles/"+name, data))
}

func TestInvalidateResetsClient(t *testing.T) {
	b, s := getTestBackend(t)
	f := newFakeApigee(t)
	configure(t, b, s, f)

	_, err := b.getClient(context.Background(), s)
	require.NoError(t, err)
	require.NotNil(t, b.client)

	b.invalidate(context.Background(), "roles/x")
	require.NotNil(t, b.client)

	b.invalidate(context.Background(), configStoragePath)
	require.Nil(t, b.client)
}
