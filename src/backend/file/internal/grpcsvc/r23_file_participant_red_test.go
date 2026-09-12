package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/principal"

	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
)

// This file is the Cycle 6 RED manifest for File's P3 participant. It must stay
// focused on the accepted reference/capability/manifest/GC contract and must not
// be weakened to preserve file_id-only authority or eager R2 deletion.
func TestR23FileParticipantREDManifest(t *testing.T) {
	want := []string{
		"signed verified-principal binding rejects raw/missing/wrong caller/RPC/hash/audience",
		"preliminary FROZEN; exact SPACE/CHAT/MESSAGING declarations; contiguous <=1000-key chunks; exact local root/count seal",
		"capability exact reference/surface/subject/use-time expiry/replay/restart/allow-list with fence recheck",
		"purge waits for all exact producer releases; acquire/release/zero-ref races preserve shared and deduplicated blobs",
		"retry/ambiguous/partial-derivative/R2-crash GC recovery; guarded DOWN; empty DOWN success; schema fingerprint",
	}
	require.Len(t, want, 5)
	require.Equal(t, "SPACE/CHAT/MESSAGING", fileProducerSet())
	deletionID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	spaceID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	require.Equal(t, "77b3beceb8b49d3b4512e133624ad471aaae8dfa125096abb0f3fb9df363a49a",
		fmt.Sprintf("%x", r23ReferencesHash(deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE, nil)),
		"zero-count declarations still hash the exact producer/deletion/Space/generation binding")
	chatManifest := r23ManifestBinding("chat-root", 0)
	declarations := make([]*filev1.FileReferenceProducerDeclaration, 0, 3)
	for _, producerID := range []filev1.FileReferenceProducerId{
		filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE,
		filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT,
		filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING,
	} {
		declarations = append(declarations, &filev1.FileReferenceProducerDeclaration{
			ProducerId: producerID, ExpectedReferencesSha256: r23ReferencesHash(deletionID, spaceID, 7, producerID, nil),
		})
	}
	require.Equal(t, "f45d3c9649ce8c601edf68bb84bb39aeee30584233e82a185d97c3a879b2f264",
		fmt.Sprintf("%x", r23SpaceDeletionManifestSet(deletionID, spaceID, 7, chatManifest, declarations).GetManifestSha256()),
		"root is the domain-separated deterministic SpaceDeletionManifestSet with root_sha256 cleared")
}

func TestR23FileServicePrincipalBinding(t *testing.T) {
	ctx := r23TestContext(t)
	request := &filev1.AcquireFileReferencesRequest{
		ProtocolVersion: 1,
		OperationId:     uuid.NewString(),
		ProducerId:      filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING,
		References:      []*filev1.FileReferenceKey{r23MessageReference(uuid.New(), uuid.New(), uuid.New())},
	}
	invoke := func(callCtx context.Context) error {
		_, err := r23FileServer(nil).AcquireFileReferences(callCtx, request)
		return err
	}
	wantDenied := func(name string, callCtx context.Context) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			require.Equal(t, codes.PermissionDenied, status.Code(invoke(callCtx)))
		})
	}

	wantDenied("missing", ctx)
	wantDenied("raw caller metadata", metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-service-id", "messaging")))
	wantDenied("wrong caller", r23PrincipalContext(t, ctx, "chat", "file", filev1.FileService_AcquireFileReferences_FullMethodName, request, ""))
	wantDenied("wrong RPC", r23PrincipalContext(t, ctx, "messaging", "file", filev1.FileService_ReleaseFileReferences_FullMethodName, request, ""))
	wantDenied("wrong request hash", r23PrincipalContext(t, ctx, "messaging", "file", filev1.FileService_AcquireFileReferences_FullMethodName, request, "sha256:"+strings.Repeat("0", 64)))
	wantDenied("wrong audience", r23PrincipalContext(t, ctx, "messaging", "chat", filev1.FileService_AcquireFileReferences_FullMethodName, request, ""))

	validErr := invoke(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, request))
	require.NotEqual(t, codes.PermissionDenied, status.Code(validErr), "a correctly signed and fully bound principal reaches File's dependency boundary")
}

func TestR23FileReferenceAcquire_DurableReplayChangedBodyAndRestart(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	fileID := insertR23ReadyFile(t, ctx, pool, "exclusive")
	reference := r23MessageReference(fileID, uuid.New(), uuid.New())
	request := &filev1.AcquireFileReferencesRequest{
		ProtocolVersion: 1,
		OperationId:     uuid.NewString(),
		ProducerId:      filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING,
		References:      []*filev1.FileReferenceKey{reference},
	}

	first := r23FileServer(pool)
	accepted, err := first.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, request), request)
	require.NoError(t, err)
	require.Equal(t, request.GetOperationId(), accepted.GetReceipt().GetOperationId())
	require.Equal(t, uint64(1), accepted.GetReceipt().GetReferenceCount())
	require.Equal(t, r23RequestHash(t, request), accepted.GetReceipt().GetRequestSha256())

	replayRequest := proto.Clone(request).(*filev1.AcquireFileReferencesRequest)
	replayed, err := first.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, replayRequest), replayRequest)
	require.NoError(t, err)
	require.True(t, proto.Equal(accepted, replayed), "identical retry must return the byte-identical durable receipt")

	restarted := r23FileServer(pool)
	restartRequest := proto.Clone(request).(*filev1.AcquireFileReferencesRequest)
	recovered, err := restarted.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, restartRequest), restartRequest)
	require.NoError(t, err)
	require.True(t, proto.Equal(accepted, recovered), "receipt must survive handler restart/response loss")

	changed := proto.Clone(request).(*filev1.AcquireFileReferencesRequest)
	changed.References[0].OwnerId = uuid.NewString()
	_, err = restarted.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, changed), changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "same operation_id with changed exact tuple must conflict")

	release := &filev1.ReleaseFileReferencesRequest{
		ProtocolVersion: 1,
		OperationId:     uuid.NewString(),
		ProducerId:      filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING,
		References:      []*filev1.FileReferenceKey{reference},
	}
	released, err := restarted.ReleaseFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseFileReferences_FullMethodName, release), release)
	require.NoError(t, err)
	require.Equal(t, uint64(1), released.GetReceipt().GetReleasedCount())
	require.Equal(t, r23RequestHash(t, release), released.GetReceipt().GetRequestSha256())
	afterResponseLoss := r23FileServer(pool)
	replayRelease := proto.Clone(release).(*filev1.ReleaseFileReferencesRequest)
	replayedRelease, err := afterResponseLoss.ReleaseFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseFileReferences_FullMethodName, replayRelease), replayRelease)
	require.NoError(t, err)
	require.True(t, proto.Equal(released, replayedRelease), "release receipt must survive response loss and restart")
	changedRelease := proto.Clone(release).(*filev1.ReleaseFileReferencesRequest)
	changedRelease.References[0].OwnerId = uuid.NewString()
	_, err = afterResponseLoss.ReleaseFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseFileReferences_FullMethodName, changedRelease), changedRelease)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestR23FileReferenceAcquireReleaseZeroRefRaceAndDeduplicatedBlob(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	primary := insertR23ReadyFile(t, ctx, pool, "dedup-primary")
	alias := insertR23AliasFile(t, ctx, pool, primary, "dedup-alias")
	spaceID := uuid.New()
	oldReference := r23MessageReference(primary, uuid.New(), spaceID)
	newReference := r23MessageReference(alias, uuid.New(), spaceID)
	client := r23FileServer(pool)
	acquire := r23AcquireRequest(oldReference)
	_, err := client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)

	release := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{oldReference}}
	reacquire := r23AcquireRequest(newReference)
	releaseCtx := r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseFileReferences_FullMethodName, release)
	reacquireCtx := r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, reacquire)
	barrier, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = barrier.Rollback(context.Background()) })
	var lockedBlob uuid.UUID
	require.NoError(t, barrier.QueryRow(ctx, `SELECT blob_id FROM file_blobs WHERE blob_id = (SELECT blob_id FROM files WHERE id = $1) FOR UPDATE`, primary).Scan(&lockedBlob))
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		_, callErr := client.ReleaseFileReferences(releaseCtx, release)
		errors <- callErr
	}()
	r23WaitForBlockedTransactions(t, ctx, pool, 1)
	wait.Add(1)
	go func() {
		defer wait.Done()
		_, callErr := client.AcquireFileReferences(reacquireCtx, reacquire)
		errors <- callErr
	}()
	r23WaitForBlockedTransactions(t, ctx, pool, 2)
	require.NoError(t, barrier.Commit(ctx), "release is queued first, acquire second, then the blob-row barrier opens")
	wait.Wait()
	close(errors)
	for callErr := range errors {
		require.NoError(t, callErr)
	}
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, alias))
	require.Equal(t, "LIVE", r23BlobGCState(t, ctx, pool, primary), "zero-ref classification and acquire serialize on the blob; a concurrent live alias prevents data loss")

	aliasRelease := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{newReference}}
	_, err = client.ReleaseFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseFileReferences_FullMethodName, aliasRelease), aliasRelease)
	require.NoError(t, err)
	require.Equal(t, "GC_PENDING", r23BlobGCState(t, ctx, pool, alias), "one deduplicated blob becomes collectable only after every file alias loses its final reference")
}

