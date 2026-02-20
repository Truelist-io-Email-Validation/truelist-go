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

    fmt.Println(result.State)     // "valid"
    fmt.Println(result.IsValid()) // true
}
```

## Methods

### Validate

Server-side email validation using your server API key.

```go
result, err := client.Validate(ctx, "user@example.com")
```

### FormValidate

Frontend email validation using your form API key. Requires `WithFormAPIKey` to be set.

```go
client := truelist.NewClient("server-key",
    truelist.WithFormAPIKey("your-form-key"),
)

result, err := client.FormValidate(ctx, "user@example.com")
```

### Account

Retrieve account information.

```go
account, err := client.Account(ctx)
fmt.Println(account.Email)   // "you@example.com"
fmt.Println(account.Plan)    // "pro"
fmt.Println(account.Credits) // 9542
```

## Result

The `Result` struct contains the full validation response:

| Field        | Type     | Description                                  |
|-------------|----------|----------------------------------------------|
| `State`      | `string` | `valid`, `invalid`, `risky`, or `unknown`    |
| `SubState`   | `string` | Detailed sub-state (see below)               |
| `Suggestion` | `string` | Suggested correction (e.g., typo fix)        |
| `FreeEmail`  | `bool`   | Whether the email is from a free provider    |
| `Role`       | `bool`   | Whether the email is a role address          |
| `Disposable` | `bool`   | Whether the email is from a disposable provider |

### Sub-States

`ok`, `accept_all`, `disposable_address`, `role_address`, `failed_mx_check`, `failed_spam_trap`, `failed_no_mailbox`, `failed_greylisted`, `failed_syntax_check`, `unknown`

### Result Predicates

```go
result.IsValid()                       // state == "valid"
result.IsValid(truelist.AllowRisky())  // state == "valid" || "risky"
result.IsInvalid()                     // state == "invalid"
result.IsRisky()                       // state == "risky"
result.IsUnknown()                     // state == "unknown"
result.IsFreeEmail()                   // free_email == true
result.IsRole()                        // role == true
result.IsDisposable()                  // disposable == true
```

## Configuration Options

| Option              | Default                       | Description                              |
|--------------------|-------------------------------|------------------------------------------|
| `WithBaseURL`       | `https://api.truelist.io`     | Custom API base URL                      |
| `WithTimeout`       | `30s`                         | HTTP client timeout                      |
| `WithMaxRetries`    | `3`                           | Max retries for transient errors (429, 5xx) |
| `WithFormAPIKey`    | (none)                        | Form API key for `FormValidate`          |
| `WithHTTPClient`    | (default client)              | Custom `*http.Client`                    |

```go
client := truelist.NewClient("your-api-key",
    truelist.WithBaseURL("https://api.truelist.io"),
    truelist.WithTimeout(10 * time.Second),
    truelist.WithMaxRetries(2),
    truelist.WithFormAPIKey("your-form-key"),
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
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    truelist "github.com/Truelist-io-Email-Validation/truelist-go"
)

func TestMyValidation(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(truelist.Result{
            State:    truelist.StateValid,
            SubState: truelist.SubStateOK,
        })
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
