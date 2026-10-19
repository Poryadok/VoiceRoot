package jwt

import "time"

// Claims holds validated Voice access token claims (see project DATA_MODEL / api-gateway docs).
type Claims struct {
	UserID           string   `json:"user_id"`
	ProfileID        string   `json:"profile_id"`
	Roles            []string `json:"roles"`
	SubscriptionTier string   `json:"subscription_tier"`
	AccountType      string   `json:"account_type"`
	// AuthenticatedAccountType preserves the exact signed claim before legacy
	// trimming/defaults. Capability issuers must not infer explicit authority
	// from the compatibility AccountType value.
	AuthenticatedAccountType string    `json:"-"`
	JTI                      string    `json:"jti"`
	SessionEpoch             int64     `json:"session_epoch"`
	ExpiresAt                time.Time `json:"-"`
}
