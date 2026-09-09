package principal

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestServiceCredential_VerifiesExactBinding(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	issuer, key := testIssuer(t, now)
	token, err := issuer.IssueService(ServiceInput{Audience: "role", RPC: "/voice.role.v1.RoleService/CheckPermission", RequestID: "req-1", RequestHash: "sha256:request"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyService(context.Background(), token, serviceConfig(key, now))
	if err != nil {
		t.Fatalf("VerifyService() error = %v", err)
	}
	if got.Kind != serviceType || got.Subject != "service:gateway" || got.RequestID != "req-1" || got.ExpiresAt.Sub(got.IssuedAt) != maxCredentialTTL {
		t.Fatalf("principal = %+v", got)
	}
}

func TestDelegatedUserCredential_VerifiesImmutableIdentity(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	issuer, key := testIssuer(t, now)
	token, err := issuer.IssueDelegatedUser(DelegatedUserInput{Audience: "role", RPC: "/voice.role.v1.RoleService/CreateRole", RequestID: "req-1", RequestHash: "sha256:request", AccountID: "account-1", ProfileID: "profile-1", SessionEpoch: 7})
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyDelegatedUser(context.Background(), token, delegatedConfig(key, now))
	if err != nil {
		t.Fatalf("VerifyDelegatedUser() error = %v", err)
	}
	if got.AccountID != "account-1" || got.Subject != "account-1" || got.ProfileID != "profile-1" || got.SessionEpoch != 7 {
		t.Fatalf("principal = %+v", got)
	}
	if _, ok := FromContext(WithVerified(context.Background(), got)); !ok {
		t.Fatal("verified principal was not retained in context")
	}
}

func TestVerifier_FailsClosedForInvalidOrMismatchedCredentials(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	issuer, key := testIssuer(t, now)
	service, err := issuer.IssueService(ServiceInput{Audience: "role", RPC: "/voice.role.v1.RoleService/CheckPermission", RequestID: "req-1", RequestHash: "sha256:request"})
	if err != nil {
		t.Fatal(err)
	}
	delegated, err := issuer.IssueDelegatedUser(DelegatedUserInput{Audience: "role", RPC: "/voice.role.v1.RoleService/CreateRole", RequestID: "req-1", RequestHash: "sha256:request", AccountID: "account-1", ProfileID: "profile-1", SessionEpoch: 7})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, token string
		config      VerifyConfig
		delegated   bool
	}{
		{"wrong audience", service, altered(serviceConfig(key, now), func(c *VerifyConfig) { c.ExpectedAudience = "space" }), false},
		{"wrong rpc", service, altered(serviceConfig(key, now), func(c *VerifyConfig) { c.ExpectedRPC = "/other" }), false},
		{"wrong request", service, altered(serviceConfig(key, now), func(c *VerifyConfig) { c.ExpectedRequestHash = "sha256:other" }), false},
		{"wrong issuer", service, altered(serviceConfig(key, now), func(c *VerifyConfig) { c.ExpectedIssuer = "space" }), false},
		{"expired", service, serviceConfig(key, now.Add(31*time.Second)), false},
		{"service as delegated", service, delegatedConfig(key, now), true},
		{"delegated as service", delegated, serviceConfig(key, now), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.delegated {
				_, err = VerifyDelegatedUser(context.Background(), tc.token, tc.config)
			} else {
				_, err = VerifyService(context.Background(), tc.token, tc.config)
			}
			if err == nil {
				t.Fatal("verification succeeded")
			}
		})
	}
}

