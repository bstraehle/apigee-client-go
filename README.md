# Apigee Client Library

[![Go Reference](https://pkg.go.dev/badge/github.com/bstraehle/apigee-client-go/v2.svg)](https://pkg.go.dev/github.com/bstraehle/apigee-client-go/v2) [![CI](https://github.com/bstraehle/apigee-client-go/actions/workflows/ci.yml/badge.svg)](https://github.com/bstraehle/apigee-client-go/actions/workflows/ci.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/bstraehle/apigee-client-go)](https://goreportcard.com/report/github.com/bstraehle/apigee-client-go) [![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/bstraehle/apigee-client-go?color=red&include_prereleases&sort=semver)](https://github.com/bstraehle/apigee-client-go/releases)

A dependency-free Go client for the Apigee management API (Apigee X, hybrid, and Edge) that creates and deletes developer app credentials (consumer key and consumer secret). Purpose-built for the [Vault Apigee secrets engine](https://github.com/bstraehle/vault-plugin-secrets-apigee).

## Table of Contents

1. [Install](#1-install)
2. [Usage](#2-usage)
3. [Behavior](#3-behavior)
4. [Development](#4-development)
5. [Migrating from v1](#5-migrating-from-v1)

## 1. Install

```shell
go get github.com/bstraehle/apigee-client-go/v2
```

## 2. Usage

```go
import apigee "github.com/bstraehle/apigee-client-go/v2"

// Apigee X / hybrid: a static token or any refreshing token source.
client, err := apigee.New(apigee.WithOAuthToken(token))

// Apigee Edge: host and basic auth.
client, err := apigee.New(
	apigee.WithHost("https://api.enterprise.apigee.com"),
	apigee.WithBasicAuth(username, password),
)

creds, err := client.CreateCredentials(ctx, apigee.CreateCredentialsInput{
	Org:            "my-org",
	DeveloperEmail: "developer@example.com",
	App:            "my-app",
	APIProducts:    []string{"my-product"},
	ExpiresIn:      time.Hour,
})
// creds.Key, creds.Secret, creds.ExpiresAt, creds.BasicAuth()

err = client.DeleteCredentials(ctx, "my-org", "developer@example.com", "my-app", creds.Key)
if apigee.IsNotFound(err) {
	// already deleted or expired
}
```

To use refreshing Google credentials, adapt an `oauth2.TokenSource`:

```go
ts := googleCreds.TokenSource
client, err := apigee.New(apigee.WithTokenSource(apigee.TokenSourceFunc(func(ctx context.Context) (string, error) {
	t, err := ts.Token()
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
})))
```

Options: `WithHost`, `WithHTTPClient`, `WithUserAgent`, and exactly one of `WithOAuthToken`, `WithTokenSource`, or `WithBasicAuth`.

## 3. Behavior

- **Attributes are preserved.** Apigee's generate-key endpoint replaces the app's attributes and callback URL with those in the request. `CreateCredentials` reads the app first and sends its current attributes and callback URL back unchanged.
- **The new key is identified exactly.** It is the key that wasn't on the app before the request, not whichever credential happens to be listed first.
- **Path segments are escaped**, and empty, `.`, and `..` segments are rejected.
- **Errors**: non-2xx responses return `*apigee.APIError`, which includes the status code and response body; `apigee.IsNotFound` checks for 404. All calls honor `context.Context` cancellation.

## 4. Development

Prerequisites: [Go](https://go.dev/dl/) 1.26+, optionally [golangci-lint](https://golangci-lint.run/) v2.

```shell
git clone https://github.com/bstraehle/apigee-client-go.git
cd apigee-client-go

go test ./...        # unit tests against an in-memory fake Apigee
golangci-lint run
```

Integration tests run against a real Apigee organization and are behind the `integration` build tag:

```shell
# Apigee X
export APIGEE_OAUTH_TOKEN=$(gcloud auth print-access-token)
# or Apigee Edge
export APIGEE_HOST=<APIGEE_HOST> APIGEE_USERNAME=<APIGEE_USERNAME> APIGEE_PASSWORD=<APIGEE_PASSWORD>

export APIGEE_ORG_NAME=<APIGEE_ORG_NAME>
export APIGEE_DEVELOPER_EMAIL=<APIGEE_DEVELOPER_EMAIL>
export APIGEE_APP_NAME=<APIGEE_APP_NAME>
export APIGEE_API_PRODUCTS=<PRODUCT_1>,<PRODUCT_2>

go test -tags integration -run Integration -v ./...
```

A Postman collection for the underlying management API calls is included: `Apigee - HashiCorp Vault.postman_collection.json`.

## 5. Migrating from v1

| v1 | v2 |
| --- | --- |
| `github.com/bstraehle/apigee-client-go` | `github.com/bstraehle/apigee-client-go/v2` |
| `NewClient(host, token, user, pass)` | `New(WithHost(host), WithOAuthToken(token))` or `New(WithHost(host), WithBasicAuth(user, pass))` |
| `CreateCredentials(org, email, app, "[\"p\"]", seconds)` | `CreateCredentials(ctx, CreateCredentialsInput{..., APIProducts: []string{"p"}, ExpiresIn: d})` |
| `ccr.Credentials` | `creds.BasicAuth()` |
| `DeleteCredentials(org, email, app, key)` | `DeleteCredentials(ctx, org, email, app, key)` |
| `Host` constant | `DefaultHost` |
