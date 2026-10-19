package store

import (
	"context"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	callsv1 "voice.app/voice/calls/v1"
)

func newSpaceGateStoreForTest(t *testing.T, prefix string) (*RedisCallStore, *redis.Client) {
	if addr := os.Getenv("VOICE_GATE_TEST_REDIS_ADDR"); addr != "" {
		client := redis.NewClient(&redis.Options{Addr: addr})
		t.Cleanup(func() { _ = client.Close() })
		require.NoError(t, client.FlushDB(t.Context()).Err())
		return NewRedisCallStore(client, prefix), client
	}
	return newRedisCallStoreForTest(t, prefix)
}
func liveFixture(generation int64) SpaceLifecycleLoader {
	return func(context.Context, string) (int64, bool, error) { return generation, true, nil }
}
func denyFixture(generation int64) SpaceLifecycleLoader {
	return func(context.Context, string) (int64, bool, error) { return generation, false, nil }
}
func fixtureTicket(ctx context.Context, s *RedisCallStore, call Call) (SpaceProjectionTicket, error) {
	t, err := s.ReadReadySpaceGate(ctx, call.SpaceID)
	t.RoomID = call.RoomID
	t.VoiceRoomID = call.VoiceRoomID
	if err != nil {
		return t, err
	}
	dispatch, err := s.EnsureAdmissionDispatch(ctx, "fixture:"+call.RoomID, liveFixture(1))
	t.OperationID = dispatch.OperationID
	t.AdmissionIncarnation = dispatch.AdmissionIncarnation
	t.AdmissionOwnerEpoch = dispatch.AdmissionOwnerEpoch
	t.AdmissionLive = dispatch.AdmissionLive
	return t, err
}

// Existing CAS tests supply explicit authority fixtures. Negative tests invoke
// production writers with missing, retired and deliberately stale tickets.
func createLifecycleCallForTest(ctx context.Context, s *RedisCallStore, call Call) (Call, error) {
	if call.SpaceID != "" {
		if err := s.EnsureSpaceLifecycle(ctx, call.SpaceID, liveFixture(call.SpaceLifecycleGeneration)); err != nil {
			return Call{}, err
		}
		ticket, err := fixtureTicket(ctx, s, call)
		if err != nil {
			return Call{}, err
		}
		call.SpaceProjectionTicket = ticket
	}
	return s.CreateCall(ctx, call)
}
func participantTicket(ctx context.Context, s *RedisCallStore, room string, p SpaceMediaParticipant) (SpaceProjectionTicket, error) {
	call, err := s.GetCall(ctx, room)
	if err != nil {
		return SpaceProjectionTicket{}, err
	}
	t, err := fixtureTicket(ctx, s, call)
	t.RoomGeneration = p.RoomGeneration
	t.OperationID = p.AdmissionOperationID
	t.AdmissionGeneration = p.Generation
	t.AccountID = p.AccountID
	t.ProfileID = p.ProfileID
	t.Identity = p.Identity
	if err != nil {
		return t, err
	}
	dispatch, err := s.EnsureAdmissionDispatch(ctx, p.AdmissionOperationID, liveFixture(1))
	t.AdmissionIncarnation = dispatch.AdmissionIncarnation
	t.AdmissionOwnerEpoch = dispatch.AdmissionOwnerEpoch
	t.AdmissionLive = dispatch.AdmissionLive
	return t, err
}
func admitLifecycleParticipantForTest(ctx context.Context, s *RedisCallStore, room string, p SpaceMediaParticipant, max int) (Call, error) {
	if p.AdmissionOperationID == "" {
		p.AdmissionOperationID = "fixture:" + p.Generation
	}
	t, err := participantTicket(ctx, s, room, p)
	if err != nil {
		return Call{}, err
	}
	p.ProjectionTicket = t
	return s.AdmitSpaceMediaParticipant(ctx, room, p, max)
}
func addLifecycleParticipantForTest(ctx context.Context, s *RedisCallStore, room, profile string, max int) (Call, error) {
	call, err := s.GetCall(ctx, room)
	if err != nil {
		return Call{}, err
	}
	if call.SpaceID != "" {
		t, err := fixtureTicket(ctx, s, call)
		if err != nil {
			return Call{}, err
		}
		t.ProfileID = profile
		ctx = WithSpaceProjectionTicket(ctx, t)
	}
	return s.AddParticipant(ctx, room, profile, max)
}
func moveLifecycleParticipantForTest(ctx context.Context, s *RedisCallStore, req VoiceRoomMoveRequest) (VoiceRoomMoveResult, error) {
	room, err := s.client.Get(ctx, s.activeVoiceRoomKey(req.FromVoiceRoomID)).Result()
	if err != nil {
		return s.MoveVoiceRoomParticipant(ctx, req)
	}
	call, err := s.GetCall(ctx, room)
	if err != nil {
		return s.MoveVoiceRoomParticipant(ctx, req)
	}
	if call.SpaceID != "" {
		t, err := fixtureTicket(ctx, s, call)
		if err != nil {
			return VoiceRoomMoveResult{}, err
		}
		req.ProjectionTicket = t
	}
	return s.MoveVoiceRoomParticipant(ctx, req)
}
func projectionShell() Call {
	return Call{RoomID: "call", SpaceID: "space", VoiceRoomID: "voice", SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, States: map[string]ParticipantState{}, Status: callsv1.CallStatus_CALL_STATUS_UNSPECIFIED}
}

