package spacemedia

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	callsv1 "voice.app/voice/calls/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/spacelifecycle"
	"voice/backend/voice/internal/store"
)

func TestP3GateRejectsOldAuthorizedDispatchAfterPartialRedisLoss(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	a.Identity, _ = livekit.SpaceParticipantIdentity(a.ProfileID.String(), a.Generation)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	require.NoError(t, s.Fence(ctx, a))
	require.NoError(t, s.MarkCommitted(ctx, a.OperationID, a.Generation))
	calls, client := newP3Redis(t)
	require.NoError(t, calls.EnsureSpaceLifecycle(ctx, a.SpaceID.String(), s.ReadSpaceLifecycle))
	gate, err := calls.ReadReadySpaceGate(ctx, a.SpaceID.String())
	require.NoError(t, err)
	ticket, err := authorizeNativeFixtureProjection(ctx, s, calls, a, gate)
	require.NoError(t, err)
	call := store.Call{RoomID: a.RoomID, VoiceRoomID: a.VoiceRoomID, SpaceID: a.SpaceID.String(), SpaceLifecycleGeneration: ticket.Generation, SpaceProjectionTicket: ticket, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, LivekitRoomName: "voice-room-" + a.VoiceRoomID, States: map[string]store.ParticipantState{}, Status: callsv1.CallStatus_CALL_STATUS_UNSPECIFIED}
	_, err = calls.CreateCall(ctx, call)
	require.NoError(t, err)
	// Pause the second dispatch after native authorization. FROZEN captures this
	// COMMITTED-but-unprojected intent before exact shell/participant cleanup.
	media := &p3Media{}
	coordinator := &Coordinator{ProjectionGates: calls, Calls: calls, Store: calls, Admissions: s}
	effects := spacelifecycle.NewEffects(pool, media)
	effects.NativeDrain = coordinator
	effects.ProjectionFence = calls
	svc := &spacelifecycle.Service{Store: spacelifecycle.NewPostgresStore(pool, BeginPartyRoomClose), Effects: effects}
	frozen := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: a.SpaceID.String(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}}
	receipt, err := svc.ApplySpaceLifecycleFence(ctx, frozen)
	require.ErrorIs(t, err, spacelifecycle.ErrRetirementUnsupported)
	require.Nil(t, receipt)
	require.NoError(t, client.Del(ctx, "p3:space_lifecycle:"+a.SpaceID.String()).Err())
	require.NoError(t, calls.EnsureSpaceLifecycle(ctx, a.SpaceID.String(), s.ReadSpaceLifecycle))
	before, err := client.Get(ctx, "p3:call:"+a.RoomID).Result()
	require.NoError(t, err)
	participant := store.SpaceMediaParticipant{ProjectionTicket: ticket, LifecycleGeneration: ticket.Generation, AccountID: a.AccountID.String(), AdmissionOperationID: a.OperationID.String(), ProfileID: a.ProfileID.String(), RoomGeneration: a.RoomGeneration, Identity: a.Identity, Generation: a.Generation, Issued: store.SpaceMediaGrant{SessionEpoch: a.SessionEpoch, AccessEpoch: a.AccessEpoch, PolicyEpoch: a.PolicyEpoch, CanJoin: true, CanSubscribe: true}}
	_, err = calls.AdmitSpaceMediaParticipant(ctx, a.RoomID, participant, a.MaxParticipants)
	require.Error(t, err)
	after, err := client.Get(ctx, "p3:call:"+a.RoomID).Result()
	require.NoError(t, err)
	require.Equal(t, before, after, "old callback may not mutate retained call after gate loss")
	// The exact pending shell is now terminal, so admission may reject its state
	// first. Also remove the call document: the gate itself must then reject the
	// delayed create, rather than relying on an existing/ended call as protection.
	require.NoError(t, client.Del(ctx, "p3:call:"+a.RoomID).Err())
	_, err = calls.CreateCall(ctx, call)
	require.ErrorIs(t, err, store.ErrSpaceMediaStaleGrant)
	require.NoError(t, client.FlushDB(ctx).Err())
	require.NoError(t, calls.EnsureSpaceLifecycle(ctx, a.SpaceID.String(), s.ReadSpaceLifecycle))
	_, err = calls.CreateCall(ctx, call)
	require.ErrorIs(t, err, store.ErrSpaceMediaStaleGrant, "FLUSHALL cannot revive the old native dispatch")
	_, err = authorizeNativeFixtureProjection(ctx, s, calls, a, gate)
	require.Error(t, err, "READY acquired before freeze cannot authorize after it")
	require.Len(t, media.removed, 1)
}

func TestP3GateSameGenerationNeedsNewNativeAuthorization(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	ctx = ownAdmission(ctx, &a)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	require.NoError(t, s.Fence(ctx, a))
	require.NoError(t, s.MarkCommitted(ctx, a.OperationID, a.Generation))
	calls, client := newP3Redis(t)
	require.NoError(t, calls.EnsureSpaceLifecycle(ctx, a.SpaceID.String(), s.ReadSpaceLifecycle))
	gate, err := calls.ReadReadySpaceGate(ctx, a.SpaceID.String())
	require.NoError(t, err)
	old, err := authorizeNativeFixtureProjection(ctx, s, calls, a, gate)
	require.NoError(t, err)
	require.NoError(t, client.Del(ctx, "p3:space_lifecycle:"+a.SpaceID.String()).Err())
	require.NoError(t, calls.EnsureSpaceLifecycle(ctx, a.SpaceID.String(), s.ReadSpaceLifecycle))
	call := store.Call{RoomID: a.RoomID, VoiceRoomID: a.VoiceRoomID, SpaceID: a.SpaceID.String(), SpaceProjectionTicket: old, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, States: map[string]store.ParticipantState{}, Status: callsv1.CallStatus_CALL_STATUS_UNSPECIFIED}
	_, err = calls.CreateCall(ctx, call)
	require.ErrorIs(t, err, store.ErrSpaceMediaStaleGrant)
	fresh, err := calls.ReadReadySpaceGate(ctx, a.SpaceID.String())
	require.NoError(t, err)
	require.NotEqual(t, old.Incarnation, fresh.Incarnation)
	call.SpaceProjectionTicket, err = authorizeNativeFixtureProjection(ctx, s, calls, a, fresh)
	require.NoError(t, err)
	_, err = calls.CreateCall(ctx, call)
	require.NoError(t, err)
}

// The native fixture acquires both gates before fresh PG authorization. A stale
// Space ticket stays stale; this helper never replaces its Space incarnation.
func authorizeNativeFixtureProjection(ctx context.Context, s *PostgresAdmissionStore, calls *store.RedisCallStore, a Admission, gate store.SpaceProjectionTicket) (store.SpaceProjectionTicket, error) {
	dispatch, err := calls.EnsureAdmissionDispatch(ctx, a.OperationID.String(), s.ReadAdmissionDispatch)
	if err != nil {
		return store.SpaceProjectionTicket{}, err
	}
	gate.AdmissionIncarnation = dispatch.AdmissionIncarnation
	gate.AdmissionOwnerEpoch = dispatch.AdmissionOwnerEpoch
	gate.AdmissionLive = dispatch.AdmissionLive
	return s.AuthorizeSpaceProjection(ctx, a.OperationID, a.Generation, gate)
}
