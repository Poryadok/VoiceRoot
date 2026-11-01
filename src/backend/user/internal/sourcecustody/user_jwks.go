package sourcecustody

import (
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"sort"

	"voice/backend/pkg/principal"
)

// NewUserJWKSHandler reconstructs public RSA values and never serves input
// bytes directly. TLS and the listener lifecycle belong to the caller.
func NewUserJWKSHandler(public []byte) (http.Handler, error) {
	if len(public) == 0 || len(public) > 65536 {
		return nil, ErrUnavailable
	}
	keys, err := principal.ParseJWKS(public)
	if err != nil || len(keys) != 2 {
		return nil, ErrUnavailable
	}
	type jwk struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
	}
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if keys[ids[0]] == nil || keys[ids[1]] == nil || keys[ids[0]].Equal(keys[ids[1]]) {
		return nil, ErrUnavailable
	}
	items := make([]jwk, 0, len(ids))
	for _, id := range ids {
		key := keys[id]
		if key.N == nil || key.N.BitLen() < 2048 || key.E < 3 {
			return nil, ErrUnavailable
		}
		items = append(items, jwk{Kty: "RSA", Kid: id, Use: "sig", Alg: "RS256", N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
	}
	body, err := json.Marshal(struct {
		Keys []jwk `json:"keys"`
	}{Keys: items})
	if err != nil {
		return nil, ErrUnavailable
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL == nil || r.URL.Path != "/.well-known/jwks.json" || r.URL.RawQuery != "" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	}), nil
}
