package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalruntime"
	"voice/backend/role/internal/store"
)

func TestRoleSpaceLifecycleFreezeRestoreDurableAndFencesOrdinaryWork(t *testing.T) {
	if testing.Short() {
		t.Skip("requires test-owned PostgreSQL")
	}
	st, cleanup := startRoleStoreTest(t)
	defer cleanup()
	ctx := context.Background()
	owner, space, operation := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	svc := &RoleGRPC{Store: st}
	fixture := newOwnershipV2RuntimeFixture(t)
	runtime, err := principalruntime.New(ctx, fixture.config)
	require.NoError(t, err)
	client, stop := startOwnershipV2ProtectedServer(t, runtime, fixture.roots, fixture.clientCert, svc)
	t.Cleanup(func() { stop(); require.NoError(t, runtime.Close()) })
	req := &rolev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: bytes.Repeat([]byte{1}, 32)}}}
	apply := func(request *rolev1.ApplySpaceLifecycleFenceRequest) (*rolev1.ApplySpaceLifecycleFenceResponse, error) {
		signed, _ := ownershipV2SignedContext(t, fixture.key, rolev1.RoleService_ApplySpaceLifecycleFence_FullMethodName, uuid.NewString(), request)
		return client.ApplySpaceLifecycleFence(signed, request)
	}
	_, err = svc.ApplySpaceLifecycleFence(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "raw handler call cannot fence Role")
	frozen, err := apply(req)
	require.NoError(t, err)
	requestWire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	require.NoError(t, err)
	expectedHash := sha256.Sum256(append([]byte("voice.role.v1.ApplySpaceLifecycleFenceRequest\x00"), requestWire...))
	require.Equal(t, expectedHash[:], frozen.Receipt.RequestSha256, "receipt must bind the full participant RPC request with its protobuf type domain")
	require.Equal(t, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, frozen.Receipt.AppliedState)
	require.Equal(t, uint64(1), frozen.Receipt.Generation)
	_, err = st.ListRoles(ctx, space)
	require.ErrorIs(t, err, store.ErrSpaceFrozen)
	require.ErrorIs(t, st.BootstrapSpaceRoles(ctx, space, owner), store.ErrSpaceFrozen)
	// Handler restart uses the same durable receipt and cannot repeat its clock.
	stop()
	require.NoError(t, runtime.Close())
	runtime, err = principalruntime.New(ctx, fixture.config)
	require.NoError(t, err)
	svc = &RoleGRPC{Store: &store.RoleStore{Pool: st.Pool}}
	client, stop = startOwnershipV2ProtectedServer(t, runtime, fixture.roots, fixture.clientCert, svc)
	replay, err := apply(req)
	require.NoError(t, err)
	require.True(t, proto.Equal(frozen, replay))
	changed := proto.Clone(req).(*rolev1.ApplySpaceLifecycleFenceRequest)
	changed.Fence.Manifest.ManifestSha256[0] ^= 1
	_, err = apply(changed)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	live := proto.Clone(req).(*rolev1.ApplySpaceLifecycleFenceRequest)
	live.Fence.Generation = 2
	live.Fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	_, err = apply(live)
	require.NoError(t, err)
	_, err = st.ListRoles(ctx, space)
	require.NoError(t, err)
	_, err = apply(req)
	require.NoError(t, err, "old exact receipt remains replayable")
	_, err = st.ListRoles(ctx, space)
	require.NoError(t, err, "old frozen replay must not replace current LIVE")
	second := proto.Clone(req).(*rolev1.ApplySpaceLifecycleFenceRequest)
	second.Fence.Generation = 3
	second.Fence.DeletionOperationId = uuid.NewString()
	second.Fence.Manifest.ManifestId = uuid.NewString()
	_, err = apply(second)
	require.NoError(t, err)
	purge := proto.Clone(second).(*rolev1.ApplySpaceLifecycleFenceRequest)
	purge.Fence.Generation = 4
	purge.Fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = apply(purge)
	require.NoError(t, err)
	late := proto.Clone(purge).(*rolev1.ApplySpaceLifecycleFenceRequest)
	late.Fence.Generation = 5
	late.Fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	_, err = apply(late)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = st.ListRoles(ctx, space)
	require.ErrorIs(t, err, store.ErrSpaceFrozen)
}

