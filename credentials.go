package apigee

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Attribute is a name/value pair attached to a developer app.
type Attribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CredentialAPIProduct is an API product associated with a credential.
type CredentialAPIProduct struct {
	Name   string `json:"apiproduct"`
	Status string `json:"status,omitempty"`
}

// Credential is a consumer key/secret pair of a developer app.
type Credential struct {
	ConsumerKey    string                 `json:"consumerKey"`
	ConsumerSecret string                 `json:"consumerSecret"`
	IssuedAt       Millis                 `json:"issuedAt"`
	ExpiresAt      Millis                 `json:"expiresAt"`
	Status         string                 `json:"status,omitempty"`
	APIProducts    []CredentialAPIProduct `json:"apiProducts,omitempty"`
}

// DeveloperApp is the subset of the Apigee developer app resource used by this client.
type DeveloperApp struct {
	Name        string       `json:"name"`
	AppID       string       `json:"appId,omitempty"`
	CallbackURL string       `json:"callbackUrl,omitempty"`
	Attributes  []Attribute  `json:"attributes,omitempty"`
	Credentials []Credential `json:"credentials,omitempty"`
}

// Millis is a timestamp in milliseconds since the Unix epoch. Apigee X encodes
// it as a JSON string, Apigee Edge as a JSON number; both are accepted.
type Millis int64

// UnmarshalJSON implements json.Unmarshaler.
func (m *Millis) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(b, `"`)
	if len(b) == 0 || string(b) == "null" {
		*m = 0
		return nil
	}
	v, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return fmt.Errorf("apigee: invalid millisecond timestamp %q: %w", b, err)
	}
	*m = Millis(v)
	return nil
}

// Time converts m to a time.Time. Zero and negative values (never expires) yield the zero Time.
func (m Millis) Time() time.Time {
	if m <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(m))
}

// CreateCredentialsInput identifies the developer app and the properties of
// the credentials to create.
type CreateCredentialsInput struct {
	Org            string
	DeveloperEmail string
	App            string
	APIProducts    []string
	// ExpiresIn is the lifetime of the credentials; it must be at least one second.
	ExpiresIn time.Duration
}

func (in CreateCredentialsInput) validate() error {
	var errs []error
	if in.Org == "" {
		errs = append(errs, errors.New("org is required"))
	}
	if in.DeveloperEmail == "" {
		errs = append(errs, errors.New("developer email is required"))
	}
	if in.App == "" {
		errs = append(errs, errors.New("app is required"))
	}
	if len(in.APIProducts) == 0 {
		errs = append(errs, errors.New("at least one API product is required"))
	}
	if in.ExpiresIn < time.Second {
		errs = append(errs, errors.New("expires in must be at least 1s"))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("apigee: invalid input: %w", err)
	}
	return nil
}

// Credentials are newly created consumer credentials.
type Credentials struct {
	Key       string
	Secret    string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// BasicAuth returns base64(key:secret), as used in the Authorization header of
// OAuth 2.0 client credentials token requests.
func (c *Credentials) BasicAuth() string {
	return base64.StdEncoding.EncodeToString([]byte(c.Key + ":" + c.Secret))
}

// GetDeveloperApp returns the developer app.
func (c *Client) GetDeveloperApp(ctx context.Context, org, developerEmail, app string) (*DeveloperApp, error) {
	if org == "" || developerEmail == "" || app == "" {
		return nil, errors.New("apigee: org, developer email, and app are required")
	}
	var out DeveloperApp
	if err := c.do(ctx, http.MethodGet, nil, &out, appPath(org, developerEmail, app)...); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateCredentials generates a new consumer key and secret for an existing
// developer app, associated with the given API products and expiring after
// in.ExpiresIn.
//
// Generating a key replaces the app's attributes and callback URL with those in
// the request, so the current values are read first and sent back unchanged.
// The new credential is identified by comparing the keys before and after.
func (c *Client) CreateCredentials(ctx context.Context, in CreateCredentialsInput) (*Credentials, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}

	app, err := c.GetDeveloperApp(ctx, in.Org, in.DeveloperEmail, in.App)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]struct{}, len(app.Credentials))
	for _, cred := range app.Credentials {
		existing[cred.ConsumerKey] = struct{}{}
	}

	attributes := app.Attributes
	if attributes == nil {
		attributes = []Attribute{}
	}
	req := struct {
		APIProducts  []string    `json:"apiProducts"`
		KeyExpiresIn string      `json:"keyExpiresIn"`
		Attributes   []Attribute `json:"attributes"`
		CallbackURL  string      `json:"callbackUrl,omitempty"`
	}{
		APIProducts:  in.APIProducts,
		KeyExpiresIn: strconv.FormatInt(in.ExpiresIn.Milliseconds(), 10),
		Attributes:   attributes,
		CallbackURL:  app.CallbackURL,
	}

	var updated DeveloperApp
	if err := c.do(ctx, http.MethodPost, req, &updated, appPath(in.Org, in.DeveloperEmail, in.App)...); err != nil {
		return nil, err
	}

	var created *Credential
	for i := range updated.Credentials {
		cred := &updated.Credentials[i]
		if _, ok := existing[cred.ConsumerKey]; ok || cred.ConsumerKey == "" {
			continue
		}
		if created == nil || cred.IssuedAt > created.IssuedAt {
			created = cred
		}
	}
	if created == nil {
		return nil, errors.New("apigee: response did not contain a new credential")
	}

	return &Credentials{
		Key:       created.ConsumerKey,
		Secret:    created.ConsumerSecret,
		IssuedAt:  created.IssuedAt.Time(),
		ExpiresAt: created.ExpiresAt.Time(),
	}, nil
}

// DeleteCredentials deletes the consumer key (and its secret) from the
// developer app. Use IsNotFound to detect keys that no longer exist.
func (c *Client) DeleteCredentials(ctx context.Context, org, developerEmail, app, key string) error {
	if org == "" || developerEmail == "" || app == "" || key == "" {
		return errors.New("apigee: org, developer email, app, and key are required")
	}
	return c.do(ctx, http.MethodDelete, nil, nil, append(appPath(org, developerEmail, app), "keys", key)...)
}

func appPath(org, developerEmail, app string) []string {
	return []string{"v1", "organizations", org, "developers", developerEmail, "apps", app}
}
