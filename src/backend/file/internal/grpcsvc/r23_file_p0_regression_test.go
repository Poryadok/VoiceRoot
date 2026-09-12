package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/store"
)

func TestR23FileGC_QueuedReferenceResurrectionPreventsPhysicalDelete(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	fileID, spaceID := insertR23ReadyFile(t, ctx, pool, "gc-resurrection"), uuid.New()
	reference := r23MessageReference(fileID, uuid.New(), spaceID)
	acquire := r23AcquireRequest(reference)
	_, err := client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)
	release := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{reference}}
	_, err = client.ReleaseFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseFileReferences_FullMethodName, release), release)
	require.NoError(t, err)
	require.Equal(t, "GC_PENDING", r23BlobGCState(t, ctx, pool, fileID))

	barrier, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = barrier.Rollback(context.Background()) })
	var blobID uuid.UUID
	require.NoError(t, barrier.QueryRow(ctx, `SELECT blob_id FROM file_blobs WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1) FOR UPDATE`, fileID).Scan(&blobID))

	reacquire := r23AcquireRequest(reference)
	reacquireDone := make(chan error, 1)
	go func() {
		_, callErr := client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, reacquire), reacquire)
		reacquireDone <- callErr
	}()
	r23WaitForBlockedTransactions(t, ctx, pool, 1)

	deleter := &r23ScriptedDeleter{}
	processed, err := client.files.RunReferenceGCOnce(ctx, deleter, 1)
	require.NoError(t, err)
	require.Zero(t, processed, "GC must skip a locked candidate instead of deleting through a concurrent reference mutation")
	deleter.mu.Lock()
	require.Empty(t, deleter.calls)
	deleter.mu.Unlock()

	require.NoError(t, barrier.Commit(ctx))
	require.NoError(t, <-reacquireDone)
	require.Equal(t, "LIVE", r23BlobGCState(t, ctx, pool, fileID))
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, fileID))
}

func TestR23FileGC_AmbiguousDeleteCannotResurrectPossiblyMissingBytes(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	fileID, spaceID := insertR23ReadyFile(t, ctx, pool, "gc-ambiguous-no-resurrection"), uuid.New()
	reference := r23MessageReference(fileID, uuid.New(), spaceID)
	acquire := r23AcquireRequest(reference)
	_, err := client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)
	release := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, References: []*filev1.FileReferenceKey{reference}}
	_, err = client.ReleaseFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseFileReferences_FullMethodName, release), release)
	require.NoError(t, err)

	deleter := &r23ScriptedDeleter{failures: map[string]int{"r23/gc-ambiguous-no-resurrection": 1}}
	_, err = client.files.RunReferenceGCOnce(ctx, deleter, 1)
	require.Error(t, err)
	reacquire := r23AcquireRequest(reference)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, reacquire), reacquire)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "once an R2 delete attempt begins, ambiguous absence cannot be revived as LIVE")
	require.Equal(t, int64(0), r23LiveReferenceCount(t, ctx, pool, fileID))
	require.Equal(t, "GC_PENDING", r23BlobGCState(t, ctx, pool, fileID))
	var deletionStarted bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM file_blobs WHERE blob_id=(SELECT blob_id FROM files WHERE id=$1)`, fileID).Scan(&deletionStarted))
	require.True(t, deletionStarted)
}

func TestR23FileLegacyAccess_RechecksCardinalityAfterSerializationLocks(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	subject, spaceID := uuid.New(), uuid.New()
	fileID := insertR23ReadyFile(t, ctx, pool, "legacy-cardinality-race")
	_, err := pool.Exec(ctx, `UPDATE files SET uploader_profile_id=$2 WHERE id=$1`, fileID, subject)
	require.NoError(t, err)
	first := r23MessageReference(fileID, uuid.New(), spaceID)
	firstAcquire := r23AcquireRequest(first)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, firstAcquire), firstAcquire)
	require.NoError(t, err)
	second := r23MessageReference(fileID, uuid.New(), spaceID)
	secondAcquire := r23AcquireRequest(second)
	secondCtx := r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, secondAcquire)
	readCtx := r23UserContext(ctx, uuid.New(), subject)

	barrier, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = barrier.Rollback(context.Background()) })
	var lockedSpace uuid.UUID
	require.NoError(t, barrier.QueryRow(ctx, `SELECT space_id FROM file_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&lockedSpace))
	acquireDone := make(chan error, 1)
	go func() {
		_, callErr := client.AcquireFileReferences(secondCtx, secondAcquire)
		acquireDone <- callErr
	}()
	r23WaitForBlockedTransactions(t, ctx, pool, 1)
	type readResult struct {
		response *filev1.GetFileURLResponse
		err      error
	}
	readDone := make(chan readResult, 1)
	go func() {
		response, callErr := client.GetFileURL(readCtx, &filev1.GetFileURLRequest{FileId: fileID.String()})
		readDone <- readResult{response: response, err: callErr}
	}()
	r23WaitForBlockedTransactions(t, ctx, pool, 2)
	require.NoError(t, barrier.Commit(ctx), "second acquire is queued ahead of the legacy read on the canonical fence lock")
	require.NoError(t, <-acquireDone)
	read := <-readDone
	require.Nil(t, read.response)
	require.Equal(t, codes.PermissionDenied, status.Code(read.err), "the locked recheck observes the second live key and denies legacy authority")
	require.Equal(t, int64(2), r23LiveReferenceCount(t, ctx, pool, fileID))
}

