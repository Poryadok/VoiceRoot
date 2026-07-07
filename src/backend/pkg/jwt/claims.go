package jwt

// Claims holds validated Voice access token claims (see project DATA_MODEL / api-gateway docs).
type Claims struct {
	UserID           string   `json:"user_id"`
	ProfileID        string   `json:"profile_id"`
	Roles            []string `json:"roles"`
	SubscriptionTier string   `json:"subscription_tier"`
	AccountType      string   `json:"account_type"`
	JTI              string   `json:"jti"`
}
