# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-20

### Added

- Initial release
- `Validate` method for server-side email validation
- `FormValidate` method for frontend email validation
- `Account` method to retrieve account info
- Result predicates: `IsValid`, `IsInvalid`, `IsRisky`, `IsUnknown`, `IsFreeEmail`, `IsRole`, `IsDisposable`
- `AllowRisky` option for `IsValid`
- Automatic retry with exponential backoff on 429 and 5xx errors
- Functional options: `WithBaseURL`, `WithTimeout`, `WithMaxRetries`, `WithFormAPIKey`, `WithHTTPClient`
- Typed errors: `ErrAuthentication`, `ErrRateLimit`, `ErrAPI`