func TestR23FileLegacyAccess_MigrationAloneDoesNotActivateReferenceAuthority(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := New(Deps{Files: store.NewFilesStore(pool), Presigner: gatePresigner{}})
	subject := uuid.New()
	fileID := insertR23ReadyFile(t, ctx, pool, "migration-only-legacy")
	_, err := pool.Exec(ctx, `UPDATE files SET uploader_profile_id=$2 WHERE id=$1`, fileID, subject)
	require.NoError(t, err)

	var referenceTable *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.file_references')::text`).Scan(&referenceTable))
	require.NotNil(t, referenceTable, "the lifecycle migration is installed")
	require.Equal(t, int64(0), r23LiveReferenceCount(t, ctx, pool, fileID), "backfill and dual-write have not populated reference authority")

	userCtx := r23UserContext(ctx, uuid.New(), subject)
	urlResponse, err := client.GetFileURL(userCtx, &filev1.GetFileURLRequest{FileId: fileID.String()})
	require.NoError(t, err, "schema presence alone cannot cut over a zero-reference legacy caller")
	require.NotNil(t, urlResponse)
	metadataResponse, err := client.GetFileMetadata(userCtx, &filev1.GetFileMetadataRequest{FileId: fileID.String()})
	require.NoError(t, err)
	require.Equal(t, fileID.String(), metadataResponse.GetFileMetadata().GetId())
	bulkResponse, err := client.GetBulkMetadata(userCtx, &filev1.GetBulkMetadataRequest{FileIds: []string{fileID.String()}})
	require.NoError(t, err)
	require.Contains(t, bulkResponse.GetBulkFileMetadata().GetByFileId(), fileID.String())
}

func TestR23FileLegacyBulkMetadata_ActiveAuthorityDeniesFrozenSpaceWithoutLeak(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	subject, spaceID, deletionID := uuid.New(), uuid.New(), uuid.New()
	fileID := insertR23ReadyFile(t, ctx, pool, "legacy-bulk-frozen")
	_, err := pool.Exec(ctx, `UPDATE files SET uploader_profile_id=$2 WHERE id=$1`, fileID, subject)
	require.NoError(t, err)
	reference := r23MessageReference(fileID, uuid.New(), spaceID)
	acquire := r23AcquireRequest(reference)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)
	require.Equal(t, "LIVE", r23SpaceFenceState(t, ctx, pool, spaceID))

	request := &filev1.GetBulkMetadataRequest{FileIds: []string{fileID.String()}}
	live, err := client.GetBulkMetadata(r23UserContext(ctx, uuid.New(), subject), request)
	require.NoError(t, err)
	require.Contains(t, live.GetBulkFileMetadata().GetByFileId(), fileID.String())

	prepare := &filev1.PrepareSpaceDeletionReferenceManifestRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), ScheduleGeneration: 1,
		ChatManifest: r23ManifestBinding("p0-legacy-bulk-freeze", 0),
	}
	_, err = client.PrepareSpaceDeletionReferenceManifest(r23ServiceContext(t, ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, prepare), prepare)
	require.NoError(t, err)
	require.Equal(t, "FROZEN", r23SpaceFenceState(t, ctx, pool, spaceID))

	frozen, err := client.GetBulkMetadata(r23UserContext(ctx, uuid.New(), subject), proto.Clone(request).(*filev1.GetBulkMetadataRequest))
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Nil(t, frozen, "legacy bulk must not return metadata from a frozen Space")
}

