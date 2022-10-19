# Vault Plugin: Apigee Secrets Engine

[![Go Reference](https://pkg.go.dev/badge/github.com/bstraehle/vault-plugin-secrets-apigee.svg)](https://pkg.go.dev/github.com/bstraehle/vault-plugin-secrets-apigee) [![Go Report Card](https://goreportcard.com/badge/github.com/bstraehle/vault-plugin-secrets-apigee)](https://goreportcard.com/report/github.com/bstraehle/vault-plugin-secrets-apigee) [![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/bstraehle/vault-plugin-secrets-apigee?color=red&include_prereleases&sort=semver)](https://github.com/bstraehle/vault-plugin-secrets-apigee/releases)

Apigee helps companies design, secure, and scale application programming interfaces (APIs).

Apigee apps contain a consumer key and consumer secret (credentials), which are typically used to obtain an OAuth2 access token for API access. These credentials have an expiry, by default never. For **zero trust security** use cases, instead of apps using static, long-lived credentials, the **Vault Apigee secrets engine** generates dynamic, short-lived credentials, aka **ephemeral credentials**, enabling frequent rotation. For other use cases, the Vault KV secrets engine can be used.

## Table of Contents

1. [Quick Links](#1-quick-links)
2. [Use Case](#2-use-case)
3. [Examples](#3-examples)
4. [Prerequisites](#4-prerequisites)
5. [Configure Access](#5-configure-access)
6. [Configure Environment](#6-configure-environment)
7. [Build/Test or Get Binary](#7-buildtest-or-get-binary)
8. [Register Plugin](#8-register-plugin)
9. [Enable Secrets Engine](#9-enable-secrets-engine)
10. [Write Config](#10-write-config)
11. [Write Role](#11-write-role)
12. [Usage: Vault CLI](#12-usage-vault-cli)
13. [Usage: Vault API](#13-usage-vault-api)

## 1. Quick Links

- [Apigee Client Library](https://github.com/bstraehle/apigee-client-go)
- [Vault Custom Secrets Engines](https://developer.hashicorp.com/vault/tutorials/custom-secrets-engine)
- [Vault Plugin Portal](https://developer.hashicorp.com/vault/docs/v1.11.x/plugins/plugin-portal)
- [HashiCorp Certified Vault Associate](https://www.credly.com/badges/87d28440-b845-4279-b44c-00f1cfdac049)
- [Google Certified Professional API Engineer](https://www.credential.net/92b59db2-d79e-4f61-810c-3e400d32f887#gs.c1xdhh)

## 2. Use Case

![HashiCorp Vault custom secrets engine for Apigee](https://github.com/bstraehle/vault-plugin-secrets-apigee/blob/master/sequence-diagram.png)

## 3. Examples

Linux, Vault API, and Apigee X

![Apigee X](https://github.com/bstraehle/vault-plugin-secrets-apigee/blob/master/example-apigee-x.png)

Windows, Vault CLI, and Apigee Edge for Private Cloud

![Apigee Edge](https://github.com/bstraehle/vault-plugin-secrets-apigee/blob/master/example-apigee-edge.png)

## 4. Prerequisites

- [Git](https://git-scm.com/downloads) (Optional)
- [Go](https://go.dev/dl/) (Optional)
- [HashiCorp Vault](https://www.vaultproject.io/downloads)
- [Apigee X](https://cloud.google.com/apigee/docs/) and [Google Cloud CLI](https://cloud.google.com/sdk/docs/install) or
- [Apigee Edge](https://docs.apigee.com/)

## 5. Configure Access

```
gcloud auth login
export APIGEE_OAUTH_TOKEN=$(gcloud auth print-access-token)
```

or

```
export APIGEE_USERNAME=<APIGEE_USERNAME>
export APIGEE_PASSWORD=<APIGEE_PASSWORD>
```

## 6. Configure Environment

```
export APIGEE_HOST=<APIGEE_HOST>
export APIGEE_ORG_NAME=<APIGEE_ORG_NAME>
export APIGEE_DEVELOPER_EMAIL=<APIGEE_DEVELOPER_EMAIL>
export APIGEE_APP_NAME=<APIGEE_APP_NAME>
export APIGEE_API_PRODUCTS=[\"<APIGEE_API_PRODUCT>\"]
```

## 7. Build/Test or Get Binary

```
git clone https://github.com/bstraehle/vault-plugin-secrets-apigee.git
cd vault-plugin-secrets-apigee

go build -o vault/plugins/vault-plugin-secrets-apigee cmd/vault-plugin-secrets-apigee/main.go
go test -v
```

or

```
cd vault/plugins
wget https://github.com/bstraehle/vault-plugin-secrets-apigee/releases/download/v1.1.2/vault-plugin-secrets-apigee-linux-amd64
mv vault-plugin-secrets-apigee-linux-amd64 vault-plugin-secrets-apigee
chmod 700 vault-plugin-secrets-apigee
```

## 8. Register Plugin

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

## 9. Enable Secrets Engine

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

## 10. Write Config

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

## 11. Write Role

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

## 12. Usage: Vault CLI

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

## 13. Usage: Vault API

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
