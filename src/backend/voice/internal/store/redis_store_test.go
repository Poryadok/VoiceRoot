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
			type outcome struct {
				move bool
				err  error
			}
			start, outcomes := make(chan struct{}), make(chan outcome, 2)
			go func() {
				<-start
				_, err := store.MoveVoiceRoomParticipant(ctx, VoiceRoomMoveRequest{ActorProfileID: actor, ParticipantProfileID: target, OperationID: "operation", FromVoiceRoomID: sourceID, ToVoiceRoomID: destinationID, SpaceID: space, MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: "destination-call", Now: time.Unix(1_700_000_000, 0).UTC()})
				outcomes <- outcome{move: true, err: err}
			}()
			go func() {
				<-start
				if remove {
					_, err := store.RemoveParticipant(ctx, source.RoomID, target)
					outcomes <- outcome{err: err}
					return
				}
				_, err := store.AddParticipant(ctx, source.RoomID, "independent-joiner", MaxVoiceRoomParticipants)
				outcomes <- outcome{err: err}
			}()
			close(start)
			first, second := <-outcomes, <-outcomes
			moveOutcome, writerOutcome := first, second
			if !moveOutcome.move {
				moveOutcome, writerOutcome = second, first
			}
			sourceAfter, sourceErr := store.GetCallByVoiceRoomID(ctx, sourceID)
			destinationAfter, destinationErr := store.GetCallByVoiceRoomID(ctx, destinationID)
			inSource := sourceErr == nil && sourceAfter.IsParticipant(target)
			inDestination := destinationErr == nil && destinationAfter.IsParticipant(target)
			require.False(t, inSource && inDestination, "no committed state may place the target in both rosters")
			sourceIndex, sourceIndexErr := store.client.Get(ctx, store.activeVoiceRoomKey(sourceID)).Result()
			destinationIndex, destinationIndexErr := store.client.Get(ctx, store.activeVoiceRoomKey(destinationID)).Result()
			switch {
			case inSource:
				require.False(t, inDestination)
				require.NoError(t, sourceIndexErr)
				require.Equal(t, sourceAfter.RoomID, sourceIndex)
				require.ErrorIs(t, destinationIndexErr, redis.Nil)
				require.Empty(t, destinationIndex)
				active, err := store.GetActiveCall(ctx, target)
				require.NoError(t, err)
				require.Equal(t, sourceAfter.RoomID, active.RoomID)
			case inDestination:
				require.False(t, inSource)
				require.NoError(t, destinationIndexErr)
				require.Equal(t, destinationAfter.RoomID, destinationIndex)
				active, err := store.GetActiveCall(ctx, target)
				require.NoError(t, err)
				require.True(t, inDestination)
				require.Equal(t, destinationAfter.RoomID, active.RoomID)
				require.NoError(t, moveOutcome.err)
				if sourceErr == nil && len(sourceAfter.States) > 0 {
					// A concurrent public join may have committed first. The move
					// must preserve that source roster while relocating only target.
					require.False(t, sourceAfter.IsParticipant(target))
					require.NoError(t, sourceIndexErr)
					require.Equal(t, sourceAfter.RoomID, sourceIndex)
					if !remove {
						require.NoError(t, writerOutcome.err)
						require.True(t, sourceAfter.IsParticipant("independent-joiner"))
						joiner, err := store.GetActiveCall(ctx, "independent-joiner")
						require.NoError(t, err)
						require.Equal(t, sourceAfter.RoomID, joiner.RoomID)
					}
				} else {
					require.ErrorIs(t, sourceIndexErr, redis.Nil)
					require.Empty(t, sourceIndex)
				}
			default:
				require.True(t, remove, "only a winning public remove may leave the target in neither roster")
				require.NoError(t, writerOutcome.err)
				require.Error(t, moveOutcome.err)
				require.ErrorIs(t, sourceIndexErr, redis.Nil)
				require.ErrorIs(t, destinationIndexErr, redis.Nil)
				_, err := store.GetActiveCall(ctx, target)
				require.ErrorIs(t, err, ErrNotFound)
				_, err = store.client.Get(ctx, store.activeKey(target)).Result()
				require.ErrorIs(t, err, redis.Nil)
			}
		})
	}
}

