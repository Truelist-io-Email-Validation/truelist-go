package truelist

// Account represents the response from the account info endpoint.
type Account struct {
	Email   string `json:"email"`
	Plan    string `json:"plan"`
	Credits int    `json:"credits"`
}
