// Package secretsengine implements the Vault Apigee secrets engine, which
// generates short-lived Apigee developer app credentials (consumer key and
// consumer secret) and deletes them from Apigee when their lease is revoked.
package secretsengine

import (
	"context"
	"errors"
	"strings"
	"sync"

	apigee "github.com/bstraehle/apigee-client-go/v2"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/helper/locksutil"
	"github.com/hashicorp/vault/sdk/logical"
)

// Version is the plugin version reported to Vault. It is set at build time:
//
//	go build -ldflags "-X github.com/bstraehle/vault-plugin-secrets-apigee.Version=v1.2.1" ./cmd/vault-plugin-secrets-apigee
var Version = ""

const backendHelp = `
The Apigee secrets engine dynamically generates Apigee developer app
credentials (consumer key and consumer secret) with an expiry matching the
Vault lease, and deletes them from Apigee when the lease is revoked.

Configure access to the Apigee management API at "config", then define roles
at "roles/<name>" and read credentials from "creds/<name>".
`

var errNotConfigured = errors.New("backend is not configured, write to the config path first")

type apigeeBackend struct {
	*framework.Backend

	lock   sync.RWMutex
	client *apigee.Client

	// appLocks serializes credential creation per developer app so that the
	// newly created key can be identified unambiguously.
	appLocks []*locksutil.LockEntry

	// roleLocks serializes read-modify-write updates per role.
	roleLocks []*locksutil.LockEntry
}

// Factory returns a configured instance of the Apigee secrets engine.
func Factory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	b := backend()
	if err := b.Setup(ctx, conf); err != nil {
		return nil, err
	}
	return b, nil
}

func backend() *apigeeBackend {
	b := &apigeeBackend{appLocks: locksutil.CreateLocks(), roleLocks: locksutil.CreateLocks()}

	b.Backend = &framework.Backend{
		Help: strings.TrimSpace(backendHelp),
		PathsSpecial: &logical.Paths{
			SealWrapStorage: []string{
				configStoragePath,
				roleStoragePrefix + "*",
			},
		},
		Paths: framework.PathAppend(
			pathRoles(b),
			[]*framework.Path{
				pathConfig(b),
				pathCredentials(b),
			},
		),
		Secrets: []*framework.Secret{
			secretCredentials(b),
		},
		BackendType:    logical.TypeLogical,
		Invalidate:     b.invalidate,
		RunningVersion: Version,
	}

	return b
}

// getClient returns the cached Apigee client, creating it from the stored
// configuration on first use.
func (b *apigeeBackend) getClient(ctx context.Context, s logical.Storage) (*apigee.Client, error) {
	b.lock.RLock()
	client := b.client
	b.lock.RUnlock()
	if client != nil {
		return client, nil
	}

	b.lock.Lock()
	defer b.lock.Unlock()
	if b.client != nil {
		return b.client, nil
	}

	config, err := getConfig(ctx, s)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, errNotConfigured
	}

	b.client, err = config.newClient()
	if err != nil {
		return nil, err
	}
	return b.client, nil
}

func (b *apigeeBackend) invalidate(_ context.Context, key string) {
	if key == configStoragePath {
		b.reset()
	}
}

func (b *apigeeBackend) reset() {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.client = nil
}
