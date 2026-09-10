package principal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseJWKS_RejectsInvalidOrAmbiguousSets(t *testing.T) {
	key := testJWKSKey(t, "current")
	valid, err := json.Marshal(map[string]any{"keys": []any{key}})
	if err != nil {
		t.Fatal(err)
	}
	if keys, err := ParseJWKS(valid); err != nil || keys["current"] == nil {
		t.Fatalf("valid set = %#v, %v", keys, err)
	}
	duplicate, _ := json.Marshal(map[string]any{"keys": []any{key, key}})
	if _, err := ParseJWKS(duplicate); err == nil {
		t.Fatal("duplicate kid accepted")
	}
	wrongAlgorithm := testJWKSKey(t, "other")
	wrongAlgorithm["alg"] = "RS512"
	bad, _ := json.Marshal(map[string]any{"keys": []any{wrongAlgorithm}})
	if _, err := ParseJWKS(bad); err == nil {
		t.Fatal("wrong algorithm accepted")
	}
}

func TestJWKSResolver_RefreshesAtSoftTTLAndExpiresLastGoodAtHardTTL(t *testing.T) {
	current := testJWKSKey(t, "current")
	valid, _ := json.Marshal(map[string]any{"keys": []any{current}})
	now := time.Unix(1_700_000_000, 0).UTC()
	var calls atomic.Int32
	resolver, err := NewJWKSResolverWithConfig(JWKSResolverConfig{
		Fetch: func(context.Context, string) ([]byte, error) {
			calls.Add(1)
			return valid, nil
		},
		Clock:        func() time.Time { return now },
		RefreshAfter: 10 * time.Second,
		HardExpiry:   20 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), "gateway", "current"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(11 * time.Second)
	if _, err := resolver.Resolve(context.Background(), "gateway", "current"); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("soft TTL refresh calls = %d, want 2", got)
	}

	var attempts atomic.Int32
	stale, err := NewJWKSResolverWithConfig(JWKSResolverConfig{
		Fetch: func(context.Context, string) ([]byte, error) {
			if attempts.Add(1) == 1 {
				return valid, nil
			}
			return nil, errors.New("unavailable")
		},
		Clock: func() time.Time { return now }, RefreshAfter: 10 * time.Second, HardExpiry: 20 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	now = time.Unix(1_700_000_000, 0).UTC()
	if _, err := stale.Resolve(context.Background(), "gateway", "current"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(11 * time.Second)
	if _, err := stale.Resolve(context.Background(), "gateway", "current"); err != nil {
		t.Fatalf("last-good before hard TTL rejected: %v", err)
	}
	now = time.Unix(1_700_000_000, 0).UTC().Add(21 * time.Second)
	if _, err := stale.Resolve(context.Background(), "gateway", "current"); err == nil {
		t.Fatal("last-good key survived hard TTL")
	}
}

func TestJWKSResolver_CoolsDownRepeatedUnknownKidRefreshes(t *testing.T) {
	current := testJWKSKey(t, "current")
	valid, _ := json.Marshal(map[string]any{"keys": []any{current}})
	now := time.Unix(1_700_000_000, 0).UTC()
	var calls atomic.Int32
	resolver, err := NewJWKSResolverWithConfig(JWKSResolverConfig{
		Fetch: func(context.Context, string) ([]byte, error) { calls.Add(1); return valid, nil },
		Clock: func() time.Time { return now }, RefreshAfter: time.Minute, HardExpiry: time.Minute, UnknownKIDCooldown: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), "gateway", "current"); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), "gateway", "missing-one"); err == nil {
		t.Fatal("unknown kid accepted")
	}
	if _, err := resolver.Resolve(context.Background(), "gateway", "missing-two"); err == nil {
		t.Fatal("distinct unknown kid bypassed issuer cooldown")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("unknown kid fetch calls = %d, want 2", got)
	}
}

func TestJWKSResolver_CoolsDownColdCacheUnknownKidRefreshes(t *testing.T) {
	var calls atomic.Int32
	resolver, err := NewJWKSResolverWithConfig(JWKSResolverConfig{
		Fetch:              func(context.Context, string) ([]byte, error) { calls.Add(1); return []byte(`{"keys":[]}`), nil },
		UnknownKIDCooldown: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), "gateway", "missing-one"); err == nil {
		t.Fatal("cold unknown kid accepted")
	}
	if _, err := resolver.Resolve(context.Background(), "gateway", "missing-two"); err == nil {
		t.Fatal("cold distinct unknown kid bypassed cooldown")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("cold unknown kid fetch calls = %d, want 1", got)
	}
}

func TestJWKSResolver_CooldownStillServesUsableKnownLastGoodKey(t *testing.T) {
	current := testJWKSKey(t, "current")
	valid, _ := json.Marshal(map[string]any{"keys": []any{current}})
	now := time.Unix(1_700_000_000, 0).UTC()
	var calls atomic.Int32
	resolver, err := NewJWKSResolverWithConfig(JWKSResolverConfig{
		Fetch: func(context.Context, string) ([]byte, error) {
			if calls.Add(1) == 1 {
				return valid, nil
			}
			return nil, errors.New("unavailable")
		},
		Clock: func() time.Time { return now }, RefreshAfter: time.Second, HardExpiry: time.Minute, UnknownKIDCooldown: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), "gateway", "current"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if _, err := resolver.Resolve(context.Background(), "gateway", "missing"); err == nil {
		t.Fatal("unknown kid accepted")
	}
	if key, err := resolver.Resolve(context.Background(), "gateway", "current"); err != nil || key == nil {
		t.Fatalf("usable known last-good key denied during issuer cooldown: %v", err)
	}
}

func TestJWKSResolver_RetainsLastGoodSetAndRefreshesUnknownKid(t *testing.T) {
	current := testJWKSKey(t, "current")
	next := testJWKSKey(t, "next")
	currentDocument, _ := json.Marshal(map[string]any{"keys": []any{current}})
	rotatedDocument, _ := json.Marshal(map[string]any{"keys": []any{current, next}})
	var calls atomic.Int32
	resolver := NewJWKSResolver(func(context.Context, string) ([]byte, error) {
		switch calls.Add(1) {
		case 1:
			return currentDocument, nil
		case 2:
			return rotatedDocument, nil
		default:
			return []byte(`{"keys":[]}`), nil
		}
	})
	if key, err := resolver.Resolve(context.Background(), "gateway", "current"); err != nil || key == nil {
		t.Fatalf("current key = %v, %v", key, err)
	}
	if key, err := resolver.Resolve(context.Background(), "gateway", "next"); err != nil || key == nil {
		t.Fatalf("next key after refresh = %v, %v", key, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("fetch calls after unknown kid = %d, want 2", got)
	}
	if err := resolver.Refresh(context.Background(), "gateway"); err == nil {
		t.Fatal("invalid refresh accepted")
	}
	if key, err := resolver.Resolve(context.Background(), "gateway", "current"); err != nil || key == nil {
		t.Fatalf("last good current key lost = %v, %v", key, err)
	}
}

func testJWKSKey(t *testing.T, kid string) map[string]any {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	e := big.NewInt(int64(key.E)).Bytes()
	return map[string]any{
		"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(e),
	}
}
