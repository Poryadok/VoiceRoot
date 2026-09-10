package principal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"sync/atomic"
	"testing"
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
	e := big.NewInt(int64(key.PublicKey.E)).Bytes()
	return map[string]any{
		"kty": "RSA", "kid": kid, "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(e),
	}
}
