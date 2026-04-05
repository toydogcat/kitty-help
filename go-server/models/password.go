package models

type Password struct {
	ID          string `json:"id"`
	UserID      string `json:"-"` // Internal only
	SiteName    string `json:"siteName"`
	Account     string `json:"account"`
	PasswordRaw string `json:"passwordRaw"`
	Category    string `json:"category"`
	Notes       string `json:"notes"`
	CreatedAt   string `json:"createdAt"`
}
