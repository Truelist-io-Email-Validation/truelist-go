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
		response verifyResponse
		want     Result
		wantErr  bool
	}{
		{
			name: "valid email",
			response: verifyResponse{
				Emails: []Result{
					{
						Email:      "user@example.com",
						Domain:     "example.com",
						Canonical:  "user",
						State:      StateOK,
						SubState:   SubStateEmailOK,
						VerifiedAt: "2026-02-21T10:00:00.000Z",
					},
				},
			},
			want: Result{
				Email:      "user@example.com",
				Domain:     "example.com",
				Canonical:  "user",
				State:      StateOK,
				SubState:   SubStateEmailOK,
				VerifiedAt: "2026-02-21T10:00:00.000Z",
			},
		},
		{
			name: "invalid email",
			response: verifyResponse{
				Emails: []Result{
					{
						Email:    "bad@example.com",
						Domain:   "example.com",
						State:    StateEmailInvalid,
						SubState: SubStateUnknownError,
					},
				},
			},
			want: Result{
				Email:    "bad@example.com",
				Domain:   "example.com",
				State:    StateEmailInvalid,
				SubState: SubStateUnknownError,
			},
		},
		{
			name: "accept_all email",
			response: verifyResponse{
				Emails: []Result{
					{
						Email:    "user@catchall.com",
						Domain:   "catchall.com",
						State:    StateAcceptAll,
						SubState: SubStateEmailOK,
					},
				},
			},
			want: Result{
				Email:    "user@catchall.com",
				Domain:   "catchall.com",
				State:    StateAcceptAll,
				SubState: SubStateEmailOK,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.URL.Path != "/api/v1/verify_inline" {
					t.Errorf("expected /api/v1/verify_inline, got %s", r.URL.Path)
				}
				email := r.URL.Query().Get("email")
				if email != "user@example.com" {
					t.Errorf("expected email=user@example.com, got %s", email)
				}
				if r.Header.Get("Authorization") != "Bearer test-api-key" {
					t.Errorf("expected Bearer test-api-key, got %s", r.Header.Get("Authorization"))
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

			if result.State != tt.want.State {
				t.Errorf("State = %q, want %q", result.State, tt.want.State)
			}
			if result.SubState != tt.want.SubState {
				t.Errorf("SubState = %q, want %q", result.SubState, tt.want.SubState)
			}
			if result.Email != tt.want.Email {
				t.Errorf("Email = %q, want %q", result.Email, tt.want.Email)
			}
			if result.Domain != tt.want.Domain {
				t.Errorf("Domain = %q, want %q", result.Domain, tt.want.Domain)
			}
		})
	}
}

func TestValidateRealAPIFormat(t *testing.T) {
	mockResponse := `{"emails":[{"address":"user@example.com","domain":"example.com","canonical":"user","mx_record":null,"first_name":null,"last_name":null,"email_state":"ok","email_sub_state":"email_ok","verified_at":"2026-02-21T10:00:00.000Z","did_you_mean":null}]}`

	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/verify_inline" {
			t.Errorf("expected /api/v1/verify_inline, got %s", r.URL.Path)
		}
		email := r.URL.Query().Get("email")
		if email != "user@example.com" {
			t.Errorf("expected email=user@example.com, got %s", email)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResponse))
	})

	result, err := client.Validate(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if result.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", result.Email, "user@example.com")
	}
	if result.Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", result.Domain, "example.com")
	}
	if result.Canonical != "user" {
		t.Errorf("Canonical = %q, want %q", result.Canonical, "user")
	}
	if result.MxRecord != nil {
		t.Errorf("MxRecord = %v, want nil", result.MxRecord)
	}
	if result.FirstName != nil {
		t.Errorf("FirstName = %v, want nil", result.FirstName)
	}
	if result.LastName != nil {
		t.Errorf("LastName = %v, want nil", result.LastName)
	}
	if result.State != "ok" {
		t.Errorf("State = %q, want %q", result.State, "ok")
	}
	if result.SubState != "email_ok" {
		t.Errorf("SubState = %q, want %q", result.SubState, "email_ok")
	}
	if result.VerifiedAt != "2026-02-21T10:00:00.000Z" {
		t.Errorf("VerifiedAt = %q, want %q", result.VerifiedAt, "2026-02-21T10:00:00.000Z")
	}
	if result.Suggestion != nil {
		t.Errorf("Suggestion = %v, want nil", result.Suggestion)
	}
	if !result.IsValid() {
		t.Error("IsValid() = false, want true")
	}
}