func TestRedisCallStore_CompetingMovesChooseOneDestination(t *testing.T) {
	ctx := context.Background()
	store, client := newRedisCallStoreForTest(t, "voice-competing-moves:")
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
	firstErr, secondErr := <-errs, <-errs
	require.Equal(t, 1, boolCount(firstErr == nil, secondErr == nil), "one competing move must become the terminal operation")
	active, err := store.GetActiveCall(ctx, target)
	require.NoError(t, err)
	require.Contains(t, []string{"destination-call-a", "destination-call-b"}, active.RoomID)
	terminalDestinations := 0
	for _, candidate := range []struct{ room, callID, operation string }{
		{room: "destination-a", callID: "destination-call-a", operation: "op-a"},
		{room: "destination-b", callID: "destination-call-b", operation: "op-b"},
	} {
		index, indexErr := client.Get(ctx, store.activeVoiceRoomKey(candidate.room)).Result()
		ledgerKey := store.moveOperationKey(actor, candidate.operation)
		if indexErr == nil {
			terminalDestinations++
			require.Equal(t, candidate.callID, index)
			call, err := store.GetCall(ctx, candidate.callID)
			require.NoError(t, err)
			require.True(t, call.IsParticipant(target))
			require.Equal(t, call.RoomID, active.RoomID)
			require.Equal(t, call.RoomID, client.Get(ctx, store.activeKey(target)).Val())
			require.NotEmpty(t, client.Get(ctx, ledgerKey).Val())
			require.GreaterOrEqual(t, client.TTL(ctx, ledgerKey).Val(), 23*time.Hour)
			continue
		}
		require.ErrorIs(t, indexErr, redis.Nil)
		_, callErr := client.Get(ctx, store.callKey(candidate.callID)).Result()
		require.ErrorIs(t, callErr, redis.Nil, "losing destination must not retain a call document")
		_, ledgerErr := client.Get(ctx, ledgerKey).Result()
		require.ErrorIs(t, ledgerErr, redis.Nil, "losing operation must not claim a terminal receipt")
	}
	require.Equal(t, 1, terminalDestinations, "competing moves cannot leave duplicate or lost roster membership")
	sourceAfter, err := store.GetCall(ctx, "source-call")
	require.NoError(t, err)
	require.False(t, sourceAfter.IsParticipant(target), "the committed source document must exclude the moved target")
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ENDED, sourceAfter.Status)
	sourceIndex, err := client.Get(ctx, store.activeVoiceRoomKey(sourceID)).Result()
	require.ErrorIs(t, err, redis.Nil)
	require.Empty(t, sourceIndex, "an ended source document must not retain its voice-room index")
}

func boolCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

