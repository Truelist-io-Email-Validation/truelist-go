package truelist

// State constants for email validation results.
const (
	StateValid   = "valid"
	StateInvalid = "invalid"
	StateRisky   = "risky"
	StateUnknown = "unknown"
)

// SubState constants for email validation results.
const (
	SubStateOK                = "ok"
	SubStateAcceptAll         = "accept_all"
	SubStateDisposableAddress = "disposable_address"
	SubStateRoleAddress       = "role_address"
	SubStateFailedMXCheck     = "failed_mx_check"
	SubStateFailedSpamTrap    = "failed_spam_trap"
	SubStateFailedNoMailbox   = "failed_no_mailbox"
	SubStateFailedGreylisted  = "failed_greylisted"
	SubStateFailedSyntaxCheck = "failed_syntax_check"
	SubStateUnknown           = "unknown"
)

// Result represents the response from an email validation request.
type Result struct {
	State      string `json:"state"`
	SubState   string `json:"sub_state"`
	Suggestion string `json:"suggestion"`
	FreeEmail  bool   `json:"free_email"`
	Role       bool   `json:"role"`
	Disposable bool   `json:"disposable"`
}

// ValidOption configures the behavior of IsValid.
type ValidOption func(*validConfig)

type validConfig struct {
	allowRisky bool
}

// AllowRisky makes IsValid return true for both "valid" and "risky" states.
func AllowRisky() ValidOption {
	return func(c *validConfig) {
		c.allowRisky = true
	}
}

// IsValid returns true if the email state is "valid".
// Pass AllowRisky() to also accept "risky" as valid.
func (r *Result) IsValid(opts ...ValidOption) bool {
	cfg := &validConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.allowRisky {
		return r.State == StateValid || r.State == StateRisky
	}
	return r.State == StateValid
}

// IsInvalid returns true if the email state is "invalid".
func (r *Result) IsInvalid() bool {
	return r.State == StateInvalid
}

// IsRisky returns true if the email state is "risky".
func (r *Result) IsRisky() bool {
	return r.State == StateRisky
}

// IsUnknown returns true if the email state is "unknown".
func (r *Result) IsUnknown() bool {
	return r.State == StateUnknown
}

// IsFreeEmail returns true if the email is from a free email provider.
func (r *Result) IsFreeEmail() bool {
	return r.FreeEmail
}

// IsRole returns true if the email is a role-based address (e.g., info@, admin@).
func (r *Result) IsRole() bool {
	return r.Role
}

// IsDisposable returns true if the email is from a disposable email provider.
func (r *Result) IsDisposable() bool {
	return r.Disposable
}
