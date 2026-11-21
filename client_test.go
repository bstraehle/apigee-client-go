package apigee

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeApigee is an in-memory Apigee management API serving a single developer app.
type fakeApigee struct {
	t *testing.T

	mu       sync.Mutex
	app      DeveloperApp
	requests []recordedRequest
	nextKey  int
	failWith int
}

type recordedRequest struct {
	Method, RawPath, Authorization, ContentType string
	Body                                        map[string]any
}

func newFakeApigee(t *testing.T) (*fakeApigee, *httptest.Server) {
	t.Helper()
	f := &fakeApigee{
		t: t,
		app: DeveloperApp{
			Name:        "my app",
			CallbackURL: "https://example.com/cb",
			Attributes:  []Attribute{{Name: "Notes", Value: "keep me"}},
			Credentials: []Credential{{ConsumerKey: "existing", ConsumerSecret: "s", IssuedAt: 9_999_999_999_999}},
		},
	}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

const testAppPath = "/v1/organizations/org/developers/dev+1@example.com/apps/my%20app"

func (f *fakeApigee) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	rec := recordedRequest{Method: r.Method, RawPath: r.URL.EscapedPath(), Authorization: r.Header.Get("Authorization"), ContentType: r.Header.Get("Content-Type")}
	if b, _ := io.ReadAll(r.Body); len(b) > 0 {
		if err := json.Unmarshal(b, &rec.Body); err != nil {
			f.t.Errorf("request body is not JSON: %s", b)
		}
	}
	f.requests = append(f.requests, rec)

	if f.failWith != 0 {
		http.Error(w, `{"error":{"message":"boom"}}`, f.failWith)
		return
	}

	switch {
	case rec.RawPath == testAppPath && r.Method == http.MethodGet:
	case rec.RawPath == testAppPath && r.Method == http.MethodPost:
		f.nextKey++
		f.app.Credentials = append(f.app.Credentials, Credential{
			ConsumerKey:    "key-" + strconv.Itoa(f.nextKey),
			ConsumerSecret: "secret-" + strconv.Itoa(f.nextKey),
			IssuedAt:       1_700_000_000_000,
			ExpiresAt:      1_700_000_060_000,
		})
	case strings.HasPrefix(rec.RawPath, testAppPath+"/keys/") && r.Method == http.MethodDelete:
		key := strings.TrimPrefix(rec.RawPath, testAppPath+"/keys/")
		for i, c := range f.app.Credentials {
			if c.ConsumerKey == key {
				f.app.Credentials = append(f.app.Credentials[:i], f.app.Credentials[i+1:]...)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
				return
			}
		}
		http.Error(w, `{"error":{"code":404}}`, http.StatusNotFound)
		return
	default:
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Apigee X encodes timestamps as strings.
	type wire struct {
		ConsumerKey    string `json:"consumerKey"`
		ConsumerSecret string `json:"consumerSecret"`
		IssuedAt       string `json:"issuedAt"`
		ExpiresAt      string `json:"expiresAt"`
	}
	creds := make([]wire, len(f.app.Credentials))
	for i, c := range f.app.Credentials {
		creds[i] = wire{c.ConsumerKey, c.ConsumerSecret, jsonInt(int64(c.IssuedAt)), jsonInt(int64(c.ExpiresAt))}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"name":        f.app.Name,
		"callbackUrl": f.app.CallbackURL,
		"attributes":  f.app.Attributes,
		"credentials": creds,
	})
}

func jsonInt(v int64) string { b, _ := json.Marshal(v); return string(b) }