func TestRoleSpaceLifecycleReceiptRetentionKeepsPermanentSemanticReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("requires test-owned PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	ctx := context.Background()
	_, err := f.store.Pool.Exec(ctx, `DELETE FROM role_space_deletion_fences WHERE space_id=$1`, f.spaceID)
	require.NoError(t, err)
	req := &rolev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: f.spaceID.String(), DeletionOperationId: f.deletionOperationID.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: f.manifestID.String(), ManifestSha256: f.manifestSHA, ItemCount: 1}}}
	apply := func(request *rolev1.ApplySpaceLifecycleFenceRequest) (*rolev1.ApplySpaceLifecycleFenceResponse, error) {
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		return f.svc.ApplySpaceLifecycleFence(principal.WithVerified(ctx, principal.Principal{Kind: "service", Subject: "service:space", Issuer: "space", Audience: "role", RPC: rolev1.RoleService_ApplySpaceLifecycleFence_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash}), request)
	}
	saved, err := apply(req)
	require.NoError(t, err)
	purge := proto.Clone(req).(*rolev1.ApplySpaceLifecycleFenceRequest)
	purge.Fence.Generation = 2
	purge.Fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = apply(purge)
	require.NoError(t, err)
	retire := f.request()
	retire.Generation = 2
	retire.PurgeDecidedAt = saved.Receipt.AppliedAt
	retire.Manifest = proto.Clone(req.Fence.Manifest).(*commonv1.ManifestBinding)
	_, err = f.retire(t, retire)
	require.NoError(t, err)
	require.NoError(t, f.store.CleanupSpaceDeletionFenceEvidence(ctx))
	var full int
	require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT count(*) FROM role_space_deletion_fence_receipts WHERE request_bytes IS NOT NULL`).Scan(&full))
	require.Equal(t, 2, full, "full evidence remains during retention")
	_, err = f.store.Pool.Exec(ctx, `UPDATE role_space_deletion_fence_receipts SET receipt_bytes=NULL,request_bytes=NULL WHERE space_id=$1`, f.spaceID)
	require.Error(t, err, "database rejects early compaction")
	// Age only this test-owned permanent fence, bypassing immutability inside an isolated fixture transaction.
	tx, err := f.store.Pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `SET LOCAL session_replication_role='replica'`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE role_space_lifecycle SET retired_at=clock_timestamp()-interval '31 days' WHERE space_id=$1`, f.spaceID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, f.store.CleanupSpaceDeletionFenceEvidence(ctx))
	require.NoError(t, f.store.Pool.QueryRow(ctx, `SELECT count(*) FROM role_space_deletion_fence_receipts WHERE request_bytes IS NOT NULL`).Scan(&full))
	require.Zero(t, full)
	replay, err := apply(req)
	require.NoError(t, err)
	require.True(t, proto.Equal(saved, replay))
	changed := proto.Clone(req).(*rolev1.ApplySpaceLifecycleFenceRequest)
	changed.Fence.Manifest.ItemCount++
	_, err = apply(changed)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = f.store.ListRoles(ctx, f.spaceID)
	require.ErrorIs(t, err, store.ErrSpaceRetired)
	_, err = f.store.Pool.Exec(ctx, `DELETE FROM role_space_deletion_fence_receipts WHERE space_id=$1`, f.spaceID)
	require.Error(t, err, "compact tuple remains permanent")
	require.WithinDuration(t, saved.Receipt.AppliedAt.AsTime(), replay.Receipt.AppliedAt.AsTime(), time.Microsecond)
}

func TestRoleRetirementRequiresExactPurgeFenceBeforeMutation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires test-owned PostgreSQL")
	}
	f := newR23RetirementFixture(t)
	ctx := context.Background()
	fixture := newOwnershipV2RuntimeFixture(t)
	runtime, err := principalruntime.New(ctx, fixture.config)
	require.NoError(t, err)
	defer runtime.Close()
	client, stop := startOwnershipV2ProtectedServer(t, runtime, fixture.roots, fixture.clientCert, f.svc)
	defer stop()
	retire := func(request *rolev1.RetireSpaceRequest) (*rolev1.RetireSpaceResponse, error) {
		signed, _ := ownershipV2SignedContext(t, fixture.key, rolev1.RoleService_RetireSpace_FullMethodName, uuid.NewString(), request)
		return client.RetireSpace(signed, request)
	}
	_, err = f.store.Pool.Exec(ctx, `DELETE FROM role_space_deletion_fences WHERE space_id=$1`, f.spaceID)
	require.NoError(t, err)
	req := f.request()
	req.Generation = 2
	before := r23PersistenceSnapshot(t, f)
	_, err = retire(req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "missing durable purge fence must deny retirement")
	require.Equal(t, before, r23PersistenceSnapshot(t, f))
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: req.SpaceId, DeletionOperationId: req.DeletionOperationId, Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: proto.Clone(req.Manifest).(*commonv1.ManifestBinding)}
	_, err = f.store.ApplySpaceDeletionFence(ctx, fence)
	require.NoError(t, err)
	_, err = retire(req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "FROZEN is insufficient for irreversible cleanup")
	require.Equal(t, before, r23PersistenceSnapshot(t, f))
	fence.Generation = 2
	fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = f.store.ApplySpaceDeletionFence(ctx, fence)
	require.NoError(t, err)
	for _, mutate := range []func(*rolev1.RetireSpaceRequest){func(r *rolev1.RetireSpaceRequest) { r.DeletionOperationId = uuid.NewString() }, func(r *rolev1.RetireSpaceRequest) { r.Generation++ }, func(r *rolev1.RetireSpaceRequest) { r.Manifest.ManifestId = uuid.NewString() }, func(r *rolev1.RetireSpaceRequest) { r.Manifest.ManifestSha256[0] ^= 1 }, func(r *rolev1.RetireSpaceRequest) { r.Manifest.ItemCount++ }} {
		changed := proto.Clone(req).(*rolev1.RetireSpaceRequest)
		mutate(changed)
		_, err = retire(changed)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		require.Equal(t, before, r23PersistenceSnapshot(t, f))
	}
	_, err = retire(req)
	require.NoError(t, err, "exact purge barrier permits retirement")
}
