package apigee

import (
	"errors"
	"fmt"
	"net/http"
)

const maxErrorBody = 1024

// APIError is returned when the management API responds with a non-2xx status.
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	body := e.Body
	if len(body) > maxErrorBody {
		body = body[:maxErrorBody] + "..."
	}
	return fmt.Sprintf("apigee: %s %s: %d %s: %s", e.Method, e.Path, e.StatusCode, http.StatusText(e.StatusCode), body)
}

// IsNotFound reports whether err is an APIError with status 404 Not Found.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}
