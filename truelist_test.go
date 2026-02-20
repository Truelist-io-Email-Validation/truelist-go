package truelist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := NewClient("test-api-key",
		WithBaseURL(srv.URL),
		WithMaxRetries(0),
	)
	return srv, client
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		response Result
		wantErr  bool
	}{
		{
			name: "valid email",
			response: Result{
				State:      StateValid,
				SubState:   SubStateOK,
				FreeEmail:  true,
				Role:       false,
				Disposable: false,
			},
		},
		{
			name: "invalid email",
			response: Result{
				State:    StateInvalid,
				SubState: SubStateFailedNoMailbox,
			},
		},
		{
			name: "risky email",
			response: Result{
				State:    StateRisky,
				SubState: SubStateAcceptAll,
			},
		},
		{
			name: "unknown email",
			response: Result{
				State:    StateUnknown,
				SubState: SubStateUnknown,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.URL.Path != "/api/v1/verify" {
					t.Errorf("expected /api/v1/verify, got %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-api-key" {
					t.Errorf("expected Bearer test-api-key, got %s", r.Header.Get("Authorization"))
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
				}

				var req verifyRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}
				if req.Email != "user@example.com" {
					t.Errorf("expected user@example.com, got %s", req.Email)
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tt.response)
			})

			result, err := client.Validate(context.Background(), "user@example.com")
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if result.State != tt.response.State {
				t.Errorf("State = %q, want %q", result.State, tt.response.State)
			}
			if result.SubState != tt.response.SubState {
				t.Errorf("SubState = %q, want %q", result.SubState, tt.response.SubState)
			}
			if result.FreeEmail != tt.response.FreeEmail {
				t.Errorf("FreeEmail = %v, want %v", result.FreeEmail, tt.response.FreeEmail)
			}
			if result.Role != tt.response.Role {
				t.Errorf("Role = %v, want %v", result.Role, tt.response.Role)
			}
			if result.Disposable != tt.response.Disposable {
				t.Errorf("Disposable = %v, want %v", result.Disposable, tt.response.Disposable)
			}
		})
	}
}

func TestFormValidate(t *testing.T) {
	t.Run("uses form API key", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/form_verify" {
				t.Errorf("expected /api/v1/form_verify, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer form-test-key" {
				t.Errorf("expected Bearer form-test-key, got %s", r.Header.Get("Authorization"))
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Result{State: StateValid, SubState: SubStateOK})
		}))
		defer srv.Close()

		client := NewClient("test-api-key",
			WithBaseURL(srv.URL),
			WithFormAPIKey("form-test-key"),
			WithMaxRetries(0),
		)

		result, err := client.FormValidate(context.Background(), "user@example.com")
		if err != nil {
			t.Fatalf("FormValidate() error = %v", err)
		}
		if result.State != StateValid {
			t.Errorf("State = %q, want %q", result.State, StateValid)
		}
	})

	t.Run("errors without form API key", func(t *testing.T) {
		client := NewClient("test-api-key", WithMaxRetries(0))

		_, err := client.FormValidate(context.Background(), "user@example.com")
		if err == nil {
			t.Fatal("expected error when form API key is not set")
		}
	})
}

func TestAccount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if r.URL.Path != "/api/v1/account" {
				t.Errorf("expected /api/v1/account, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-api-key" {
				t.Errorf("expected Bearer test-api-key, got %s", r.Header.Get("Authorization"))
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Account{
				Email:   "user@example.com",
				Plan:    "pro",
				Credits: 9542,
			})
		})

		account, err := client.Account(context.Background())
		if err != nil {
			t.Fatalf("Account() error = %v", err)
		}
		if account.Email != "user@example.com" {
			t.Errorf("Email = %q, want %q", account.Email, "user@example.com")
		}
		if account.Plan != "pro" {
			t.Errorf("Plan = %q, want %q", account.Plan, "pro")
		}
		if account.Credits != 9542 {
			t.Errorf("Credits = %d, want %d", account.Credits, 9542)
		}
	})

	t.Run("authentication error", func(t *testing.T) {
		_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Invalid API key"}`))
		})

		_, err := client.Account(context.Background())
		if err == nil {
			t.Fatal("expected error for 401 response")
		}
		if !errors.Is(err, ErrAuthentication) {
			t.Errorf("expected ErrAuthentication, got %v", err)
		}
	})
}