func TestRedisSpaceGatePendingShellPreservesAnotherRoomOwner(t *testing.T) {
	ctx := t.Context()
	s, client := newSpaceGateStoreForTest(t, "shell-owner:")
	call := projectionShell()
	require.NoError(t, s.EnsureSpaceLifecycle(ctx, call.SpaceID, liveFixture(0)))
	ticket, err := fixtureTicket(ctx, s, call)
	require.NoError(t, err)
	call.SpaceProjectionTicket = ticket
	require.NoError(t, client.Set(ctx, s.activeVoiceRoomKey(call.VoiceRoomID), "new-call", 0).Err())
	_, err = s.CreateCall(ctx, call)
	require.ErrorIs(t, err, ErrActiveCall)
	require.Equal(t, "new-call", client.Get(ctx, s.activeVoiceRoomKey(call.VoiceRoomID)).Val())
	require.Zero(t, client.Exists(ctx, s.callKey(call.RoomID)).Val())
}

func TestRedisSpaceGateAddRejectsTicketForDifferentNativeTarget(t *testing.T) {
	ctx := t.Context()
	s, client := newSpaceGateStoreForTest(t, "add-target:")
	call := projectionShell()
	call.Status = callsv1.CallStatus_CALL_STATUS_ACTIVE
	_, err := createLifecycleCallForTest(ctx, s, call)
	require.NoError(t, err)
	ticket, err := fixtureTicket(ctx, s, call)
	require.NoError(t, err)
	ticket.ProfileID = "authorized-member"
	before, err := client.Get(ctx, s.callKey(call.RoomID)).Result()
	require.NoError(t, err)
	_, err = s.AddParticipant(WithSpaceProjectionTicket(ctx, ticket), call.RoomID, "different-member", 32)
	require.ErrorIs(t, err, ErrSpaceMediaStaleGrant)
	require.Equal(t, before, client.Get(ctx, s.callKey(call.RoomID)).Val())
	require.Zero(t, client.Exists(ctx, s.activeKey("different-member")).Val())
}