func TestR23FileCanonicalBulkMetadata_ReversedDuplicateFileSelectorsUseOneLockOrder(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	subject, firstSpace, secondSpace := uuid.New(), uuid.New(), uuid.New()
	fileID := insertR23ReadyFile(t, ctx, pool, "canonical-bulk-lock-order")
	_, err := pool.Exec(ctx, `UPDATE files SET uploader_profile_id=$2 WHERE id=$1`, fileID, subject)
	require.NoError(t, err)
	firstReference := r23MessageReference(fileID, uuid.MustParse("00000000-0000-0000-0000-000000000001"), firstSpace)
	secondReference := r23MessageReference(fileID, uuid.MustParse("00000000-0000-0000-0000-000000000002"), secondSpace)
	for _, reference := range []*filev1.FileReferenceKey{firstReference, secondReference} {
		acquire := r23AcquireRequest(reference)
		_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
		require.NoError(t, err)
	}
	require.Equal(t, "LIVE", r23SpaceFenceState(t, ctx, pool, firstSpace))
	require.Equal(t, "LIVE", r23SpaceFenceState(t, ctx, pool, secondSpace))

	firstItem := &filev1.FileAccessItem{FileId: fileID.String(), Access: &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{Reference: firstReference}}}
	secondItem := &filev1.FileAccessItem{FileId: fileID.String(), Access: &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{Reference: secondReference}}}
	forward := &filev1.GetBulkMetadataRequest{Items: []*filev1.FileAccessItem{firstItem, secondItem}}
	reversed := &filev1.GetBulkMetadataRequest{Items: []*filev1.FileAccessItem{secondItem, firstItem}}
	preparedForward, err := prepareBulkMetadataItems(forward.GetItems())
	require.NoError(t, err)
	preparedReversed, err := prepareBulkMetadataItems(reversed.GetItems())
	require.NoError(t, err)
	require.Equal(t, preparedForward[0].selectorKey, preparedReversed[0].selectorKey, "duplicate file IDs use the same canonical selector tie-break in either request order")
	require.Equal(t, firstReference.GetOwnerId(), preparedForward[0].selector.GetReference().GetOwnerId())
	userCtx := r23UserContext(ctx, uuid.New(), subject)

	validationBarrier, err := pool.Begin(ctx)
	require.NoError(t, err)
	var validationLockedSpace uuid.UUID
	require.NoError(t, validationBarrier.QueryRow(ctx, `SELECT space_id FROM file_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, firstSpace).Scan(&validationLockedSpace))
	invalidRequest := &filev1.GetBulkMetadataRequest{Items: []*filev1.FileAccessItem{
		firstItem,
		{FileId: fileID.String(), Access: &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_CapabilityId{CapabilityId: "not-a-uuid"}}},
	}}
	validationCtx, cancelValidation := context.WithTimeout(userCtx, 3*time.Second)
	type bulkResult struct {
		response *filev1.GetBulkMetadataResponse
		err      error
	}
	invalidDone := make(chan bulkResult, 1)
	go func() {
		response, callErr := client.GetBulkMetadata(validationCtx, invalidRequest)
		invalidDone <- bulkResult{response: response, err: callErr}
	}()
	select {
	case result := <-invalidDone:
		require.Nil(t, result.response)
		require.Equal(t, codes.PermissionDenied, status.Code(result.err), "all selectors are validated before the first canonical fence lock")
	case <-validationCtx.Done():
		_ = validationBarrier.Rollback(context.Background())
		cancelValidation()
		require.FailNow(t, "bulk validation acquired a lock", "the malformed trailing selector waited behind the valid first item's fence: %v", validationCtx.Err())
	}
	cancelValidation()
	require.NoError(t, validationBarrier.Rollback(ctx))

	barrier, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = barrier.Rollback(context.Background()) })
	var lockedSpace uuid.UUID
	require.NoError(t, barrier.QueryRow(ctx, `SELECT space_id FROM file_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, firstSpace).Scan(&lockedSpace))

	bulkDone := make(chan bulkResult, 2)
	for _, request := range []*filev1.GetBulkMetadataRequest{forward, reversed} {
		request := request
		go func() {
			response, callErr := client.GetBulkMetadata(userCtx, request)
			bulkDone <- bulkResult{response: response, err: callErr}
		}()
	}
	r23WaitForBlockedTransactions(t, ctx, pool, 2)

	mutationFile := insertR23ReadyFile(t, ctx, pool, "canonical-bulk-unlocked-fence-mutation")
	mutationReference := r23MessageReference(mutationFile, uuid.New(), secondSpace)
	mutation := r23AcquireRequest(mutationReference)
	mutationCtx, cancelMutation := context.WithTimeout(ctx, 3*time.Second)
	defer cancelMutation()
	mutationDone := make(chan error, 1)
	go func() {
		_, callErr := client.AcquireFileReferences(r23ServiceContext(t, mutationCtx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, mutation), mutation)
		mutationDone <- callErr
	}()
	select {
	case mutationErr := <-mutationDone:
		require.NoError(t, mutationErr, "both reversed bulk calls must block on the canonical first fence before either locks the second fence")
	case <-mutationCtx.Done():
		_ = barrier.Rollback(context.Background())
		require.FailNow(t, "non-canonical bulk lock order", "the second-fence reference mutation was blocked: %v", mutationCtx.Err())
	}

	require.NoError(t, barrier.Commit(ctx))
	for range 2 {
		result := <-bulkDone
		require.NoError(t, result.err)
		require.NotNil(t, result.response)
		require.Contains(t, result.response.GetBulkFileMetadata().GetByFileId(), fileID.String())
	}
}

