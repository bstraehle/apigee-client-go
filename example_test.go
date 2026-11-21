package apigee_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	apigee "github.com/bstraehle/apigee-client-go/v2"
)

func Example() {
	client, err := apigee.New(apigee.WithOAuthToken(os.Getenv("APIGEE_OAUTH_TOKEN")))
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	creds, err := client.CreateCredentials(ctx, apigee.CreateCredentialsInput{
		Org:            "my-org",
		DeveloperEmail: "developer@example.com",
		App:            "my-app",
		APIProducts:    []string{"my-product"},
		ExpiresIn:      time.Hour,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Authorization: Basic " + creds.BasicAuth())

	if err := client.DeleteCredentials(ctx, "my-org", "developer@example.com", "my-app", creds.Key); err != nil && !apigee.IsNotFound(err) {
		log.Fatal(err)
	}
}