func TestR23FileDeletionManifest_ExactProducersBoundedChunksAndSeal(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	spaceID, deletionID := uuid.New(), uuid.New()
	manifest := r23ManifestBinding("chat-root", 0)

	prepareRequest := &filev1.PrepareSpaceDeletionReferenceManifestRequest{
		ProtocolVersion:     1,
		SpaceId:             spaceID.String(),
		DeletionOperationId: deletionID.String(),
		ScheduleGeneration:  7,
		ChatManifest:        manifest,
	}
	_, err := client.PrepareSpaceDeletionReferenceManifest(r23ServiceContext(t, ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, prepareRequest), prepareRequest)
	require.NoError(t, err)
	earlyFenceRequest := &filev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion:     1,
		SpaceId:             spaceID.String(),
		DeletionOperationId: deletionID.String(),
		Generation:          7,
		DesiredState:        commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:            manifest,
	}}
	_, err = client.ApplySpaceLifecycleFence(r23ServiceContext(t, ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, earlyFenceRequest), earlyFenceRequest)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "final FROZEN receipt requires all three explicit producer seals")

	zero := r23RegisterChunk(deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE, 0, nil, true)
	zeroReceipt, err := client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "space", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, zero), zero)
	require.NoError(t, err)
	require.True(t, zeroReceipt.GetReceipt().GetProducerSealed(), "an explicit zero-count producer is required and sealable")
	replayZero := proto.Clone(zero).(*filev1.RegisterSpaceDeletionReferenceChunkRequest)
	replayedZero, err := client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "space", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, replayZero), replayZero)
	require.NoError(t, err)
	require.True(t, proto.Equal(zeroReceipt, replayedZero), "accepted chunk replay must be byte-identical")
	changedZero := proto.Clone(zero).(*filev1.RegisterSpaceDeletionReferenceChunkRequest)
	changedZero.ExpectedReferencesSha256 = bytes32("changed")
	_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "space", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, changedZero), changedZero)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "changed chunk under the same operation id must conflict")

	wrongProducer := r23RegisterChunk(deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, nil, true)
	_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "chat", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, wrongProducer), wrongProducer)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "caller identity fixes its producer id")

	tooLarge := make([]*filev1.FileReferenceKey, 1001)
	for index := range tooLarge {
		tooLarge[index] = r23MessageReference(uuid.New(), uuid.New(), spaceID)
	}
	tooLarge = sortedR23References(tooLarge)
	tooLargeRequest := r23RegisterChunk(
		deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, tooLarge, false,
	)
	_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "messaging", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, tooLargeRequest), tooLargeRequest)
	require.Equal(t, codes.InvalidArgument, status.Code(err), "the 1001-key bound is tested with correctly sorted unique keys")
	low := r23MessageReference(uuid.MustParse("00000000-0000-0000-0000-000000000030"), uuid.New(), spaceID)
	high := r23MessageReference(uuid.MustParse("00000000-0000-0000-0000-000000000031"), uuid.New(), spaceID)
	unsortedChunk := r23RegisterChunk(deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, []*filev1.FileReferenceKey{high, low}, true)
	_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "messaging", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, unsortedChunk), unsortedChunk)
	require.Equal(t, codes.InvalidArgument, status.Code(err), "canonical sorting is an independent <=1000-key validation")

	mismatchDeletion, mismatchSpace := uuid.New(), uuid.New()
	mismatchPrepare := &filev1.PrepareSpaceDeletionReferenceManifestRequest{ProtocolVersion: 1, SpaceId: mismatchSpace.String(), DeletionOperationId: mismatchDeletion.String(), ScheduleGeneration: 1, ChatManifest: r23ManifestBinding("aggregate-mismatch", 0)}
	_, err = client.PrepareSpaceDeletionReferenceManifest(r23ServiceContext(t, ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, mismatchPrepare), mismatchPrepare)
	require.NoError(t, err)
	mismatchReferences := sortedR23References([]*filev1.FileReferenceKey{
		r23MessageReference(uuid.MustParse("00000000-0000-0000-0000-000000000021"), uuid.New(), mismatchSpace),
		r23MessageReference(uuid.MustParse("00000000-0000-0000-0000-000000000020"), uuid.New(), mismatchSpace),
	})
	aggregateMismatch := r23RegisterChunk(mismatchDeletion, mismatchSpace, 1, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, mismatchReferences, true)
	aggregateMismatch.ExpectedTotalCount = 2
	aggregateMismatch.ExpectedReferencesSha256 = bytes32("syntactically-valid-but-wrong-aggregate")
	_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "messaging", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, aggregateMismatch), aggregateMismatch)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "a correct count and canonical key order cannot mask an aggregate hash mismatch")

	one := r23MessageReference(uuid.New(), uuid.New(), spaceID)
	outOfOrder := r23RegisterChunk(deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 1, []*filev1.FileReferenceKey{one}, false)
	_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "messaging", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, outOfOrder), outOfOrder)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "chunk indexes are contiguous from zero")
	duplicate := r23RegisterChunk(deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, []*filev1.FileReferenceKey{one, proto.Clone(one).(*filev1.FileReferenceKey)}, true)
	_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "messaging", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, duplicate), duplicate)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "duplicate exact reference keys cannot seal")
	early := r23RegisterChunk(deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, nil, true)
	early.ExpectedTotalCount = 1
	early.ExpectedReferencesSha256 = r23ReferencesHash(deletionID, spaceID, 7, early.GetProducerId(), []*filev1.FileReferenceKey{one})
	_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "messaging", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, early), early)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "early seal cannot infer a missing reference")

	chat := r23RegisterChunk(deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT, 0, nil, true)
	chatReceipt, err := client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "chat", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, chat), chat)
	require.NoError(t, err)
	require.True(t, chatReceipt.GetReceipt().GetProducerSealed())
	firstProgressReference := r23MessageReference(uuid.MustParse("00000000-0000-0000-0000-000000000010"), uuid.New(), spaceID)
	secondProgressReference := r23MessageReference(uuid.MustParse("00000000-0000-0000-0000-000000000011"), uuid.New(), spaceID)
	progressReferences := []*filev1.FileReferenceKey{firstProgressReference, secondProgressReference}
	messaging := r23RegisterChunk(deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, []*filev1.FileReferenceKey{firstProgressReference}, false)
	messaging.ExpectedTotalCount = 2
	messaging.ExpectedReferencesSha256 = r23ReferencesHash(deletionID, spaceID, 7, messaging.GetProducerId(), progressReferences)
	progressReceipt, err := client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "messaging", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, messaging), messaging)
	require.NoError(t, err)
	require.Equal(t, uint64(0), progressReceipt.GetReceipt().GetChunkIndex())
	require.Equal(t, uint64(1), progressReceipt.GetReceipt().GetAcceptedCount())
	require.False(t, progressReceipt.GetReceipt().GetProducerSealed())
	seal := r23RegisterChunk(deletionID, spaceID, 7, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 1, []*filev1.FileReferenceKey{secondProgressReference}, true)
	seal.ExpectedTotalCount = 2
	seal.ExpectedReferencesSha256 = messaging.GetExpectedReferencesSha256()
	messagingReceipt, err := client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "messaging", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, seal), seal)
	require.NoError(t, err)
	require.True(t, messagingReceipt.GetReceipt().GetProducerSealed())
	require.Equal(t, uint64(1), messagingReceipt.GetReceipt().GetChunkIndex())
	require.Equal(t, uint64(1), messagingReceipt.GetReceipt().GetAcceptedCount())

	declarations := []*filev1.FileReferenceProducerDeclaration{
		{ProducerId: zero.GetProducerId(), ExpectedTotalCount: zero.GetExpectedTotalCount(), ExpectedReferencesSha256: zero.GetExpectedReferencesSha256()},
		{ProducerId: chat.GetProducerId(), ExpectedTotalCount: chat.GetExpectedTotalCount(), ExpectedReferencesSha256: chat.GetExpectedReferencesSha256()},
		{ProducerId: seal.GetProducerId(), ExpectedTotalCount: seal.GetExpectedTotalCount(), ExpectedReferencesSha256: seal.GetExpectedReferencesSha256()},
	}
	localRoot := r23SpaceDeletionManifestSet(deletionID, spaceID, 7, manifest, declarations)
	callerEcho := &filev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), Generation: 7,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest,
	}}
	_, err = client.ApplySpaceLifecycleFence(r23ServiceContext(t, ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, callerEcho), callerEcho)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "File computes the root and item_count from its stored manifest set; it never echoes Chat's caller binding")
	require.Equal(t, manifest.GetItemCount()+2, localRoot.GetItemCount(), "the root count is computed from Chat plus all exact producer declarations")

	finalFenceRequest := &filev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion:     1,
		SpaceId:             spaceID.String(),
		DeletionOperationId: deletionID.String(),
		Generation:          7,
		DesiredState:        commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:            localRoot,
	}}
	frozen, err := client.ApplySpaceLifecycleFence(r23ServiceContext(t, ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, finalFenceRequest), finalFenceRequest)
	require.NoError(t, err)
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_FILE, frozen.GetReceipt().GetParticipantId())
	require.Equal(t, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, frozen.GetReceipt().GetAppliedState())
}

