package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	callsv1 "voice.app/voice/calls/v1"
)

func ordinaryProjectionCall(room, profile, chat string, version uint64) Call {
	return Call{RoomID: room, ChatID: chat, LivekitRoomName: "voice-group-" + room,
		OrdinaryOperationID: uuid.NewString(), OrdinaryVersion: version,
		OrdinaryMediaIdentities: map[string]string{profile: "ov:" + profile + ":" + uuid.NewString()},
		SessionKind:             callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE,
		Status:                  callsv1.CallStatus_CALL_STATUS_ACTIVE, InitiatorProfileID: profile,
		States: map[string]ParticipantState{profile: {ProfileID: profile}}}
}

func TestOrdinaryProjectionKeepsNewCallIndexesWhenOldCleanupArrives(t *testing.T) {
	ctx := context.Background()
	s, client := newRedisCallStoreForTest(t, "ordinary:")
	profile, chat := uuid.NewString(), uuid.NewString()
	old := ordinaryProjectionCall(uuid.NewString(), profile, chat, 1)
	_, err := s.ApplyOrdinaryProjection(ctx, old, nil)
	require.NoError(t, err)
	next := ordinaryProjectionCall(uuid.NewString(), profile, chat, 1)
	authorize := func(_ context.Context, room string) error { require.Equal(t, old.RoomID, room); return nil }
	_, err = s.ApplyOrdinaryProjection(ctx, next, authorize)
	require.NoError(t, err)
	old.OrdinaryVersion = 2
	old.Status = callsv1.CallStatus_CALL_STATUS_ENDED
	old.States = map[string]ParticipantState{}
	_, err = s.ApplyOrdinaryProjection(ctx, old, nil)
	require.NoError(t, err)
	require.Equal(t, next.RoomID, client.Get(ctx, s.activeKey(profile)).Val())
	require.Equal(t, next.RoomID, client.Get(ctx, s.activeChatKey(chat)).Val())
}

func TestOrdinaryProjectionPreservesFlagsOnlyForSameMediaGeneration(t *testing.T) {
	ctx := context.Background()
	s, _ := newRedisCallStoreForTest(t, "ordinary:")
	profile := uuid.NewString()
	call := ordinaryProjectionCall(uuid.NewString(), profile, uuid.NewString(), 1)
	_, err := s.ApplyOrdinaryProjection(ctx, call, nil)
	require.NoError(t, err)
	muted := true
	_, _, err = s.UpdateVoiceState(ctx, call.RoomID, profile, VoiceStatePatch{IsMuted: &muted})
	require.NoError(t, err)
	call.OrdinaryVersion = 2
	got, err := s.ApplyOrdinaryProjection(ctx, call, nil)
	require.NoError(t, err)
	require.True(t, got.States[profile].IsMuted)
	call.OrdinaryVersion = 3
	call.OrdinaryMediaIdentities[profile] = "ov:" + profile + ":" + uuid.NewString()
	got, err = s.ApplyOrdinaryProjection(ctx, call, nil)
	require.NoError(t, err)
	require.False(t, got.States[profile].IsMuted)
	call.OrdinaryVersion = 1
	got, err = s.ApplyOrdinaryProjection(ctx, call, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(3), got.OrdinaryVersion)
}

func TestOrdinaryProjectionCannotReplaceForeignOwnerWithoutAuthority(t *testing.T) {
	ctx := context.Background()
	s, _ := newRedisCallStoreForTest(t, "ordinary:")
	profile := uuid.NewString()
	first := ordinaryProjectionCall(uuid.NewString(), profile, uuid.NewString(), 1)
	_, err := s.ApplyOrdinaryProjection(ctx, first, nil)
	require.NoError(t, err)
	next := ordinaryProjectionCall(uuid.NewString(), profile, uuid.NewString(), 1)
	_, err = s.ApplyOrdinaryProjection(ctx, next, nil)
	require.ErrorIs(t, err, ErrActiveCall)
}

func TestOrdinaryProjectionGroupChatSurvivesInitiatorLeave(t *testing.T) {
	ctx := context.Background()
	s, client := newRedisCallStoreForTest(t, "ordinary:")
	owner, member := uuid.NewString(), uuid.NewString()
	call := ordinaryProjectionCall(uuid.NewString(), owner, uuid.NewString(), 1)
	call.OrdinaryMediaIdentities[member] = "ov:" + member + ":" + uuid.NewString()
	call.States[member] = ParticipantState{ProfileID: member}
	_, err := s.ApplyOrdinaryProjection(ctx, call, nil)
	require.NoError(t, err)
	call.OrdinaryVersion = 2
	delete(call.States, owner)
	_, err = s.ApplyOrdinaryProjection(ctx, call, nil)
	require.NoError(t, err)
	require.Equal(t, call.RoomID, client.Get(ctx, s.activeChatKey(call.ChatID)).Val())
	require.Equal(t, call.RoomID, client.Get(ctx, s.activeKey(member)).Val())
	require.Zero(t, client.Exists(ctx, s.activeKey(owner)).Val())
}

func TestMemoryOrdinaryProjectionLateCleanupPreservesNewActiveLookup(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryCallStore()
	profile, chat := uuid.NewString(), uuid.NewString()
	old := ordinaryProjectionCall(uuid.NewString(), profile, chat, 1)
	_, err := s.ApplyOrdinaryProjection(ctx, old, nil)
	require.NoError(t, err)
	next := ordinaryProjectionCall(uuid.NewString(), profile, chat, 1)
	_, err = s.ApplyOrdinaryProjection(ctx, next, func(context.Context, string) error { return nil })
	require.NoError(t, err)
	old.OrdinaryVersion = 2
	old.Status = callsv1.CallStatus_CALL_STATUS_ENDED
	old.States = map[string]ParticipantState{}
	_, err = s.ApplyOrdinaryProjection(ctx, old, nil)
	require.NoError(t, err)
	got, err := s.GetActiveCall(ctx, profile)
	require.NoError(t, err)
	require.Equal(t, next.RoomID, got.RoomID)
}
