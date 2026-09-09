package principal

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type KeyResolver func(ctx context.Context, issuer, keyID string) (*rsa.PublicKey, error)
type ReplayGuard func(ctx context.Context, issuer, jwtID string, expiresAt time.Time) error
type SessionEpochChecker func(ctx context.Context, accountID string, sessionEpoch int64) error

type VerifyConfig struct {
	ExpectedIssuer      string
	ExpectedAudience    string
	ExpectedRPC         string
	ExpectedRequestID   string
	ExpectedRequestHash string
	Clock               func() time.Time
	KeyResolver         KeyResolver
	ReplayGuard         ReplayGuard
	SessionEpochChecker SessionEpochChecker
}

type Principal struct {
	Kind         string
	Issuer       string
	Subject      string
	Audience     string
	RPC          string
	RequestID    string
	RequestHash  string
	AccountID    string
	ProfileID    string
	SessionEpoch int64
	JWTID        string
	IssuedAt     time.Time
	ExpiresAt    time.Time
}

func VerifyService(ctx context.Context, token string, config VerifyConfig) (Principal, error) {
	claims, err := verify(ctx, token, config, serviceType)
	if err != nil {
		return Principal{}, err
	}
	if claims.Subject != "service:"+claims.Issuer {
		return Principal{}, fmt.Errorf("service subject does not match issuer")
	}
	return principalFromClaims(claims), nil
}

func VerifyDelegatedUser(ctx context.Context, token string, config VerifyConfig) (Principal, error) {
	if config.ExpectedIssuer != "gateway" {
		return Principal{}, fmt.Errorf("delegated user credentials must be issued by gateway")
	}
	claims, err := verify(ctx, token, config, delegatedUserType)
	if err != nil {
		return Principal{}, err
	}
	if claims.Subject == "" || claims.AccountID == "" || claims.ProfileID == "" || claims.Subject != claims.AccountID || claims.SessionEpoch <= 0 {
		return Principal{}, fmt.Errorf("invalid delegated user identity")
	}
	if config.SessionEpochChecker != nil {
		if err := config.SessionEpochChecker(ctx, claims.AccountID, claims.SessionEpoch); err != nil {
			return Principal{}, fmt.Errorf("validate session epoch: %w", err)
		}
	}
	return principalFromClaims(claims), nil
}

func verify(ctx context.Context, token string, config VerifyConfig, expectedType string) (rawClaims, error) {
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.KeyResolver == nil || strings.TrimSpace(config.ExpectedIssuer) == "" || strings.TrimSpace(config.ExpectedAudience) == "" || strings.TrimSpace(config.ExpectedRPC) == "" || strings.TrimSpace(config.ExpectedRequestID) == "" || strings.TrimSpace(config.ExpectedRequestHash) == "" {
		return rawClaims{}, fmt.Errorf("exact issuer, audience, rpc, request binding, and key resolver are required")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return rawClaims{}, fmt.Errorf("invalid token segments")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return rawClaims{}, err
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return rawClaims{}, err
	}
	if header.Alg != "RS256" || strings.TrimSpace(header.Kid) == "" {
		return rawClaims{}, fmt.Errorf("unsupported signing header")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return rawClaims{}, err
	}
	var claims rawClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return rawClaims{}, err
	}
	if claims.Issuer != config.ExpectedIssuer {
		return rawClaims{}, fmt.Errorf("issuer mismatch")
	}
	key, err := config.KeyResolver(ctx, claims.Issuer, header.Kid)
	if err != nil {
		return rawClaims{}, fmt.Errorf("resolve signing key: %w", err)
	}
	if key == nil {
		return rawClaims{}, fmt.Errorf("resolve signing key: no key for credential kid")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return rawClaims{}, err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return rawClaims{}, err
	}
	if claims.Type != expectedType || claims.Audience != config.ExpectedAudience || claims.RPC != config.ExpectedRPC || claims.RequestID != config.ExpectedRequestID || claims.RequestHash != config.ExpectedRequestHash {
		return rawClaims{}, fmt.Errorf("credential binding mismatch")
	}
	if err := validateTemporal(claims, config.Clock().UTC()); err != nil {
		return rawClaims{}, err
	}
	if config.ReplayGuard != nil {
		if err := config.ReplayGuard(ctx, claims.Issuer, claims.JWTID, time.Unix(claims.ExpiresAt, 0).UTC()); err != nil {
			return rawClaims{}, fmt.Errorf("replay guard: %w", err)
		}
	}
	return claims, nil
}

func validateTemporal(claims rawClaims, now time.Time) error {
	if claims.IssuedAt <= 0 || claims.NotBefore <= 0 || claims.ExpiresAt <= 0 || strings.TrimSpace(claims.JWTID) == "" {
		return fmt.Errorf("required temporal claims missing")
	}
	issuedAt, notBefore, expiresAt := time.Unix(claims.IssuedAt, 0), time.Unix(claims.NotBefore, 0), time.Unix(claims.ExpiresAt, 0)
	if notBefore.Before(issuedAt) || notBefore.After(now) || issuedAt.After(now) || !expiresAt.After(now) || expiresAt.Before(notBefore) || expiresAt.Sub(issuedAt) > maxCredentialTTL {
		return fmt.Errorf("credential temporal claims invalid")
	}
	return nil
}

func principalFromClaims(c rawClaims) Principal {
	return Principal{Kind: c.Type, Issuer: c.Issuer, Subject: c.Subject, Audience: c.Audience, RPC: c.RPC, RequestID: c.RequestID, RequestHash: c.RequestHash, AccountID: c.AccountID, ProfileID: c.ProfileID, SessionEpoch: c.SessionEpoch, JWTID: c.JWTID, IssuedAt: time.Unix(c.IssuedAt, 0).UTC(), ExpiresAt: time.Unix(c.ExpiresAt, 0).UTC()}
}

type principalContextKey struct{}

func WithVerified(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}
func FromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}
