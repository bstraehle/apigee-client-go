//go:build integration

package apigee

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestIntegration creates and deletes credentials against a real Apigee
// organization. Run with: go test -tags integration -run Integration -v
//
// Apigee X / hybrid: APIGEE_OAUTH_TOKEN (and optionally APIGEE_HOST).
// Apigee Edge: APIGEE_HOST, APIGEE_USERNAME, APIGEE_PASSWORD.
// Both: APIGEE_ORG_NAME, APIGEE_DEVELOPER_EMAIL, APIGEE_APP_NAME, and
// APIGEE_API_PRODUCTS (comma-separated).
func TestIntegration(t *testing.T) {
	env := func(k string) string {
		v := os.Getenv(k)
		if v == "" {
			t.Skipf("%s not set", k)
		}
		return v
	}

	opts := []Option{WithHost(os.Getenv("APIGEE_HOST"))}
	if token := os.Getenv("APIGEE_OAUTH_TOKEN"); token != "" {
		opts = append(opts, WithOAuthToken(token))
	} else {
		opts = append(opts, WithBasicAuth(env("APIGEE_USERNAME"), env("APIGEE_PASSWORD")))
	}
	c, err := New(opts...)
	if err != nil {
		t.Fatal(err)
	}

	in := CreateCredentialsInput{
		Org:            env("APIGEE_ORG_NAME"),
		DeveloperEmail: env("APIGEE_DEVELOPER_EMAIL"),
		App:            env("APIGEE_APP_NAME"),
		APIProducts:    strings.Split(env("APIGEE_API_PRODUCTS"), ","),
		ExpiresIn:      10 * time.Minute,
	}
	ctx := context.Background()

	creds, err := c.CreateCredentials(ctx, in)
	if err != nil {
		t.Fatalf("CreateCredentials() error = %v", err)
	}
	if creds.Key == "" || creds.Secret == "" {
		t.Fatal("empty key or secret")
	}

	if err := c.DeleteCredentials(ctx, in.Org, in.DeveloperEmail, in.App, creds.Key); err != nil {
		t.Fatalf("DeleteCredentials() error = %v", err)
	}
	if err := c.DeleteCredentials(ctx, in.Org, in.DeveloperEmail, in.App, creds.Key); !IsNotFound(err) {
		t.Fatalf("second DeleteCredentials() error = %v, want not found", err)
	}
}