func TestR23FileCapability_IsSubjectSurfaceExpiryBoundAndFreezeWins(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	fileID := insertR23ReadyFile(t, ctx, pool, "capability")
	spaceID := uuid.New()
	reference := r23MessageReference(fileID, uuid.New(), spaceID)
	acquire := &filev1.AcquireFileReferencesRequest{
		ProtocolVersion: 1,
		OperationId:     uuid.NewString(),
		ProducerId:      filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING,
		References:      []*filev1.FileReferenceKey{reference},
	}
	client := r23FileServer(pool)
	_, err := client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)

	subject := uuid.New()
	unsortedRequest := &filev1.IssueFileAccessCapabilityRequest{
		ProtocolVersion:  1,
		OperationId:      uuid.NewString(),
		Reference:        reference,
		SubjectProfileId: subject.String(),
		AllowedSurfaces: []filev1.FileReadSurface{
			filev1.FileReadSurface_FILE_READ_SURFACE_METADATA,
			filev1.FileReadSurface_FILE_READ_SURFACE_URL,
		},
		ExpiresAt: timestamppb.New(time.Now().UTC().Add(30 * time.Minute)),
	}
	_, err = client.IssueFileAccessCapability(r23ServiceContext(t, ctx, "messaging", filev1.FileService_IssueFileAccessCapability_FullMethodName, unsortedRequest), unsortedRequest)
	require.Equal(t, codes.InvalidArgument, status.Code(err), "capability surfaces must be strictly sorted and unique")
	tooLongRequest := &filev1.IssueFileAccessCapabilityRequest{
		ProtocolVersion:  1,
		OperationId:      uuid.NewString(),
		Reference:        reference,
		SubjectProfileId: subject.String(),
		AllowedSurfaces:  []filev1.FileReadSurface{filev1.FileReadSurface_FILE_READ_SURFACE_URL},
		ExpiresAt:        timestamppb.New(time.Now().UTC().Add(time.Hour + time.Minute)),
	}
	_, err = client.IssueFileAccessCapability(r23ServiceContext(t, ctx, "messaging", filev1.FileService_IssueFileAccessCapability_FullMethodName, tooLongRequest), tooLongRequest)
	require.Equal(t, codes.InvalidArgument, status.Code(err), "capability expiry cannot exceed one hour of database time")

	capabilityRequest := &filev1.IssueFileAccessCapabilityRequest{
		ProtocolVersion:  1,
		OperationId:      uuid.NewString(),
		Reference:        reference,
		SubjectProfileId: subject.String(),
		AllowedSurfaces: []filev1.FileReadSurface{
			filev1.FileReadSurface_FILE_READ_SURFACE_URL,
			filev1.FileReadSurface_FILE_READ_SURFACE_METADATA,
		},
		ExpiresAt: timestamppb.New(time.Now().UTC().Add(30 * time.Minute)),
	}
	capability, err := client.IssueFileAccessCapability(r23ServiceContext(t, ctx, "messaging", filev1.FileService_IssueFileAccessCapability_FullMethodName, capabilityRequest), capabilityRequest)
	require.NoError(t, err)
	require.NotEmpty(t, capability.GetReceipt().GetCapabilityId())
	replayRequest := proto.Clone(capabilityRequest).(*filev1.IssueFileAccessCapabilityRequest)
	replayed, err := client.IssueFileAccessCapability(r23ServiceContext(t, ctx, "messaging", filev1.FileService_IssueFileAccessCapability_FullMethodName, replayRequest), replayRequest)
	require.NoError(t, err)
	require.True(t, proto.Equal(capability, replayed), "capability issue replay is byte-identical")
	restarted := r23FileServer(pool)
	restartRequest := proto.Clone(capabilityRequest).(*filev1.IssueFileAccessCapabilityRequest)
	recovered, err := restarted.IssueFileAccessCapability(r23ServiceContext(t, ctx, "messaging", filev1.FileService_IssueFileAccessCapability_FullMethodName, restartRequest), restartRequest)
	require.NoError(t, err)
	require.True(t, proto.Equal(capability, recovered), "capability receipt and exact scope survive restart")
	changed := proto.Clone(capabilityRequest).(*filev1.IssueFileAccessCapabilityRequest)
	changed.SubjectProfileId = uuid.NewString()
	_, err = restarted.IssueFileAccessCapability(r23ServiceContext(t, ctx, "messaging", filev1.FileService_IssueFileAccessCapability_FullMethodName, changed), changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "same operation with changed capability body conflicts")

	storyReference := &filev1.FileReferenceKey{
		FileId: fileID.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STORY, OwnerId: uuid.NewString(),
	}
	insertR23Reference(t, ctx, pool, storyReference)
	story := proto.Clone(capabilityRequest).(*filev1.IssueFileAccessCapabilityRequest)
	story.OperationId = uuid.NewString()
	story.Reference = storyReference
	_, err = client.IssueFileAccessCapability(r23ServiceContext(t, ctx, "messaging", filev1.FileService_IssueFileAccessCapability_FullMethodName, story), story)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "caller allow-list prevents Messaging from minting authority for an existing live Story reference")

	selector := &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_CapabilityId{CapabilityId: capability.GetReceipt().GetCapabilityId()}}
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: fileID.String(), Access: selector})
	require.NoError(t, err)
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), uuid.New()), &filev1.GetFileURLRequest{FileId: fileID.String(), Access: selector})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "capability is bound to the authenticated subject")
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: uuid.NewString(), Access: selector})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "capability use must match the exact reference and file")
	metadataOnly := issueR23MetadataCapability(t, ctx, client, subject, reference)
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: fileID.String(), Access: r23CapabilitySelector(metadataOnly)})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "a metadata capability cannot be replayed on the URL surface")
	freezeURL := issueR23Capability(t, ctx, client, "messaging", subject, reference, filev1.FileReadSurface_FILE_READ_SURFACE_URL)
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: fileID.String(), Access: r23CapabilitySelector(freezeURL)})
	require.NoError(t, err, "the URL capability is valid and unexpired immediately before the freeze")

	_, err = pool.Exec(ctx, `UPDATE file_access_capabilities SET expires_at = clock_timestamp() WHERE capability_id = $1`, capability.GetReceipt().GetCapabilityId())
	require.NoError(t, err)
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: fileID.String(), Access: selector})
	require.Equal(t, codes.PermissionDenied, status.Code(err), "DB time equal to expiry is expired and every use rechecks expiry")

	prepareR23SealedManifest(t, ctx, client, uuid.New(), spaceID, 1, []*filev1.FileReferenceKey{reference})
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: fileID.String(), Access: r23CapabilitySelector(freezeURL)})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "the same still-unexpired URL capability loses authority at the durable freeze")
}

