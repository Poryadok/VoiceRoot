package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"voice/backend/pkg/principal"
)

var moderationKeyID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// One issuer is shared by evidence, Auth reconciliation and report thresholds.
// Production activation requires the explicit current+next mounted key ring.
func loadModerationIssuerFromEnv() (*principal.Issuer, []principal.JWKSKey, error) {
	for _, alias := range []string{"S2S_SIGNING_KEY_PEM", "S2S_SIGNING_KID"} {
		if _, set := os.LookupEnv(alias); set {
			return nil, nil, errors.New("unsupported moderation principal signing alias")
		}
	}
	return loadModerationIssuer(strings.TrimSpace(os.Getenv("MODERATION_PRINCIPAL_SIGNING_KEYS_DIR")), strings.TrimSpace(os.Getenv("MODERATION_PRINCIPAL_ACTIVE_KID")))
}
func loadModerationIssuer(directory, active string) (*principal.Issuer, []principal.JWKSKey, error) {
	if directory == "" || !moderationKeyID.MatchString(active) {
		return nil, nil, errors.New("moderation principal key directory and active kid are required")
	}
	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, err
	}
	var public []principal.JWKSKey
	var issuer *principal.Issuer
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), "..") {
			continue
		}
		if filepath.Ext(entry.Name()) != ".pem" {
			return nil, nil, errors.New("moderation principal key ring contains an unexpected file")
		}
		kid := strings.TrimSuffix(entry.Name(), ".pem")
		if !moderationKeyID.MatchString(kid) {
			return nil, nil, errors.New("invalid moderation principal key id")
		}
		file, err := filepath.EvalSymlinks(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, nil, err
		}
		relative, err := filepath.Rel(root, file)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return nil, nil, errors.New("moderation signing key escapes its configured directory")
		}
		info, err := os.Stat(file)
		if err != nil || !info.Mode().IsRegular() {
			return nil, nil, errors.New("moderation signing key is not a regular file")
		}
		key, err := principal.LoadRSAPrivateKeyFile(file)
		if err != nil {
			return nil, nil, err
		}
		if key.N.BitLen() < 2048 {
			return nil, nil, errors.New("moderation RSA signing key is too small")
		}
		public = append(public, principal.JWKSKey{KeyID: kid, PublicKey: &key.PublicKey})
		if kid == active {
			issuer, err = principal.NewIssuer(principal.IssuerConfig{Issuer: "moderation", KeyID: kid, PrivateKey: key})
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if len(public) != 2 || issuer == nil {
		return nil, nil, errors.New("moderation requires exactly two distinct current/next keys and a present active kid")
	}
	if _, err := principal.PublicJWKSKeys(public); err != nil {
		return nil, nil, err
	}
	return issuer, public, nil
}