func TestVerifier_RejectsTamperingMissingKeyAndLongLivedCredential(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	issuer, key := testIssuer(t, now)
	token, err := issuer.IssueService(ServiceInput{Audience: "role", RPC: "/voice.role.v1.RoleService/CheckPermission", RequestID: "req-1", RequestHash: "sha256:request"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyService(context.Background(), token+"x", serviceConfig(key, now)); err == nil {
		t.Fatal("tampered signature accepted")
	}
	noKey := serviceConfig(key, now)
	noKey.KeyResolver = func(context.Context, string, string) (*rsa.PublicKey, error) { return nil, nil }
	if _, err := VerifyService(context.Background(), token, noKey); err == nil {
		t.Fatal("missing key accepted")
	}
	longLived, err := signClaims(key, "current", rawClaims{Type: serviceType, Issuer: "gateway", Subject: "service:gateway", Audience: "role", RPC: "/voice.role.v1.RoleService/CheckPermission", RequestID: "req-1", RequestHash: "sha256:request", IssuedAt: now.Unix(), NotBefore: now.Unix(), ExpiresAt: now.Add(maxCredentialTTL + time.Second).Unix(), JWTID: "id"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyService(context.Background(), longLived, serviceConfig(key, now)); err == nil {
		t.Fatal("credential longer than 30 seconds accepted")
	}
}

func TestVerifier_InvokesFailClosedReplayAndEpochHooks(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	issuer, key := testIssuer(t, now)
	service, err := issuer.IssueService(ServiceInput{Audience: "role", RPC: "/voice.role.v1.RoleService/CheckPermission", RequestID: "req-1", RequestHash: "sha256:request"})
	if err != nil {
		t.Fatal(err)
	}
	serviceConfig := serviceConfig(key, now)
	serviceConfig.ReplayGuard = func(context.Context, string, string, time.Time) error { return errors.New("replayed") }
	if _, err := VerifyService(context.Background(), service, serviceConfig); err == nil {
		t.Fatal("replayed service credential accepted")
	}

	delegated, err := issuer.IssueDelegatedUser(DelegatedUserInput{Audience: "role", RPC: "/voice.role.v1.RoleService/CreateRole", RequestID: "req-1", RequestHash: "sha256:request", AccountID: "account-1", ProfileID: "profile-1", SessionEpoch: 7})
	if err != nil {
		t.Fatal(err)
	}
	delegatedConfig := delegatedConfig(key, now)
	delegatedConfig.SessionEpochChecker = func(context.Context, string, int64) error { return errors.New("revoked") }
	if _, err := VerifyDelegatedUser(context.Background(), delegated, delegatedConfig); err == nil {
		t.Fatal("revoked delegated credential accepted")
	}
}

func TestIssuer_RejectsIncompleteInputs(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	issuer, _ := testIssuer(t, now)
	if _, err := issuer.IssueService(ServiceInput{Audience: "role"}); err == nil {
		t.Fatal("incomplete service input accepted")
	}
	if _, err := issuer.IssueDelegatedUser(DelegatedUserInput{Audience: "role", RPC: "rpc", RequestID: "request", RequestHash: "hash", AccountID: "account", ProfileID: "profile"}); err == nil {
		t.Fatal("zero session epoch accepted")
	}
}
func testIssuer(t *testing.T, now time.Time) (*Issuer, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := NewIssuer(IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return issuer, key
}
func baseConfig(key *rsa.PrivateKey, now time.Time, rpc string) VerifyConfig {
	return VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "role", ExpectedRPC: rpc, ExpectedRequestID: "req-1", ExpectedRequestHash: "sha256:request", Clock: func() time.Time { return now }, KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
		if issuer != "gateway" || kid != "current" {
			return nil, nil
		}
		return &key.PublicKey, nil
	}}
}
func serviceConfig(key *rsa.PrivateKey, now time.Time) VerifyConfig {
	return baseConfig(key, now, "/voice.role.v1.RoleService/CheckPermission")
}
func delegatedConfig(key *rsa.PrivateKey, now time.Time) VerifyConfig {
	return baseConfig(key, now, "/voice.role.v1.RoleService/CreateRole")
}
func altered(c VerifyConfig, change func(*VerifyConfig)) VerifyConfig { change(&c); return c }
func signClaims(key *rsa.PrivateKey, keyID string, claims rawClaims) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": keyID})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