func TestR23FileCapability_PreliminaryFreezeLinearizesAgainstIssue(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	fileID, spaceID, deletionID, subject := insertR23ReadyFile(t, ctx, pool, "capability-freeze-race"), uuid.New(), uuid.New(), uuid.New()
	reference := r23MessageReference(fileID, uuid.New(), spaceID)
	acquire := r23AcquireRequest(reference)
	_, err := client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)
	issue := &filev1.IssueFileAccessCapabilityRequest{
		ProtocolVersion: 1, OperationId: uuid.NewString(), Reference: reference, SubjectProfileId: subject.String(),
		AllowedSurfaces: []filev1.FileReadSurface{filev1.FileReadSurface_FILE_READ_SURFACE_URL}, ExpiresAt: timestamppb.New(time.Now().UTC().Add(30 * time.Minute)),
	}
	prepare := &filev1.PrepareSpaceDeletionReferenceManifestRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), ScheduleGeneration: 1,
		ChatManifest: r23ManifestBinding("preliminary-freeze-race", 0),
	}
	issueCtx := r23ServiceContext(t, ctx, "messaging", filev1.FileService_IssueFileAccessCapability_FullMethodName, issue)
	prepareCtx := r23ServiceContext(t, ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, prepare)
	start := make(chan struct{})
	type issueResult struct {
		response *filev1.IssueFileAccessCapabilityResponse
		err      error
	}
	issueDone := make(chan issueResult, 1)
	prepareDone := make(chan error, 1)
	go func() {
		<-start
		response, callErr := client.IssueFileAccessCapability(issueCtx, issue)
		issueDone <- issueResult{response: response, err: callErr}
	}()
	go func() {
		<-start
		_, callErr := client.PrepareSpaceDeletionReferenceManifest(prepareCtx, prepare)
		prepareDone <- callErr
	}()
	close(start)
	require.NoError(t, <-prepareDone)
	result := <-issueDone
	if result.err != nil {
		require.Equal(t, codes.FailedPrecondition, status.Code(result.err), "preliminary FROZEN may linearize first and reject issuance")
		return
	}
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{
		FileId: fileID.String(), Access: r23CapabilitySelector(result.response.GetReceipt().GetCapabilityId()),
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "if issue linearizes first, use after preliminary FROZEN is still denied")
}

func TestR23FileReadAccess_LegacyAmbiguityAndBulkAreFailClosed(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	subject := uuid.New()

	ambiguousFile := insertR23ReadyFile(t, ctx, pool, "ambiguous")
	ambiguousSpaceReference := r23MessageReference(ambiguousFile, uuid.New(), uuid.New())
	storyReference := &filev1.FileReferenceKey{FileId: ambiguousFile.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STORY, OwnerId: uuid.NewString()}
	profileReference := &filev1.FileReferenceKey{FileId: ambiguousFile.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_PROFILE_AVATAR, OwnerId: uuid.NewString()}
	otherSpaceReference := r23MessageReference(ambiguousFile, uuid.New(), uuid.New())
	insertR23Reference(t, ctx, pool, ambiguousSpaceReference)
	insertR23Reference(t, ctx, pool, storyReference)
	insertR23Reference(t, ctx, pool, profileReference)
	otherSpaceAcquire := r23AcquireRequest(otherSpaceReference)
	_, err := client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, otherSpaceAcquire), otherSpaceAcquire)
	require.NoError(t, err, "Acquire creates or confirms the other Space's canonical LIVE lifecycle fence")
	require.Equal(t, "LIVE", r23SpaceFenceState(t, ctx, pool, uuid.MustParse(otherSpaceReference.GetScopeSpaceId())), "missing fence is fail-closed; the positive selector requires observed canonical LIVE state")
	storyCapability := issueR23Capability(t, ctx, client, "user", subject, storyReference, filev1.FileReadSurface_FILE_READ_SURFACE_URL)
	profileCapability := issueR23Capability(t, ctx, client, "user", subject, profileReference, filev1.FileReadSurface_FILE_READ_SURFACE_URL)
	otherSpaceCapability := issueR23Capability(t, ctx, client, "messaging", subject, otherSpaceReference, filev1.FileReadSurface_FILE_READ_SURFACE_URL)
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: ambiguousFile.String()})
	require.Error(t, err, "legacy file_id authority must fail closed when multiple live references exist")
	prepareR23SealedManifest(t, ctx, client, uuid.New(), uuid.MustParse(ambiguousSpaceReference.GetScopeSpaceId()), 1, []*filev1.FileReferenceKey{ambiguousSpaceReference})
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: ambiguousFile.String()})
	require.Error(t, err, "legacy authority fails closed for a true mixed frozen Story/profile/other-live-Space reference set")
	for name, capabilityID := range map[string]string{"Story": storyCapability, "profile": profileCapability, "other live Space": otherSpaceCapability} {
		t.Run(name+" selector survives unrelated freeze", func(t *testing.T) {
			_, readErr := client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: ambiguousFile.String(), Access: r23CapabilitySelector(capabilityID)})
			require.NoError(t, readErr, "an exact surviving reference remains authorized")
		})
	}

	liveFile := insertR23ReadyFile(t, ctx, pool, "bulk-live")
	frozenFile := insertR23ReadyFile(t, ctx, pool, "bulk-frozen")
	liveReference := r23MessageReference(liveFile, uuid.New(), uuid.New())
	frozenSpaceID := uuid.New()
	frozenReference := r23MessageReference(frozenFile, uuid.New(), frozenSpaceID)
	liveAcquire := r23AcquireRequest(liveReference)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, liveAcquire), liveAcquire)
	require.NoError(t, err)
	frozenAcquire := r23AcquireRequest(frozenReference)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, frozenAcquire), frozenAcquire)
	require.NoError(t, err)
	require.Equal(t, "LIVE", r23SpaceFenceState(t, ctx, pool, uuid.MustParse(liveReference.GetScopeSpaceId())))
	require.Equal(t, "LIVE", r23SpaceFenceState(t, ctx, pool, frozenSpaceID))
	liveCapability := issueR23MetadataCapability(t, ctx, client, subject, liveReference)
	frozenCapability := issueR23MetadataCapability(t, ctx, client, subject, frozenReference)
	_, err = client.GetFileMetadata(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileMetadataRequest{FileId: liveFile.String(), Access: r23CapabilitySelector(liveCapability)})
	require.NoError(t, err, "the allowed bulk sibling is positively readable while both canonical fences are LIVE")
	bulkRequest := &filev1.GetBulkMetadataRequest{Items: []*filev1.FileAccessItem{
		{FileId: liveFile.String(), Access: r23CapabilitySelector(liveCapability)},
		{FileId: frozenFile.String(), Access: r23CapabilitySelector(frozenCapability)},
	}}
	liveBulk, err := client.GetBulkMetadata(r23UserContext(ctx, uuid.New(), subject), bulkRequest)
	require.NoError(t, err, "the exact two-item selector request succeeds while both fences are LIVE")
	require.NotNil(t, liveBulk)
	require.Len(t, liveBulk.GetBulkFileMetadata().GetByFileId(), 2)
	var liveBulkIDs []string
	for fileID := range liveBulk.GetBulkFileMetadata().GetByFileId() {
		liveBulkIDs = append(liveBulkIDs, fileID)
	}
	require.ElementsMatch(t, []string{liveFile.String(), frozenFile.String()}, liveBulkIDs, "the positive bulk response contains exactly the two requested file IDs")
	prepareR23SealedManifest(t, ctx, client, uuid.New(), frozenSpaceID, 1, []*filev1.FileReferenceKey{frozenReference})

	replayBulkRequest := proto.Clone(bulkRequest).(*filev1.GetBulkMetadataRequest)
	require.True(t, proto.Equal(bulkRequest, replayBulkRequest))
	bulk, err := client.GetBulkMetadata(r23UserContext(ctx, uuid.New(), subject), replayBulkRequest)
	require.Error(t, err, "one denied item must deny the entire bulk request")
	require.Nil(t, bulk, "exact request replay after the counterpart freezes must not leak metadata for the still-allowed sibling")
}

func TestR23FileRestore_ReusesSavedManifestAndReleasesNothing(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	fileID := insertR23ReadyFile(t, ctx, pool, "restore-live")
	spaceID, deletionID := uuid.New(), uuid.New()
	reference := r23MessageReference(fileID, uuid.New(), spaceID)
	insertR23Reference(t, ctx, pool, reference)
	client := r23FileServer(pool)
	rootManifest := prepareR23SealedManifest(t, ctx, client, deletionID, spaceID, 1, []*filev1.FileReferenceKey{reference})

	restoreRequest := &filev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion:     1,
		SpaceId:             spaceID.String(),
		DeletionOperationId: deletionID.String(),
		Generation:          2,
		DesiredState:        commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE,
		Manifest:            rootManifest,
	}}
	restored, err := client.ApplySpaceLifecycleFence(r23ServiceContext(t, ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, restoreRequest), restoreRequest)
	require.NoError(t, err)
	require.Equal(t, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, restored.GetReceipt().GetAppliedState())
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, fileID), "restore reuses the saved manifest and releases no reference")
	require.Equal(t, "LIVE", r23BlobGCState(t, ctx, pool, fileID))
}

