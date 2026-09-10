package principal

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// ParseJWKS accepts only an unambiguous usable RS256 signature key set. It is
// all-or-nothing, so an invalid refresh cannot partially replace a cache.
func ParseJWKS(document []byte) (map[string]*rsa.PublicKey, error) {
	var parsed jwksDocument
	if err := json.Unmarshal(document, &parsed); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}
	if len(parsed.Keys) == 0 {
		return nil, errors.New("jwks contains no signing keys")
	}
	keys := make(map[string]*rsa.PublicKey, len(parsed.Keys))
	for _, candidate := range parsed.Keys {
		kid := strings.TrimSpace(candidate.Kid)
		if candidate.Kty != "RSA" || candidate.Use != "sig" || candidate.Alg != "RS256" || kid == "" || candidate.N == "" || candidate.E == "" {
			return nil, errors.New("jwks contains invalid signing key")
		}
		if _, exists := keys[kid]; exists {
			return nil, errors.New("jwks contains duplicate kid")
		}
		key, err := rsaPublicKey(candidate)
		if err != nil {
			return nil, fmt.Errorf("jwks key %q: %w", kid, err)
		}
		keys[kid] = key
	}
	return keys, nil
}

func rsaPublicKey(candidate jwk) (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(candidate.N)
	if err != nil || len(n) == 0 {
		return nil, errors.New("invalid rsa modulus")
	}
	e, err := base64.RawURLEncoding.DecodeString(candidate.E)
	if err != nil || len(e) == 0 {
		return nil, errors.New("invalid rsa exponent")
	}
	if len(e) > 4 {
		return nil, errors.New("invalid rsa exponent")
	}
	exponent := 0
	for _, b := range e {
		exponent = exponent<<8 + int(b)
	}
	if exponent < 3 || exponent%2 == 0 {
		return nil, errors.New("invalid rsa exponent")
	}
	key := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}
	if key.N.Sign() <= 0 {
		return nil, errors.New("invalid rsa modulus")
	}
	return key, nil
}

// JWKSFetcher remains transport-agnostic; HTTP/TLS configuration belongs to service wiring.
type JWKSFetcher func(ctx context.Context, issuer string) ([]byte, error)

const (
	defaultJWKSRefreshAfter = 30 * time.Second
	defaultJWKSHardExpiry   = 2 * time.Minute
)

// JWKSResolverConfig sets bounded cache behavior. HardExpiry must be at least
// RefreshAfter so failed refreshes can temporarily use a last-good current+next set.
// UnknownKIDCooldown is issuer-wide to bound attacker-controlled key-id cardinality.
type JWKSResolverConfig struct {
	Fetch              JWKSFetcher
	Clock              func() time.Time
	RefreshAfter       time.Duration
	HardExpiry         time.Duration
	UnknownKIDCooldown time.Duration
}

type jwksCacheEntry struct {
	keys       map[string]*rsa.PublicKey
	lastGoodAt time.Time
}

// JWKSResolver caches the last complete valid set per issuer only until its
// hard expiry. Current+next rotation is safe because a successful refresh
// replaces the complete set; a failed refresh uses last-good only before that limit.
type JWKSResolver struct {
	fetch              JWKSFetcher
	clock              func() time.Time
	refreshAfter       time.Duration
	hardExpiry         time.Duration
	unknownKIDCooldown time.Duration
	mu                 sync.RWMutex
	refresh            sync.Mutex
	sets               map[string]jwksCacheEntry
	unknownKIDRefresh  map[string]time.Time
}

func NewJWKSResolver(fetch JWKSFetcher) *JWKSResolver {
	resolver, err := NewJWKSResolverWithConfig(JWKSResolverConfig{Fetch: fetch})
	if err != nil {
		return nil
	}
	return resolver
}

