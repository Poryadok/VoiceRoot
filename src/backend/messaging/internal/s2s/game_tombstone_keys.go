package s2s

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"voice/backend/messaging/internal/gameprotocol"
)

// GameTombstoneKeySource holds a Messaging-owned signing key. Callers should
// never log this value; its public representation is available via PublicJWKS.
type GameTombstoneKeySource struct {
	key gameprotocol.GameMessageTombstoneKey
}

type GameTombstoneKeyConfig struct {
	KeyID          string
	PrivateKeyFile string
	NotBefore      string
	NotAfter       string
}

func LoadGameTombstoneKeySource(config GameTombstoneKeyConfig) (*GameTombstoneKeySource, error) {
	if config.KeyID == "" || config.PrivateKeyFile == "" || config.NotBefore == "" || config.NotAfter == "" {
		return nil, errors.New("messaging tombstone key configuration is incomplete")
	}
	id, err := uuid.Parse(config.KeyID)
	if err != nil || id.String() != config.KeyID {
		return nil, errors.New("messaging tombstone key id must be a canonical UUID")
	}
	notBefore, err := time.Parse(time.RFC3339, config.NotBefore)
	if err != nil {
		return nil, errors.New("messaging tombstone key not-before must be RFC3339")
	}
	notAfter, err := time.Parse(time.RFC3339, config.NotAfter)
	if err != nil || !notBefore.Before(notAfter) {
		return nil, errors.New("messaging tombstone key validity window is invalid")
	}
	pemBytes, err := os.ReadFile(config.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read messaging tombstone private key: %w", err)
	}
	block, rest := pem.Decode(pemBytes)
	if block == nil || len(rest) != 0 {
		return nil, errors.New("messaging tombstone private key must be one PEM block")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("messaging tombstone private key must be PKCS8 Ed25519")
	}
	privateKey, ok := parsed.(ed25519.PrivateKey)
	if !ok || len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("messaging tombstone private key must be Ed25519")
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return &GameTombstoneKeySource{key: gameprotocol.GameMessageTombstoneKey{KeyID: id, PublicKey: publicKey, PrivateKey: privateKey, NotBefore: notBefore.UTC(), NotAfter: notAfter.UTC()}}, nil
}

func (s *GameTombstoneKeySource) CurrentGameTombstoneKey(context.Context) (gameprotocol.GameMessageTombstoneKey, error) {
	if s == nil || len(s.key.PrivateKey) != ed25519.PrivateKeySize {
		return gameprotocol.GameMessageTombstoneKey{}, errors.New("messaging tombstone signing key unavailable")
	}
	return s.key, nil
}

func (s *GameTombstoneKeySource) PublicJWKS() ([]byte, error) {
	if s == nil || len(s.key.PublicKey) != ed25519.PublicKeySize {
		return nil, errors.New("messaging tombstone verification key unavailable")
	}
	document := map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA", "kid": s.key.KeyID.String(), "x": base64.RawURLEncoding.EncodeToString(s.key.PublicKey)}}}
	return json.Marshal(document)
}

func (s *GameTombstoneKeySource) JWKSHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/.well-known/game-tombstone-jwks.json" {
			http.NotFound(writer, request)
			return
		}
		document, err := s.PublicJWKS()
		if err != nil {
			http.Error(writer, "verification keys unavailable", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/jwk-set+json")
		writer.Header().Set("Cache-Control", "public, max-age=30")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(document)
	})
}