func TestR23FilePurge_ExactReleaseAndGCRowClassification(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	spaceID := uuid.New()
	exclusiveFile := insertR23ReadyFile(t, ctx, pool, "exclusive-gc")
	sharedFile := insertR23ReadyFile(t, ctx, pool, "shared-live")
	exclusiveReference := r23MessageReference(exclusiveFile, uuid.New(), spaceID)
	sharedSpaceReference := r23MessageReference(sharedFile, uuid.New(), spaceID)

	insertR23Reference(t, ctx, pool, exclusiveReference)
	insertR23Reference(t, ctx, pool, sharedSpaceReference)
	insertR23Reference(t, ctx, pool, &filev1.FileReferenceKey{
		FileId:    sharedFile.String(),
		OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STORY,
		OwnerId:   uuid.NewString(),
	})

	deletionID := uuid.New()
	client := r23FileServer(pool)
	rootManifest := prepareR23SealedManifest(t, ctx, client, deletionID, spaceID, 4, []*filev1.FileReferenceKey{
		exclusiveReference,
		sharedSpaceReference,
	})
	applyR23PurgeDecision(t, ctx, client, deletionID, spaceID, 5, rootManifest)
	request := &filev1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{
		ProtocolVersion:     1,
		SpaceId:             spaceID.String(),
		DeletionOperationId: deletionID.String(),
		Generation:          5,
		PurgeDecidedAt:      timestamppb.New(time.Now().UTC()),
		ParticipantId:       commonv1.ParticipantId_PARTICIPANT_ID_FILE,
		Manifest:            rootManifest,
	}}
	_, err := client.PurgeSpace(r23ServiceContext(t, ctx, "space", filev1.FileService_PurgeSpace_FullMethodName, request), request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "File cannot emit its purge receipt before every sealed producer is released")
	r23ReleaseProducerReference(t, ctx, client, deletionID, spaceID, 4, 5, "space", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE, nil)
	_, err = client.PurgeSpace(r23ServiceContext(t, ctx, "space", filev1.FileService_PurgeSpace_FullMethodName, request), request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "SPACE alone cannot satisfy the exact producer release barrier")
	r23ReleaseProducerReference(t, ctx, client, deletionID, spaceID, 4, 5, "chat", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT, nil)
	_, err = client.PurgeSpace(r23ServiceContext(t, ctx, "space", filev1.FileService_PurgeSpace_FullMethodName, request), request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "SPACE+CHAT cannot satisfy the exact producer release barrier")
	r23ReleaseProducerReference(t, ctx, client, deletionID, spaceID, 4, 5, "messaging", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, []*filev1.FileReferenceKey{exclusiveReference, sharedSpaceReference})
	receipt, err := client.PurgeSpace(r23ServiceContext(t, ctx, "space", filev1.FileService_PurgeSpace_FullMethodName, request), request)
	require.NoError(t, err)
	require.Equal(t, commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, receipt.GetReceipt().GetState())
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_FILE, receipt.GetReceipt().GetParticipantId())

	require.Equal(t, "GC_PENDING", r23BlobGCState(t, ctx, pool, exclusiveFile), "zero global references must hand the blob durably to GC")
	require.Equal(t, "LIVE", r23BlobGCState(t, ctx, pool, sharedFile), "a Story/profile/other-Space reference must retain shared bytes")
	require.Equal(t, int64(0), r23LiveReferenceCount(t, ctx, pool, exclusiveFile))
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, sharedFile))
	require.Equal(t, "PURGED", r23SpaceFenceState(t, ctx, pool, spaceID), "permanent compact fence prevents identifier reuse")
	postPurgeAcquire := r23AcquireRequest(r23MessageReference(exclusiveFile, uuid.New(), spaceID))
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, postPurgeAcquire), postPurgeAcquire)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "an existing file cannot gain a replacement reference in the PURGED Space")
	reacquireReleased := r23AcquireRequest(exclusiveReference)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, reacquireReleased), reacquireReleased)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "the exact released reference cannot be resurrected after PURGED")
	postPurgeCapability := &filev1.IssueFileAccessCapabilityRequest{
		ProtocolVersion: 1, OperationId: uuid.NewString(), Reference: exclusiveReference,
		SubjectProfileId: uuid.NewString(), AllowedSurfaces: []filev1.FileReadSurface{filev1.FileReadSurface_FILE_READ_SURFACE_URL},
		ExpiresAt: timestamppb.New(time.Now().UTC().Add(time.Minute)),
	}
	_, err = client.IssueFileAccessCapability(r23ServiceContext(t, ctx, "messaging", filev1.FileService_IssueFileAccessCapability_FullMethodName, postPurgeCapability), postPurgeCapability)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "capability issuance cannot revive PURGED authority")
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), uuid.New()), &filev1.GetFileURLRequest{FileId: exclusiveFile.String()})
	require.Error(t, err, "reads cannot derive authority from released references after PURGED")

	restarted := r23FileServer(pool)
	lookupRequest := &filev1.GetSpacePurgeReceiptRequest{
		ProtocolVersion:     1,
		SpaceId:             spaceID.String(),
		DeletionOperationId: deletionID.String(),
		Generation:          5,
		PurgeRequestSha256:  r23RequestHash(t, request),
		ManifestSha256:      request.GetPurge().GetManifest().GetManifestSha256(),
	}
	recovered, err := restarted.GetSpacePurgeReceipt(r23ServiceContext(t, ctx, "space", filev1.FileService_GetSpacePurgeReceipt_FullMethodName, lookupRequest), lookupRequest)
	require.NoError(t, err)
	require.True(t, proto.Equal(receipt.GetReceipt(), recovered.GetReceipt()), "response-loss lookup after restart must return the durable purge receipt")

	changed := proto.Clone(request).(*filev1.PurgeSpaceRequest)
	changed.Purge.Manifest.ManifestSha256 = bytes32("changed-root")
	_, err = client.PurgeSpace(r23ServiceContext(t, ctx, "space", filev1.FileService_PurgeSpace_FullMethodName, changed), changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "same purge generation with changed body cannot release a replacement set")
}

