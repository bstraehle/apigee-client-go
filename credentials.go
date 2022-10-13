package apigee

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type CreateCredentialsResponse struct {
	Key         string `json:"key"`
	Secret      string `json:"secret"`
	Credentials string `json:"credentials"`
}

type CredentialsArrayApigee struct {
	Credentials []CredentialsApigee `json:"credentials"`
}

type CredentialsApigee struct {
	ConsumerKey    string `json:"consumerKey"`
	ConsumerSecret string `json:"consumerSecret"`
}

func (c *Client) CreateCredentials(orgName string, developerEmail string, appName string, apiProducts string, expiresInSeconds int) (*CreateCredentialsResponse, error) {
	if orgName == "" || developerEmail == "" || appName == "" || apiProducts == "" || expiresInSeconds == 0 {
		return nil, fmt.Errorf("define orgName, developerEmail, appName, apiProducts, and expiresInSeconds")
	}

	body := fmt.Sprintf("{\"keyExpiresIn\":\"%s\",\"apiProducts\":%s}",
		strconv.Itoa(expiresInSeconds*1000), apiProducts)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/organizations/%s/developers/%s/apps/%s",
		c.Host, orgName, developerEmail, appName), strings.NewReader(body))

	if err != nil {
		return nil, err
	}

	res, err := c.doRequest(req)

	if err != nil {
		return nil, err
	}

	key, secret := getCredentials(res)

	credentials := fmt.Sprintf("%s:%s", key, secret)
	b64Credentials := b64.StdEncoding.EncodeToString([]byte(credentials))

	ccr := CreateCredentialsResponse{key, secret, b64Credentials}

	return &ccr, nil
}

func (c *Client) DeleteCredentials(orgName string, developerEmail string, appName string, key string) error {
	if orgName == "" || developerEmail == "" || appName == "" || key == "" {
		return fmt.Errorf("define orgName, developerEmail, appName, and key")
	}

	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/organizations/%s/developers/%s/apps/%s/keys/%s",
		c.Host, orgName, developerEmail, appName, key), strings.NewReader(""))

	if err != nil {
		return err
	}

	_, err = c.doRequest(req)

	if err != nil {
		return err
	}

	return nil
}

func getCredentials(data []byte) (string, string) {
	var caa CredentialsArrayApigee
	json.Unmarshal(data, &caa)
	return caa.Credentials[0].ConsumerKey, caa.Credentials[0].ConsumerSecret
}
