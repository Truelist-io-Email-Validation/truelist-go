package truelist

import (
	"errors"
	"fmt"
)

var (
	// ErrAuthentication is returned when the API key is invalid or missing.
	ErrAuthentication = errors.New("authentication failed")

	// ErrRateLimit is returned when the API rate limit is exceeded.
	ErrRateLimit = errors.New("rate limit exceeded")

	// ErrAPI is returned for general API errors.
	ErrAPI = errors.New("API error")
)

// APIError wraps an API error with status code and message details.
type APIError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("truelist: %s (status %d: %s)", e.Err.Error(), e.StatusCode, e.Message)
	}
	return fmt.Sprintf("truelist: %s (status %d)", e.Err.Error(), e.StatusCode)
}

func (e *APIError) Unwrap() error {
	return e.Err
}