func TestAccount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockResponse := `{"email":"team@company.com","name":"Team Lead","uuid":"a3828d19-1234-5678-9abc-def012345678","time_zone":"America/New_York","is_admin_role":true,"token":"test_token","api_keys":[],"account":{"name":"Company Inc","payment_plan":"pro","users":[]}}`

		_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if r.URL.Path != "/me" {
				t.Errorf("expected /me, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-api-key" {
				t.Errorf("expected Bearer test-api-key, got %s", r.Header.Get("Authorization"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(mockResponse))
		})

		account, err := client.Account(context.Background())
		if err != nil {
			t.Fatalf("Account() error = %v", err)
		}
		if account.Email != "team@company.com" {
			t.Errorf("Email = %q, want %q", account.Email, "team@company.com")
		}
		if account.Name != "Team Lead" {
			t.Errorf("Name = %q, want %q", account.Name, "Team Lead")
		}
		if account.UUID != "a3828d19-1234-5678-9abc-def012345678" {
			t.Errorf("UUID = %q, want %q", account.UUID, "a3828d19-1234-5678-9abc-def012345678")
		}
		if account.TimeZone != "America/New_York" {
			t.Errorf("TimeZone = %q, want %q", account.TimeZone, "America/New_York")
		}
		if !account.IsAdminRole {
			t.Error("IsAdminRole = false, want true")
		}
		if account.Account.Name != "Company Inc" {
			t.Errorf("Account.Name = %q, want %q", account.Account.Name, "Company Inc")
		}
		if account.Account.PaymentPlan != "pro" {
			t.Errorf("Account.PaymentPlan = %q, want %q", account.Account.PaymentPlan, "pro")
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
		mockResponse := `{"emails":[{"address":"user@example.com","domain":"example.com","canonical":"user","mx_record":null,"first_name":null,"last_name":null,"email_state":"ok","email_sub_state":"email_ok","verified_at":"2026-02-21T10:00:00.000Z","did_you_mean":null}]}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			if n <= 2 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"Rate limit"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(mockResponse))
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
		if result.State != StateOK {
			t.Errorf("State = %q, want %q", result.State, StateOK)
		}
		if atomic.LoadInt32(&attempts) != 3 {
			t.Errorf("attempts = %d, want 3", atomic.LoadInt32(&attempts))
		}
	})

	t.Run("retries on 500", func(t *testing.T) {
		var attempts int32
		mockResponse := `{"emails":[{"address":"user@example.com","domain":"example.com","canonical":"user","mx_record":null,"first_name":null,"last_name":null,"email_state":"ok","email_sub_state":"email_ok","verified_at":"2026-02-21T10:00:00.000Z","did_you_mean":null}]}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			if n <= 1 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"Server error"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(mockResponse))
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
		if result.State != StateOK {
			t.Errorf("State = %q, want %q", result.State, StateOK)
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
		mockResponse := `{"emails":[{"address":"user@example.com","domain":"example.com","canonical":"user","mx_record":null,"first_name":null,"last_name":null,"email_state":"ok","email_sub_state":"email_ok","verified_at":"2026-02-21T10:00:00.000Z","did_you_mean":null}]}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(mockResponse))
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
		if result.State != StateOK {
			t.Errorf("State = %q, want %q", result.State, StateOK)
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
		name      string
		result    Result
		isValid   bool
		isInvalid bool
		isAccAll  bool
		isDisp    bool
		isRole    bool
	}{
		{
			name: "valid email",
			result: Result{
				State:    StateOK,
				SubState: SubStateEmailOK,
			},
			isValid: true,
		},
		{
			name: "invalid email",
			result: Result{
				State:    StateEmailInvalid,
				SubState: SubStateUnknownError,
			},
			isInvalid: true,
		},
		{
			name: "accept all email",
			result: Result{
				State:    StateAcceptAll,
				SubState: SubStateEmailOK,
			},
			isAccAll: true,
		},
		{
			name: "disposable email",
			result: Result{
				State:    StateOK,
				SubState: SubStateIsDisposable,
			},
			isValid: true,
			isDisp:  true,
		},
		{
			name: "role email",
			result: Result{
				State:    StateOK,
				SubState: SubStateIsRole,
			},
			isValid: true,
			isRole:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &tt.result
			if got := r.IsValid(); got != tt.isValid {
				t.Errorf("IsValid() = %v, want %v", got, tt.isValid)
			}
			if got := r.IsInvalid(); got != tt.isInvalid {
				t.Errorf("IsInvalid() = %v, want %v", got, tt.isInvalid)
			}
			if got := r.IsAcceptAll(); got != tt.isAccAll {
				t.Errorf("IsAcceptAll() = %v, want %v", got, tt.isAccAll)
			}
			if got := r.IsDisposable(); got != tt.isDisp {
				t.Errorf("IsDisposable() = %v, want %v", got, tt.isDisp)
			}
			if got := r.IsRole(); got != tt.isRole {
				t.Errorf("IsRole() = %v, want %v", got, tt.isRole)
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
	suggestion := "user@gmail.com"
	mockResponse := `{"emails":[{"address":"user@gmial.com","domain":"gmial.com","canonical":"user","mx_record":null,"first_name":null,"last_name":null,"email_state":"email_invalid","email_sub_state":"unknown_error","verified_at":"2026-02-21T10:00:00.000Z","did_you_mean":"user@gmail.com"}]}`

	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResponse))
	})

	result, err := client.Validate(context.Background(), "user@gmial.com")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if result.Suggestion == nil || *result.Suggestion != suggestion {
		t.Errorf("Suggestion = %v, want %q", result.Suggestion, suggestion)
	}
}