func TestR23FileReferenceGC_RetryAmbiguousDeletePartialDerivativesAndCrashRecovery(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	files := store.NewFilesStore(pool)
	runner, ok := any(files).(interface {
		RunReferenceGCOnce(context.Context, r2file.ObjectDeleter, int) (int64, error)
	})
	require.True(t, ok, "FilesStore must expose the bounded durable reference-GC worker")

	fileID := insertR23ReadyFile(t, ctx, pool, "gc-partial")
	_, err := pool.Exec(ctx, `UPDATE file_blobs SET converted_r2_key = 'r23/gc-partial-converted', thumbnail_r2_key = 'r23/gc-partial-thumb' WHERE blob_id = (SELECT blob_id FROM files WHERE id = $1)`, fileID)
	require.NoError(t, err)
	reference := r23MessageReference(fileID, uuid.New(), uuid.New())
	client := r23FileServer(pool)
	acquire := r23AcquireRequest(reference)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)
	release := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{reference}}
	_, err = client.ReleaseFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseFileReferences_FullMethodName, release), release)
	require.NoError(t, err)

	partial := &r23ScriptedDeleter{failures: map[string]int{"r23/gc-partial-converted": 1}}
	processed, err := runner.RunReferenceGCOnce(ctx, partial, 10)
	require.Error(t, err, "timeout/5xx or ambiguous failure leaves durable retry work")
	require.Zero(t, processed)
	require.Equal(t, "GC_PENDING", r23BlobGCState(t, ctx, pool, fileID), "partial derivative deletion cannot complete the blob")
	var operationBefore uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT gc_operation_id FROM file_blobs WHERE blob_id = (SELECT blob_id FROM files WHERE id = $1)`, fileID).Scan(&operationBefore))
	processed, err = runner.RunReferenceGCOnce(ctx, partial, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), processed)
	require.Equal(t, "GC_COMPLETE", r23BlobGCState(t, ctx, pool, fileID), "retry treats an already-absent object as success and finishes all derivatives")
	var operationAfter uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT gc_operation_id FROM file_blobs WHERE blob_id = (SELECT blob_id FROM files WHERE id = $1)`, fileID).Scan(&operationAfter))
	require.Equal(t, operationBefore, operationAfter, "ambiguous retry resumes the same gc_operation_id")
	require.Contains(t, partial.calls, "r23/gc-partial")
	require.Contains(t, partial.calls, "r23/gc-partial-converted")
	require.Contains(t, partial.calls, "r23/gc-partial-thumb")

	crashFile := insertR23ReadyFile(t, ctx, pool, "gc-crash")
	crashReference := r23MessageReference(crashFile, uuid.New(), uuid.New())
	crashAcquire := r23AcquireRequest(crashReference)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, crashAcquire), crashAcquire)
	require.NoError(t, err)
	crashRelease := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{crashReference}}
	_, err = client.ReleaseFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseFileReferences_FullMethodName, crashRelease), crashRelease)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE OR REPLACE FUNCTION r23_fail_gc_complete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state = 'GC_COMPLETE' THEN RAISE EXCEPTION 'simulated commit crash'; END IF; RETURN NEW; END $$; CREATE TRIGGER r23_gc_commit_crash BEFORE UPDATE ON file_blobs FOR EACH ROW EXECUTE FUNCTION r23_fail_gc_complete()`)
	require.NoError(t, err)
	crashDeleter := &r23ScriptedDeleter{}
	_, err = runner.RunReferenceGCOnce(ctx, crashDeleter, 10)
	require.Error(t, err, "R2 success before metadata commit must remain recoverable")
	var crashOperation uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT gc_operation_id FROM file_blobs WHERE blob_id = (SELECT blob_id FROM files WHERE id = $1)`, crashFile).Scan(&crashOperation))
	_, err = pool.Exec(ctx, `DROP TRIGGER r23_gc_commit_crash ON file_blobs; DROP FUNCTION r23_fail_gc_complete()`)
	require.NoError(t, err)
	_, err = runner.RunReferenceGCOnce(ctx, crashDeleter, 10)
	require.NoError(t, err)
	var recoveredOperation uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT gc_operation_id FROM file_blobs WHERE blob_id = (SELECT blob_id FROM files WHERE id = $1)`, crashFile).Scan(&recoveredOperation))
	require.Equal(t, crashOperation, recoveredOperation, "crash recovery resumes the same gc_operation_id and accepts R2 NotFound")
	require.Equal(t, "GC_COMPLETE", r23BlobGCState(t, ctx, pool, crashFile))
}

type r23ScriptedDeleter struct {
	mu       sync.Mutex
	failures map[string]int
	calls    []string
}

func (d *r23ScriptedDeleter) DeleteObject(_ context.Context, key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, key)
	if d.failures != nil && d.failures[key] > 0 {
		d.failures[key]--
		return fmt.Errorf("ambiguous R2 delete for %s", key)
	}
	return nil // Includes R2 NotFound: absence is the desired idempotent state.
}

func r23ReleaseProducerReference(t *testing.T, ctx context.Context, client *FileGRPC, deletionID, spaceID uuid.UUID, sourceGeneration, purgeGeneration uint64, caller string, producerID filev1.FileReferenceProducerId, references []*filev1.FileReferenceKey) *filev1.ReleaseSpaceDeletionProducerReferencesResponse {
	t.Helper()
	request := &filev1.ReleaseSpaceDeletionProducerReferencesRequest{
		ProtocolVersion: 1, DeletionOperationId: deletionID.String(), SpaceId: spaceID.String(),
		PurgeGeneration: purgeGeneration, SourceScheduleGeneration: sourceGeneration, ProducerId: producerID,
		ExpectedReferencesSha256: r23ReferencesHash(deletionID, spaceID, sourceGeneration, producerID, references),
	}
	response, err := client.ReleaseSpaceDeletionProducerReferences(r23ServiceContext(t, ctx, caller, filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, request), request)
	require.NoError(t, err)
	require.Equal(t, uint64(len(references)), response.GetReceipt().GetReleasedCount())
	require.Equal(t, request.GetExpectedReferencesSha256(), response.GetReceipt().GetExpectedReferencesSha256())
	replay := proto.Clone(request).(*filev1.ReleaseSpaceDeletionProducerReferencesRequest)
	replayed, replayErr := client.ReleaseSpaceDeletionProducerReferences(r23ServiceContext(t, ctx, caller, filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, replay), replay)
	require.NoError(t, replayErr)
	require.True(t, proto.Equal(response, replayed), "ambiguous release outcome is recovered by exact idempotent replay")
	return response
}

func applyR23PurgeDecision(t *testing.T, ctx context.Context, client *FileGRPC, deletionID, spaceID uuid.UUID, generation uint64, manifest *commonv1.ManifestBinding) *filev1.ApplySpaceLifecycleFenceResponse {
	t.Helper()
	request := &filev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), Generation: generation,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, Manifest: manifest,
	}}
	response, err := client.ApplySpaceLifecycleFence(r23ServiceContext(t, ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, request), request)
	require.NoError(t, err)
	return response
}

func TestR23FileLifecycleMigration_DOWNRefusesDurableEvidenceAndGC(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	fileID, spaceID, deletionID := insertR23ReadyFile(t, ctx, pool, "down-refusal"), uuid.New(), uuid.New()
	reference := r23MessageReference(fileID, uuid.New(), spaceID)
	acquire := r23AcquireRequest(reference)
	_, err := client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)
	require.Equal(t, "LIVE", r23SpaceFenceState(t, ctx, pool, spaceID), "guarded-DOWN Space evidence begins from a canonical LIVE fence")
	root := prepareR23SealedManifest(t, ctx, client, deletionID, spaceID, 1, []*filev1.FileReferenceKey{reference})
	applyR23PurgeDecision(t, ctx, client, deletionID, spaceID, 2, root)
	r23ReleaseProducerReference(t, ctx, client, deletionID, spaceID, 1, 2, "space", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE, nil)
	r23ReleaseProducerReference(t, ctx, client, deletionID, spaceID, 1, 2, "chat", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT, nil)
	r23ReleaseProducerReference(t, ctx, client, deletionID, spaceID, 1, 2, "messaging", filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, []*filev1.FileReferenceKey{reference})
	purge := &filev1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), Generation: 2,
		PurgeDecidedAt: timestamppb.New(time.Now().UTC()), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_FILE, Manifest: root,
	}}
	_, err = client.PurgeSpace(r23ServiceContext(t, ctx, "space", filev1.FileService_PurgeSpace_FullMethodName, purge), purge)
	require.NoError(t, err)

	capabilityFile, capabilitySpace := insertR23ReadyFile(t, ctx, pool, "down-capability"), uuid.New()
	capabilityReference := r23MessageReference(capabilityFile, uuid.New(), capabilitySpace)
	capabilityAcquire := r23AcquireRequest(capabilityReference)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, capabilityAcquire), capabilityAcquire)
	require.NoError(t, err)
	require.Equal(t, "LIVE", r23SpaceFenceState(t, ctx, pool, capabilitySpace), "capability DOWN evidence cannot rely on an absent fence")
	_ = issueR23Capability(t, ctx, client, "messaging", uuid.New(), capabilityReference, filev1.FileReadSurface_FILE_READ_SURFACE_URL)
	for _, table := range []string{
		"file_references", "file_reference_operations", "file_access_capabilities", "file_space_lifecycle_fences",
		"file_space_deletion_manifests", "file_space_deletion_manifest_chunks", "file_space_deletion_producer_releases", "file_space_purge_receipts",
	} {
		var evidenceCount int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&evidenceCount))
		require.Positive(t, evidenceCount, "%s must contain durable evidence before guarded DOWN", table)
	}
	require.Equal(t, "GC_PENDING", r23BlobGCState(t, ctx, pool, fileID))

	downPath := filepath.Join(fileGateRepoRoot(t), "src", "backend", "migrations", "file_db", "000004_file_reference_lifecycle.down.sql")
	downSQL, err := os.ReadFile(downPath)
	require.NoError(t, err)
	guardSQL := strings.ToLower(string(downSQL))
	for _, evidence := range []string{
		"file_references", "file_reference_operations", "file_access_capabilities", "file_space_lifecycle_fences",
		"file_space_deletion_manifests", "file_space_deletion_manifest_chunks", "file_space_deletion_producer_releases", "file_space_purge_receipts", "gc_pending", "gc_complete",
	} {
		require.Contains(t, guardSQL, evidence, "DOWN guard must explicitly preserve every class of durable evidence and GC state")
	}
	_, err = pool.Exec(ctx, string(downSQL))
	require.Error(t, err, "DOWN must refuse while durable references, receipts, fences, manifests or GC work exist")

	var retained int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_references WHERE file_id = $1`, fileID).Scan(&retained))
	require.Equal(t, int64(1), retained, "refusal must leave the released reference evidence intact")
}