func TestRedisSpaceGateSameGenerationABARejectsOldDispatch(t *testing.T) {
	ctx := t.Context()
	s, client := newSpaceGateStoreForTest(t, "gate:")
	call := projectionShell()
	_, err := s.CreateCall(ctx, call)
	require.ErrorIs(t, err, ErrSpaceMediaStaleGrant)
	require.NoError(t, s.EnsureSpaceLifecycle(ctx, call.SpaceID, liveFixture(0)))
	old, err := fixtureTicket(ctx, s, call)
	require.NoError(t, err)
	call.SpaceProjectionTicket = old
	require.NoError(t, client.Del(ctx, s.spaceLifecycleKey(call.SpaceID)).Err())
	require.NoError(t, s.EnsureSpaceLifecycle(ctx, call.SpaceID, liveFixture(0)))
	fresh, err := fixtureTicket(ctx, s, call)
	require.NoError(t, err)
	require.NotEqual(t, old.Incarnation, fresh.Incarnation)
	_, err = s.CreateCall(ctx, call)
	require.ErrorIs(t, err, ErrSpaceMediaStaleGrant, "same lifecycle generation cannot revive E1")
	call.SpaceProjectionTicket = fresh
	_, err = s.CreateCall(ctx, call)
	require.NoError(t, err)
	require.NoError(t, s.SetSpaceLifecycle(ctx, call.SpaceID, 1, false))
	require.NoError(t, client.Del(ctx, s.spaceLifecycleKey(call.SpaceID)).Err())
	require.NoError(t, s.EnsureSpaceLifecycle(ctx, call.SpaceID, denyFixture(1)))
	call.SpaceProjectionTicket = old
	_, err = s.CreateCall(ctx, call)
	require.ErrorIs(t, err, ErrSpaceMediaStaleGrant, "loss of ONLY gate while call survives must not restore LIVE")
	require.ErrorIs(t, s.SetSpaceLifecycle(ctx, call.SpaceID, 0, true), ErrInvalidState, "ordinary projection never initializes LIVE")
}
func TestRedisSpaceGateStaleInitializerCannotPublishAcrossLossOrDeny(t *testing.T) {
	for _, mode := range []string{"loss", "freeze"} {
		t.Run(mode, func(t *testing.T) {
			ctx := t.Context()
			s, client := newSpaceGateStoreForTest(t, "restore:")
			sampled := make(chan struct{})
			resume := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				done <- s.EnsureSpaceLifecycle(ctx, "space", func(context.Context, string) (int64, bool, error) { close(sampled); <-resume; return 0, true, nil })
			}()
			<-sampled
			if mode == "loss" {
				require.NoError(t, client.Del(ctx, s.spaceLifecycleKey("space")).Err())
				require.NoError(t, s.EnsureSpaceLifecycle(ctx, "space", liveFixture(0)))
			} else {
				require.NoError(t, s.SetSpaceLifecycle(ctx, "space", 1, false))
			}
			winner, err := client.Get(ctx, s.spaceLifecycleKey("space")).Result()
			require.NoError(t, err)
			close(resume)
			require.NoError(t, <-done)
			current, err := client.Get(ctx, s.spaceLifecycleKey("space")).Result()
			require.NoError(t, err)
			require.Equal(t, winner, current, "old PG candidate cannot overwrite current E/deny")
		})
	}
}
func TestRedisSpaceGateRestoringOneSpaceDoesNotPauseAnother(t *testing.T) {
	ctx := t.Context()
	s, _ := newSpaceGateStoreForTest(t, "scoped:")
	sampled := make(chan struct{})
	resume := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.EnsureSpaceLifecycle(ctx, "restoring", func(context.Context, string) (int64, bool, error) { close(sampled); <-resume; return 0, true, nil })
	}()
	<-sampled
	_, err := s.ReadReadySpaceGate(ctx, "restoring")
	require.Error(t, err)
	call := projectionShell()
	_, err = createLifecycleCallForTest(ctx, s, call)
	require.NoError(t, err)
	close(resume)
	require.NoError(t, <-done)
}
func TestRedisSpaceGateSnapshotRollbackNeedsFencedRecovery(t *testing.T) {
	ctx := t.Context()
	s, client := newSpaceGateStoreForTest(t, "rollback:")
	call := projectionShell()
	require.NoError(t, s.EnsureSpaceLifecycle(ctx, call.SpaceID, liveFixture(0)))
	old, err := fixtureTicket(ctx, s, call)
	require.NoError(t, err)
	snapshot, err := client.Get(ctx, s.spaceLifecycleKey(call.SpaceID)).Result()
	require.NoError(t, err)
	require.NoError(t, s.SetSpaceLifecycle(ctx, call.SpaceID, 1, false))
	require.NoError(t, client.Set(ctx, s.spaceLifecycleKey(call.SpaceID), snapshot, 0).Err())
	restored, err := s.ReadReadySpaceGate(ctx, call.SpaceID)
	require.NoError(t, err)
	require.Equal(t, old.Incarnation, restored.Incarnation, "negative fixture: nonce alone cannot detect same-history rollback")
	require.NoError(t, s.ReinitializeSpaceLifecycle(ctx, call.SpaceID, denyFixture(1)))
	_, err = s.ReadReadySpaceGate(ctx, call.SpaceID)
	require.Error(t, err, "recovery must fence writes and reinitialize before exposure")
}

