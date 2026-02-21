# truelist-go

Go SDK for the [Truelist.io](https://truelist.io) email validation API.

## Installation

```bash
go get github.com/Truelist-io-Email-Validation/truelist-go
```

Requires Go 1.21 or later.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    truelist "github.com/Truelist-io-Email-Validation/truelist-go"
)

func main() {
    client := truelist.NewClient("your-api-key")

    result, err := client.Validate(context.Background(), "user@example.com")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.State)     // "ok"
    fmt.Println(result.IsValid()) // true
}
```

## Methods

### Validate

Email validation using your API key. Sends a `POST` to `/api/v1/verify_inline` with the email as a query parameter.

```go
result, err := client.Validate(ctx, "user@example.com")
```

### Account

Retrieve account information via `GET /me`.

```go
account, err := client.Account(ctx)
fmt.Println(account.Email)                // "you@example.com"
fmt.Println(account.Name)                 // "Your Name"
fmt.Println(account.Account.PaymentPlan)  // "pro"
```

## Result

The `Result` struct contains the full validation response:

| Field        | Type      | JSON Field         | Description                                  |
|-------------|-----------|-------------------|----------------------------------------------|
| `Email`      | `string`  | `address`          | The email address validated                  |
| `Domain`     | `string`  | `domain`           | The domain of the email                      |
| `Canonical`  | `string`  | `canonical`        | The canonical (local) part of the email      |
| `MxRecord`   | `*string` | `mx_record`        | MX record for the domain                    |
| `FirstName`  | `*string` | `first_name`       | First name associated with the email         |
| `LastName`   | `*string` | `last_name`        | Last name associated with the email          |
| `State`      | `string`  | `email_state`      | `ok`, `email_invalid`, or `accept_all`       |
| `SubState`   | `string`  | `email_sub_state`  | Detailed sub-state (see below)               |
| `VerifiedAt` | `string`  | `verified_at`      | Timestamp of verification                    |
| `Suggestion` | `*string` | `did_you_mean`     | Suggested correction (e.g., typo fix)        |

### Sub-States

`email_ok`, `is_disposable`, `is_role`, `unknown_error`, `failed_smtp_check`

### Result Predicates

```go
result.IsValid()      // State == "ok"
result.IsInvalid()    // State == "email_invalid"
result.IsAcceptAll()  // State == "accept_all"
result.IsDisposable() // SubState == "is_disposable"
result.IsRole()       // SubState == "is_role"
```

## Account

The `Account` struct contains account information:

| Field         | Type          | Description                    |
|--------------|---------------|--------------------------------|
| `Email`       | `string`      | Account email                  |
| `Name`        | `string`      | Account holder name            |
| `UUID`        | `string`      | Account UUID                   |
| `TimeZone`    | `string`      | Account time zone              |
| `IsAdminRole` | `bool`        | Whether the user is an admin   |
| `Account`     | `AccountInfo` | Nested account details         |

The `AccountInfo` struct:

| Field         | Type     | Description          |
|--------------|----------|----------------------|
| `Name`        | `string` | Organization name    |
| `PaymentPlan` | `string` | Current payment plan |

## Configuration Options

| Option              | Default                       | Description                              |
|--------------------|-------------------------------|------------------------------------------|
| `WithBaseURL`       | `https://api.truelist.io`     | Custom API base URL                      |
| `WithTimeout`       | `30s`                         | HTTP client timeout                      |
| `WithMaxRetries`    | `3`                           | Max retries for transient errors (429, 5xx) |
| `WithHTTPClient`    | (default client)              | Custom `*http.Client`                    |

```go
client := truelist.NewClient("your-api-key",
    truelist.WithBaseURL("https://api.truelist.io"),
    truelist.WithTimeout(10 * time.Second),
    truelist.WithMaxRetries(2),
)
```

## Error Handling

The SDK provides typed errors that can be checked with `errors.Is`:

```go
result, err := client.Validate(ctx, "user@example.com")
if err != nil {
    if errors.Is(err, truelist.ErrAuthentication) {
        // Invalid API key (401) - never retried
    }
    if errors.Is(err, truelist.ErrRateLimit) {
        // Rate limit exceeded (429) - retried up to MaxRetries
    }
    if errors.Is(err, truelist.ErrAPI) {
        // Other API error (5xx) - retried up to MaxRetries
    }
}
```

For more details, use `errors.As` to access the `*APIError`:

```go
var apiErr *truelist.APIError
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.StatusCode) // 429
    fmt.Println(apiErr.Message)    // response body
}
```

### Retry Behavior

- **401 errors** are never retried and always return `ErrAuthentication`.
- **429 and 5xx errors** are retried with exponential backoff up to `MaxRetries` times.
- Set `WithMaxRetries(0)` to disable retries entirely.

## Testing

Run the tests:

```bash
go test -race -v ./...
```

### Mocking the Client

For testing your own code, create a test server with `net/http/httptest`:

```go
package myapp

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    truelist "github.com/Truelist-io-Email-Validation/truelist-go"
)

func TestMyValidation(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"emails":[{"address":"user@example.com","domain":"example.com","canonical":"user","mx_record":null,"first_name":null,"last_name":null,"email_state":"ok","email_sub_state":"email_ok","verified_at":"2026-02-21T10:00:00.000Z","did_you_mean":null}]}`))
    }))
    defer srv.Close()

    client := truelist.NewClient("test-key",
        truelist.WithBaseURL(srv.URL),
        truelist.WithMaxRetries(0),
    )

    result, err := client.Validate(context.Background(), "user@example.com")
    if err != nil {
        t.Fatal(err)
    }
    if !result.IsValid() {
        t.Error("expected valid result")
    }
}
```

## License

MIT
