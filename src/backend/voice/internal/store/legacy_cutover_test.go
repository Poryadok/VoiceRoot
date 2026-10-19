package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
)

func TestLegacyCutoverCaptureDoesNotGrantWholeDocumentDeletion(t *testing.T) {
	s, client, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	raw, err := json.Marshal(Call{RoomID: "room", ChatID: "chat", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE, States: map[string]ParticipantState{"profile": {ProfileID: "profile"}}})
	require.NoError(t, err)
	require.NoError(t, client.Set(ctx, prefix+"call:room", raw, time.Hour).Err())
	require.NoError(t, client.Set(ctx, prefix+"session:profile", "room", time.Hour).Err())
	require.NoError(t, client.Set(ctx, prefix+"active_chat:chat", "room", time.Hour).Err())
	require.NoError(t, client.Set(ctx, "other:call:room", "untouched", 0).Err())
	snapshot, err := s.CaptureLegacyCutover(ctx)
	require.NoError(t, err)
	require.Equal(t, prefix, snapshot.Prefix)
	require.Len(t, snapshot.Entries, 3)
	require.ErrorIs(t, s.CompareCleanupLegacyCutover(ctx, snapshot), ErrLegacyPartitionRequired)
	require.EqualValues(t, 3, client.Exists(ctx, prefix+"call:room", prefix+"session:profile", prefix+"active_chat:chat").Val())
	require.Equal(t, "untouched", client.Get(ctx, "other:call:room").Val())
	// Neither replay nor raw equality supplies missing semantic ownership.
	require.ErrorIs(t, s.CompareCleanupLegacyCutover(ctx, snapshot), ErrLegacyPartitionRequired)
}

func TestLegacyCutoverNewerValuePreventsPartialCleanup(t *testing.T) {
	s, client, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	raw, err := json.Marshal(Call{RoomID: "room"})
	require.NoError(t, err)
	require.NoError(t, client.Set(ctx, prefix+"call:room", raw, 0).Err())
	snapshot, err := s.CaptureLegacyCutover(ctx)
	require.NoError(t, err)
	require.NoError(t, client.Set(ctx, prefix+"call:room", `{"room_id":"room","ordinary_version":2}`, 0).Err())
	require.ErrorIs(t, s.CompareCleanupLegacyCutover(ctx, snapshot), ErrLegacyPartitionRequired)
	require.Equal(t, `{"room_id":"room","ordinary_version":2}`, client.Get(ctx, prefix+"call:room").Val())
}

