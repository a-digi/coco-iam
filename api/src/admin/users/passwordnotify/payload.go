package adminpwnotify

type Payload struct {
	UserID          string `json:"user_id"`
	Email           string `json:"email"`
	Username        string `json:"username"`
	DaysUntilExpiry int    `json:"days_until_expiry"`
	ExpiryDate      string `json:"expiry_date"` // formatted "02 Jan 2006"
}
