package apigee

import (
	b64 "encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"
)

const Host string = "https://apigee.googleapis.com"

type Client struct {
	HTTPClient *http.Client
	Host       string
	OAuthToken string
	Username   string
	Password   string
}

func NewClient(host, oAuthToken string, username string, password string) (*Client, error) {
	c := Client{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Host:       Host,
	}

	if host != "" {
		c.Host = host
	}

	if oAuthToken == "" {
		if username == "" || password == "" {
			return nil, fmt.Errorf("define oAuthToken or username and password")
		} else {
			c.Username = username
			c.Password = password
		}
	} else {
		c.OAuthToken = oAuthToken
	}

	return &c, nil
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	if c.Username != "" && c.Password != "" {
		creds := fmt.Sprintf("%s:%s", c.Username, c.Password)
		b64Creds := b64.StdEncoding.EncodeToString([]byte(creds))
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", b64Creds))
	}

	if c.OAuthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.OAuthToken))
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := c.HTTPClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("status: %d, body: %s", res.StatusCode, body)
	}

	return body, err
}
