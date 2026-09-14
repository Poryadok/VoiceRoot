package store

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	callsv1 "voice.app/voice/calls/v1"
)

func TestRedisCallStore_PersistsGroupVoiceLifecycle(t *testing.T) {
	ctx := context.Background()
	store, client := newRedisCallStoreForTest(t, "voice-test:")

	created, err := store.CreateCall(ctx, Call{
		RoomID:             "room-group",
		LivekitRoomName:    "lk-group",
		ChatID:             "chat-group",
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE,
		InitiatorProfileID: "owner",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          time.Unix(1_700_000_000, 0).UTC(),
	})
	require.NoError(t, err)
	require.Equal(t, "owner", created.States["owner"].ProfileID)

	// A new store instance reads the data and indexes written through Redis.
	persisted := NewRedisCallStore(client, "voice-test:")
	active, err := persisted.GetActiveGroupCallForChat(ctx, "chat-group")
	require.NoError(t, err)
	require.Equal(t, "room-group", active.RoomID)
	_, err = persisted.GetActiveCall(ctx, "owner")
	require.NoError(t, err)

	_, err = persisted.AddParticipant(ctx, "room-group", "member", MaxGroupVoiceParticipants)
	require.NoError(t, err)
	muted := true
	updated, state, err := persisted.UpdateVoiceState(ctx, "room-group", "member", VoiceStatePatch{IsMuted: &muted})
	require.NoError(t, err)
	require.True(t, state.IsMuted)
	require.True(t, updated.States["member"].IsMuted)

	_, entry, err := persisted.StartScreenShare(ctx, "room-group", "member", "stream-member")
	require.NoError(t, err)
	require.Equal(t, "stream-member", entry.StreamID)

	stored, err := store.GetCall(ctx, "room-group")
	require.NoError(t, err)
	require.True(t, stored.States["member"].IsMuted)
	require.True(t, stored.States["member"].IsScreenSharing)
	require.Equal(t, []ScreenShareEntry{{ProfileID: "member", StreamID: "stream-member"}}, stored.ScreenShares)

	removed, err := store.RemoveParticipant(ctx, "room-group", "member")
	require.NoError(t, err)
	require.NotContains(t, removed.States, "member")
	require.Empty(t, removed.ScreenShares)
	_, err = persisted.GetActiveCall(ctx, "member")
	require.ErrorIs(t, err, ErrNotFound)

	_, err = persisted.SetStatus(ctx, "room-group", callsv1.CallStatus_CALL_STATUS_ENDED, time.Unix(1_700_000_100, 0).UTC())
	require.NoError(t, err)
	_, err = store.GetActiveGroupCallForChat(ctx, "chat-group")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = store.GetActiveCall(ctx, "owner")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRedisCallStore_IndexesVoiceRoomsAndExpiredRingingCalls(t *testing.T) {
	ctx := context.Background()
	store, _ := newRedisCallStoreForTest(t, "voice-test:")

	_, err := store.CreateCall(ctx, Call{
		RoomID:             "room-space",
		LivekitRoomName:    "lk-space",
		VoiceRoomID:        "voice-room-1",
		SpaceID:            "space-1",
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: "space-owner",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
	})
	require.NoError(t, err)

	room, err := store.GetCallByVoiceRoomID(ctx, "voice-room-1")
	require.NoError(t, err)
	require.Equal(t, "room-space", room.RoomID)

	_, err = store.SetStatus(ctx, "room-space", callsv1.CallStatus_CALL_STATUS_ENDED, time.Unix(1_700_000_100, 0).UTC())
	require.NoError(t, err)
	_, err = store.GetCallByVoiceRoomID(ctx, "voice-room-1")
	require.ErrorIs(t, err, ErrNotFound)

	expiresAt := time.Unix(1_700_000_000, 0).UTC()
	_, err = store.CreateCall(ctx, Call{
		RoomID:             "room-ringing",
		InitiatorProfileID: "caller",
		CalleeProfileID:    "callee",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_RINGING,
		ExpiresAt:          expiresAt,
	})
	require.NoError(t, err)
	_, err = store.CreateCall(ctx, Call{
		RoomID:             "room-active",
		InitiatorProfileID: "other-caller",
		CalleeProfileID:    "other-callee",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		ExpiresAt:          expiresAt,
	})
	require.NoError(t, err)

	expired, err := store.ListExpiredRinging(ctx, expiresAt.Add(time.Second))
	require.NoError(t, err)
	require.Len(t, expired, 1)
	require.Equal(t, "room-ringing", expired[0].RoomID)
}

func newRedisCallStoreForTest(t *testing.T, prefix string) (*RedisCallStore, *redis.Client) {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, client.Ping(context.Background()).Err())
	return NewRedisCallStore(client, prefix), client
}

