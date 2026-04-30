package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJWTValidator(t *testing.T) {
	key1 := mustRSAKey(t)
	key2 := mustRSAKey(t)
	activeKey := key1
	activeKid := "key-1"
	jwksCalls := 0
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		jwksCalls++
		writeJSON(w, http.StatusOK, map[string]any{
			"keys": []map[string]string{rsaJWK(activeKid, &activeKey.PublicKey)},
		})
	}))
	t.Cleanup(jwks.Close)

	validator := newJWTValidator(jwks.URL, "voice-auth", "voice-client")
	validator.now = func() time.Time { return time.Unix(1000, 0) }

	valid := signJWT(t, "key-1", key1, map[string]any{
		"sub":               "account-1",
		"profile_id":        "profile-1",
		"roles":             []string{"member"},
		"subscription_tier": "free",
		"jti":               "token-1",
		"iss":               "voice-auth",
		"aud":               "voice-client",
		"exp":               int64(1100),
	})
	claims, code := validator.Validate(requestWithToken(valid))
	if code != "" {
		t.Fatalf("valid token code = %q", code)
	}
	if claims.UserID != "account-1" || claims.ProfileID != "profile-1" || claims.JTI != "token-1" {
		t.Fatalf("claims = %+v", claims)
	}

	expired := signJWT(t, "key-1", key1, map[string]any{
		"sub": "account-1", "iss": "voice-auth", "aud": "voice-client", "exp": int64(999),
	})
	if _, code := validator.Validate(requestWithToken(expired)); code != "invalid_token" {
		t.Fatalf("expired code = %q, want invalid_token", code)
	}

	wrongIssuer := signJWT(t, "key-1", key1, map[string]any{
		"sub": "account-1", "iss": "other", "aud": "voice-client", "exp": int64(1100),
	})
	if _, code := validator.Validate(requestWithToken(wrongIssuer)); code != "invalid_token" {
		t.Fatalf("wrong issuer code = %q, want invalid_token", code)
	}

	activeKey = key2
	activeKid = "key-2"
	rotated := signJWT(t, "key-2", key2, map[string]any{
		"user_id": "account-2", "iss": "voice-auth", "aud": []string{"voice-client"}, "exp": int64(1100),
	})
	claims, code = validator.Validate(requestWithToken(rotated))
	if code != "" {
		t.Fatalf("rotated token code = %q", code)
	}
	if claims.UserID != "account-2" {
		t.Fatalf("rotated user id = %q", claims.UserID)
	}
	if jwksCalls < 2 {
		t.Fatalf("jwks calls = %d, want refresh on unknown kid", jwksCalls)
	}
}

type fakeBlacklist struct {
	revoked bool
	err     error
}

func (b fakeBlacklist) IsRevoked(_ context.Context, _ string) (bool, error) {
	return b.revoked, b.err
}

type fixedValidator struct {
	claims tokenClaims
	code   string
}

func (v fixedValidator) Validate(_ *http.Request) (tokenClaims, string) {
	return v.claims, v.code
}

func requestWithToken(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func mustRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	return key
}

func rsaJWK(kid string, key *rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"use": "sig",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}
}

func signJWT(t *testing.T, kid string, key *rsa.PrivateKey, payload map[string]any) string {
	t.Helper()
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid}
	headerBytes, _ := json.Marshal(header)
	payloadBytes, _ := json.Marshal(payload)
	unsigned := base64.RawURLEncoding.EncodeToString(headerBytes) + "." + base64.RawURLEncoding.EncodeToString(payloadBytes)
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}
