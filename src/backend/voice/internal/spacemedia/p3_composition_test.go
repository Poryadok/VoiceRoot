package spacemedia

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/spacelifecycle"
	"voice/backend/voice/internal/store"
)

type p3Media struct{ removed [][2]string }

func (*p3Media) RetirementCapability() string { return "EJECT_ONLY" }

func (m *p3Media) RemoveParticipant(_ context.Context, room, identity string) error {
	m.removed = append(m.removed, [2]string{room, identity})
	return nil
}
func (*p3Media) CloseRoom(context.Context, string) error { return nil }

func TestP3CurrentNativeCompositionWithoutLegacyOrRedis(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	admissions := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	a.Identity, _ = livekit.SpaceParticipantIdentity(a.ProfileID.String(), a.Generation)
	require.NoError(t, prepareAdmission(ctx, admissions, &a))
	require.NoError(t, admissions.Fence(ctx, a))
	require.NoError(t, admissions.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, admissions.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	calls := store.NewRedisCallStore(client, "p3:")
	media := &p3Media{}
	coordinator := &Coordinator{Calls: calls, Store: calls, Admissions: admissions, Media: media}
	effects := spacelifecycle.NewEffects(pool, media)
	effects.NativeDrain = coordinator
	effects.ProjectionFence = calls
	svc := &spacelifecycle.Service{Store: spacelifecycle.NewPostgresStore(pool), Effects: effects}
	frozen := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: a.SpaceID.String(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}}
	receipt, err := svc.ApplySpaceLifecycleFence(ctx, frozen)
	require.ErrorIs(t, err, spacelifecycle.ErrUnavailable)
	require.Nil(t, receipt)
	require.Equal(t, [][2]string{{"voice-room-" + a.VoiceRoomID, a.Identity}}, media.removed, "absent Redis must not suppress exact SFU ejection")
	var targets int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_lifecycle_targets WHERE space_id=$1`, a.SpaceID).Scan(&targets))
	require.Equal(t, 2, targets)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT participant_state FROM voice_space_media_admissions WHERE operation_id=$1`, a.OperationID).Scan(&state))
	require.Equal(t, "ENDED", state)
	require.Error(t, admissions.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.Error(t, admissions.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	_, err = admissions.ProjectionGeneration(ctx, a.OperationID, a.Generation)
	require.Error(t, err)
	require.NoError(t, svc.ResumePending(ctx))
	require.Len(t, media.removed, 2, "restart resumes same saved target without discovering a newer participant")
	var remaining int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_lifecycle_targets WHERE space_id=$1`, a.SpaceID).Scan(&remaining))
	require.Equal(t, targets, remaining)
	changed := proto.Clone(frozen).(*commonv1.SpaceLifecycleFenceRequest)
	changed.Manifest.ItemCount++
	_, err = svc.ApplySpaceLifecycleFence(ctx, changed)
	require.ErrorIs(t, err, spacelifecycle.ErrConflict)
	require.Len(t, media.removed, 2)
	live := proto.Clone(frozen).(*commonv1.SpaceLifecycleFenceRequest)
	live.Generation = 2
	live.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	_, err = svc.ApplySpaceLifecycleFence(ctx, live)
	require.Error(t, err)
	require.Equal(t, "EJECT_ONLY", svc.RetirementCapability())
	other := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, admissions, &other), "unaffected LIVE Space remains usable")
}

func TestSpaceGrantReservationFixedExpiryAndFreezeBarrier(t *testing.T) {
	ctx := context.Background()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	s := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	a.Identity, _ = livekit.SpaceParticipantIdentity(a.ProfileID.String(), a.Generation)
	require.NoError(t, prepareAdmission(ctx, s, &a))
	require.NoError(t, s.Fence(ctx, a))
	require.NoError(t, s.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, s.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	grant := livekit.SpaceRoomGrant{AccountID: a.AccountID.String(), ProfileID: a.ProfileID.String(), SpaceID: a.SpaceID.String(), VoiceRoomID: a.VoiceRoomID, LiveKitRoomName: "voice-room-" + a.VoiceRoomID, MediaGeneration: a.Generation, SessionEpoch: a.SessionEpoch, AccessEpoch: a.AccessEpoch, PolicyEpoch: a.PolicyEpoch, CanJoin: true, CanSubscribe: true}
	id := uuid.New()
	issued, expires, err := s.ReserveSpaceGrant(ctx, id, a.OperationID, a.Generation, grant)
	require.NoError(t, err)
	require.Equal(t, 60*time.Second, expires.Sub(issued))
	reissued, reexpires, err := s.ReserveSpaceGrant(ctx, id, a.OperationID, a.Generation, grant)
	require.NoError(t, err)
	require.True(t, issued.Equal(reissued))
	require.True(t, expires.Equal(reexpires))
	frozen := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: a.SpaceID.String(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}}
	_, err = spacelifecycle.NewPostgresStore(pool).ApplyFence(ctx, frozen, func(context.Context, *commonv1.SpaceLifecycleFenceRequest) error { return nil })
	require.Error(t, err)
	_, _, err = s.ReserveSpaceGrant(ctx, uuid.New(), a.OperationID, a.Generation, grant)
	require.Error(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_grants WHERE operation_id=$1`, a.OperationID).Scan(&count))
	require.Equal(t, 1, count, "no new grant authorized after freeze")
}
