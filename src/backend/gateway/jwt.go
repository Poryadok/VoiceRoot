package main

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

type jwtValidator struct {
	jwksURL    string
	issuer     string
	audience   string
	httpClient *http.Client
	now        func() time.Time
	mu         sync.Mutex
	keys       map[string]crypto.PublicKey
}

func newJWTValidator(jwksURL, issuer, audience string) *jwtValidator {
	return &jwtValidator{
		jwksURL:  jwksURL,
		issuer:   issuer,
		audience: audience,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		now:  time.Now,
		keys: map[string]crypto.PublicKey{},
	}
}

func (v *jwtValidator) Validate(r *http.Request) (tokenClaims, string) {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return tokenClaims{}, "invalid_token"
	}
	claims, err := v.validateJWT(r.Context(), strings.TrimPrefix(auth, prefix))
	if err != nil {
		return tokenClaims{}, "invalid_token"
	}
	return claims, ""
}

func (v *jwtValidator) validateJWT(ctx context.Context, token string) (tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return tokenClaims{}, errors.New("invalid token segments")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return tokenClaims{}, err
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenClaims{}, err
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return tokenClaims{}, err
	}
	if header.Alg != "RS256" {
		return tokenClaims{}, errors.New("unsupported jwt alg")
	}
	key, err := v.key(ctx, header.Kid)
	if err != nil {
		return tokenClaims{}, err
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return tokenClaims{}, errors.New("jwks key is not rsa")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return tokenClaims{}, err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(rsaKey, crypto.SHA256, digest[:], signature); err != nil {
		return tokenClaims{}, err
	}

	var raw jwtPayload
	if err := json.Unmarshal(payloadBytes, &raw); err != nil {
		return tokenClaims{}, err
	}
	return raw.toTokenClaims(v.issuer, v.audience, v.now())
}

func (v *jwtValidator) key(ctx context.Context, kid string) (crypto.PublicKey, error) {
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

func (v *jwtValidator) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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

type jwksSet struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func rsaPublicKeyFromJWK(jwk jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, err
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, errors.New("invalid exponent")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
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

func (p jwtPayload) toTokenClaims(issuer, audience string, now time.Time) (tokenClaims, error) {
	if issuer != "" && p.Issuer != issuer {
		return tokenClaims{}, errors.New("issuer mismatch")
	}
	if audience != "" && !p.hasAudience(audience) {
		return tokenClaims{}, errors.New("audience mismatch")
	}
	if p.ExpiresAt <= now.Unix() {
		return tokenClaims{}, errors.New("token expired")
	}
	userID := p.UserID
	if userID == "" {
		userID = p.Subject
	}
	if userID == "" {
		return tokenClaims{}, errors.New("missing subject")
	}
	return tokenClaims{
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