func testInput() CreateCredentialsInput {
	return CreateCredentialsInput{
		Org:            "org",
		DeveloperEmail: "dev+1@example.com",
		App:            "my app",
		APIProducts:    []string{"product-a", "product-b"},
		ExpiresIn:      time.Hour,
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr string
		host    string
	}{
		{name: "no auth", wantErr: "no authentication"},
		{name: "oauth default host", opts: []Option{WithOAuthToken("t")}, host: DefaultHost},
		{name: "basic auth custom host", opts: []Option{WithBasicAuth("u", "p"), WithHost("https://api.enterprise.apigee.com/")}, host: "https://api.enterprise.apigee.com"},
		{name: "empty host keeps default", opts: []Option{WithOAuthToken("t"), WithHost("")}, host: DefaultHost},
		{name: "multiple auth", opts: []Option{WithOAuthToken("t"), WithBasicAuth("u", "p")}, wantErr: "multiple authentication"},
		{name: "empty token", opts: []Option{WithOAuthToken("")}, wantErr: "must not be empty"},
		{name: "empty password", opts: []Option{WithBasicAuth("u", "")}, wantErr: "must not be empty"},
		{name: "relative host", opts: []Option{WithOAuthToken("t"), WithHost("apigee.googleapis.com")}, wantErr: "invalid host"},
		{name: "nil token source", opts: []Option{WithTokenSource(nil)}, wantErr: "must not be nil"},
		{name: "nil http client", opts: []Option{WithOAuthToken("t"), WithHTTPClient(nil)}, wantErr: "must not be nil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(tt.opts...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("New() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if c.Host() != tt.host {
				t.Errorf("Host() = %q, want %q", c.Host(), tt.host)
			}
		})
	}
}

func TestCreateCredentials(t *testing.T) {
	f, srv := newFakeApigee(t)
	c, err := New(WithHost(srv.URL), WithBasicAuth("user", "pass"))
	if err != nil {
		t.Fatal(err)
	}

	creds, err := c.CreateCredentials(context.Background(), testInput())
	if err != nil {
		t.Fatalf("CreateCredentials() error = %v", err)
	}

	if creds.Key != "key-1" || creds.Secret != "secret-1" {
		t.Errorf("got key/secret %q/%q, want the newly created key-1/secret-1", creds.Key, creds.Secret)
	}
	if want := time.UnixMilli(1_700_000_060_000); !creds.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", creds.ExpiresAt, want)
	}
	if got, want := creds.BasicAuth(), "a2V5LTE6c2VjcmV0LTE="; got != want {
		t.Errorf("BasicAuth() = %q, want %q", got, want)
	}

	if len(f.requests) != 2 {
		t.Fatalf("got %d requests, want GET then POST", len(f.requests))
	}
	get, post := f.requests[0], f.requests[1]
	if get.Method != http.MethodGet || get.RawPath != testAppPath {
		t.Errorf("first request = %s %s, want GET %s", get.Method, get.RawPath, testAppPath)
	}
	if post.Method != http.MethodPost || post.ContentType != "application/json" {
		t.Errorf("second request = %s (%s), want POST application/json", post.Method, post.ContentType)
	}
	if post.Authorization != "Basic dXNlcjpwYXNz" {
		t.Errorf("Authorization = %q, want basic auth", post.Authorization)
	}
	if got := post.Body["keyExpiresIn"]; got != "3600000" {
		t.Errorf("keyExpiresIn = %v, want 3600000", got)
	}
	if got, _ := json.Marshal(post.Body["apiProducts"]); string(got) != `["product-a","product-b"]` {
		t.Errorf("apiProducts = %s", got)
	}
	if got, _ := json.Marshal(post.Body["attributes"]); string(got) != `[{"name":"Notes","value":"keep me"}]` {
		t.Errorf("attributes were not preserved: %s", got)
	}
	if got := post.Body["callbackUrl"]; got != "https://example.com/cb" {
		t.Errorf("callbackUrl was not preserved: %v", got)
	}
}