func TestErrorHandling(t *testing.T) {
	t.Run("401 returns ErrAuthentication", func(t *testing.T) {
		_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
		})

		_, err := client.Validate(context.Background(), "user@example.com")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrAuthentication) {
			t.Errorf("expected ErrAuthentication, got %v", err)
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatal("expected *APIError")
		}
		if apiErr.StatusCode != 401 {
			t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
		}
	})

	t.Run("429 returns ErrRateLimit", func(t *testing.T) {
		_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Rate limit exceeded"}`))
		})

		_, err := client.Validate(context.Background(), "user@example.com")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrRateLimit) {
			t.Errorf("expected ErrRateLimit, got %v", err)
		}
	})

	t.Run("500 returns ErrAPI", func(t *testing.T) {
		_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"Internal Server Error"}`))
		})

		_, err := client.Validate(context.Background(), "user@example.com")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrAPI) {
			t.Errorf("expected ErrAPI, got %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(5 * time.Second)
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := client.Validate(ctx, "user@example.com")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRetryBehavior(t *testing.T) {
	t.Run("retries on 429", func(t *testing.T) {
		var attempts int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			if n <= 2 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"Rate limit"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Result{State: StateValid, SubState: SubStateOK})
		}))
		defer srv.Close()

		client := NewClient("test-api-key",
			WithBaseURL(srv.URL),
			WithMaxRetries(3),
		)

		result, err := client.Validate(context.Background(), "user@example.com")
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if result.State != StateValid {
			t.Errorf("State = %q, want %q", result.State, StateValid)
		}
		if atomic.LoadInt32(&attempts) != 3 {
			t.Errorf("attempts = %d, want 3", atomic.LoadInt32(&attempts))
		}
	})

	t.Run("retries on 500", func(t *testing.T) {
		var attempts int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			if n <= 1 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"Server error"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Result{State: StateValid, SubState: SubStateOK})
		}))
		defer srv.Close()

		client := NewClient("test-api-key",
			WithBaseURL(srv.URL),
			WithMaxRetries(2),
		)

		result, err := client.Validate(context.Background(), "user@example.com")
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if result.State != StateValid {
			t.Errorf("State = %q, want %q", result.State, StateValid)
		}
	})

	t.Run("does not retry on 401", func(t *testing.T) {
		var attempts int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"Unauthorized"}`))
		}))
		defer srv.Close()

		client := NewClient("test-api-key",
			WithBaseURL(srv.URL),
			WithMaxRetries(3),
		)

		_, err := client.Validate(context.Background(), "user@example.com")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrAuthentication) {
			t.Errorf("expected ErrAuthentication, got %v", err)
		}
		if atomic.LoadInt32(&attempts) != 1 {
			t.Errorf("attempts = %d, want 1 (should not retry on 401)", atomic.LoadInt32(&attempts))
		}
	})

	t.Run("exhausts retries", func(t *testing.T) {
		var attempts int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Rate limit"}`))
		}))
		defer srv.Close()

		client := NewClient("test-api-key",
			WithBaseURL(srv.URL),
			WithMaxRetries(2),
		)

		_, err := client.Validate(context.Background(), "user@example.com")
		if err == nil {
			t.Fatal("expected error after exhausting retries")
		}
		if !errors.Is(err, ErrRateLimit) {
			t.Errorf("expected ErrRateLimit, got %v", err)
		}
		// 1 initial + 2 retries = 3 attempts
		if atomic.LoadInt32(&attempts) != 3 {
			t.Errorf("attempts = %d, want 3", atomic.LoadInt32(&attempts))
		}
	})
}