// NewJWKSResolverWithConfig builds a resolver with testable bounded cache semantics.
func NewJWKSResolverWithConfig(config JWKSResolverConfig) (*JWKSResolver, error) {
	if config.Fetch == nil {
		return nil, errors.New("jwks fetcher is required")
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.RefreshAfter == 0 {
		config.RefreshAfter = defaultJWKSRefreshAfter
	}
	if config.HardExpiry == 0 {
		config.HardExpiry = defaultJWKSHardExpiry
	}
	if config.UnknownKIDCooldown == 0 {
		config.UnknownKIDCooldown = 5 * time.Second
	}
	if config.RefreshAfter <= 0 || config.HardExpiry < config.RefreshAfter || config.UnknownKIDCooldown <= 0 {
		return nil, errors.New("jwks cache ttl configuration is invalid")
	}
	return &JWKSResolver{fetch: config.Fetch, clock: config.Clock, refreshAfter: config.RefreshAfter, hardExpiry: config.HardExpiry, unknownKIDCooldown: config.UnknownKIDCooldown, sets: make(map[string]jwksCacheEntry), unknownKIDRefresh: make(map[string]time.Time)}, nil
}

func (r *JWKSResolver) Resolve(ctx context.Context, issuer, keyID string) (*rsa.PublicKey, error) {
	issuer, keyID = strings.TrimSpace(issuer), strings.TrimSpace(keyID)
	if r == nil || r.fetch == nil || issuer == "" || keyID == "" {
		return nil, errors.New("jwks resolver configuration is invalid")
	}
	if key, fresh, _ := r.cached(issuer, keyID); key != nil && fresh {
		return key, nil
	}
	r.refresh.Lock()
	defer r.refresh.Unlock()
	if key, fresh, _ := r.cached(issuer, keyID); key != nil && fresh {
		return key, nil
	}
	if r.unknownKIDCoolingDown(issuer) {
		if key, _, usable := r.cached(issuer, keyID); key != nil && usable {
			return key, nil
		}
		return nil, errors.New("jwks kid refresh is cooling down")
	}
	if err := r.refreshLocked(ctx, issuer); err != nil {
		if key, _, usable := r.cached(issuer, keyID); key != nil && usable {
			return key, nil
		}
		r.markUnknownKIDRefresh(issuer)
		return nil, err
	}
	if key, _, _ := r.cached(issuer, keyID); key != nil {
		return key, nil
	}
	r.markUnknownKIDRefresh(issuer)
	return nil, errors.New("jwks kid is unknown")
}

func (r *JWKSResolver) Refresh(ctx context.Context, issuer string) error {
	issuer = strings.TrimSpace(issuer)
	if r == nil || r.fetch == nil || issuer == "" {
		return errors.New("jwks resolver configuration is invalid")
	}
	r.refresh.Lock()
	defer r.refresh.Unlock()
	return r.refreshLocked(ctx, issuer)
}

func (r *JWKSResolver) refreshLocked(ctx context.Context, issuer string) error {
	document, err := r.fetch(ctx, issuer)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	keys, err := ParseJWKS(document)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.sets[issuer] = jwksCacheEntry{keys: keys, lastGoodAt: r.clock().UTC()}
	delete(r.unknownKIDRefresh, issuer)
	r.mu.Unlock()
	return nil
}

func (r *JWKSResolver) cached(issuer, keyID string) (*rsa.PublicKey, bool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.sets[issuer]
	if !ok || entry.lastGoodAt.IsZero() {
		return nil, false, false
	}
	age := r.clock().UTC().Sub(entry.lastGoodAt)
	if age < 0 || age > r.hardExpiry {
		return nil, false, false
	}
	return entry.keys[keyID], age < r.refreshAfter, true
}

func (r *JWKSResolver) unknownKIDCoolingDown(issuer string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	last := r.unknownKIDRefresh[issuer]
	return !last.IsZero() && r.clock().UTC().Sub(last) < r.unknownKIDCooldown
}

func (r *JWKSResolver) markUnknownKIDRefresh(issuer string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unknownKIDRefresh[issuer] = r.clock().UTC()
}