func TestR23FileLifecycleMigration_EmptyDOWNSucceedsAndSchemaFingerprintIsExact(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	wantTables := []string{
		"file_access_capabilities", "file_blobs", "file_reference_operations", "file_references",
		"file_space_deletion_manifest_chunks", "file_space_deletion_manifests", "file_space_deletion_producer_releases",
		"file_space_lifecycle_fences", "file_space_purge_receipts",
	}
	tableRows, err := pool.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name LIKE 'file_%' AND table_name NOT IN ('files') ORDER BY table_name`)
	require.NoError(t, err)
	var gotTables []string
	for tableRows.Next() {
		var table string
		require.NoError(t, tableRows.Scan(&table))
		gotTables = append(gotTables, table)
	}
	tableRows.Close()
	require.NoError(t, tableRows.Err())
	require.Equal(t, wantTables, gotTables, "schema fingerprint includes chunk, exact producer-release, purge-receipt and GC ownership tables")

	wantColumns := []string{
		"file_access_capabilities.allowed_surfaces", "file_access_capabilities.capability_id", "file_access_capabilities.created_at", "file_access_capabilities.expires_at", "file_access_capabilities.file_id", "file_access_capabilities.issue_operation_id", "file_access_capabilities.owner_id", "file_access_capabilities.owner_type", "file_access_capabilities.request_bytes", "file_access_capabilities.request_sha256", "file_access_capabilities.scope_space_id", "file_access_capabilities.subject_profile_id", "file_access_capabilities.subresource_id",
		"file_blobs.blob_id", "file_blobs.converted_r2_key", "file_blobs.deleted_at", "file_blobs.gc_attempt", "file_blobs.gc_operation_id", "file_blobs.next_attempt_at", "file_blobs.original_r2_key", "file_blobs.sha256", "file_blobs.state", "file_blobs.thumbnail_r2_key",
		"file_reference_operations.caller_service", "file_reference_operations.completed_at", "file_reference_operations.created_at", "file_reference_operations.operation_id", "file_reference_operations.receipt_bytes", "file_reference_operations.receipt_sha256", "file_reference_operations.request_bytes", "file_reference_operations.request_sha256", "file_reference_operations.state",
		"file_references.created_at", "file_references.file_id", "file_references.owner_id", "file_references.owner_type", "file_references.release_operation_id", "file_references.released_at", "file_references.scope_space_id", "file_references.subresource_id",
		"file_space_deletion_manifests.deletion_operation_id", "file_space_deletion_manifests.expected_references_sha256", "file_space_deletion_manifests.expected_total_count", "file_space_deletion_manifests.producer_id", "file_space_deletion_manifests.received_chunk_count", "file_space_deletion_manifests.received_reference_count", "file_space_deletion_manifests.schedule_generation", "file_space_deletion_manifests.sealed_at", "file_space_deletion_manifests.space_id",
		"file_space_lifecycle_fences.deletion_operation_id", "file_space_lifecycle_fences.final_manifest_hash", "file_space_lifecycle_fences.generation", "file_space_lifecycle_fences.preliminary_chat_hash", "file_space_lifecycle_fences.space_id", "file_space_lifecycle_fences.state", "file_space_lifecycle_fences.updated_at",
	}
	rows, err := pool.Query(ctx, `SELECT table_name || '.' || column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name IN ('file_blobs','file_references','file_reference_operations','file_access_capabilities','file_space_lifecycle_fences','file_space_deletion_manifests') ORDER BY 1`)
	require.NoError(t, err)
	defer rows.Close()
	var gotColumns []string
	for rows.Next() {
		var column string
		require.NoError(t, rows.Scan(&column))
		gotColumns = append(gotColumns, column)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, wantColumns, gotColumns, "the additive schema fingerprint must not silently drop durable replay, fence, manifest, capability or GC evidence")

	downPath := filepath.Join(fileGateRepoRoot(t), "src", "backend", "migrations", "file_db", "000004_file_reference_lifecycle.down.sql")
	downSQL, err := os.ReadFile(downPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(downSQL))
	require.NoError(t, err, "guarded DOWN succeeds only when all durable lifecycle tables are empty")
	for _, table := range wantTables {
		var dropped *string
		require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass($1)::text`, "public."+table).Scan(&dropped))
		require.Nil(t, dropped, "%s must be removed by an empty guarded DOWN", table)
	}
}

func startR23FilePostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := startFileGatePostgres(t, ctx)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000004_file_reference_lifecycle.up.sql"))
	return pool
}

func r23FileServer(pool *pgxpool.Pool) *FileGRPC {
	return New(Deps{Files: store.NewFilesStore(pool), Presigner: gatePresigner{}, ReferenceAuthorityActive: true})
}

func r23TestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func r23WaitForBlockedTransactions(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int64) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&blocked))
		if blocked >= want {
			return
		}
		select {
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		case <-deadline.C:
			require.FailNow(t, "reference mutation did not reach the blob-row barrier", "wanted %d blocked transactions", want)
		case <-ticker.C:
		}
	}
}

func r23ServiceContext(t *testing.T, ctx context.Context, service, rpc string, request proto.Message) context.Context {
	t.Helper()
	return r23PrincipalContext(t, ctx, service, "file", rpc, request, "")
}

func r23PrincipalContext(t *testing.T, ctx context.Context, service, audience, rpc string, request proto.Message, hashOverride string) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	if hashOverride != "" {
		hash = hashOverride
	}
	return principal.WithVerified(ctx, principal.Principal{
		Kind:        "service",
		Issuer:      service,
		Subject:     "service:" + service,
		Audience:    audience,
		RPC:         rpc,
		RequestID:   uuid.NewString(),
		RequestHash: hash,
	})
}

func r23UserContext(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.Pairs(
		"x-voice-user-id", accountID.String(),
		"x-voice-profile-id", profileID.String(),
	))
}

func r23MessageReference(fileID, ownerID, spaceID uuid.UUID) *filev1.FileReferenceKey {
	return &filev1.FileReferenceKey{
		FileId:       fileID.String(),
		OwnerType:    filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE,
		OwnerId:      ownerID.String(),
		ScopeSpaceId: proto.String(spaceID.String()),
	}
}

func r23AcquireRequest(reference *filev1.FileReferenceKey) *filev1.AcquireFileReferencesRequest {
	return &filev1.AcquireFileReferencesRequest{
		ProtocolVersion: 1, OperationId: uuid.NewString(),
		ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING,
		References: []*filev1.FileReferenceKey{reference},
	}
}

func r23ManifestBinding(id string, count uint64) *commonv1.ManifestBinding {
	return &commonv1.ManifestBinding{ManifestId: id, ManifestSha256: bytes32(id), ItemCount: count}
}

func r23RegisterChunk(deletionID, spaceID uuid.UUID, generation uint64, producer filev1.FileReferenceProducerId, index uint64, references []*filev1.FileReferenceKey, seal bool) *filev1.RegisterSpaceDeletionReferenceChunkRequest {
	return &filev1.RegisterSpaceDeletionReferenceChunkRequest{
		ProtocolVersion:          1,
		ProducerId:               producer,
		OperationId:              uuid.NewString(),
		DeletionOperationId:      deletionID.String(),
		SpaceId:                  spaceID.String(),
		ScheduleGeneration:       generation,
		ChunkIndex:               index,
		References:               references,
		ExpectedTotalCount:       uint64(len(references)),
		ExpectedReferencesSha256: r23ReferencesHash(deletionID, spaceID, generation, producer, references),
		SealsProducer:            seal,
	}
}

func r23ReferencesHash(deletionID, spaceID uuid.UUID, generation uint64, producer filev1.FileReferenceProducerId, references []*filev1.FileReferenceKey) []byte {
	h := sha256.New()
	h.Write([]byte("voice.file.v1.SpaceDeletionReferenceProducer"))
	h.Write([]byte{0})
	h.Write(deletionID[:])
	h.Write(spaceID[:])
	var generationBytes [8]byte
	binary.BigEndian.PutUint64(generationBytes[:], generation)
	h.Write(generationBytes[:])
	var producerBytes [4]byte
	binary.BigEndian.PutUint32(producerBytes[:], uint32(producer))
	h.Write(producerBytes[:])
	for _, reference := range sortedR23References(references) {
		wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(reference)
		if err != nil {
			panic(err)
		}
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(wire)))
		h.Write(length[:])
		h.Write(wire)
	}
	return h.Sum(nil)
}

func sortedR23References(references []*filev1.FileReferenceKey) []*filev1.FileReferenceKey {
	result := append([]*filev1.FileReferenceKey(nil), references...)
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		leftFileID, rightFileID := uuid.MustParse(left.GetFileId()), uuid.MustParse(right.GetFileId())
		if comparison := bytes.Compare(leftFileID[:], rightFileID[:]); comparison != 0 {
			return comparison < 0
		}
		if left.GetOwnerType() != right.GetOwnerType() {
			return left.GetOwnerType() < right.GetOwnerType()
		}
		return r23OptionalUUIDTuple(left) < r23OptionalUUIDTuple(right)
	})
	return result
}

func r23OptionalUUIDTuple(reference *filev1.FileReferenceKey) string {
	var tuple []byte
	for _, raw := range []string{reference.GetOwnerId(), reference.GetSubresourceId(), reference.GetScopeSpaceId()} {
		if raw == "" {
			tuple = append(tuple, make([]byte, 16)...)
			continue
		}
		id := uuid.MustParse(raw)
		tuple = append(tuple, id[:]...)
	}
	return string(tuple)
}

