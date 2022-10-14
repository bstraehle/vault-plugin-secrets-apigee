# Vault Plugin: Apigee Secrets Engine

[![Go Reference](https://pkg.go.dev/badge/github.com/bstraehle/vault-plugin-secrets-apigee.svg)](https://pkg.go.dev/github.com/bstraehle/vault-plugin-secrets-apigee) [![Go Report Card](https://goreportcard.com/badge/github.com/bstraehle/vault-plugin-secrets-apigee)](https://goreportcard.com/report/github.com/bstraehle/vault-plugin-secrets-apigee) [![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/bstraehle/vault-plugin-secrets-apigee?color=red&include_prereleases&sort=semver)](https://github.com/bstraehle/vault-plugin-secrets-apigee/releases)

Apigee helps companies design, secure, and scale application programming interfaces (APIs).

Apigee apps contain a consumer key and consumer secret (credentials), which are typically used to obtain an OAuth2 access token for API access. These credentials have an expiry, by default never. For **zero trust security** use cases, instead of apps using static, long-lived credentials, the **Vault Apigee secrets engine** generates dynamic, short-lived credentials, aka **ephemeral credentials**, enabling frequent rotation. For other use cases, the [Vault KV secrets engine](https://developer.hashicorp.com/vault/docs/secrets/kv) can be used.

## Table of Contents

1. [Use Case](#1-use-case)
2. [Examples](#2-examples)
3. [Prerequisites](#3-prerequisites)
4. [Configure Access](#4-configure-access)
5. [Configure Environment](#5-configure-environment)
6. [Build/Test or Get Binary](#6-buildtest-or-get-binary)
7. [Register Plugin](#7-register-plugin)
8. [Enable Secrets Engine](#8-enable-secrets-engine)
9. [Write Config](#9-write-config)
10. [Write Role](#10-write-role)
11. [Usage: Vault CLI](#11-usage-vault-cli)
12. [Usage: Vault API](#12-usage-vault-api)
13. [References](#13-references)

## 1. Use Case

![HashiCorp Vault custom secrets engine for Apigee](https://github.com/bstraehle/vault-plugin-secrets-apigee/blob/master/sequence-diagram.png)

## 2. Examples

Vault API and Apigee X

![Apigee X and Linux](https://github.com/bstraehle/vault-plugin-secrets-apigee/blob/master/example-apigee-x.png)

Vault CLI and Apigee Edge for Private Cloud

![Apigee Edge and Windows](https://github.com/bstraehle/vault-plugin-secrets-apigee/blob/master/example-apigee-edge.png)

## 3. Prerequisites

- [Git](https://git-scm.com/downloads) (Optional)
- [Go](https://go.dev/dl/) (Optional)
- [HashiCorp Vault](https://www.vaultproject.io/downloads)
- [Apigee X](https://cloud.google.com/apigee/docs/) and [Google Cloud CLI](https://cloud.google.com/sdk/docs/install) or
- [Apigee Edge](https://docs.apigee.com/)

## 4. Configure Access

```
gcloud auth login
export APIGEE_OAUTH_TOKEN=$(gcloud auth print-access-token)
```

or

```
export APIGEE_USERNAME=<APIGEE_USERNAME>
export APIGEE_PASSWORD=<APIGEE_PASSWORD>
```

## 5. Configure Environment

```
export APIGEE_HOST=<APIGEE_HOST>
export APIGEE_ORG_NAME=<APIGEE_ORG_NAME>
export APIGEE_DEVELOPER_EMAIL=<APIGEE_DEVELOPER_EMAIL>
export APIGEE_APP_NAME=<APIGEE_APP_NAME>
export APIGEE_API_PRODUCTS=[\"<APIGEE_API_PRODUCT>\"]
```

## 6. Build/Test or Get Binary

```
git clone https://github.com/bstraehle/vault-plugin-secrets-apigee.git
cd vault-plugin-secrets-apigee

go build -o vault/plugins/vault-plugin-secrets-apigee cmd/vault-plugin-secrets-apigee/main.go
go test -v
```

or

```
cd vault/plugins
wget https://github.com/bstraehle/vault-plugin-secrets-apigee/releases/download/v1.1.1/vault-plugin-secrets-apigee-linux-amd64
mv vault-plugin-secrets-apigee-linux-amd64 vault-plugin-secrets-apigee
chmod 700 vault-plugin-secrets-apigee
```

## 7. Register Plugin

Add plugin directory to server configuration

```
cat > vault/server.hcl << EOF
plugin_directory = "$(pwd)/vault/plugins"
api_addr         = "http://127.0.0.1:8200"

listener "tcp" {
  address     = "127.0.0.1:8200"
  tls_disable = "true"
}

storage "file" {
  path = "/tmp/vault-data"
}
EOF
```

Start server

```
vault server -config=vault/server.hcl -log-level=trace
```

In new terminal, initialize and unseal Vault

```
export VAULT_ADDR='http://127.0.0.1:8200'

vault operator init
vault operator unseal
```

Register plugin

```
vault login

SHA256=$(sha256sum vault/plugins/vault-plugin-secrets-apigee | cut -d ' ' -f1)

vault plugin register -sha256=$SHA256 secret vault-plugin-secrets-apigee
```
```
Success! Registered plugin: vault-plugin-secrets-apigee
```

## 8. Enable Secrets Engine

Enable secrets engine

```
vault secrets enable -path=apigee -description="apigee secrets engine" vault-plugin-secrets-apigee
```
```
Success! Enabled the vault-plugin-secrets-apigee secrets engine at: apigee/
```

Disable secrets engine (optional)

```
vault secrets disable apigee
```
```
Success! Disabled the secrets engine (if it existed) at: apigee/
```

## 9. Write Config

Write config

```
vault write apigee/config host=$APIGEE_HOST oauth_token=$APIGEE_OAUTH_TOKEN
```
```
Success! Data written to: apigee/config
```

Read config (optional)

```
vault read apigee/config
```
```
Key     Value
---     -----
host    <APIGEE_HOST>
```

Delete config (optional)

```
vault delete apigee/config
```
```
Success! Data deleted (if it existed) at: apigee/config
```

## 10. Write Role

Write role

```
vault write apigee/roles/test \
org_name=$APIGEE_ORG_NAME \
developer_email=$APIGEE_DEVELOPER_EMAIL \
app_name=$APIGEE_APP_NAME \
api_products=$APIGEE_API_PRODUCTS \
ttl=24h
```
```
Success! Data written to: apigee/roles/test
```

Read role (optional)

```
  vault read apigee/roles/test
```
```
Key                Value
---                -----
api_products       <APIGEE_API_PRODUCTS>
app_name           <APIGEE_APP_NAME>
developer_email    <APIGEE_DEVELOPER_EMAIL>
org_name           <APIGEE_ORG_NAME>
ttl                24h
```

Delete role (optional)

```
vault delete apigee/roles/test
```
```
Success! Data deleted (if it existed) at: apigee/roles/test
```

## 11. Usage: Vault CLI

Read creds

```
vault read apigee/creds/test
```
```
Key                Value
---                -----
lease_id           <LEASE_ID>
lease_duration     24h
lease_renewable    false
api_products       <APIGEE_API_PRODUCTS>
app_name           <APIGEE_APP_NAME>
credentials        RkRJTUdqbXJ1dDRmY2hTdUdKaEZETVZhNDAwN2MwM3NXQThEVEpobnJ3NTk3MmkzOkp2x...
developer_email    <APIGEE_DEVELOPER_EMAIL>
key                FDIMGjmrut4fchSuGJhFDMVa4007c03sWA8DTJhnrw5972i3
org_name           <APIGEE_ORG_NAME>
secret             JvmsfZaajNoqT6Ei7XAYmSTsTA8APSWdu9JxYKtZmEonZ862jKg3ROluxr6Bb710
```

Revoke lease (optional)

```
vault lease revoke <LEASE_ID>
```
```
All revocation operations queued successfully!
```

## 12. Usage: Vault API

Read creds

```
curl --header "X-Vault-Token: <VAULT_TOKEN>" http://127.0.0.1:8200/v1/apigee/creds/test | jq
```
```
{
	"request_id": "<REQUEST_ID>",
	"lease_id": "<LEASE_ID>",
	"renewable": false,
	"lease_duration": 86400,
	"data": {
		"api_products": "<APIGEE_API_PRODUCTS>",
		"app_name": "<APIGEE_APP_NAME>",
		"credentials": "RkRJTUdqbXJ1dDRmY2hTdUdKaEZETVZhNDAwN2MwM3NXQThEVEpobnJ3NTk3MmkzOkp2x...",
		"developer_email": "<APIGEE_DEVELOPER_EMAIL>",
		"key": "FDIMGjmrut4fchSuGJhFDMVa4007c03sWA8DTJhnrw5972i3",
		"org_name": "<APIGEE_ORG_NAME>",
		"secret": "JvmsfZaajNoqT6Ei7XAYmSTsTA8APSWdu9JxYKtZmEonZ862jKg3ROluxr6Bb710"
	},
	"wrap_info": null,
	"warnings": null,
	"auth": null
}
```

Revoke lease (optional)

```
curl --header "X-Vault-Token: <VAULT_TOKEN>" --request POST --data @payload.json \
http://127.0.0.1:8200/v1/sys/leases/revoke
```
```
{
  "lease_id": "<LEASE_ID>"
}
```

## 13. References

- https://github.com/bstraehle/apigee-client-go
- https://developer.hashicorp.com/vault/docs/plugins/plugin-portal
- https://developer.hashicorp.com/vault/tutorials/custom-secrets-engine
- https://www.credential.net/92b59db2-d79e-4f61-810c-3e400d32f887#gs.c1xdhh
- https://www.credly.com/badges/87d28440-b845-4279-b44c-00f1cfdac049
