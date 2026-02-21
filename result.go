package truelist

// State constants for email validation results.
const (
	StateOK           = "ok"
	StateEmailInvalid = "email_invalid"
	StateAcceptAll    = "accept_all"
)

// SubState constants for email validation results.
const (
	SubStateEmailOK        = "email_ok"
	SubStateIsDisposable   = "is_disposable"
	SubStateIsRole         = "is_role"
	SubStateUnknownError   = "unknown_error"
	SubStateFailedSMTP     = "failed_smtp_check"
)

// Result represents the response from an email validation request.
type Result struct {
	Email      string  `json:"address"`
	Domain     string  `json:"domain"`
	Canonical  string  `json:"canonical"`
	MxRecord   *string `json:"mx_record"`
	FirstName  *string `json:"first_name"`
	LastName   *string `json:"last_name"`
	State      string  `json:"email_state"`
	SubState   string  `json:"email_sub_state"`
	VerifiedAt string  `json:"verified_at"`
	Suggestion *string `json:"did_you_mean"`
}

// IsValid returns true if the email state is "ok".
func (r *Result) IsValid() bool {
	return r.State == StateOK
}

// IsInvalid returns true if the email state is "email_invalid".
func (r *Result) IsInvalid() bool {
	return r.State == StateEmailInvalid
}

// IsAcceptAll returns true if the email state is "accept_all".
func (r *Result) IsAcceptAll() bool {
	return r.State == StateAcceptAll
}

// IsDisposable returns true if the email sub-state is "is_disposable".
func (r *Result) IsDisposable() bool {
	return r.SubState == SubStateIsDisposable
}

// IsRole returns true if the email sub-state is "is_role".
func (r *Result) IsRole() bool {
	return r.SubState == SubStateIsRole
}