func r23RequestHash(t *testing.T, message proto.Message) []byte {
	t.Helper()
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	require.NoError(t, err)
	fqn := string(message.ProtoReflect().Descriptor().FullName())
	joined := append(append([]byte(fqn), 0), wire...)
	sum := sha256.Sum256(joined)
	return sum[:]
}

func bytes32(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

func fileProducerSet() string {
	return fmt.Sprintf("%s/%s/%s", "SPACE", "CHAT", "MESSAGING")
}

func insertR23ReadyFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string) uuid.UUID {
	t.Helper()
	id, blobID := uuid.New(), uuid.New()
	digest := sha256.Sum256([]byte(label))
	_, err := pool.Exec(ctx, `
INSERT INTO file_blobs (blob_id, sha256, original_r2_key, state)
VALUES ($1, $2, $3, 'LIVE')`, blobID, digest[:], "r23/"+label)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO files (id, blob_id, uploader_profile_id, original_name, mime_type, size_bytes, sha256_hash, r2_key, status, file_type, is_e2e, scan_result)
VALUES ($1, $2, $3, $4, 'application/octet-stream', 1, $5, $6, 'ready', 'other', false, 'clean')`,
		id, blobID, uuid.New(), label+".bin", fmt.Sprintf("%x", digest), "r23/"+label)
	require.NoError(t, err)
	return id
}

func insertR23AliasFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, source uuid.UUID, label string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO files (id, blob_id, uploader_profile_id, original_name, mime_type, size_bytes, sha256_hash, r2_key, status, file_type, is_e2e, scan_result)
SELECT $1, blob_id, $2, $3, mime_type, size_bytes, sha256_hash, r2_key, status, file_type, is_e2e, scan_result
FROM files WHERE id = $4`, id, uuid.New(), label+".bin", source)
	require.NoError(t, err)
	return id
}

func insertR23Reference(t *testing.T, ctx context.Context, pool *pgxpool.Pool, reference *filev1.FileReferenceKey) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO file_references (file_id, owner_type, owner_id, subresource_id, scope_space_id)
VALUES ($1, $2, $3, $4, $5)`, reference.GetFileId(), int32(reference.GetOwnerType()), reference.GetOwnerId(), optionalString(reference.SubresourceId), optionalString(reference.ScopeSpaceId))
	require.NoError(t, err)
}

func prepareR23SealedManifest(t *testing.T, ctx context.Context, client *FileGRPC, deletionID, spaceID uuid.UUID, sourceGeneration uint64, references []*filev1.FileReferenceKey) *commonv1.ManifestBinding {
	t.Helper()
	chatManifest := r23ManifestBinding("chat-root", 3)
	prepare := &filev1.PrepareSpaceDeletionReferenceManifestRequest{
		ProtocolVersion:     1,
		SpaceId:             spaceID.String(),
		DeletionOperationId: deletionID.String(),
		ScheduleGeneration:  sourceGeneration,
		ChatManifest:        chatManifest,
	}
	_, err := client.PrepareSpaceDeletionReferenceManifest(r23ServiceContext(t, ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, prepare), prepare)
	require.NoError(t, err)
	declarations := make([]*filev1.FileReferenceProducerDeclaration, 0, 3)
	for _, producer := range []struct {
		caller     string
		producerID filev1.FileReferenceProducerId
		references []*filev1.FileReferenceKey
	}{
		{caller: "space", producerID: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE},
		{caller: "chat", producerID: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT},
		{caller: "messaging", producerID: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, references: sortedR23References(references)},
	} {
		chunk := r23RegisterChunk(
			deletionID, spaceID, sourceGeneration, producer.producerID, 0, producer.references, true,
		)
		_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, producer.caller, filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, chunk), chunk)
		require.NoError(t, err)
		declarations = append(declarations, &filev1.FileReferenceProducerDeclaration{
			ProducerId:               producer.producerID,
			ExpectedTotalCount:       uint64(len(producer.references)),
			ExpectedReferencesSha256: chunk.GetExpectedReferencesSha256(),
		})
	}
	manifest := r23SpaceDeletionManifestSet(deletionID, spaceID, sourceGeneration, chatManifest, declarations)
	fence := &filev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion:     1,
		SpaceId:             spaceID.String(),
		DeletionOperationId: deletionID.String(),
		Generation:          sourceGeneration,
		DesiredState:        commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:            manifest,
	}}
	_, err = client.ApplySpaceLifecycleFence(r23ServiceContext(t, ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, fence), fence)
	require.NoError(t, err)
	return manifest
}

func issueR23MetadataCapability(t *testing.T, ctx context.Context, client *FileGRPC, subject uuid.UUID, reference *filev1.FileReferenceKey) string {
	return issueR23Capability(t, ctx, client, "messaging", subject, reference, filev1.FileReadSurface_FILE_READ_SURFACE_METADATA)
}

func issueR23Capability(t *testing.T, ctx context.Context, client *FileGRPC, caller string, subject uuid.UUID, reference *filev1.FileReferenceKey, surfaces ...filev1.FileReadSurface) string {
	t.Helper()
	request := &filev1.IssueFileAccessCapabilityRequest{
		ProtocolVersion:  1,
		OperationId:      uuid.NewString(),
		Reference:        reference,
		SubjectProfileId: subject.String(),
		AllowedSurfaces:  surfaces,
		ExpiresAt:        timestamppb.New(time.Now().UTC().Add(30 * time.Minute)),
	}
	response, err := client.IssueFileAccessCapability(r23ServiceContext(t, ctx, caller, filev1.FileService_IssueFileAccessCapability_FullMethodName, request), request)
	require.NoError(t, err)
	return response.GetReceipt().GetCapabilityId()
}

func r23SpaceDeletionManifestSet(deletionID, spaceID uuid.UUID, generation uint64, chatManifest *commonv1.ManifestBinding, declarations []*filev1.FileReferenceProducerDeclaration) *commonv1.ManifestBinding {
	var wire []byte
	wire = protowire.AppendTag(wire, 1, protowire.VarintType)
	wire = protowire.AppendVarint(wire, 1)
	wire = protowire.AppendTag(wire, 2, protowire.BytesType)
	wire = protowire.AppendString(wire, spaceID.String())
	wire = protowire.AppendTag(wire, 3, protowire.BytesType)
	wire = protowire.AppendString(wire, deletionID.String())
	wire = protowire.AppendTag(wire, 4, protowire.VarintType)
	wire = protowire.AppendVarint(wire, generation)
	chatWire, err := proto.MarshalOptions{Deterministic: true}.Marshal(chatManifest)
	if err != nil {
		panic(err)
	}
	wire = protowire.AppendTag(wire, 5, protowire.BytesType)
	wire = protowire.AppendBytes(wire, chatWire)
	count := chatManifest.GetItemCount()
	for _, declaration := range declarations {
		declarationWire, marshalErr := proto.MarshalOptions{Deterministic: true}.Marshal(declaration)
		if marshalErr != nil {
			panic(marshalErr)
		}
		wire = protowire.AppendTag(wire, 6, protowire.BytesType)
		wire = protowire.AppendBytes(wire, declarationWire)
		count += declaration.GetExpectedTotalCount()
	}
	h := sha256.New()
	h.Write([]byte("voice.space.v1.SpaceDeletionManifestSet"))
	h.Write([]byte{0})
	h.Write(wire) // root_sha256 field 7 is deliberately cleared for the hash.
	return &commonv1.ManifestBinding{ManifestId: deletionID.String(), ManifestSha256: h.Sum(nil), ItemCount: count}
}

func r23CapabilitySelector(capabilityID string) *filev1.FileAccessSelector {
	return &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_CapabilityId{CapabilityId: capabilityID}}
}

func r23BlobGCState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fileID uuid.UUID) string {
	t.Helper()
	var state string
	require.NoError(t, pool.QueryRow(ctx, `
SELECT blobs.state
FROM files
JOIN file_blobs AS blobs ON blobs.blob_id = files.blob_id
WHERE files.id = $1`, fileID).Scan(&state))
	return state
}

func r23LiveReferenceCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fileID uuid.UUID) int64 {
	t.Helper()
	var count int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_references WHERE file_id = $1 AND released_at IS NULL`, fileID).Scan(&count))
	return count
}

func r23SpaceFenceState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, spaceID uuid.UUID) string {
	t.Helper()
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM file_space_lifecycle_fences WHERE space_id = $1`, spaceID).Scan(&state))
	return state
}

func optionalString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
