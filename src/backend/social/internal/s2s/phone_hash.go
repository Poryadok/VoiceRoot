package s2s

import (
	"context"

	"github.com/google/uuid"
)

// StaticPhoneHashLookup maps hashed phone numbers to profile IDs (tests and staged rollout).
type StaticPhoneHashLookup struct {
	ByHash map[string]uuid.UUID
}

func (s *StaticPhoneHashLookup) ProfileIDsByPhoneHashes(_ context.Context, hashes []string) (map[string]uuid.UUID, error) {
	out := make(map[string]uuid.UUID, len(hashes))
	for _, h := range hashes {
		if id, ok := s.ByHash[h]; ok {
			out[h] = id
		}
	}
	return out, nil
}

// EmptyPhoneHashLookup returns no matches when Auth phone-hash RPC is not wired yet.
type EmptyPhoneHashLookup struct{}

func (EmptyPhoneHashLookup) ProfileIDsByPhoneHashes(context.Context, []string) (map[string]uuid.UUID, error) {
	return map[string]uuid.UUID{}, nil
}