func TestRedisCallStore_MoveFailureLeavesRosterIndexesAndLedgerUntouched(t *testing.T) {
	ctx := context.Background()
	store, client := newRedisCallStoreForTest(t, "voice-move-rollback:")
	source, err := store.CreateCall(ctx, Call{RoomID: "source-call", VoiceRoomID: "source", SpaceID: "space", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: "target", MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
	require.NoError(t, err)
	request := VoiceRoomMoveRequest{ActorProfileID: "actor", ParticipantProfileID: "target", OperationID: "op", FromVoiceRoomID: "source", ToVoiceRoomID: "destination", SpaceID: "space", MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: "destination-call", Now: time.Now().UTC()}
	keys := []string{store.callKey(source.RoomID), store.callKey(request.DestinationRoomID), store.activeVoiceRoomKey(request.FromVoiceRoomID), store.activeVoiceRoomKey(request.ToVoiceRoomID), store.activeKey(request.ParticipantProfileID), store.moveOperationKey(request.ActorProfileID, request.OperationID)}
	before := redisRawKeySnapshot(t, ctx, client, keys)
	// A cancelled Redis transaction must commit no part of the roster/session/index/ledger transition.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = store.MoveVoiceRoomParticipant(cancelled, request)
	require.Error(t, err)
	require.Equal(t, before, redisRawKeySnapshot(t, ctx, client, keys), "failure must retain the exact source/destination documents, both room indices, active session and ledger bytes")
}

func TestRedisCallStore_MoveRejectsStaleSourceIndexWithoutMutatingRedisProjection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, ctx context.Context, store *RedisCallStore, client *redis.Client)
	}{
		{
			name: "source index points to a missing call document",
			setup: func(t *testing.T, ctx context.Context, store *RedisCallStore, client *redis.Client) {
				require.NoError(t, client.Set(ctx, store.activeVoiceRoomKey("source"), "source-call", 24*time.Hour).Err())
				require.NoError(t, client.Set(ctx, store.activeKey("target"), "source-call", 24*time.Hour).Err())
			},
		},
		{
			name: "source index points to an ended call document",
			setup: func(t *testing.T, ctx context.Context, store *RedisCallStore, client *redis.Client) {
				_, err := store.CreateCall(ctx, Call{RoomID: "source-call", VoiceRoomID: "source", SpaceID: "space", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: "target", MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
				require.NoError(t, err)
				_, err = store.SetStatus(ctx, "source-call", callsv1.CallStatus_CALL_STATUS_ENDED, time.Unix(1_700_000_100, 0).UTC())
				require.NoError(t, err)
				require.NoError(t, client.Set(ctx, store.activeVoiceRoomKey("source"), "source-call", 24*time.Hour).Err())
				require.NoError(t, client.Set(ctx, store.activeKey("target"), "source-call", 24*time.Hour).Err())
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			store, client := newRedisCallStoreForTest(t, "voice-move-stale-source:")
			tc.setup(t, ctx, store, client)
			request := VoiceRoomMoveRequest{ActorProfileID: "actor", ParticipantProfileID: "target", OperationID: "op", FromVoiceRoomID: "source", ToVoiceRoomID: "destination", SpaceID: "space", MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: "destination-call", Now: time.Unix(1_700_000_000, 0).UTC()}
			keys := []string{store.callKey("source-call"), store.callKey(request.DestinationRoomID), store.activeVoiceRoomKey(request.FromVoiceRoomID), store.activeVoiceRoomKey(request.ToVoiceRoomID), store.activeKey(request.ParticipantProfileID), store.moveOperationKey(request.ActorProfileID, request.OperationID)}
			before := redisRawKeySnapshot(t, ctx, client, keys)

			_, err := store.MoveVoiceRoomParticipant(ctx, request)
			require.ErrorIs(t, err, ErrInvalidState, "a stale source projection is an obsolete move state, never a successful move or discoverability response")
			require.Equal(t, before, redisRawKeySnapshot(t, ctx, client, keys), "stale source rejection must preserve source/destination documents, both room indices, active session and ledger bytes")
		})
	}
}

func redisRawKeySnapshot(t *testing.T, ctx context.Context, client *redis.Client, keys []string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string, len(keys))
	for _, key := range keys {
		value, err := client.Get(ctx, key).Result()
		if err == nil {
			snapshot[key] = "present:" + value
			continue
		}
		require.ErrorIs(t, err, redis.Nil, "key=%s", key)
		snapshot[key] = "absent"
	}
	return snapshot
}