func TestR23FileReferenceMutation_ProducerOwnerAllowListIsClosed(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	fileID, spaceID := insertR23ReadyFile(t, ctx, pool, "producer-owner-allow-list"), uuid.New()
	message := r23MessageReference(fileID, uuid.New(), spaceID)

	crossAcquire := r23AcquireRequest(message)
	crossAcquire.ProducerId = filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE
	_, err := client.AcquireFileReferences(r23ServiceContext(t, ctx, "space", filev1.FileService_AcquireFileReferences_FullMethodName, crossAcquire), crossAcquire)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	unknown := proto.Clone(message).(*filev1.FileReferenceKey)
	unknown.OwnerType = filev1.FileReferenceOwnerType(99)
	unknownAcquire := r23AcquireRequest(unknown)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, unknownAcquire), unknownAcquire)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, int64(0), r23LiveReferenceCount(t, ctx, pool, fileID))
	require.Equal(t, "LIVE", r23BlobGCState(t, ctx, pool, fileID))

	validAcquire := r23AcquireRequest(message)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, validAcquire), validAcquire)
	require.NoError(t, err)
	crossRelease := &filev1.ReleaseFileReferencesRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ProducerId: filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_SPACE, References: []*filev1.FileReferenceKey{message}}
	_, err = client.ReleaseFileReferences(r23ServiceContext(t, ctx, "space", filev1.FileService_ReleaseFileReferences_FullMethodName, crossRelease), crossRelease)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, fileID))
	require.Equal(t, "LIVE", r23BlobGCState(t, ctx, pool, fileID))
}

