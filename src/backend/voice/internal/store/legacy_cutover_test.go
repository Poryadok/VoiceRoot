package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
)

func TestLegacyCutoverCapturesRawCallAndIndexesAndCleansExactly(t *testing.T) {
	s, client := newRedisCallStoreForTest(t, "legacy:")
	ctx := context.Background()
	raw, err := json.Marshal(Call{RoomID: "room", ChatID: "chat", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE, States: map[string]ParticipantState{"profile": {ProfileID: "profile"}}})
	require.NoError(t, err)
	require.NoError(t, client.Set(ctx, "legacy:call:room", raw, time.Hour).Err())
	require.NoError(t, client.Set(ctx, "legacy:session:profile", "room", time.Hour).Err())
	require.NoError(t, client.Set(ctx, "legacy:active_chat:chat", "room", time.Hour).Err())
	require.NoError(t, client.Set(ctx, "other:call:room", "untouched", 0).Err())
	snapshot, err := s.CaptureLegacyCutover(ctx)
	require.NoError(t, err)
	require.Equal(t, "legacy:", snapshot.Prefix)
	require.Len(t, snapshot.Entries, 3)
	require.NoError(t, s.CompareCleanupLegacyCutover(ctx, snapshot))
	require.EqualValues(t, 0, client.Exists(ctx, "legacy:call:room", "legacy:session:profile", "legacy:active_chat:chat").Val())
	require.Equal(t, "untouched", client.Get(ctx, "other:call:room").Val())
	// Lost cleanup acknowledgement resumes the same exact evidence.
	require.NoError(t, s.CompareCleanupLegacyCutover(ctx, snapshot))
}

func TestLegacyCutoverNewerValuePreventsPartialCleanup(t *testing.T) {
	s, client := newRedisCallStoreForTest(t, "legacy:")
	ctx := context.Background()
	raw, err := json.Marshal(Call{RoomID: "room"})
	require.NoError(t, err)
	require.NoError(t, client.Set(ctx, "legacy:call:room", raw, 0).Err())
	snapshot, err := s.CaptureLegacyCutover(ctx)
	require.NoError(t, err)
	require.NoError(t, client.Set(ctx, "legacy:call:room", `{"room_id":"room","ordinary_version":2}`, 0).Err())
	require.ErrorIs(t, s.CompareCleanupLegacyCutover(ctx, snapshot), ErrOperationConflict)
	require.Equal(t, `{"room_id":"room","ordinary_version":2}`, client.Get(ctx, "legacy:call:room").Val())
}

func TestLegacyCutoverMalformedDanglingAndUnknownEvidenceBlocksCapture(t *testing.T) {
	for _, tc := range []struct{ name, key, value string }{
		{"malformed", "call:room", "bad"},
		{"wrong-room", "call:room", `{"room_id":"another"}`},
		{"dangling", "session:profile", "expired-room"},
		{"unexplained", "unknown:record", "unexplained"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, client := newRedisCallStoreForTest(t, "legacy:")
			require.NoError(t, client.Set(context.Background(), "legacy:"+tc.key, tc.value, 0).Err())
			_, err := s.CaptureLegacyCutover(context.Background())
			require.Error(t, err)
			require.EqualValues(t, 1, client.Exists(context.Background(), "legacy:"+tc.key).Val())
		})
	}
}

func TestLegacyCutoverRetainsEpochAndLifecycleDenialEvidence(t *testing.T) {
	s, client := newRedisCallStoreForTest(t, "legacy:")
	ctx := context.Background()
	require.NoError(t, client.Set(ctx, "legacy:match_squad_terminal_epoch:room:profile:epoch", "1", 0).Err())
	require.NoError(t, s.SetSpaceLifecycle(ctx, "space", 1, false))
	snapshot, err := s.CaptureLegacyCutover(ctx)
	require.NoError(t, err)
	require.Len(t, snapshot.Entries, 2)
	require.NoError(t, s.CompareCleanupLegacyCutover(ctx, snapshot))
	require.EqualValues(t, 2, client.Exists(ctx, "legacy:match_squad_terminal_epoch:room:profile:epoch", "legacy:space_lifecycle:space").Val())
}
