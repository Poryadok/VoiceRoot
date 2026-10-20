package grpcsvc

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
	"voice/backend/space/internal/store"
)

type nativeP3ProviderDecision struct {
	OperationID     uuid.UUID
	Generation      uint64
	Manifest        *commonv1.ManifestBinding
	Frozen, Decided *commonv1.SpaceLifecycleFenceRequest
	Purge           *commonv1.SpacePurgeRequest
	Role            *rolev1.RetireSpaceReceipt
	PurgeDecidedAt  time.Time
}

// This is controlled upstream fixture evidence admitted through the actual
// qualified coordinator. It is not remote Auth/Role/Chat/participant execution.
// Neither Bot nor aggregate completion is manufactured here.
func nativeP3ProviderDecide(t *testing.T, ctx context.Context, admin, serving *store.SpaceStore, space, account, owner uuid.UUID, name string) nativeP3ProviderDecision {
	t.Helper()
	op := uuid.New()
	r := &spacev1.DeleteSpaceRequest{SpaceId: space.String(), OperationId: op.String(), ConfirmationName: name, Proof: "controlled-auth-proof"}
	_, err := serving.ReserveLifecycleSchedule(ctx, account, owner, 7, r)
	require.NoError(t, err)
	auth := validDeletionProofResponse(account, owner, 7, space, op, name, r.Proof)
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(auth)
	require.NoError(t, err)
	hash := lifecycleEvidenceHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", raw)
	_, err = serving.RecordLifecycleDeletionProofReceipt(ctx, owner, op, raw, hash)
	require.NoError(t, err)
	// Explicit controlled empty sealed-Chat manifest binding. No Messaging
	// original/coverage or actual Chat seal transport is claimed by this mode.
	empty := sha256.Sum256([]byte("voice.test.sealed.empty-chat-manifest.v1"))
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: empty[:], ItemCount: 0}
	a, err := serving.BeginLifecycleFreeze(ctx, space, manifest)
	require.NoError(t, err)
	frozen, err := a.FenceRequest(commonv1.ParticipantId_PARTICIPANT_ID_BOT)
	require.NoError(t, err)
	packages := []string{"role", "chat", "messaging", "file", "calls", "matchmaking", "search", "subscription", "bot", "notification", "realtime"}
	record := func(state commonv1.LifecycleFenceState) {
		for i, p := range spacecore.CanonicalLifecycleParticipants() {
			r, err := a.FenceRequest(p)
			require.NoError(t, err)
			inner, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
			require.NoError(t, err)
			wrapper := protowire.AppendBytes(protowire.AppendTag(nil, 1, protowire.BytesType), inner)
			h := lifecycleEvidenceHash("voice."+packages[i]+".v1.ApplySpaceLifecycleFenceRequest", wrapper)
			a, err = serving.RecordLifecycleFenceReceipt(ctx, &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: space.String(), DeletionOperationId: op.String(), Generation: a.Generation(), ParticipantId: p, AppliedState: state, RequestSha256: h[:], ManifestSha256: manifest.ManifestSha256, AppliedAt: timestamppb.Now()})
			require.NoError(t, err)
		}
	}
	record(commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	_, _, err = serving.CompleteLifecycleSchedule(ctx, space)
	require.NoError(t, err)
	// The database clock was set before ALL persisted samples. Only now bring
	// it to actual wall time: the real seven-day predicate observes elapsed
	// fixture time, without rewriting a persisted first clock or Auth/JWT time.
	setNativeP3ProviderDBTime(t, admin, time.Now().UTC().Truncate(time.Second))
	a, err = serving.DecideLifecycleRecovery(ctx, space, 2)
	require.NoError(t, err)
	decided, err := a.FenceRequest(commonv1.ParticipantId_PARTICIPANT_ID_BOT)
	require.NoError(t, err)
	record(commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	require.NoError(t, a.BeginPurging())
	require.NoError(t, serving.PersistLifecycle(ctx, a))
	retire, err := a.RoleRetirementRequest()
	require.NoError(t, err)
	raw, err = proto.MarshalOptions{Deterministic: true}.Marshal(retire)
	require.NoError(t, err)
	hash = lifecycleEvidenceHash("voice.role.v1.RetireSpaceRequest", raw)
	role := &rolev1.RetireSpaceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: space.String(), DeletionOperationId: op.String(), Generation: 2, State: rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, RequestSha256: hash[:], ManifestSha256: manifest.ManifestSha256, RetiredAt: timestamppb.Now()}
	a, err = serving.RecordLifecycleRoleRetirementReceipt(ctx, role)
	require.NoError(t, err)
	purge, err := a.PurgeRequest(commonv1.ParticipantId_PARTICIPANT_ID_BOT)
	require.NoError(t, err)
	return nativeP3ProviderDecision{op, a.Generation(), manifest, frozen, decided, purge, role, a.Snapshot().PurgeDecidedAt}
}
