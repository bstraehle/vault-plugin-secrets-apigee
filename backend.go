package secretsengine

import (
	"context"
	"strings"
	"sync"

	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	help = "The Apigee secrets backend dynamically generates Apigee credentials."

	envVarApigeeHost       = "APIGEE_HOST"
	envVarApigeeOAuthToken = "APIGEE_OAUTH_TOKEN"
	envVarApigeeUsername   = "APIGEE_USERNAME"
	envVarApigeePassword   = "APIGEE_PASSWORD"

	envVarApigeeOrgName        = "APIGEE_ORG_NAME"
	envVarApigeeDeveloperEmail = "APIGEE_DEVELOPER_EMAIL"
	envVarApigeeAppName        = "APIGEE_APP_NAME"
	envVarApigeeApiProducts    = "APIGEE_API_PRODUCTS"
)

type apigeeBackend struct {
	*framework.Backend
	lock   sync.RWMutex
	client *apigeeClient
}

func Factory(ctx context.Context, conf *logical.BackendConfig) (logical.Backend, error) {
	b := backend()

	if err := b.Setup(ctx, conf); err != nil {
		return nil, err
	}

	return b, nil
}

func backend() *apigeeBackend {
	var b = apigeeBackend{}

	b.Backend = &framework.Backend{
		Help: strings.TrimSpace(help),
		PathsSpecial: &logical.Paths{
			LocalStorage: []string{},
			SealWrapStorage: []string{
				"config",
				"roles/*",
			},
		},
		Paths: framework.PathAppend(
			pathRoles(&b),
			[]*framework.Path{
				pathConfig(&b),
				pathCredentials(&b),
			},
		),
		Secrets: []*framework.Secret{
			b.apigeeToken(),
		},
		BackendType: logical.TypeLogical,
		Invalidate:  b.invalidate,
	}

	return &b
}

func (b *apigeeBackend) getClient(ctx context.Context, s logical.Storage) (*apigeeClient, error) {
	b.lock.RLock()
	unlockFunc := b.lock.RUnlock
	defer func() { unlockFunc() }()

	if b.client != nil {
		return b.client, nil
	}

	b.lock.RUnlock()
	b.lock.Lock()
	unlockFunc = b.lock.Unlock

	config, err := getConfig(ctx, s)

	if err != nil {
		return nil, err
	}

	if config == nil {
		config = new(apigeeConfig)
	}

	b.client, err = newClient(config)

	if err != nil {
		return nil, err
	}

	return b.client, nil
}

func (b *apigeeBackend) invalidate(ctx context.Context, key string) {
	if key == "config" {
		b.reset()
	}
}

func (b *apigeeBackend) reset() {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.client = nil
}
