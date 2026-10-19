package principal

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestJWKSReadinessObservesOriginalExpiryWithoutFetchOrRenewal(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	calls := 0
	document, _ := json.Marshal(map[string]any{"keys": []any{testJWKSKey(t, "current"), testJWKSKey(t, "next")}})
	resolver, err := NewJWKSResolverWithConfig(JWKSResolverConfig{Fetch: func(context.Context, string) ([]byte, error) { calls++; return document, nil }, Clock: func() time.Time { return now }, RefreshAfter: time.Second, HardExpiry: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if resolver.Usable("space") || (*JWKSResolver)(nil).Usable("space") {
		t.Fatal("missing cache ready")
	}
	if err = resolver.Refresh(context.Background(), "space"); err != nil {
		t.Fatal(err)
	}
	if !resolver.Usable("space") || resolver.Usable("chat") {
		t.Fatal("issuer-specific cache readiness lost")
	}
	now = now.Add(time.Second)
	if !resolver.Usable("space") || calls != 1 {
		t.Fatal("readiness fetched or rejected usable last-good cache")
	}
	now = now.Add(time.Second)
	if resolver.Usable("space") || calls != 1 {
		t.Fatal("readiness extended hard expiry")
	}
	now = now.Add(-3 * time.Second)
	if resolver.Usable("space") {
		t.Fatal("future last-good cache ready")
	}
}
