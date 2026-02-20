// Package truelist provides a Go client for the Truelist.io email validation API.
//
// Create a client with your API key and use it to validate emails:
//
//	client := truelist.NewClient("your-api-key")
//	result, err := client.Validate(ctx, "user@example.com")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.IsValid())
package truelist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const (
	defaultBaseURL    = "https://api.truelist.io"
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 3
)

// Client is the Truelist API client.
type Client struct {
	apiKey     string
	formAPIKey string
	baseURL    string
	maxRetries int
	httpClient *http.Client
}

// Option configures the Client.
type Option func(*Client)

// WithBaseURL sets a custom base URL for the API.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithMaxRetries sets the maximum number of retries for transient errors (429, 5xx).
// Set to 0 to disable retries. Default is 3.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		c.maxRetries = n
	}
}

// WithFormAPIKey sets the form API key for frontend validation.
func WithFormAPIKey(key string) Option {
	return func(c *Client) {
		c.formAPIKey = key
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// NewClient creates a new Truelist API client with the given server API key.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		maxRetries: defaultMaxRetries,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type verifyRequest struct {
	Email string `json:"email"`
}

// Validate performs server-side email validation using the server API key.
func (c *Client) Validate(ctx context.Context, email string) (*Result, error) {
	return c.validate(ctx, "/api/v1/verify", c.apiKey, email)
}

// FormValidate performs frontend email validation using the form API key.
// A form API key must be set via WithFormAPIKey when creating the client.
func (c *Client) FormValidate(ctx context.Context, email string) (*Result, error) {
	if c.formAPIKey == "" {
		return nil, fmt.Errorf("truelist: form API key not set; use WithFormAPIKey when creating the client")
	}
	return c.validate(ctx, "/api/v1/form_verify", c.formAPIKey, email)
}

func (c *Client) validate(ctx context.Context, path, token, email string) (*Result, error) {
	body, err := json.Marshal(verifyRequest{Email: email})
	if err != nil {
		return nil, fmt.Errorf("truelist: failed to marshal request: %w", err)
	}

	respBody, err := c.doWithRetry(ctx, http.MethodPost, path, token, body)
	if err != nil {
		return nil, err
	}

	var result Result
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("truelist: failed to decode response: %w", err)
	}
	return &result, nil
}

// Account retrieves account information using the server API key.
func (c *Client) Account(ctx context.Context) (*Account, error) {
	respBody, err := c.doWithRetry(ctx, http.MethodGet, "/api/v1/account", c.apiKey, nil)
	if err != nil {
		return nil, err
	}

	var account Account
	if err := json.Unmarshal(respBody, &account); err != nil {
		return nil, fmt.Errorf("truelist: failed to decode response: %w", err)
	}
	return &account, nil
}

func (c *Client) doWithRetry(ctx context.Context, method, path, token string, body []byte) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		respBody, err := c.do(ctx, method, path, token, body)
		if err == nil {
			return respBody, nil
		}

		// Auth errors are never retried.
		var apiErr *APIError
		if isAPIError(err, &apiErr) && apiErr.StatusCode == 401 {
			return nil, err
		}

		// Only retry transient errors (429, 5xx).
		if isAPIError(err, &apiErr) && isTransient(apiErr.StatusCode) {
			lastErr = err
			continue
		}

		return nil, err
	}

	return nil, lastErr
}

func (c *Client) do(ctx context.Context, method, path, token string, body []byte) ([]byte, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("truelist: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("truelist: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("truelist: failed to read response: %w", err)
	}

	if resp.StatusCode == 401 {
		return nil, &APIError{
			StatusCode: 401,
			Message:    string(respBody),
			Err:        ErrAuthentication,
		}
	}

	if resp.StatusCode == 429 {
		return nil, &APIError{
			StatusCode: 429,
			Message:    string(respBody),
			Err:        ErrRateLimit,
		}
	}

	if resp.StatusCode >= 400 {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
			Err:        ErrAPI,
		}
	}

	return respBody, nil
}

func isAPIError(err error, target **APIError) bool {
	var apiErr *APIError
	if asOK := errorAs(err, &apiErr); asOK {
		*target = apiErr
		return true
	}
	return false
}

// errorAs is a thin wrapper so we can avoid import cycle issues with errors.As.
func errorAs(err error, target interface{}) bool {
	switch t := target.(type) {
	case **APIError:
		for err != nil {
			if e, ok := err.(*APIError); ok {
				*t = e
				return true
			}
			unwrapper, ok := err.(interface{ Unwrap() error })
			if !ok {
				return false
			}
			err = unwrapper.Unwrap()
		}
		return false
	default:
		return false
	}
}

func isTransient(statusCode int) bool {
	return statusCode == 429 || statusCode >= 500
}
