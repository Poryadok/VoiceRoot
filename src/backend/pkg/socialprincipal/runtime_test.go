package socialprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
	"time"
	"voice/backend/pkg/principal"
)

func jwksDocument(t *testing.T, keys map[string]*rsa.PublicKey) []byte {
	t.Helper()
	list := []map[string]string{}
	for kid, key := range keys {
		list = append(list, map[string]string{"kty": "RSA", "alg": "RS256", "use": "sig", "kid": kid, "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
	}
	doc, err := json.Marshal(map[string]any{"keys": list})
	require.NoError(t, err)
	return doc
}
func TestJWKSRequiresTwoDistinctValidKeys(t *testing.T) {
	first, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	second, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	require.NoError(t, validateJWKS(jwksDocument(t, map[string]*rsa.PublicKey{"current": &first.PublicKey, "next": &second.PublicKey})))
	for _, keys := range []map[string]*rsa.PublicKey{{"current": &first.PublicKey}, {"current": &first.PublicKey, "next": &first.PublicKey}, {"invalid kid": &first.PublicKey, "next": &second.PublicKey}} {
		require.Error(t, validateJWKS(jwksDocument(t, keys)))
	}
}
func TestJWKSRotationRejectsIncompleteRefreshUntilHardExpiry(t *testing.T) {
	first, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	doc := jwksDocument(t, map[string]*rsa.PublicKey{"current": &first.PublicKey, "next": &next.PublicKey})
	now := time.Now()
	fetches := 0
	resolver, err := principal.NewJWKSResolverWithConfig(principal.JWKSResolverConfig{Clock: func() time.Time { return now }, Fetch: func(context.Context, string) ([]byte, error) { fetches++; return doc, validateJWKS(doc) }, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second})
	require.NoError(t, err)
	_, err = resolver.Resolve(context.Background(), "social", "current")
	require.NoError(t, err)
	_, err = resolver.Resolve(context.Background(), "social", "next")
	require.NoError(t, err)
	doc = jwksDocument(t, map[string]*rsa.PublicKey{"current": &first.PublicKey})
	now = now.Add(31 * time.Second)
	_, err = resolver.Resolve(context.Background(), "social", "current")
	require.NoError(t, err)
	_, err = resolver.Resolve(context.Background(), "social", "unknown")
	require.Error(t, err)
	before := fetches
	_, err = resolver.Resolve(context.Background(), "social", "unknown-2")
	require.Error(t, err)
	require.Equal(t, before, fetches)
	now = now.Add(2 * time.Minute)
	_, err = resolver.Resolve(context.Background(), "social", "current")
	require.Error(t, err)
}