func TestRedisCallStore_RestoresPersistedRoomBindingAcrossInstances(t *testing.T) {
	for _, tc := range []struct{ name, space string }{
		{"complete tuple", "22222222-2222-4222-8222-222222222222"},
		{"legacy without space", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			first, client := newRedisCallStoreForTest(t, "binding-test:")
			room := "11111111-1111-4111-8111-111111111111"
			call, err := first.CreateCall(ctx, Call{RoomID: "33333333-3333-4333-8333-333333333333", VoiceRoomID: room, SpaceID: tc.space, LivekitRoomName: "lk-restored", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: "owner", MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
			require.NoError(t, err)
			raw, err := client.Get(ctx, first.callKey(call.RoomID)).Result()
			require.NoError(t, err)
			if tc.space == "" {
				require.NotContains(t, raw, `"space_id"`)
			} else {
				require.Contains(t, raw, tc.space)
			}
			restarted := NewRedisCallStore(client, "binding-test:")
			byID, err := restarted.GetCall(ctx, call.RoomID)
			require.NoError(t, err)
			active, err := restarted.GetActiveCall(ctx, "owner")
			require.NoError(t, err)
			for _, got := range []Call{byID, active} {
				require.Equal(t, call.RoomID, got.RoomID)
				require.Equal(t, room, got.VoiceRoomID)
				require.Equal(t, tc.space, got.SpaceID)
				require.Equal(t, call.SessionKind, got.SessionKind)
				require.Equal(t, "lk-restored", got.LivekitRoomName)
				require.True(t, got.IsActiveForProfile("owner"))
			}
		})
	}
}

func TestRedisCallStore_MoveConcurrentPublicAddAndRemoveKeepSingleActiveRoster(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "add", true: "remove"}[remove], func(t *testing.T) {
			ctx := context.Background()
			store, _ := newRedisCallStoreForTest(t, "voice-move-public-writer:")
			space, sourceID, destinationID, target, actor := "space", "source", "destination", "target", "actor"
			source, err := store.CreateCall(ctx, Call{RoomID: "source-call", VoiceRoomID: sourceID, SpaceID: space, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: target, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
			require.NoError(t, err)
			start, errs := make(chan struct{}), make(chan error, 2)
			go func() {
				<-start
				_, err := store.MoveVoiceRoomParticipant(ctx, VoiceRoomMoveRequest{ActorProfileID: actor, ParticipantProfileID: target, OperationID: "operation", FromVoiceRoomID: sourceID, ToVoiceRoomID: destinationID, SpaceID: space, MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: "destination-call", Now: time.Unix(1_700_000_000, 0).UTC()})
				errs <- err
			}()
			go func() {
				<-start
				if remove {
					_, err := store.RemoveParticipant(ctx, source.RoomID, target)
					errs <- err
					return
				}
				_, err := store.AddParticipant(ctx, source.RoomID, "independent-joiner", MaxVoiceRoomParticipants)
				errs <- err
			}()
			close(start)
			<-errs
			<-errs
			sourceAfter, sourceErr := store.GetCallByVoiceRoomID(ctx, sourceID)
			destinationAfter, destinationErr := store.GetCallByVoiceRoomID(ctx, destinationID)
			inSource := sourceErr == nil && sourceAfter.IsParticipant(target)
			inDestination := destinationErr == nil && destinationAfter.IsParticipant(target)
			require.NotEqual(t, inSource, inDestination, "public writers and move must serialize: target is in exactly one roster")
			if inDestination {
				active, err := store.GetActiveCall(ctx, target)
				require.NoError(t, err)
				require.Equal(t, destinationAfter.RoomID, active.RoomID)
			}
		})
	}
}

