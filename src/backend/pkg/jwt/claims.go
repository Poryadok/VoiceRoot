package jwt

// Claims holds validated Voice access token claims (see project DATA_MODEL / api-gateway docs).
type Claims struct {
	UserID           string
	ProfileID        string
	Roles            []string
	SubscriptionTier string
	JTI              string
}
