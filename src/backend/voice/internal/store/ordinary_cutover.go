package store

import (
	"context"
	"encoding/json"
	callsv1 "voice.app/voice/calls/v1"
)

// ScanOrdinaryCutover is a startup inventory, not a concurrent legacy-writer
// fence. Operators must quiesce every legacy writer and drain its media first.
// Foreign resource owners are excluded; malformed cache data fails closed.
func (s *RedisCallStore) ScanOrdinaryCutover(ctx context.Context, check func(context.Context, Call) error) error {
	if s == nil || s.client == nil || check == nil {
		return ErrInvalidState
	}
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, s.prefix+"call:*", 128).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			raw, err := s.client.Get(ctx, key).Bytes()
			if err != nil {
				return err
			}
			var c Call
			if err := json.Unmarshal(raw, &c); err != nil || key != s.callKey(c.RoomID) {
				return ErrInvalidState
			}
			if c.ManagedGameSession || c.MatchSquadMatchID != "" || c.SpaceID != "" || c.VoiceRoomID != "" || c.ApplicationID != "" || c.EnvironmentID != "" || c.SessionID != "" || c.IsVoiceRoom() {
				continue
			}
			if c.Status == callsv1.CallStatus_CALL_STATUS_RINGING || c.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
				if err := check(ctx, c); err != nil {
					return err
				}
			}
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}
