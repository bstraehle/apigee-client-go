package apigee

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	envVarHost       = "APIGEE_HOST"
	envVarOAuthToken = "APIGEE_OAUTH_TOKEN"
	envVarUsername   = "APIGEE_USERNAME"
	envVarPassword   = "APIGEE_PASSWORD"

	envVarOrgName        = "APIGEE_ORG_NAME"
	envVarDeveloperEmail = "APIGEE_DEVELOPER_EMAIL"
	envVarAppName        = "APIGEE_APP_NAME"
	envVarApiProducts    = "APIGEE_API_PRODUCTS"
)

var key string

func TestNewClient(t *testing.T) {
	c, err := NewClient(
		os.Getenv(envVarHost),
		os.Getenv(envVarOAuthToken),
		os.Getenv(envVarUsername),
		os.Getenv(envVarPassword))

	require.NotNil(t, c)
	require.NotEmpty(t, c.Host)
	require.Nil(t, err)
}

func TestCreateCredentials(t *testing.T) {
	c, _ := NewClient(
		os.Getenv(envVarHost),
		os.Getenv(envVarOAuthToken),
		os.Getenv(envVarUsername),
		os.Getenv(envVarPassword))

	ccr, err := c.CreateCredentials(
		os.Getenv(envVarOrgName),
		os.Getenv(envVarDeveloperEmail),
		os.Getenv(envVarAppName),
		os.Getenv(envVarApiProducts),
		86400)

	if ccr != nil {
		key = ccr.Key
	}

	require.Nil(t, err)
	require.NotNil(t, ccr)
	require.NotEmpty(t, ccr.Key)
	require.NotEmpty(t, ccr.Secret)
}

func TestDeleteCredentials(t *testing.T) {
	c, _ := NewClient(
		os.Getenv(envVarHost),
		os.Getenv(envVarOAuthToken),
		os.Getenv(envVarUsername),
		os.Getenv(envVarPassword))

	err := c.DeleteCredentials(
		os.Getenv(envVarOrgName),
		os.Getenv(envVarDeveloperEmail),
		os.Getenv(envVarAppName),
		key)

	require.Nil(t, err)
}