func TestCreateCredentialsTokenSource(t *testing.T) {
	f, srv := newFakeApigee(t)
	calls := 0
	c, err := New(WithHost(srv.URL), WithTokenSource(TokenSourceFunc(func(context.Context) (string, error) {
		calls++
		return "tok", nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateCredentials(context.Background(), testInput()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("token source called %d times, want once per request", calls)
	}
	for _, r := range f.requests {
		if r.Authorization != "Bearer tok" {
			t.Errorf("Authorization = %q, want bearer token", r.Authorization)
		}
	}
}

func TestCreateCredentialsTokenSourceError(t *testing.T) {
	f, srv := newFakeApigee(t)
	c, _ := New(WithHost(srv.URL), WithTokenSource(TokenSourceFunc(func(context.Context) (string, error) {
		return "", errors.New("expired")
	})))
	_, err := c.CreateCredentials(context.Background(), testInput())
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("error = %v, want token source error", err)
	}
	if len(f.requests) != 0 {
		t.Errorf("sent %d requests without a token", len(f.requests))
	}
}

func TestCreateCredentialsValidation(t *testing.T) {
	c, _ := New(WithOAuthToken("t"), WithHost("http://127.0.0.1:1"))
	for name, mutate := range map[string]func(*CreateCredentialsInput){
		"org":          func(in *CreateCredentialsInput) { in.Org = "" },
		"developer":    func(in *CreateCredentialsInput) { in.DeveloperEmail = "" },
		"app":          func(in *CreateCredentialsInput) { in.App = "" },
		"API product":  func(in *CreateCredentialsInput) { in.APIProducts = nil },
		"expires in":   func(in *CreateCredentialsInput) { in.ExpiresIn = 0 },
		"path segment": func(in *CreateCredentialsInput) { in.App = ".." },
	} {
		t.Run(name, func(t *testing.T) {
			in := testInput()
			mutate(&in)
			_, err := c.CreateCredentials(context.Background(), in)
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("error = %v, want mention of %q", err, name)
			}
		})
	}
}

func TestCreateCredentialsAPIError(t *testing.T) {
	f, srv := newFakeApigee(t)
	f.failWith = http.StatusForbidden
	c, _ := New(WithHost(srv.URL), WithOAuthToken("t"))

	_, err := c.CreateCredentials(context.Background(), testInput())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("error = %v, want *APIError 403", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q does not include response body", err)
	}
	if IsNotFound(err) {
		t.Error("IsNotFound(403) = true")
	}
}

func TestCreateCredentialsNoNewCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"credentials":[{"consumerKey":"existing","consumerSecret":"s","issuedAt":1}]}`))
	}))
	t.Cleanup(srv.Close)
	c, _ := New(WithHost(srv.URL), WithOAuthToken("t"))

	_, err := c.CreateCredentials(context.Background(), testInput())
	if err == nil || !strings.Contains(err.Error(), "did not contain a new credential") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateCredentialsContextCanceled(t *testing.T) {
	_, srv := newFakeApigee(t)
	c, _ := New(WithHost(srv.URL), WithOAuthToken("t"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.CreateCredentials(ctx, testInput()); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestDeleteCredentials(t *testing.T) {
	f, srv := newFakeApigee(t)
	c, _ := New(WithHost(srv.URL), WithOAuthToken("t"))
	ctx := context.Background()
	in := testInput()

	if err := c.DeleteCredentials(ctx, in.Org, in.DeveloperEmail, in.App, "existing"); err != nil {
		t.Fatalf("DeleteCredentials() error = %v", err)
	}
	if got := f.requests[0]; got.Method != http.MethodDelete || got.RawPath != testAppPath+"/keys/existing" {
		t.Errorf("request = %s %s", got.Method, got.RawPath)
	}

	err := c.DeleteCredentials(ctx, in.Org, in.DeveloperEmail, in.App, "existing")
	if !IsNotFound(err) {
		t.Fatalf("second delete error = %v, want not found", err)
	}

	if err := c.DeleteCredentials(ctx, in.Org, in.DeveloperEmail, in.App, ""); err == nil {
		t.Fatal("expected error for empty key")
	}
	if err := c.DeleteCredentials(ctx, in.Org, in.DeveloperEmail, in.App, "a/b"); !IsNotFound(err) {
		t.Fatalf("slash in key must be escaped, got %v", err)
	}
	if last := f.requests[len(f.requests)-1]; last.RawPath != testAppPath+"/keys/a%2Fb" {
		t.Errorf("RawPath = %q, want escaped slash", last.RawPath)
	}
}

func TestMillisUnmarshal(t *testing.T) {
	for in, want := range map[string]Millis{`"1700000000000"`: 1_700_000_000_000, `1700000000000`: 1_700_000_000_000, `"-1"`: -1, `null`: 0, `""`: 0} {
		var m Millis
		if err := json.Unmarshal([]byte(in), &m); err != nil || m != want {
			t.Errorf("Unmarshal(%s) = %d, %v; want %d", in, m, err, want)
		}
	}
	if (Millis(-1)).Time() != (time.Time{}) {
		t.Error("negative Millis should map to zero time")
	}
	var m Millis
	if err := json.Unmarshal([]byte(`"abc"`), &m); err == nil {
		t.Error("expected error for non-numeric timestamp")
	}
}
