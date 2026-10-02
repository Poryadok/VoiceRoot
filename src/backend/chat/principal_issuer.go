package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"voice/backend/pkg/principal"
)

const (
	chatPrincipalKeysDirEnv   = "CHAT_PRINCIPAL_SIGNING_KEYS_DIR"
	chatPrincipalActiveKIDEnv = "CHAT_PRINCIPAL_ACTIVE_KID"
)

type chatPrincipalJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type chatPrincipalJWKS struct {
	Keys []chatPrincipalJWK `json:"keys"`
}

func loadChatPrincipalIssuerFromEnv() (*principal.Issuer, chatPrincipalJWKS, error) {
	directory, dirSet := os.LookupEnv(chatPrincipalKeysDirEnv)
	kid, kidSet := os.LookupEnv(chatPrincipalActiveKIDEnv)
	directory, kid = strings.TrimSpace(directory), strings.TrimSpace(kid)
	if !dirSet && !kidSet {
		return nil, chatPrincipalJWKS{}, nil
	}
	if directory == "" || kid == "" {
		return nil, chatPrincipalJWKS{}, fmt.Errorf("%s and %s must both be configured", chatPrincipalKeysDirEnv, chatPrincipalActiveKIDEnv)
	}
	if !validChatPrincipalKID(kid) {
		return nil, chatPrincipalJWKS{}, errors.New("chat active principal key ID is invalid")
	}
	canonicalDir, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, chatPrincipalJWKS{}, fmt.Errorf("resolve Chat principal signing key directory: %w", err)
	}
	info, err := os.Stat(canonicalDir)
	if err != nil || !info.IsDir() {
		return nil, chatPrincipalJWKS{}, errors.New("chat principal signing key path is not a directory")
	}
	entries, err := os.ReadDir(canonicalDir)
	if err != nil {
		return nil, chatPrincipalJWKS{}, err
	}
	keys := make(map[string]*rsa.PrivateKey)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".pem") {
			return nil, chatPrincipalJWKS{}, fmt.Errorf("chat principal key %q must be PEM", entry.Name())
		}
		keyID := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if !validChatPrincipalKID(keyID) {
			return nil, chatPrincipalJWKS{}, fmt.Errorf("chat principal key %q has invalid ID", entry.Name())
		}
		path, err := filepath.EvalSymlinks(filepath.Join(canonicalDir, entry.Name()))
		if err != nil {
			return nil, chatPrincipalJWKS{}, fmt.Errorf("resolve Chat principal key %q: %w", entry.Name(), err)
		}
		relative, err := filepath.Rel(canonicalDir, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return nil, chatPrincipalJWKS{}, errors.New("chat principal key resolves outside configured directory")
		}
		fileInfo, err := os.Stat(path)
		if err != nil || !fileInfo.Mode().IsRegular() {
			return nil, chatPrincipalJWKS{}, errors.New("chat principal key is not a regular file")
		}
		encoded, err := os.ReadFile(path)
		if err != nil {
			return nil, chatPrincipalJWKS{}, err
		}
		block, rest := pem.Decode(encoded)
		if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
			return nil, chatPrincipalJWKS{}, errors.New("chat principal key must contain one PKCS#8 PEM block")
		}
		private, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, chatPrincipalJWKS{}, fmt.Errorf("parse Chat principal key %q: %w", entry.Name(), err)
		}
		key, ok := private.(*rsa.PrivateKey)
		if !ok || key.Validate() != nil || key.N.BitLen() < 2048 {
			return nil, chatPrincipalJWKS{}, errors.New("chat principal key must be valid RSA of at least 2048 bits")
		}
		keys[keyID] = key
	}
	if len(keys) != 2 {
		return nil, chatPrincipalJWKS{}, errors.New("chat principal key directory must contain exactly two rotation keys")
	}
	active := keys[kid]
	if active == nil {
		return nil, chatPrincipalJWKS{}, errors.New("chat active principal key is missing")
	}
	for id, key := range keys {
		if id != kid && active.N.Cmp(key.N) == 0 {
			return nil, chatPrincipalJWKS{}, errors.New("chat principal rotation keys must have distinct public keys")
		}
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: kid, PrivateKey: active})
	if err != nil {
		return nil, chatPrincipalJWKS{}, err
	}
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	doc := chatPrincipalJWKS{Keys: make([]chatPrincipalJWK, 0, len(ids))}
	for _, id := range ids {
		key := keys[id].PublicKey
		doc.Keys = append(doc.Keys, chatPrincipalJWK{Kty: "RSA", Kid: id, Use: "sig", Alg: "RS256", N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
	}
	return issuer, doc, nil
}

func validChatPrincipalKID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || i > 0 && (r == '.' || r == '_' || r == '-') {
			continue
		}
		return false
	}
	return true
}

func chatPrincipalJWKSHandler(document chatPrincipalJWKS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(document.Keys) == 0 {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=30")
		if err := json.NewEncoder(w).Encode(document); err != nil {
			http.Error(w, "encode public keys", http.StatusInternalServerError)
		}
	})
}