func TestOptions(t *testing.T) {
	t.Run("custom base URL", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Result{State: StateValid})
		}))
		defer srv.Close()

		client := NewClient("test-api-key",
			WithBaseURL(srv.URL),
			WithMaxRetries(0),
		)

		result, err := client.Validate(context.Background(), "user@example.com")
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if result.State != StateValid {
			t.Errorf("State = %q, want %q", result.State, StateValid)
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		client := NewClient("test-api-key",
			WithTimeout(5*time.Second),
		)
		if client.httpClient.Timeout != 5*time.Second {
			t.Errorf("Timeout = %v, want %v", client.httpClient.Timeout, 5*time.Second)
		}
	})

	t.Run("custom max retries", func(t *testing.T) {
		client := NewClient("test-api-key",
			WithMaxRetries(5),
		)
		if client.maxRetries != 5 {
			t.Errorf("maxRetries = %d, want 5", client.maxRetries)
		}
	})

	t.Run("custom HTTP client", func(t *testing.T) {
		hc := &http.Client{Timeout: 99 * time.Second}
		client := NewClient("test-api-key",
			WithHTTPClient(hc),
		)
		if client.httpClient != hc {
			t.Error("expected custom HTTP client to be set")
		}
	})

	t.Run("default values", func(t *testing.T) {
		client := NewClient("test-api-key")
		if client.baseURL != defaultBaseURL {
			t.Errorf("baseURL = %q, want %q", client.baseURL, defaultBaseURL)
		}
		if client.maxRetries != defaultMaxRetries {
			t.Errorf("maxRetries = %d, want %d", client.maxRetries, defaultMaxRetries)
		}
		if client.httpClient.Timeout != defaultTimeout {
			t.Errorf("Timeout = %v, want %v", client.httpClient.Timeout, defaultTimeout)
		}
		if client.apiKey != "test-api-key" {
			t.Errorf("apiKey = %q, want %q", client.apiKey, "test-api-key")
		}
	})
}

func TestResultPredicates(t *testing.T) {
	tests := []struct {
		name       string
		result     Result
		isValid    bool
		isInvalid  bool
		isRisky    bool
		isUnknown  bool
		isFree     bool
		isRole     bool
		isDisp     bool
		validRisky bool
	}{
		{
			name: "valid free email",
			result: Result{
				State:     StateValid,
				FreeEmail: true,
			},
			isValid:    true,
			isFree:     true,
			validRisky: true,
		},
		{
			name: "invalid email",
			result: Result{
				State: StateInvalid,
			},
			isInvalid: true,
		},
		{
			name: "risky role email",
			result: Result{
				State: StateRisky,
				Role:  true,
			},
			isRisky:    true,
			isRole:     true,
			validRisky: true,
		},
		{
			name: "unknown disposable email",
			result: Result{
				State:      StateUnknown,
				Disposable: true,
			},
			isUnknown: true,
			isDisp:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &tt.result
			if got := r.IsValid(); got != tt.isValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.isValid)
			}
			if got := r.IsValid(AllowRisky()); got != tt.validRisky {
				t.Errorf("IsValid(AllowRisky()) = %v, want %v", got, tt.validRisky)
			}
			if got := r.IsInvalid(); got != tt.isInvalid {
				t.Errorf("IsInvalid() = %v, want %v", got, tt.isInvalid)
			}
			if got := r.IsRisky(); got != tt.isRisky {
				t.Errorf("IsRisky() = %v, want %v", got, tt.isRisky)
			}
			if got := r.IsUnknown(); got != tt.isUnknown {
				t.Errorf("IsUnknown() = %v, want %v", got, tt.isUnknown)
			}
			if got := r.IsFreeEmail(); got != tt.isFree {
				t.Errorf("IsFreeEmail() = %v, want %v", got, tt.isFree)
			}
			if got := r.IsRole(); got != tt.isRole {
				t.Errorf("IsRole() = %v, want %v", got, tt.isRole)
			}
			if got := r.IsDisposable(); got != tt.isDisp {
				t.Errorf("IsDisposable() = %v, want %v", got, tt.isDisp)
			}
		})
	}
}

func TestAPIErrorFormat(t *testing.T) {
	t.Run("with message", func(t *testing.T) {
		err := &APIError{
			StatusCode: 401,
			Message:    "Invalid API key",
			Err:        ErrAuthentication,
		}
		expected := "truelist: authentication failed (status 401: Invalid API key)"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("without message", func(t *testing.T) {
		err := &APIError{
			StatusCode: 500,
			Err:        ErrAPI,
		}
		expected := "truelist: API error (status 500)"
		if err.Error() != expected {
			t.Errorf("Error() = %q, want %q", err.Error(), expected)
		}
	})

	t.Run("unwrap", func(t *testing.T) {
		err := &APIError{
			StatusCode: 429,
			Err:        ErrRateLimit,
		}
		if !errors.Is(err, ErrRateLimit) {
			t.Error("expected errors.Is to match ErrRateLimit")
		}
	})
}

func TestSuggestionField(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Result{
			State:      StateInvalid,
			SubState:   SubStateFailedSyntaxCheck,
			Suggestion: "user@gmail.com",
		})
	})

	result, err := client.Validate(context.Background(), "user@gmial.com")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if result.Suggestion != "user@gmail.com" {
		t.Errorf("Suggestion = %q, want %q", result.Suggestion, "user@gmail.com")
	}
}
