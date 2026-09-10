package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"voice/backend/pkg/principal"
)

const (
	gatewayPrincipalSigningKeysDirEnv = "GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR"
	gatewayPrincipalActiveKIDEnv      = "GATEWAY_PRINCIPAL_ACTIVE_KID"
)

type principalJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type principalJWKS struct {
	Keys []principalJWK `json:"keys"`
}

// loadGatewayPrincipalIssuerFromEnv keeps the Phase-0 signer disabled until both
// values are explicitly configured. Partial or unusable configuration is a
// startup error: issuing a principal with an accidental key is not safe.
func loadGatewayPrincipalIssuerFromEnv() (*principal.Issuer, principalJWKS, error) {
	if _, set := os.LookupEnv("S2S_SIGNING_KEY_PEM"); set {
		return nil, principalJWKS{}, errors.New("S2S_SIGNING_KEY_PEM is not supported; configure Gateway principal key directory")
	}
	if _, set := os.LookupEnv("S2S_SIGNING_KID"); set {
		return nil, principalJWKS{}, errors.New("S2S_SIGNING_KID is not supported; configure GATEWAY_PRINCIPAL_ACTIVE_KID")
	}
	dir, dirSet := os.LookupEnv(gatewayPrincipalSigningKeysDirEnv)
	kid, kidSet := os.LookupEnv(gatewayPrincipalActiveKIDEnv)
	dir, kid = strings.TrimSpace(dir), strings.TrimSpace(kid)
	if !dirSet && !kidSet {
		return nil, principalJWKS{}, nil
	}
	if dir == "" || kid == "" {
		return nil, principalJWKS{}, fmt.Errorf("%s and %s must both be configured", gatewayPrincipalSigningKeysDirEnv, gatewayPrincipalActiveKIDEnv)
	}
	if !validPrincipalKeyID(kid) {
		return nil, principalJWKS{}, fmt.Errorf("%s is invalid", gatewayPrincipalActiveKIDEnv)
	}
	canonicalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, principalJWKS{}, fmt.Errorf("resolve principal signing key directory: %w", err)
	}
	info, err := os.Stat(canonicalDir)
	if err != nil || !info.IsDir() {
		return nil, principalJWKS{}, errors.New("principal signing key directory is not a directory")
	}
	entries, err := os.ReadDir(canonicalDir)
	if err != nil {
		return nil, principalJWKS{}, fmt.Errorf("read principal signing key directory: %w", err)
	}
	keys := make(map[string]*rsa.PrivateKey)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "..") {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".pem") {
			return nil, principalJWKS{}, fmt.Errorf("principal signing key file %q must end in .pem", entry.Name())
		}
		keyID := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if !validPrincipalKeyID(keyID) {
			return nil, principalJWKS{}, fmt.Errorf("principal signing key file %q has invalid kid", entry.Name())
		}
		keyPath, err := resolvedPrincipalKeyPath(canonicalDir, entry.Name())
		if err != nil {
			return nil, principalJWKS{}, err
		}
		privateKey, err := readRSAPrivateKey(keyPath)
		if err != nil {
			return nil, principalJWKS{}, fmt.Errorf("read principal signing key %q: %w", entry.Name(), err)
		}
		keys[keyID] = privateKey
	}
	if len(keys) != 2 {
		return nil, principalJWKS{}, errors.New("principal signing key directory must contain exactly two keys")
	}
	if duplicatePrincipalPublicKey(keys) {
		return nil, principalJWKS{}, errors.New("principal signing key directory contains duplicate public keys")
	}
	current, ok := keys[kid]
	if !ok {
		return nil, principalJWKS{}, fmt.Errorf("principal current signing key %q is missing", kid)
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: kid, PrivateKey: current})
	if err != nil {
		return nil, principalJWKS{}, fmt.Errorf("create principal issuer: %w", err)
	}
	return issuer, principalJWKSFromPrivateKeys(keys), nil
}

func resolvedPrincipalKeyPath(canonicalDir, name string) (string, error) {
	resolved, err := filepath.EvalSymlinks(filepath.Join(canonicalDir, name))
	if err != nil {
		return "", fmt.Errorf("resolve principal signing key %q: %w", name, err)
	}
	relative, err := filepath.Rel(canonicalDir, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("principal signing key %q resolves outside its configured directory", name)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("principal signing key %q is not a regular file", name)
	}
	return resolved, nil
}

func readRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(encoded)
	if block == nil || strings.TrimSpace(string(rest)) != "" {
		return nil, errors.New("expected exactly one PEM block")
	}
	var privateKey any
	if block.Type == "PRIVATE KEY" {
		privateKey, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	} else {
		return nil, fmt.Errorf("unsupported PEM block %q", block.Type)
	}
	if err != nil {
		return nil, err
	}
	key, ok := privateKey.(*rsa.PrivateKey)
	if !ok || key.Validate() != nil || key.N.BitLen() < 2048 {
		return nil, errors.New("expected a valid RSA key of at least 2048 bits")
	}
	return key, nil
}

func duplicatePrincipalPublicKey(keys map[string]*rsa.PrivateKey) bool {
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		fingerprint := key.N.Text(16) + ":" + fmt.Sprintf("%d", key.E)
		if _, exists := seen[fingerprint]; exists {
			return true
		}
		seen[fingerprint] = struct{}{}
	}
	return false
}

func principalJWKSFromPrivateKeys(keys map[string]*rsa.PrivateKey) principalJWKS {
	kids := make([]string, 0, len(keys))
	for kid := range keys {
		kids = append(kids, kid)
	}
	sort.Strings(kids)
	document := principalJWKS{Keys: make([]principalJWK, 0, len(kids))}
	for _, kid := range kids {
		key := keys[kid].PublicKey
		document.Keys = append(document.Keys, principalJWK{Kty: "RSA", Kid: kid, Use: "sig", Alg: "RS256", N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())})
	}
	return document
}

func validPrincipalKeyID(value string) bool {
	if len(value) == 0 || len(value) > 128 || (value[0] < 'a' || value[0] > 'z') && (value[0] < 'A' || value[0] > 'Z') && (value[0] < '0' || value[0] > '9') {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '.' && character != '_' && character != '-' {
			return false
		}
	}
	return true
}
