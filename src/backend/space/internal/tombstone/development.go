// Package tombstone keeps account HMAC computation behind the Space workload.
package tombstone

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"os"
	"time"
)

// DevelopmentKey is a local acceptance fixture provider, never a production
// KMS/HSM implementation. Production callers must inject an audited provider.
type DevelopmentKey struct {
	key       []byte
	version   string
	createdAt time.Time
	now       func() time.Time
}

func LoadDevelopmentKey(path string) (*DevelopmentKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("development tombstone key unavailable")
	}
	var data struct {
		Version   string    `json:"version"`
		KeyBase64 string    `json:"key_base64"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		return nil, errors.New("invalid development tombstone key")
	}
	key, err := base64.StdEncoding.DecodeString(data.KeyBase64)
	if err != nil || len(key) != 32 || data.Version == "" || data.CreatedAt.IsZero() {
		return nil, errors.New("invalid development tombstone key")
	}
	provider := &DevelopmentKey{key: key, version: data.Version, createdAt: data.CreatedAt, now: time.Now}
	if !provider.usable() {
		return nil, errors.New("development tombstone key is outside its rotation window")
	}
	return provider, nil
}
func (p *DevelopmentKey) usable() bool {
	if p == nil || p.now == nil {
		return false
	}
	now := p.now().UTC()
	return !now.Before(p.createdAt) && now.Before(p.createdAt.Add(90*24*time.Hour))
}
func (p *DevelopmentKey) HashAccount(ctx context.Context, id uuid.UUID) ([]byte, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if id == uuid.Nil || !p.usable() {
		return nil, "", errors.New("tombstone HMAC provider unavailable")
	}
	digest := hmac.New(sha256.New, p.key)
	digest.Write([]byte("voice-space-tombstone-v1\x00"))
	digest.Write(id[:])
	return digest.Sum(nil), p.version, nil
}
