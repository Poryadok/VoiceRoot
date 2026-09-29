package principaljwks

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net"
	"strings"

	"voice/backend/pkg/principal"
)

type Config struct {
	ListenAddr     string
	KeyID          string
	SigningKey     *rsa.PrivateKey
	NextKeyID      string
	NextSigningKey *rsa.PrivateKey
	TLSCert        string
	TLSKey         string
}

func LoadFromEnv(getenv func(string) string) (Config, bool, error) {
	if getenv == nil {
		return Config{}, false, errors.New("environment reader is required")
	}
	privatePath := strings.TrimSpace(getenv("VOICE_PRINCIPAL_PRIVATE_KEY_FILE"))
	keyID := strings.TrimSpace(getenv("VOICE_PRINCIPAL_KID"))
	nextPrivatePath := strings.TrimSpace(getenv("VOICE_PRINCIPAL_NEXT_PRIVATE_KEY_FILE"))
	nextKeyID := strings.TrimSpace(getenv("VOICE_PRINCIPAL_NEXT_KID"))
	cert := strings.TrimSpace(getenv("VOICE_PRINCIPAL_JWKS_TLS_CERT_FILE"))
	key := strings.TrimSpace(getenv("VOICE_PRINCIPAL_JWKS_TLS_KEY_FILE"))
	listen := strings.TrimSpace(getenv("VOICE_PRINCIPAL_JWKS_LISTEN_ADDR"))
	configured := 0
	for _, value := range []string{privatePath, keyID, nextPrivatePath, nextKeyID, cert, key, listen} {
		if value != "" {
			configured++
		}
	}
	if configured == 0 {
		return Config{}, false, nil
	}
	if privatePath == "" || keyID == "" || nextPrivatePath == "" || nextKeyID == "" || cert == "" || key == "" {
		return Config{}, true, errors.New("VOICE principal current/next signers and JWKS TLS files must be configured together")
	}
	if len(keyID) > 128 || strings.ContainsAny(keyID, " \t\r\n") || len(nextKeyID) > 128 || strings.ContainsAny(nextKeyID, " \t\r\n") {
		return Config{}, true, errors.New("invalid VOICE_PRINCIPAL_KID")
	}
	if keyID == nextKeyID {
		return Config{}, true, errors.New("current and next VOICE principal key IDs must be distinct")
	}
	if listen == "" {
		listen = ":8443"
	}
	if _, _, err := net.SplitHostPort(listen); err != nil {
		return Config{}, true, fmt.Errorf("invalid VOICE_PRINCIPAL_JWKS_LISTEN_ADDR")
	}
	signingKey, err := principal.LoadRSAPrivateKeyFile(privatePath)
	if err != nil || signingKey.N.BitLen() < 2048 {
		return Config{}, true, errors.New("VOICE principal signing key is unavailable or too small")
	}
	nextSigningKey, err := principal.LoadRSAPrivateKeyFile(nextPrivatePath)
	if err != nil || nextSigningKey.N.BitLen() < 2048 {
		return Config{}, true, errors.New("VOICE next principal signing key is unavailable or too small")
	}
	if signingKey.PublicKey.Equal(&nextSigningKey.PublicKey) {
		return Config{}, true, errors.New("current and next VOICE principal public keys must be distinct")
	}
	return Config{ListenAddr: listen, KeyID: keyID, SigningKey: signingKey, NextKeyID: nextKeyID, NextSigningKey: nextSigningKey, TLSCert: cert, TLSKey: key}, true, nil
}
