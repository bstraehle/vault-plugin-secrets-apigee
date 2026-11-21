package secretsengine

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"time"

	apigee "github.com/bstraehle/apigee-client-go/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// tokenHTTPTimeout bounds service account token exchanges. The token source
// is shared by all requests on a mount, so a stalled exchange must not hang.
var tokenHTTPTimeout = 30 * time.Second

// newClient builds an Apigee client from the configuration.
func (c *apigeeConfig) newClient() (*apigee.Client, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	opts := []apigee.Option{
		apigee.WithHost(c.Host),
		apigee.WithUserAgent(userAgent()),
	}

	switch c.authType() {
	case authTypeOAuthToken:
		opts = append(opts, apigee.WithOAuthToken(c.OAuthToken))
	case authTypeBasic:
		opts = append(opts, apigee.WithBasicAuth(c.Username, c.Password))
	case authTypeServiceAccount:
		if err := validateServiceAccount(c.Credentials); err != nil {
			return nil, fmt.Errorf("invalid service account credentials: %w", err)
		}
		// The token source outlives the request, so it must not use the request
		// context; the HTTP client's timeout bounds each token exchange instead.
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Timeout: tokenHTTPTimeout})
		creds, err := google.CredentialsFromJSONWithType(ctx, []byte(c.Credentials), google.ServiceAccount, cloudPlatformScope)
		if err != nil {
			return nil, fmt.Errorf("invalid service account credentials: %w", err)
		}
		opts = append(opts, apigee.WithTokenSource(apigee.TokenSourceFunc(func(context.Context) (string, error) {
			token, err := creds.TokenSource.Token()
			if err != nil {
				return "", err
			}
			return token.AccessToken, nil
		})))
	default:
		return nil, errors.New("no authentication method configured")
	}

	return apigee.New(opts...)
}

// validateServiceAccount checks the fields required for the JWT token exchange
// and parses the private key, which the oauth2 library only does on first use.
func validateServiceAccount(raw string) error {
	var sa struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}
	if err := json.Unmarshal([]byte(raw), &sa); err != nil {
		return err
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return errors.New("client_email and private_key are required")
	}
	key := []byte(sa.PrivateKey)
	if block, _ := pem.Decode(key); block != nil {
		key = block.Bytes
	}
	parsed, err := x509.ParsePKCS8PrivateKey(key)
	if err != nil {
		if parsed, err = x509.ParsePKCS1PrivateKey(key); err != nil {
			return errors.New("private_key must be a PEM or plain PKCS #1 or PKCS #8 RSA key")
		}
	}
	if _, ok := parsed.(*rsa.PrivateKey); !ok {
		return errors.New("private_key must be an RSA key")
	}
	return nil
}

func userAgent() string {
	if Version == "" {
		return "vault-plugin-secrets-apigee"
	}
	return "vault-plugin-secrets-apigee/" + Version
}
