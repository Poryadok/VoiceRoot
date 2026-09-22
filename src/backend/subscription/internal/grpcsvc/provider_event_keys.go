package grpcsvc

import (
	"context"
	"errors"
	"strings"
)

// StaticProviderEventHMACKeys provides one retained key version. Deployments
// can replace this source with a rotation-aware secret provider without
// weakening replay verification at the gRPC boundary.
type StaticProviderEventHMACKeys struct {
	version string
	key     []byte
}

func NewStaticProviderEventHMACKeys(version string, key []byte) *StaticProviderEventHMACKeys {
	return &StaticProviderEventHMACKeys{version: strings.TrimSpace(version), key: append([]byte(nil), key...)}
}

func (s *StaticProviderEventHMACKeys) CurrentProviderEventHMACKey(context.Context, string) (string, []byte, error) {
	if s == nil || s.version == "" || len(s.key) == 0 {
		return "", nil, errors.New("provider event HMAC key unavailable")
	}
	return s.version, append([]byte(nil), s.key...), nil
}

func (s *StaticProviderEventHMACKeys) ProviderEventHMACVerificationKeys(ctx context.Context, provider string) ([]ProviderEventHMACKey, error) {
	version, key, err := s.CurrentProviderEventHMACKey(ctx, provider)
	if err != nil {
		return nil, err
	}
	return []ProviderEventHMACKey{{Version: version, Key: key}}, nil
}