func TestRedisCallStore_CompetingMovesChooseOneDestination(t *testing.T) {
	ctx := context.Background()
	store, _ := newRedisCallStoreForTest(t, "voice-competing-moves:")
	space, sourceID, target, actor := "space", "source", "target", "actor"
	_, err := store.CreateCall(ctx, Call{RoomID: "source-call", VoiceRoomID: sourceID, SpaceID: space, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: target, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
	require.NoError(t, err)
	start, errs := make(chan struct{}), make(chan error, 2)
	for i, dest := range []string{"destination-a", "destination-b"} {
		i, dest := i, dest
		go func() {
			<-start
			_, err := store.MoveVoiceRoomParticipant(ctx, VoiceRoomMoveRequest{ActorProfileID: actor, ParticipantProfileID: target, OperationID: "op-" + string(rune('a'+i)), FromVoiceRoomID: sourceID, ToVoiceRoomID: dest, SpaceID: space, MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: "destination-call-" + string(rune('a'+i)), Now: time.Unix(1_700_000_000, 0).UTC()})
			errs <- err
		}()
	}
	close(start)
	<-errs
	<-errs
	active, err := store.GetActiveCall(ctx, target)
	require.NoError(t, err)
	require.Contains(t, []string{"destination-call-a", "destination-call-b"}, active.RoomID)
}

func TestRedisCallStore_MoveFailureLeavesRosterIndexesAndLedgerUntouched(t *testing.T) {
	ctx := context.Background()
	store, client := newRedisCallStoreForTest(t, "voice-move-rollback:")
	source, err := store.CreateCall(ctx, Call{RoomID: "source-call", VoiceRoomID: "source", SpaceID: "space", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: "target", MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
	require.NoError(t, err)
	before, err := store.GetCall(ctx, source.RoomID)
	require.NoError(t, err)
	// A cancelled Redis transaction must commit no part of the roster/session/index/ledger transition.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = store.MoveVoiceRoomParticipant(cancelled, VoiceRoomMoveRequest{ActorProfileID: "actor", ParticipantProfileID: "target", OperationID: "op", FromVoiceRoomID: "source", ToVoiceRoomID: "destination", SpaceID: "space", MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: "destination-call", Now: time.Now().UTC()})
	require.Error(t, err)
	after, err := store.GetCall(ctx, source.RoomID)
	require.NoError(t, err)
	require.Equal(t, before, after)
	_, err = store.GetCallByVoiceRoomID(ctx, "destination")
	require.ErrorIs(t, err, ErrNotFound)
	require.Equal(t, "source-call", client.Get(ctx, store.activeKey("target")).Val())
	require.False(t, client.Exists(ctx, store.moveOperationKey("actor", "op")).Val() > 0)
}

func TestRedisCallStore_MoveRetriesRealWatchConflictThenSucceeds(t *testing.T) {
	ctx := context.Background()
	store, client := newRedisCallStoreForTest(t, "voice-real-watch:")
	_, err := store.CreateCall(ctx, Call{RoomID: "source-call", VoiceRoomID: "source", SpaceID: "space", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: "target", MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
	require.NoError(t, err)
	other := redis.NewClient(&redis.Options{Addr: client.Options().Addr})
	t.Cleanup(func() { require.NoError(t, other.Close()) })
	calls := 0
	store.beforeMoveExec = func(_ context.Context, attempt int, _ []string) {
		calls++
		if attempt == 0 {
			raw := other.Get(ctx, store.callKey("source-call")).Val()
			require.NoError(t, other.Set(ctx, store.callKey("source-call"), raw, 24*time.Hour).Err())
		}
	}
	result, err := store.MoveVoiceRoomParticipant(ctx, VoiceRoomMoveRequest{ActorProfileID: "actor", ParticipantProfileID: "target", OperationID: "op", FromVoiceRoomID: "source", ToVoiceRoomID: "destination", SpaceID: "space", MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: "destination-call", Now: time.Now().UTC()})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.True(t, result.Destination.IsParticipant("target"))
	active, err := store.GetActiveCall(ctx, "target")
	require.NoError(t, err)
	require.Equal(t, "destination-call", active.RoomID)
}

func TestRedisCallStore_MoveRealWatchExhaustionLeavesNoPartialState(t *testing.T) {
	ctx := context.Background()
	store, client := newRedisCallStoreForTest(t, "voice-real-exhaust:")
	_, err := store.CreateCall(ctx, Call{RoomID: "source-call", VoiceRoomID: "source", SpaceID: "space", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: "target", MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
	require.NoError(t, err)
	other := redis.NewClient(&redis.Options{Addr: client.Options().Addr})
	t.Cleanup(func() { require.NoError(t, other.Close()) })
	store.beforeMoveExec = func(_ context.Context, _ int, _ []string) {
		raw := other.Get(ctx, store.callKey("source-call")).Val()
		require.NoError(t, other.Set(ctx, store.callKey("source-call"), raw, 24*time.Hour).Err())
	}
	_, err = store.MoveVoiceRoomParticipant(ctx, VoiceRoomMoveRequest{ActorProfileID: "actor", ParticipantProfileID: "target", OperationID: "op", FromVoiceRoomID: "source", ToVoiceRoomID: "destination", SpaceID: "space", MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: "destination-call", Now: time.Now().UTC()})
	require.ErrorIs(t, err, redis.TxFailedErr)
	_, err = store.GetCallByVoiceRoomID(ctx, "destination")
	require.ErrorIs(t, err, ErrNotFound)
	require.Equal(t, "source-call", client.Get(ctx, store.activeKey("target")).Val())
	require.Zero(t, client.Exists(ctx, store.moveOperationKey("actor", "op")).Val())
}