func TestRedisCallStore_MovePersistsAtomicSnapshotAndReplayAcrossFreshInstance(t *testing.T) {
	ctx := t.Context()
	store, client := newRedisCallStoreForTest(t, "voice-move-replay:")
	request := VoiceRoomMoveRequest{
		ActorProfileID: "actor", ParticipantProfileID: "target", OperationID: "operation",
		FromVoiceRoomID: "source", ToVoiceRoomID: "destination", SpaceID: "space",
		MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: "destination-call",
		Now: time.Unix(1_700_000_000, 0).UTC(),
	}
	_, err := store.CreateCall(ctx, Call{RoomID: "source-call", VoiceRoomID: request.FromVoiceRoomID, SpaceID: request.SpaceID, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: request.ParticipantProfileID, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
	require.NoError(t, err)
	_, err = store.AddParticipant(ctx, "source-call", "anchor", MaxVoiceRoomParticipants)
	require.NoError(t, err)

	first, err := store.MoveVoiceRoomParticipant(ctx, request)
	require.NoError(t, err)
	require.False(t, first.Replayed)
	snapshot := redisMoveSnapshot(t, ctx, client, store, request)
	sourceAfter, err := store.GetCall(ctx, "source-call")
	require.NoError(t, err)
	require.False(t, sourceAfter.IsParticipant(request.ParticipantProfileID))
	destinationAfter, err := store.GetCall(ctx, request.DestinationRoomID)
	require.NoError(t, err)
	require.True(t, destinationAfter.IsParticipant(request.ParticipantProfileID))
	require.Equal(t, "source-call", snapshot.sourceIndex)
	require.Equal(t, "destination-call", snapshot.destinationIndex)
	require.Equal(t, "destination-call", snapshot.targetSession)
	require.NotEmpty(t, snapshot.ledger)
	require.GreaterOrEqual(t, snapshot.ledgerTTL, 23*time.Hour)

	// RedisCallStore has no event publisher. Event suppression is therefore
	// asserted at the gRPC boundary; this store contract proves the durable
	// replay input remains available after a process/store reconstruction.
	restarted := NewRedisCallStore(client, "voice-move-replay:")
	replay, found, err := restarted.FindVoiceRoomMove(ctx, request)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, replay.Replayed)
	require.Equal(t, first.Source, replay.Source)
	require.Equal(t, first.Destination, replay.Destination)
	require.Equal(t, snapshot, redisMoveSnapshot(t, ctx, client, restarted, request), "a durable replay must not write any roster/index/session/ledger key")
}

type redisMoveStateSnapshot struct {
	sourceCall, destinationCall                  string
	sourceIndex, destinationIndex, targetSession string
	ledger                                       string
	ledgerTTL                                    time.Duration
}

func redisMoveSnapshot(t *testing.T, ctx context.Context, client *redis.Client, store *RedisCallStore, request VoiceRoomMoveRequest) redisMoveStateSnapshot {
	t.Helper()
	get := func(key string) string {
		value, err := client.Get(ctx, key).Result()
		require.NoError(t, err, "key=%s", key)
		return value
	}
	return redisMoveStateSnapshot{
		sourceCall:       get(store.callKey("source-call")),
		destinationCall:  get(store.callKey(request.DestinationRoomID)),
		sourceIndex:      get(store.activeVoiceRoomKey(request.FromVoiceRoomID)),
		destinationIndex: get(store.activeVoiceRoomKey(request.ToVoiceRoomID)),
		targetSession:    get(store.activeKey(request.ParticipantProfileID)),
		ledger:           get(store.moveOperationKey(request.ActorProfileID, request.OperationID)),
		ledgerTTL:        client.TTL(ctx, store.moveOperationKey(request.ActorProfileID, request.OperationID)).Val(),
	}
}

func TestRedisCallStore_MoveAndPublicJoinUseDeterministicLegalOrders(t *testing.T) {
	const sourceID, destinationID, sourceCall, destinationCall, target, joiner = "source", "destination", "source-call", "destination-call", "target", "joiner"

	newFixture := func(t *testing.T, prefix string) (*RedisCallStore, *redis.Client, VoiceRoomMoveRequest) {
		t.Helper()
		ctx := t.Context()
		store, client := newRedisCallStoreForTest(t, prefix)
		_, err := store.CreateCall(ctx, Call{RoomID: sourceCall, VoiceRoomID: sourceID, SpaceID: "space", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: target, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
		require.NoError(t, err)
		return store, client, VoiceRoomMoveRequest{ActorProfileID: "actor", ParticipantProfileID: target, OperationID: "op", FromVoiceRoomID: sourceID, ToVoiceRoomID: destinationID, SpaceID: "space", MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: destinationCall, Now: time.Unix(1_700_000_000, 0).UTC()}
	}

	t.Run("move before add rejects stale source call without joiner residue", func(t *testing.T) {
		ctx := t.Context()
		store, client, request := newFixture(t, "voice-move-before-public-join:")

		_, err := store.MoveVoiceRoomParticipant(ctx, request)
		require.NoError(t, err)
		_, err = store.AddParticipant(ctx, sourceCall, joiner, MaxVoiceRoomParticipants)
		require.ErrorIs(t, err, ErrInvalidState)

		source, err := store.GetCall(ctx, sourceCall)
		require.NoError(t, err)
		require.Equal(t, callsv1.CallStatus_CALL_STATUS_ENDED, source.Status)
		require.Empty(t, source.States)
		require.False(t, source.IsParticipant(target))
		require.False(t, source.IsParticipant(joiner))
		destination, err := store.GetCallByVoiceRoomID(ctx, destinationID)
		require.NoError(t, err)
		require.Len(t, destination.States, 1)
		require.True(t, destination.IsParticipant(target))
		require.False(t, destination.IsParticipant(joiner))

		_, err = client.Get(ctx, store.activeVoiceRoomKey(sourceID)).Result()
		require.ErrorIs(t, err, redis.Nil)
		require.Equal(t, destinationCall, client.Get(ctx, store.activeVoiceRoomKey(destinationID)).Val())
		require.Equal(t, destinationCall, client.Get(ctx, store.activeKey(target)).Val())
		_, err = client.Get(ctx, store.activeKey(joiner)).Result()
		require.ErrorIs(t, err, redis.Nil)
		_, err = store.GetActiveCall(ctx, joiner)
		require.ErrorIs(t, err, ErrNotFound)
		require.NotEmpty(t, client.Get(ctx, store.moveOperationKey(request.ActorProfileID, request.OperationID)).Val())
	})

	t.Run("committed add before move exec retries and retains joiner", func(t *testing.T) {
		ctx := t.Context()
		store, client, request := newFixture(t, "voice-public-join-before-move-exec:")
		secondClient := redis.NewClient(&redis.Options{Addr: client.Options().Addr})
		t.Cleanup(func() { require.NoError(t, secondClient.Close()) })
		secondStore := NewRedisCallStore(secondClient, "voice-public-join-before-move-exec:")

		attempts := 0
		store.moveTestHooks.beforeMoveExec = func(ctx context.Context, attempt int, _ []string) error {
			attempts++
			if attempt != 0 {
				return nil
			}
			_, err := secondStore.AddParticipant(ctx, sourceCall, joiner, MaxVoiceRoomParticipants)
			return err
		}
		_, err := store.MoveVoiceRoomParticipant(ctx, request)
		require.NoError(t, err)
		require.Equal(t, 2, attempts, "the committed add must invalidate the first WATCH and force a move retry")

		source, err := store.GetCallByVoiceRoomID(ctx, sourceID)
		require.NoError(t, err)
		require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, source.Status)
		require.Len(t, source.States, 1)
		require.False(t, source.IsParticipant(target))
		require.True(t, source.IsParticipant(joiner))
		destination, err := store.GetCallByVoiceRoomID(ctx, destinationID)
		require.NoError(t, err)
		require.Len(t, destination.States, 1)
		require.True(t, destination.IsParticipant(target))
		require.False(t, destination.IsParticipant(joiner))

		require.Equal(t, sourceCall, client.Get(ctx, store.activeVoiceRoomKey(sourceID)).Val())
		require.Equal(t, destinationCall, client.Get(ctx, store.activeVoiceRoomKey(destinationID)).Val())
		require.Equal(t, sourceCall, client.Get(ctx, store.activeKey(joiner)).Val())
		require.Equal(t, destinationCall, client.Get(ctx, store.activeKey(target)).Val())
		require.Equal(t, sourceCall, mustActiveCall(t, ctx, store, joiner).RoomID)
		require.Equal(t, destinationCall, mustActiveCall(t, ctx, store, target).RoomID)
		require.NotEmpty(t, client.Get(ctx, store.moveOperationKey(request.ActorProfileID, request.OperationID)).Val())
	})
}

func TestRedisCallStore_MoveWatchInvalidationRetriesThenExhaustsWithoutPartialState(t *testing.T) {
	ctx := t.Context()
	store, client := newRedisCallStoreForTest(t, "voice-move-watch:")
	second := redis.NewClient(&redis.Options{Addr: client.Options().Addr})
	t.Cleanup(func(c *redis.Client) func() { return func() { require.NoError(t, c.Close()) } }(second))
	request := VoiceRoomMoveRequest{ActorProfileID: "actor", ParticipantProfileID: "target", OperationID: "op", FromVoiceRoomID: "source", ToVoiceRoomID: "destination", SpaceID: "space", MaxParticipants: MaxVoiceRoomParticipants, DestinationRoomID: "destination-call", Now: time.Unix(1_700_000_000, 0).UTC()}
	_, err := store.CreateCall(ctx, Call{RoomID: "source-call", VoiceRoomID: request.FromVoiceRoomID, SpaceID: request.SpaceID, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: request.ParticipantProfileID, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
	require.NoError(t, err)

	var invalidations int
	store.moveTestHooks.beforeMoveExec = func(ctx context.Context, attempt int, _ []string) error {
		invalidations++
		if attempt != 0 {
			return nil
		}
		// A second real client writes the exact existing source bytes. Redis must
		// invalidate WATCH because the key changed, rather than a fake EXEC error.
		bytes, err := second.Get(ctx, store.callKey("source-call")).Bytes()
		if err != nil {
			return err
		}
		return second.Set(ctx, store.callKey("source-call"), bytes, 24*time.Hour).Err()
	}
	_, err = store.MoveVoiceRoomParticipant(ctx, request)
	require.NoError(t, err)
	require.Equal(t, 2, invalidations, "the first real WATCH invalidation must retry before committing")

	// A fresh fixture invalidates every WATCH attempt. It must leave every
	// transactional key at its old logical value and return the typed result.
	store, client = newRedisCallStoreForTest(t, "voice-move-watch-exhaust:")
	second = redis.NewClient(&redis.Options{Addr: client.Options().Addr})
	t.Cleanup(func(c *redis.Client) func() { return func() { require.NoError(t, c.Close()) } }(second))
	request.OperationID = "exhaust"
	_, err = store.CreateCall(ctx, Call{RoomID: "source-call", VoiceRoomID: request.FromVoiceRoomID, SpaceID: request.SpaceID, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: request.ParticipantProfileID, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE})
	require.NoError(t, err)
	keys := []string{store.callKey("source-call"), store.callKey(request.DestinationRoomID), store.activeVoiceRoomKey(request.FromVoiceRoomID), store.activeVoiceRoomKey(request.ToVoiceRoomID), store.activeKey(request.ParticipantProfileID), store.moveOperationKey(request.ActorProfileID, request.OperationID)}
	before := redisRawKeySnapshot(t, ctx, client, keys)
	store.moveTestHooks.beforeMoveExec = func(ctx context.Context, _ int, _ []string) error {
		bytes, err := second.Get(ctx, store.callKey("source-call")).Bytes()
		if err != nil {
			return err
		}
		return second.Set(ctx, store.callKey("source-call"), bytes, 24*time.Hour).Err()
	}
	_, err = store.MoveVoiceRoomParticipant(ctx, request)
	require.ErrorIs(t, err, ErrMoveContention)
	require.Equal(t, before, redisRawKeySnapshot(t, ctx, client, keys))
}

func mustActiveCall(t *testing.T, ctx context.Context, store *RedisCallStore, profileID string) Call {
	t.Helper()
	call, err := store.GetActiveCall(ctx, profileID)
	require.NoError(t, err)
	return call
}
