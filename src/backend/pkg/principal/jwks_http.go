package principal

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"os"
	"strings"
)

type JWKSKey struct {
	KeyID     string
	PublicKey *rsa.PublicKey
}

// LoadRSAPrivateKeyFile loads an unencrypted PKCS#1 or PKCS#8 RSA PEM key.
// Service issuer keys are provided as mounted files so private material never
// appears in process arguments or public JWKS responses.
func LoadRSAPrivateKeyFile(path string) (*rsa.PrivateKey, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("RSA private key file is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(raw)
	if block == nil || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, errors.New("invalid RSA private key PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		if err := key.Validate(); err != nil {
			return nil, errors.New("invalid RSA private key")
		}
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("invalid RSA private key")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok || key.Validate() != nil {
		return nil, errors.New("invalid RSA private key")
	}
	return key, nil
}

// PublicJWKS returns a single-key RS256 JWKS document. It contains no private
// key material and is suitable for a workload-TLS-protected issuer endpoint.
func PublicJWKS(keyID string, key *rsa.PublicKey) ([]byte, error) {
	return PublicJWKSKeys([]JWKSKey{{KeyID: keyID, PublicKey: key}})
}

func PublicJWKSKeys(keys []JWKSKey) ([]byte, error) {
	if len(keys) == 0 {
		return nil, errors.New("at least one public signing key is required")
	}
	items := make([]struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
	}, len(keys))
	seenIDs := make(map[string]bool, len(keys))
	seenKeys := make(map[string]bool, len(keys))
	for i, candidate := range keys {
		keyID := strings.TrimSpace(candidate.KeyID)
		key := candidate.PublicKey
		if keyID == "" || len(keyID) > 128 || strings.ContainsAny(keyID, " \t\r\n") || key == nil || key.N == nil || key.N.Sign() <= 0 || key.E < 3 || key.E%2 == 0 {
			return nil, errors.New("invalid public signing key")
		}
		keyFingerprint := key.N.String() + ":" + big.NewInt(int64(key.E)).String()
		if seenIDs[keyID] || seenKeys[keyFingerprint] {
			return nil, errors.New("JWKS key IDs and public keys must be distinct")
		}
		seenIDs[keyID], seenKeys[keyFingerprint] = true, true
		exponent := big.NewInt(int64(key.E)).Bytes()
		items[i] = struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		}{Kty: "RSA", Kid: keyID, Use: "sig", Alg: "RS256", N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(exponent)}
	}
	return json.Marshal(struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}{Keys: items})
}

func JWKSHandler(keyID string, key *rsa.PublicKey) http.Handler {
	return JWKSHandlerKeys([]JWKSKey{{KeyID: keyID, PublicKey: key}})
}

func JWKSHandlerKeys(keys []JWKSKey) http.Handler {
	document, err := PublicJWKSKeys(keys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err != nil {
			http.Error(w, "JWKS unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/jwk-set+json")
		w.Header().Set("Cache-Control", "public, max-age=30")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(document)
	})
}