func TestRedisSpaceGateChangeBeforeExecRetainsOldTicketAndNoMoveEffects(t *testing.T) {
	ctx := t.Context()
	s, client := newSpaceGateStoreForTest(t, "exec:")
	source := projectionShell()
	source.RoomID = "source-call"
	source.VoiceRoomID = "source"
	source.Status = callsv1.CallStatus_CALL_STATUS_ACTIVE
	source.States = map[string]ParticipantState{"member": {ProfileID: "member"}}
	destination := projectionShell()
	destination.RoomID = "destination-call"
	destination.VoiceRoomID = "destination"
	destination.Status = callsv1.CallStatus_CALL_STATUS_ACTIVE
	destination.States = map[string]ParticipantState{"other": {ProfileID: "other"}}
	_, err := createLifecycleCallForTest(ctx, s, source)
	require.NoError(t, err)
	_, err = createLifecycleCallForTest(ctx, s, destination)
	require.NoError(t, err)
	ticket, err := fixtureTicket(ctx, s, source)
	require.NoError(t, err)
	beforeSource, err := client.Get(ctx, s.callKey(source.RoomID)).Result()
	require.NoError(t, err)
	beforeDestination, err := client.Get(ctx, s.callKey(destination.RoomID)).Result()
	require.NoError(t, err)
	attempts := 0
	s.moveTestHooks.beforeMoveExec = func(ctx context.Context, _ int, _ []string) error {
		attempts++
		return s.SetSpaceLifecycle(ctx, source.SpaceID, 1, false)
	}
	req := VoiceRoomMoveRequest{ProjectionTicket: ticket, ActorProfileID: "actor", ParticipantProfileID: "member", OperationID: "operation", FromVoiceRoomID: source.VoiceRoomID, ToVoiceRoomID: destination.VoiceRoomID, SpaceID: source.SpaceID, MaxParticipants: 32}
	_, err = s.MoveVoiceRoomParticipant(ctx, req)
	require.ErrorIs(t, err, ErrSpaceMediaStaleGrant)
	require.Equal(t, 1, attempts, "internal retry must retain E/G rather than authorize anew")
	afterSource, err := client.Get(ctx, s.callKey(source.RoomID)).Result()
	require.NoError(t, err)
	afterDestination, err := client.Get(ctx, s.callKey(destination.RoomID)).Result()
	require.NoError(t, err)
	require.Equal(t, beforeSource, afterSource)
	require.Equal(t, beforeDestination, afterDestination)
	require.Zero(t, client.Exists(ctx, s.moveOperationKey("actor", "operation")).Val())
}

func TestRedisSpaceGateGenericMetadataAndExactCleanupNeverRebuildNewOwnerIndexes(t *testing.T) {
	ctx := t.Context()
	s, client := newSpaceGateStoreForTest(t, "cleanup:")
	call := projectionShell()
	call.Status = callsv1.CallStatus_CALL_STATUS_ACTIVE
	call.States = map[string]ParticipantState{"member": {ProfileID: "member"}}
	_, err := createLifecycleCallForTest(ctx, s, call)
	require.NoError(t, err)
	p := testSpaceMediaParticipant("member", "identity", "generation", 1, 1)
	_, err = admitLifecycleParticipantForTest(ctx, s, call.RoomID, p, 32)
	require.NoError(t, err)
	require.NoError(t, s.SetSpaceLifecycle(ctx, call.SpaceID, 1, false))
	require.NoError(t, client.Del(ctx, s.activeKey("member"), s.activeVoiceRoomKey(call.VoiceRoomID)).Err())
	muted := true
	_, _, err = s.UpdateVoiceState(ctx, call.RoomID, "member", VoiceStatePatch{IsMuted: &muted})
	require.NoError(t, err)
	require.Zero(t, client.Exists(ctx, s.activeKey("member"), s.activeVoiceRoomKey(call.VoiceRoomID)).Val(), "state writer is metadata-only and may not restore indexes")
	require.NoError(t, client.Set(ctx, s.activeKey("member"), "new-call", 0).Err())
	require.NoError(t, client.Set(ctx, s.activeVoiceRoomKey(call.VoiceRoomID), "new-call", 0).Err())
	ctx = fixtureCleanupContext(t, ctx, s, call.RoomID, p)
	_, matched, err := s.BeginSpaceMediaRevocation(ctx, call.RoomID, "member", p.Identity, p.Generation)
	require.NoError(t, err)
	require.True(t, matched)
	_, matched, err = s.CompleteSpaceMediaRevocation(ctx, call.RoomID, "member", p.Identity, p.Generation)
	require.NoError(t, err)
	require.True(t, matched)
	require.Equal(t, "new-call", client.Get(ctx, s.activeKey("member")).Val())
	require.Equal(t, "new-call", client.Get(ctx, s.activeVoiceRoomKey(call.VoiceRoomID)).Val())
}

// This fixture explicitly obtains a fresh restrictive admission gate. It is
// separate from production PG authority and makes no database ownership claim.
func fixtureCleanupContext(t *testing.T, ctx context.Context, s *RedisCallStore, room string, p SpaceMediaParticipant) context.Context {
	t.Helper()
	call, err := s.GetCall(ctx, room)
	require.NoError(t, err)
	ticket, err := s.EnsureAdmissionDispatch(ctx, p.AdmissionOperationID, func(context.Context, string) (int64, bool, error) { return 1, false, nil })
	require.NoError(t, err)
	ticket.OperationID = p.AdmissionOperationID
	ticket.RoomID = room
	ticket.SpaceID = call.SpaceID
	ticket.VoiceRoomID = call.VoiceRoomID
	ticket.RoomGeneration = p.RoomGeneration
	ticket.AdmissionGeneration = p.Generation
	ticket.AccountID = p.AccountID
	ticket.ProfileID = p.ProfileID
	ticket.Identity = p.Identity
	return WithSpaceCleanupTarget(ctx, ticket)
}
