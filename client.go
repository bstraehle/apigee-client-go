// Package apigee is a minimal client for the Apigee management API (Apigee X,
// Apigee hybrid, and Apigee Edge) that creates and deletes developer app
// credentials, i.e. consumer key and consumer secret pairs.
//
// It is purpose-built for the Vault Apigee secrets engine
// (https://github.com/bstraehle/vault-plugin-secrets-apigee).
package apigee

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultHost is the Apigee X / hybrid management API endpoint.
const DefaultHost = "https://apigee.googleapis.com"

const (
	defaultTimeout   = 30 * time.Second
	defaultUserAgent = "apigee-client-go/v2"
	maxResponseBytes = 10 << 20
)

// ErrNoAuth is returned by New when no authentication option is supplied.
var ErrNoAuth = errors.New("apigee: no authentication configured, use WithOAuthToken, WithTokenSource, or WithBasicAuth")

// TokenSource supplies an OAuth 2.0 access token for each request. It lets
// callers plug in refreshing token sources (e.g. golang.org/x/oauth2/google).
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// TokenSourceFunc adapts a function to the TokenSource interface.
type TokenSourceFunc func(ctx context.Context) (string, error)

// Token implements TokenSource.
func (f TokenSourceFunc) Token(ctx context.Context) (string, error) { return f(ctx) }

// Client is an Apigee management API client. It is safe for concurrent use.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	userAgent  string
	authorize  func(ctx context.Context, req *http.Request) error
	authCount  int
}

// Option configures a Client.
type Option func(*Client) error

// WithHost sets the management API base URL, e.g. https://api.enterprise.apigee.com
// for Apigee Edge. Defaults to DefaultHost.
func WithHost(host string) Option {
	return func(c *Client) error {
		if host == "" {
			return nil
		}
		u, err := url.Parse(strings.TrimRight(host, "/"))
		if err != nil {
			return fmt.Errorf("apigee: invalid host %q: %w", host, err)
		}
		if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("apigee: invalid host %q: must be an absolute http(s) URL", host)
		}
		c.baseURL = u
		return nil
	}
}

// WithHTTPClient sets the HTTP client used for requests.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc == nil {
			return errors.New("apigee: http client must not be nil")
		}
		c.httpClient = hc
		return nil
	}
}

// WithUserAgent sets the User-Agent header sent with each request.
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		c.userAgent = ua
		return nil
	}
}

// WithOAuthToken authenticates with a static OAuth 2.0 access token (Apigee X / hybrid).
func WithOAuthToken(token string) Option {
	return func(c *Client) error {
		if token == "" {
			return errors.New("apigee: oauth token must not be empty")
		}
		return WithTokenSource(TokenSourceFunc(func(context.Context) (string, error) { return token, nil }))(c)
	}
}

// WithTokenSource authenticates with OAuth 2.0 access tokens obtained from ts
// for each request (Apigee X / hybrid).
func WithTokenSource(ts TokenSource) Option {
	return func(c *Client) error {
		if ts == nil {
			return errors.New("apigee: token source must not be nil")
		}
		c.authCount++
		c.authorize = func(ctx context.Context, req *http.Request) error {
			token, err := ts.Token(ctx)
			if err != nil {
				return fmt.Errorf("apigee: obtaining access token: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			return nil
		}
		return nil
	}
}

// WithBasicAuth authenticates with a username and password (Apigee Edge).
func WithBasicAuth(username, password string) Option {
	return func(c *Client) error {
		if username == "" || password == "" {
			return errors.New("apigee: username and password must not be empty")
		}
		c.authCount++
		c.authorize = func(_ context.Context, req *http.Request) error {
			req.SetBasicAuth(username, password)
			return nil
		}
		return nil
	}
}

// New returns a Client configured by opts. Exactly one authentication option
// (WithOAuthToken, WithTokenSource, or WithBasicAuth) is required.
func New(opts ...Option) (*Client, error) {
	base, _ := url.Parse(DefaultHost)
	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    base,
		userAgent:  defaultUserAgent,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	switch {
	case c.authCount == 0:
		return nil, ErrNoAuth
	case c.authCount > 1:
		return nil, errors.New("apigee: multiple authentication methods configured, use exactly one")
	}
	return c, nil
}

// Host returns the management API base URL.
func (c *Client) Host() string { return c.baseURL.String() }

// endpoint appends the path-escaped segments to the base URL. Empty and dot
// segments are rejected so names cannot alter the request path.
func (c *Client) endpoint(segments []string) (*url.URL, error) {
	escaped := make([]string, len(segments))
	for i, s := range segments {
		if s == "" || s == "." || s == ".." {
			return nil, fmt.Errorf("apigee: invalid path segment %q", s)
		}
		escaped[i] = url.PathEscape(s)
	}
	u := *c.baseURL
	u.RawPath = strings.TrimRight(c.baseURL.EscapedPath(), "/") + "/" + strings.Join(escaped, "/")
	p, err := url.PathUnescape(u.RawPath)
	if err != nil {
		return nil, fmt.Errorf("apigee: building path: %w", err)
	}
	u.Path = p
	return &u, nil
}

// do sends a request to the path built from the escaped segments, encoding in
// as the JSON body (if non-nil) and decoding the JSON response into out (if non-nil).
func (c *Client) do(ctx context.Context, method string, in, out any, segments ...string) error {
	u, err := c.endpoint(segments)
	if err != nil {
		return err
	}

	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("apigee: encoding request: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return fmt.Errorf("apigee: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if err := c.authorize(ctx, req); err != nil {
		return err
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("apigee: %s %s: %w", method, u.Path, err)
	}
	defer func() { _ = res.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("apigee: reading response: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return &APIError{Method: method, Path: u.Path, StatusCode: res.StatusCode, Body: string(data)}
	}

	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("apigee: decoding response: %w", err)
		}
	}
	return nil
}