func TestR23FileManifest_RejectsForeignAndProducerMismatchedReferences(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	targetSpace, foreignSpace, deletionID := uuid.New(), uuid.New(), uuid.New()
	prepare := &filev1.PrepareSpaceDeletionReferenceManifestRequest{
		ProtocolVersion: 1, SpaceId: targetSpace.String(), DeletionOperationId: deletionID.String(), ScheduleGeneration: 1,
		ChatManifest: r23ManifestBinding("p0-foreign-manifest", 0),
	}
	_, err := client.PrepareSpaceDeletionReferenceManifest(r23ServiceContext(t, ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, prepare), prepare)
	require.NoError(t, err)

	fileID := insertR23ReadyFile(t, ctx, pool, "foreign-manifest")
	foreign := r23MessageReference(fileID, uuid.New(), foreignSpace)
	wrongScope := r23RegisterChunk(deletionID, targetSpace, 1, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, []*filev1.FileReferenceKey{foreign}, true)
	_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "messaging", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, wrongScope), wrongScope)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	storyInSpace := &filev1.FileReferenceKey{FileId: fileID.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_STORY, OwnerId: uuid.NewString(), ScopeSpaceId: ptrString(targetSpace.String())}
	wrongOwner := r23RegisterChunk(deletionID, targetSpace, 1, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, []*filev1.FileReferenceKey{storyInSpace}, true)
	_, err = client.RegisterSpaceDeletionReferenceChunk(r23ServiceContext(t, ctx, "messaging", filev1.FileService_RegisterSpaceDeletionReferenceChunk_FullMethodName, wrongOwner), wrongOwner)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	var chunks int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_space_deletion_manifest_chunks WHERE deletion_operation_id=$1`, deletionID).Scan(&chunks))
	require.Zero(t, chunks)
}

func TestR23FileRelease_DefendsAgainstForeignTupleInDurableManifest(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	targetSpace, foreignSpace, deletionID := uuid.New(), uuid.New(), uuid.New()
	targetFile, foreignFile := insertR23ReadyFile(t, ctx, pool, "release-target"), insertR23ReadyFile(t, ctx, pool, "release-foreign")
	substituteFile := insertR23ReadyFile(t, ctx, pool, "release-substitute")
	target := r23MessageReference(targetFile, uuid.New(), targetSpace)
	foreign := r23MessageReference(foreignFile, uuid.New(), foreignSpace)
	substitute := r23MessageReference(substituteFile, uuid.New(), targetSpace)
	insertR23Reference(t, ctx, pool, target)
	insertR23Reference(t, ctx, pool, foreign)
	insertR23Reference(t, ctx, pool, substitute)
	root := prepareR23SealedManifest(t, ctx, client, deletionID, targetSpace, 1, []*filev1.FileReferenceKey{target})

	tampered := r23RegisterChunk(deletionID, targetSpace, 1, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, []*filev1.FileReferenceKey{foreign}, true)
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(tampered)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE file_space_deletion_manifest_chunks SET references_bytes=$1 WHERE deletion_operation_id=$2 AND producer_id=$3`, wire, deletionID, int32(filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING))
	require.NoError(t, err)
	applyR23PurgeDecision(t, ctx, client, deletionID, targetSpace, 2, root)

	release := &filev1.ReleaseSpaceDeletionProducerReferencesRequest{
		ProtocolVersion: 1, DeletionOperationId: deletionID.String(), SpaceId: targetSpace.String(), PurgeGeneration: 2, SourceScheduleGeneration: 1,
		ProducerId:               filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING,
		ExpectedReferencesSha256: r23ReferencesHash(deletionID, targetSpace, 1, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, []*filev1.FileReferenceKey{target}),
	}
	_, err = client.ReleaseSpaceDeletionProducerReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, release), release)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, targetFile))
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, foreignFile))

	sameScopeTamper := r23RegisterChunk(deletionID, targetSpace, 1, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, 0, []*filev1.FileReferenceKey{substitute}, true)
	wire, err = proto.MarshalOptions{Deterministic: true}.Marshal(sameScopeTamper)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE file_space_deletion_manifest_chunks SET references_bytes=$1 WHERE deletion_operation_id=$2 AND producer_id=$3`, wire, deletionID, int32(filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING))
	require.NoError(t, err)
	_, err = client.ReleaseSpaceDeletionProducerReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, release), release)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "same-Space same-producer tuple substitution cannot match the sealed canonical digest")
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, targetFile))
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, substituteFile))
}

func TestR23FileAccess_ExactReferenceAndLegacyUseLiveFence(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	subject, spaceID, deletionID := uuid.New(), uuid.New(), uuid.New()
	fileID := insertR23ReadyFile(t, ctx, pool, "exact-reference-read")
	_, err := pool.Exec(ctx, `UPDATE files SET uploader_profile_id=$2 WHERE id=$1`, fileID, subject)
	require.NoError(t, err)
	reference := r23MessageReference(fileID, uuid.New(), spaceID)
	acquire := r23AcquireRequest(reference)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)

	selector := &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{Reference: reference}}
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: fileID.String(), Access: selector})
	require.NoError(t, err, "an exact live reference is a valid selector before capability activation")
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: fileID.String()})
	require.NoError(t, err, "legacy file_id is allowed only because exactly one live reference exists")

	prepare := &filev1.PrepareSpaceDeletionReferenceManifestRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), ScheduleGeneration: 1,
		ChatManifest: r23ManifestBinding("p0-read-freeze", 0),
	}
	_, err = client.PrepareSpaceDeletionReferenceManifest(r23ServiceContext(t, ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, prepare), prepare)
	require.NoError(t, err)
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: fileID.String(), Access: selector})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	_, err = client.GetFileURL(r23UserContext(ctx, uuid.New(), subject), &filev1.GetFileURLRequest{FileId: fileID.String()})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "uploader convenience cannot bypass the exact reference's frozen fence")
}

func TestR23FileLifecycle_ReplayMonotonicityAndPurgeDecisionGate(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	replaySpace, replayDeletion := uuid.New(), uuid.New()
	prepare := &filev1.PrepareSpaceDeletionReferenceManifestRequest{
		ProtocolVersion: 1, SpaceId: replaySpace.String(), DeletionOperationId: replayDeletion.String(), ScheduleGeneration: 7,
		ChatManifest: r23ManifestBinding("p0-prepare-replay", 0),
	}
	prepared, err := client.PrepareSpaceDeletionReferenceManifest(r23ServiceContext(t, ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, prepare), prepare)
	require.NoError(t, err)
	preparedReplay, err := client.PrepareSpaceDeletionReferenceManifest(r23ServiceContext(t, ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, prepare), proto.Clone(prepare).(*filev1.PrepareSpaceDeletionReferenceManifestRequest))
	require.NoError(t, err)
	require.True(t, proto.Equal(prepared, preparedReplay), "prepare replay returns immutable receipt bytes")
	changedPrepare := proto.Clone(prepare).(*filev1.PrepareSpaceDeletionReferenceManifestRequest)
	changedPrepare.ChatManifest = r23ManifestBinding("p0-prepare-changed", 0)
	_, err = client.PrepareSpaceDeletionReferenceManifest(r23ServiceContext(t, ctx, "space", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, changedPrepare), changedPrepare)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "same generation and deletion with changed body conflicts")

	fileID, spaceID, deletionID := insertR23ReadyFile(t, ctx, pool, "lifecycle-monotonic"), uuid.New(), uuid.New()
	reference := r23MessageReference(fileID, uuid.New(), spaceID)
	acquire := r23AcquireRequest(reference)
	_, err = client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)
	root := prepareR23SealedManifest(t, ctx, client, deletionID, spaceID, 1, []*filev1.FileReferenceKey{reference})

	finalFreeze := &filev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), Generation: 1,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: root,
	}}
	replayed, err := client.ApplySpaceLifecycleFence(r23ServiceContext(t, ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, finalFreeze), finalFreeze)
	require.NoError(t, err, "exact final-fence replay returns its immutable stored receipt")
	require.Equal(t, root.GetManifestSha256(), replayed.GetReceipt().GetManifestSha256())
	changedFinalFreeze := proto.Clone(finalFreeze).(*filev1.ApplySpaceLifecycleFenceRequest)
	changedFinalFreeze.Fence.Manifest.ItemCount++
	_, err = client.ApplySpaceLifecycleFence(r23ServiceContext(t, ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, changedFinalFreeze), changedFinalFreeze)
	require.Equal(t, codes.AlreadyExists, status.Code(err), "same generation/state with changed manifest bytes conflicts")

	release := &filev1.ReleaseSpaceDeletionProducerReferencesRequest{
		ProtocolVersion: 1, DeletionOperationId: deletionID.String(), SpaceId: spaceID.String(), PurgeGeneration: 2, SourceScheduleGeneration: 1,
		ProducerId:               filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING,
		ExpectedReferencesSha256: r23ReferencesHash(deletionID, spaceID, 1, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, []*filev1.FileReferenceKey{reference}),
	}
	_, err = client.ReleaseSpaceDeletionProducerReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, release), release)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "restorable FROZEN cannot authorize destructive release")
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, fileID))

	decision := applyR23PurgeDecision(t, ctx, client, deletionID, spaceID, 2, root)
	decisionReplay := applyR23PurgeDecision(t, ctx, client, deletionID, spaceID, 2, root)
	require.True(t, proto.Equal(decision, decisionReplay))
	restore := &filev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), Generation: 3,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, Manifest: root,
	}}
	_, err = client.ApplySpaceLifecycleFence(r23ServiceContext(t, ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, restore), restore)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "PURGE_DECIDED is irreversible")
	_, err = client.ReleaseSpaceDeletionProducerReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, release), release)
	require.NoError(t, err)
}

func TestR23FilePurgeBarrier_BindsCurrentScheduleGeneration(t *testing.T) {
	ctx := r23TestContext(t)
	pool := startR23FilePostgres(t, ctx)
	client := r23FileServer(pool)
	fileID, spaceID, deletionID := insertR23ReadyFile(t, ctx, pool, "purge-current-schedule"), uuid.New(), uuid.New()
	reference := r23MessageReference(fileID, uuid.New(), spaceID)
	acquire := r23AcquireRequest(reference)
	_, err := client.AcquireFileReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_AcquireFileReferences_FullMethodName, acquire), acquire)
	require.NoError(t, err)
	firstRoot := prepareR23SealedManifest(t, ctx, client, deletionID, spaceID, 1, []*filev1.FileReferenceKey{reference})
	restore := &filev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), Generation: 2,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, Manifest: firstRoot,
	}}
	_, err = client.ApplySpaceLifecycleFence(r23ServiceContext(t, ctx, "space", filev1.FileService_ApplySpaceLifecycleFence_FullMethodName, restore), restore)
	require.NoError(t, err)
	currentRoot := prepareR23SealedManifest(t, ctx, client, deletionID, spaceID, 3, []*filev1.FileReferenceKey{reference})
	applyR23PurgeDecision(t, ctx, client, deletionID, spaceID, 4, currentRoot)

	oldScheduleRelease := &filev1.ReleaseSpaceDeletionProducerReferencesRequest{
		ProtocolVersion: 1, DeletionOperationId: deletionID.String(), SpaceId: spaceID.String(), PurgeGeneration: 4, SourceScheduleGeneration: 1,
		ProducerId:               filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING,
		ExpectedReferencesSha256: r23ReferencesHash(deletionID, spaceID, 1, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_MESSAGING, []*filev1.FileReferenceKey{reference}),
	}
	_, err = client.ReleaseSpaceDeletionProducerReferences(r23ServiceContext(t, ctx, "messaging", filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, oldScheduleRelease), oldScheduleRelease)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "an old sealed schedule cannot release under the current purge generation")
	require.Equal(t, int64(1), r23LiveReferenceCount(t, ctx, pool, fileID))

	for producer := int32(1); producer <= 3; producer++ {
		_, err = pool.Exec(ctx, `
INSERT INTO file_space_deletion_producer_releases(
  space_id,deletion_operation_id,purge_generation,source_schedule_generation,producer_id,
  expected_references_sha256,request_bytes,request_sha256,receipt_bytes,released_count
) VALUES($1,$2,4,1,$3,$4,$4,$4,$4,0)`, spaceID, deletionID, producer, bytes32("old-schedule-release"))
		require.NoError(t, err)
	}
	purge := &filev1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(), Generation: 4,
		PurgeDecidedAt: timestamppb.Now(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_FILE, Manifest: currentRoot,
	}}
	_, err = client.PurgeSpace(r23ServiceContext(t, ctx, "space", filev1.FileService_PurgeSpace_FullMethodName, purge), purge)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "three stale release rows cannot satisfy the current source schedule barrier")
	require.Equal(t, "PURGE_DECIDED", r23SpaceFenceState(t, ctx, pool, spaceID))
}

func ptrString(value string) *string { return &value }
