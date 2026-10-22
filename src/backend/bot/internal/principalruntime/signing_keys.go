package principalruntime

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"voice/backend/pkg/principal"
)

var botSigningKID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// Canonical directory mode is an immutable startup snapshot. It publishes the
// peer before use while only ACTIVE_KID signs. Existing file-mode callers retain
// their original single-key behavior until explicitly configured for rotation.
func loadBotSigningKeys(cfg Config) (*rsa.PrivateKey, string, []principal.JWKSKey, error) {
	if cfg.BotSigningKeysDir == "" && cfg.BotActiveKID == "" {
		if strings.TrimSpace(cfg.BotPrivateKeyFile) == "" || strings.TrimSpace(cfg.BotKeyID) == "" {
			return nil, "", nil, errors.New("bot principal signer is incomplete")
		}
		key, err := principal.LoadRSAPrivateKeyFile(cfg.BotPrivateKeyFile)
		if err != nil {
			return nil, "", nil, err
		}
		return key, cfg.BotKeyID, []principal.JWKSKey{{KeyID: cfg.BotKeyID, PublicKey: &key.PublicKey}}, nil
	}
	if cfg.BotSigningKeysDir == "" || !botSigningKID.MatchString(cfg.BotActiveKID) || cfg.BotPrivateKeyFile != "" || cfg.BotKeyID != "" {
		return nil, "", nil, errors.New("invalid bot canonical signing configuration")
	}
	dir, err := filepath.EvalSymlinks(cfg.BotSigningKeysDir)
	if err != nil {
		return nil, "", nil, fmt.Errorf("resolve bot signing directory: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", nil, err
	}
	var active *rsa.PrivateKey
	public := make([]principal.JWKSKey, 0, 2)
	for _, entry := range entries {
		// Kubernetes secret-volume implementation directories are not key entries.
		if entry.IsDir() || strings.HasPrefix(entry.Name(), "..") {
			continue
		}
		kid := strings.TrimSuffix(entry.Name(), ".pem")
		if kid == entry.Name() || !botSigningKID.MatchString(kid) {
			return nil, "", nil, errors.New("bot signing files must use valid <kid>.pem names")
		}
		path, err := filepath.EvalSymlinks(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, "", nil, err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, "", nil, errors.New("bot signing key escapes configured directory")
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, "", nil, errors.New("bot signing key is not a regular file")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, "", nil, err
		}
		block, rest := pem.Decode(raw)
		if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
			return nil, "", nil, errors.New("bot signing key must be one unencrypted PKCS8 PEM block")
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, "", nil, errors.New("invalid bot PKCS8 signing key")
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok || key.Validate() != nil || key.N.BitLen() < 2048 {
			return nil, "", nil, errors.New("bot signing key must be valid RSA of at least 2048 bits")
		}
		for _, prior := range public {
			if prior.PublicKey.N.Cmp(key.N) == 0 {
				return nil, "", nil, errors.New("bot current and next RSA moduli must differ")
			}
		}
		public = append(public, principal.JWKSKey{KeyID: kid, PublicKey: &key.PublicKey})
		if kid == cfg.BotActiveKID {
			active = key
		}
	}
	if len(public) != 2 || active == nil {
		return nil, "", nil, errors.New("bot signing directory must contain exactly two keys including active kid")
	}
	return active, cfg.BotActiveKID, public, nil
}