func TestLegacyCutoverMalformedDanglingAndUnknownEvidenceBlocksCapture(t *testing.T) {
	for _, tc := range []struct{ name, key, value string }{
		{"malformed", "call:room", "bad"},
		{"wrong-room", "call:room", `{"room_id":"another"}`},
		{"dangling", "session:profile", "expired-room"},
		{"unexplained", "unknown:record", "unexplained"},
		{"unknown-json-field", "call:room", `{"room_id":"room","future_unowned_fact":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, client, prefix := newLegacyRedisFixture(t)
			require.NoError(t, client.Set(context.Background(), prefix+tc.key, tc.value, 0).Err())
			_, err := s.CaptureLegacyCutover(context.Background())
			require.ErrorIs(t, err, ErrInvalidState)
			require.EqualValues(t, 1, client.Exists(context.Background(), prefix+tc.key).Val())
		})
	}
}

func TestLegacyAtomicCaptureRetainsExpiryAndUnsupportedRawDump(t *testing.T) {
	s, c, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, prefix+"call:room", `{"room_id":"room"}`, time.Hour).Err())
	deadline, err := c.PExpireTime(ctx, prefix+"call:room").Result()
	require.NoError(t, err)
	capture, err := s.CaptureLegacyCutover(ctx)
	require.NoError(t, err)
	require.Positive(t, capture.ObservedAtUnixMilli)
	require.Equal(t, "DEADLINE", capture.Entries[0].Expiry)
	require.Equal(t, deadline.Milliseconds(), capture.Entries[0].ExpiresAtUnixMilli)
	require.NotEmpty(t, capture.Entries[0].Dump)
	require.NoError(t, c.RPush(ctx, prefix+"unknown:list", string([]byte{0, 255, 1})).Err())
	diagnostic, err := s.CaptureLegacyCutover(ctx)
	require.Error(t, err)
	require.Equal(t, "UNSUPPORTED_REDIS_TYPE", diagnostic.Diagnostic)
	require.Len(t, diagnostic.Entries, 2)
	require.NotEmpty(t, diagnostic.Entries[1].Dump)
}

func TestLegacyMoveReceiptSurvivesCallTTLWithoutRecreatingHistory(t *testing.T) {
	s, c, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	ledger := redisVoiceRoomMoveLedger{Request: VoiceRoomMoveRequest{ActorProfileID: "actor", OperationID: "move", ParticipantProfileID: "profile", SpaceID: "space", FromVoiceRoomID: "from", ToVoiceRoomID: "to", DestinationRoomID: "destination"}, Result: VoiceRoomMoveResult{Source: Call{RoomID: "source", SpaceID: "space", VoiceRoomID: "from"}, Destination: Call{RoomID: "destination", SpaceID: "space", VoiceRoomID: "to"}}}
	raw, err := json.Marshal(ledger)
	require.NoError(t, err)
	require.NoError(t, c.Set(ctx, prefix+"voice_move_operation:actor:move", raw, 0).Err())
	capture, err := s.CaptureLegacyCutover(ctx)
	require.NoError(t, err)
	require.Len(t, capture.Entries, 1)
	require.Equal(t, string(raw), capture.Entries[0].Value)
	require.EqualValues(t, 0, c.Exists(ctx, prefix+"call:source", prefix+"call:destination").Val())
	ledger.Result.Destination.SpaceID = "foreign"
	raw, err = json.Marshal(ledger)
	require.NoError(t, err)
	require.NoError(t, c.Set(ctx, prefix+"voice_move_operation:actor:move", raw, 0).Err())
	diagnostic, err := s.CaptureLegacyCutover(ctx)
	require.Error(t, err)
	require.NotEmpty(t, diagnostic.Entries)
}

func TestLegacyAtomicCaptureHasNoAcceptedPartialOnLimit(t *testing.T) {
	s, c, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	require.NoError(t, c.Set(ctx, prefix+"call:room", string(make([]byte, 2*1024*1024+1)), 0).Err())
	capture, err := s.CaptureLegacyCutover(ctx)
	require.Error(t, err)
	require.Equal(t, "CAPTURE_BYTE_LIMIT", capture.Diagnostic)
	require.Error(t, capture.Validate())
	require.EqualValues(t, 1, c.Exists(ctx, prefix+"call:room").Val())
}

func TestLegacyCutoverRetainsEpochAndLifecycleDenialEvidence(t *testing.T) {
	s, client, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	require.NoError(t, client.Set(ctx, prefix+"match_squad_terminal_epoch:room:profile:epoch", "1", 0).Err())
	require.NoError(t, s.SetSpaceLifecycle(ctx, "space", 1, false))
	snapshot, err := s.CaptureLegacyCutover(ctx)
	require.NoError(t, err)
	require.Len(t, snapshot.Entries, 2)
	require.ErrorIs(t, s.CompareCleanupLegacyCutover(ctx, snapshot), ErrLegacyPartitionRequired)
	require.EqualValues(t, 2, client.Exists(ctx, prefix+"match_squad_terminal_epoch:room:profile:epoch", prefix+"space_lifecycle:space").Val())
}

func newLegacyRedisFixture(t *testing.T) (*RedisCallStore, *redis.Client, string) {
	t.Helper()
	addr := os.Getenv("VOICE_LEGACY_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("owned real Redis required for bounded atomic capture")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Ping(context.Background()).Err())
	prefix := "legacy-" + uuid.NewString() + ":"
	return NewRedisCallStore(client, prefix), client, prefix
}
