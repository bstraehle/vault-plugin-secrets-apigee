package secretsengine

import (
	"context"
	"os"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/helper/logging"
	"github.com/hashicorp/vault/sdk/logical"
)

func newTestEnv() (*testEnv, error) {
	ctx := context.Background()

	defaultLease, _ := time.ParseDuration("1d")
	maxLease, _ := time.ParseDuration("1d")

	conf := &logical.BackendConfig{
		System: &logical.StaticSystemView{
			DefaultLeaseTTLVal: defaultLease,
			MaxLeaseTTLVal:     maxLease,
		},
		Logger: logging.NewVaultLogger(log.Debug),
	}

	b, err := Factory(ctx, conf)

	if err != nil {
		return nil, err
	}

	return &testEnv{
		Host:       os.Getenv(envVarApigeeHost),
		OAuthToken: os.Getenv(envVarApigeeOAuthToken),
		Username:   os.Getenv(envVarApigeeUsername),
		Password:   os.Getenv(envVarApigeePassword),

		OrgName:        os.Getenv(envVarApigeeOrgName),
		DeveloperEmail: os.Getenv(envVarApigeeDeveloperEmail),
		AppName:        os.Getenv(envVarApigeeAppName),
		ApiProducts:    os.Getenv(envVarApigeeApiProducts),

		Backend: b,
		Context: ctx,
		Storage: &logical.InmemStorage{},
	}, nil
}

func TestCreds(t *testing.T) {
	testEnv, err := newTestEnv()

	if err != nil {
		t.Fatal(err)
	}

	t.Run("CreateConfig", testEnv.CreateConfig)
	t.Run("CreateRole", testEnv.CreateRole)
	t.Run("ReadCred1", testEnv.ReadCred)
	t.Run("ReadCred2", testEnv.ReadCred)
	t.Run("ReadCred3", testEnv.ReadCred)
	t.Run("DeleteCreds", testEnv.DeleteCreds)
}
