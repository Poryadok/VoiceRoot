package main

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"voice/backend/pkg/principal"
)

const (
	managedChatPurgeSigningKeysDirEnv = "MESSAGING_PURGE_PRINCIPAL_SIGNING_KEYS_DIR"
	managedChatPurgeActiveKIDEnv      = "MESSAGING_PURGE_PRINCIPAL_ACTIVE_KID"
)

var managedChatPurgeKIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func loadManagedChatPurgePrincipal() (*principal.Issuer, http.Handler, bool, error) {
	dirRaw, dirSet := os.LookupEnv(managedChatPurgeSigningKeysDirEnv)
	kidRaw, kidSet := os.LookupEnv(managedChatPurgeActiveKIDEnv)
	if !dirSet && !kidSet {
		return nil, nil, false, nil
	}
	dir, activeKID := strings.TrimSpace(dirRaw), strings.TrimSpace(kidRaw)
	if dir == "" || activeKID == "" || !managedChatPurgeKIDPattern.MatchString(activeKID) {
		return nil, nil, true, errors.New("managed chat purge principal key directory and active KID are required")
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, nil, true, fmt.Errorf("resolve managed chat purge principal key directory: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, nil, true, errors.New("managed chat purge principal key directory is invalid")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, true, err
	}
	keys := make(map[string]*rsa.PrivateKey, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), "..") {
			continue
		}
		if filepath.Ext(entry.Name()) != ".pem" {
			return nil, nil, true, fmt.Errorf("managed chat purge principal key %q must be a PEM file", entry.Name())
		}
		kid := strings.TrimSuffix(entry.Name(), ".pem")
		if !managedChatPurgeKIDPattern.MatchString(kid) {
			return nil, nil, true, errors.New("managed chat purge principal key ID is invalid")
		}
		path := filepath.Join(root, entry.Name())
		fileInfo, err := os.Lstat(path)
		if err != nil || !fileInfo.Mode().IsRegular() {
			return nil, nil, true, fmt.Errorf("managed chat purge principal key %q must be a regular file", entry.Name())
		}
		key, err := principal.LoadRSAPrivateKeyFile(path)
		if err != nil || key.N.BitLen() < 2048 {
			return nil, nil, true, fmt.Errorf("managed chat purge principal key %q is invalid", entry.Name())
		}
		keys[kid] = key
	}
	if len(keys) != 2 {
		return nil, nil, true, errors.New("managed chat purge principal key directory must contain exactly two rotation keys")
	}
	active := keys[activeKID]
	if active == nil {
		return nil, nil, true, errors.New("managed chat purge principal active KID is missing")
	}
	publicKeys := make([]principal.JWKSKey, 0, 2)
	var first *rsa.PublicKey
	for kid, key := range keys {
		if first != nil && first.N.Cmp(key.N) == 0 && first.E == key.E {
			return nil, nil, true, errors.New("managed chat purge principal rotation keys must be distinct")
		}
		first = &key.PublicKey
		publicKeys = append(publicKeys, principal.JWKSKey{KeyID: kid, PublicKey: &key.PublicKey})
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: activeKID, PrivateKey: active})
	if err != nil {
		return nil, nil, true, err
	}
	mux := http.NewServeMux()
	mux.Handle("/.well-known/principal-jwks.json", principal.JWKSHandlerKeys(publicKeys))
	return issuer, mux, true, nil
}
