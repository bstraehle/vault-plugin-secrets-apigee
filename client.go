package secretsengine

import (
	"context"
	"errors"
	"fmt"

	apigee "github.com/bstraehle/apigee-client-go/v2"
	"golang.org/x/oauth2/google"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

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
		// The token source outlives the request, so it must not use the request context.
		creds, err := google.CredentialsFromJSONWithType(context.Background(), []byte(c.Credentials), google.ServiceAccount, cloudPlatformScope)
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

func userAgent() string {
	if Version == "" {
		return "vault-plugin-secrets-apigee"
	}
	return "vault-plugin-secrets-apigee/" + Version
}
