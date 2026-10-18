package store

import "context"

// ApplyVoiceRosterEvent is the durable inbox/whole-party reset boundary.
// The test-first placeholder is replaced before any runtime wiring.
func (s *MatchStore) ApplyVoiceRosterEvent(ctx context.Context, raw []byte) (VoiceRosterResetResult, error) {
	return VoiceRosterResetResult{}, nil
}
