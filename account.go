package truelist

// AccountInfo represents the nested account details.
type AccountInfo struct {
	Name        string `json:"name"`
	PaymentPlan string `json:"payment_plan"`
}

// Account represents the response from the account info endpoint.
type Account struct {
	Email       string      `json:"email"`
	Name        string      `json:"name"`
	UUID        string      `json:"uuid"`
	TimeZone    string      `json:"time_zone"`
	IsAdminRole bool        `json:"is_admin_role"`
	Account     AccountInfo `json:"account"`
}
