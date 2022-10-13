package secretsengine

import (
	"errors"

	apigee "github.com/bstraehle/apigee-client-go"
)

type apigeeClient struct {
	*apigee.Client
}

func newClient(config *apigeeConfig) (*apigeeClient, error) {
	if config == nil {
		return nil, errors.New("client configuration was nil")
	}

	if config.Host == "" {
		return nil, errors.New("client host was not defined")
	}

	if config.OAuthToken == "" {
		if config.Username == "" || config.Password == "" {
			return nil, errors.New("client oauth_token or username and password was not defined")
		}
	}

	c, err := apigee.NewClient(config.Host, config.OAuthToken, config.Username, config.Password)

	if err != nil {
		return nil, err
	}

	return &apigeeClient{c}, nil
}
