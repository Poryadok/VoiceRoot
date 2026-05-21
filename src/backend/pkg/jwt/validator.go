package jwt

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Validator validates RS256 JWTs against a JWKS URL (issuer/audience/exp per project rules).
type Validator struct {
	jwksURL    string
	issuer     string
	audience   string
	httpClient *http.Client
	now        func() time.Time
	mu         sync.Mutex
	keys       map[string]crypto.PublicKey
}

// Option configures a Validator.
type Option func(*Validator)

// WithClock overrides the time source (for tests).
func WithClock(now func() time.Time) Option {
	return func(v *Validator) {
		v.now = now
	}
}

// WithHTTPClient overrides the HTTP client used to fetch JWKS.
func WithHTTPClient(c *http.Client) Option {
	return func(v *Validator) {
		v.httpClient = c
	}
}

// NewJWKSValidator builds a validator for Bearer JWTs signed with keys from jwksURL.
// issuer and audience may be empty to skip those checks.
func NewJWKSValidator(jwksURL, issuer, audience string, opts ...Option) *Validator {
	v := &Validator{
		jwksURL:  jwksURL,
		issuer:   issuer,
		audience: audience,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		now:  time.Now,
		keys: map[string]crypto.PublicKey{},
	}
	for _, o := range opts {
		o(v)
	}
	return v
}

// BearerToken returns the JWT from Authorization: Bearer or the access_token query param
// (browser WebSocket cannot set custom headers).
func BearerToken(r *http.Request) string {
	const prefix = "Bearer "
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	}
	return strings.TrimSpace(r.URL.Query().Get("access_token"))
}

// Validate reads BearerToken(r) and returns claims or a stable error code string.
// Empty code means success. On failure the code is "invalid_token" (gateway-compatible).
func (v *Validator) Validate(r *http.Request) (Claims, string) {
	token := BearerToken(r)
	if token == "" {
		return Claims{}, "invalid_token"
	}
	claims, err := v.validateJWT(r.Context(), token)
	if err != nil {
		return Claims{}, "invalid_token"
	}
	return claims, ""
}

func (v *Validator) validateJWT(ctx context.Context, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid token segments")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, err
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, err
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return Claims{}, err
	}
	if header.Alg != "RS256" {
		return Claims{}, errors.New("unsupported jwt alg")
	}
	key, err := v.key(ctx, header.Kid)
	if err != nil {
		return Claims{}, err
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return Claims{}, errors.New("jwks key is not rsa")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(rsaKey, crypto.SHA256, digest[:], signature); err != nil {
		return Claims{}, err
	}

	var raw jwtPayload
	if err := json.Unmarshal(payloadBytes, &raw); err != nil {
		return Claims{}, err
	}
	return raw.toClaims(v.issuer, v.audience, v.now())
}

func (v *Validator) key(ctx context.Context, kid string) (crypto.PublicKey, error) {
	v.mu.Lock()
	key, ok := v.keys[kid]
	v.mu.Unlock()
	if ok {
		return key, nil
	}
	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	key, ok = v.keys[kid]
	if !ok {
		return nil, errors.New("unknown jwks kid")
	}
	return key, nil
}

func (v *Validator) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("jwks unavailable")
	}
	var set jwksSet
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return err
	}
	keys := map[string]crypto.PublicKey{}
	for _, jwk := range set.Keys {
		if jwk.Kty != "RSA" || jwk.N == "" || jwk.E == "" {
			continue
		}
		key, err := rsaPublicKeyFromJWK(jwk)
		if err != nil {
			continue
		}
		keys[jwk.Kid] = key
	}
	v.mu.Lock()
	v.keys = keys
	v.mu.Unlock()
	return nil
}

type jwtPayload struct {
	Subject          string          `json:"sub"`
	UserID           string          `json:"user_id"`
	ProfileID        string          `json:"profile_id"`
	Roles            []string        `json:"roles"`
	SubscriptionTier string          `json:"subscription_tier"`
	JTI              string          `json:"jti"`
	Issuer           string          `json:"iss"`
	Audience         json.RawMessage `json:"aud"`
	ExpiresAt        int64           `json:"exp"`
}

func (p jwtPayload) toClaims(issuer, audience string, now time.Time) (Claims, error) {
	if issuer != "" && p.Issuer != issuer {
		return Claims{}, errors.New("issuer mismatch")
	}
	if audience != "" && !p.hasAudience(audience) {
		return Claims{}, errors.New("audience mismatch")
	}
	if p.ExpiresAt <= now.Unix() {
		return Claims{}, errors.New("token expired")
	}
	userID := p.UserID
	if userID == "" {
		userID = p.Subject
	}
	if userID == "" {
		return Claims{}, errors.New("missing subject")
	}
	return Claims{
		UserID:           userID,
		ProfileID:        p.ProfileID,
		Roles:            p.Roles,
		SubscriptionTier: p.SubscriptionTier,
		JTI:              p.JTI,
	}, nil
}

func (p jwtPayload) hasAudience(want string) bool {
	if len(p.Audience) == 0 {
		return false
	}
	var one string
	if err := json.Unmarshal(p.Audience, &one); err == nil {
		return one == want
	}
	var many []string
	if err := json.Unmarshal(p.Audience, &many); err == nil {
		for _, candidate := range many {
			if candidate == want {
				return true
			}
		}
	}
	return false
}
