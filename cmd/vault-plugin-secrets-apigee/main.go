// Command vault-plugin-secrets-apigee serves the Apigee secrets engine as a Vault plugin.
package main

import (
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/sdk/plugin"

	apigee "github.com/bstraehle/vault-plugin-secrets-apigee"
)

func main() {
	logger := hclog.New(&hclog.LoggerOptions{})

	apiClientMeta := &api.PluginAPIClientMeta{}
	flags := apiClientMeta.FlagSet()
	if err := flags.Parse(os.Args[1:]); err != nil {
		logger.Error("invalid plugin flags", "error", err)
		os.Exit(1)
	}

	err := plugin.ServeMultiplex(&plugin.ServeOpts{
		BackendFactoryFunc: apigee.Factory,
		TLSProviderFunc:    api.VaultPluginTLSProvider(apiClientMeta.GetTLSConfig()),
	})
	if err != nil {
		logger.Error("plugin shutting down", "error", err)
		os.Exit(1)
	}
}
