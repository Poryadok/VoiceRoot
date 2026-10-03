package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

type producerTestStore struct{ refs []*filev1.FileReferenceKey }

func (s producerTestStore) SpaceFileProducerReferences(context.Context, uuid.UUID, uuid.UUID, uint64) ([]*filev1.FileReferenceKey, error) {
	return s.refs, nil
}

type producerTestFile struct {
	filev1.FileServiceClient
	requests     []*filev1.RegisterSpaceDeletionReferenceChunkRequest
	corrupt      bool
	releaseCalls int
	releaseErr   error
}

func (f *producerTestFile) RegisterSpaceDeletionReferenceChunk(_ context.Context, r *filev1.RegisterSpaceDeletionReferenceChunkRequest, _ ...grpc.CallOption) (*filev1.RegisterSpaceDeletionReferenceChunkResponse, error) {
	f.requests = append(f.requests, proto.Clone(r).(*filev1.RegisterSpaceDeletionReferenceChunkRequest))
	receipt := &filev1.RegisterSpaceDeletionReferenceChunkReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: r.OperationId, DeletionOperationId: r.DeletionOperationId, SpaceId: r.SpaceId, ScheduleGeneration: r.ScheduleGeneration, ChunkIndex: r.ChunkIndex, AcceptedCount: uint64(len(r.References)), ProducerId: r.ProducerId, ExpectedTotalCount: r.ExpectedTotalCount, ExpectedReferencesSha256: r.ExpectedReferencesSha256, ProducerSealed: r.SealsProducer, RequestSha256: lifecycleHash(r), CompletedAt: timestamppb.Now()}
	if f.corrupt {
		receipt.ExpectedTotalCount++
	}
	return &filev1.RegisterSpaceDeletionReferenceChunkResponse{Receipt: receipt}, nil
}
func TestSpaceChildUsesVerifiedProducerReleaseAndRejectsUncoveredAttachment(t *testing.T) {
	parent := validMessagingSpacePurgeRequest()
	scope := parent.Purge.SpaceId
	message := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	file := uuid.MustParse("00000000-0000-4000-8000-000000000003")
	producer := testSpaceFileProducer(t)
	producer.Store = producerTestStore{refs: []*filev1.FileReferenceKey{{FileId: file.String(), OwnerId: message.String(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, ScopeSpaceId: &scope}}}
	ctx, err := store.SpacePurgeContext(messagingSpacePurgeContext(t, parent), parent)
	require.NoError(t, err)
	child := &messagingv1.PurgeManagedChatContentRequest{OperationId: spaceChatPurgeOperationID(uuid.MustParse(scope), uuid.MustParse(parent.Purge.DeletionOperationId), uuid.New()).String(), ChatId: uuid.NewString(), PurgeAfter: parent.Purge.PurgeDecidedAt}
	// The ordinary File owner would return pending GC. P3 must use only its
	// sealed producer receipt and let File's later participant complete the GC.
	files := &purgeFileOwnerStub{pending: true}
	st := &purgeStoreStub{}
	owner := &ManagedChatPurgeCoordinator{Store: st, Files: files, Search: &purgeSearchOwnerStub{}, Issuer: producer.Issuer}
	_, err = owner.PurgeManagedChatContent(ctx, child)
	require.Error(t, err)
	require.Zero(t, st.completeN)
	released, err := producer.ReleaseContext(ctx, parent.Purge)
	require.NoError(t, err)
	_, err = owner.PurgeManagedChatContent(released, child)
	require.NoError(t, err)
	require.Equal(t, 1, st.completeN)
	require.Nil(t, files.release)
	require.Zero(t, files.statusN)
	_, childHash, err := hashManagedChatPurgeRequest(child)
	require.NoError(t, err)
	bad := &purgeStoreStub{work: &store.ManagedChatPurgeWork{OperationID: uuid.MustParse(child.OperationId), ChatID: uuid.MustParse(child.ChatId), PurgeAfter: child.PurgeAfter.AsTime(), RequestSHA256: childHash, State: "PENDING", MessageIDs: []uuid.UUID{message}, MessageAttachments: []store.ManagedChatPurgeMessage{{ID: message, FileIDs: []uuid.UUID{uuid.New()}}}}}
	owner.Store = bad
	_, err = owner.PurgeManagedChatContent(released, child)
	require.ErrorContains(t, err, "absent from released")
	require.Zero(t, bad.completeN)
}
func (f *producerTestFile) ReleaseSpaceDeletionProducerReferences(_ context.Context, r *filev1.ReleaseSpaceDeletionProducerReferencesRequest, _ ...grpc.CallOption) (*filev1.ReleaseSpaceDeletionProducerReferencesResponse, error) {
	f.releaseCalls++
	if f.releaseErr != nil {
		return nil, f.releaseErr
	}
	return &filev1.ReleaseSpaceDeletionProducerReferencesResponse{Receipt: &filev1.ReleaseSpaceDeletionProducerReferencesReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), DeletionOperationId: r.DeletionOperationId, SpaceId: r.SpaceId, PurgeGeneration: r.PurgeGeneration, SourceScheduleGeneration: r.SourceScheduleGeneration, ProducerId: r.ProducerId, ExpectedReferencesSha256: r.ExpectedReferencesSha256, RequestSha256: lifecycleHash(r), CompletedAt: timestamppb.Now()}}, nil
}
func testSpaceFileProducer(t *testing.T) *SpaceFileProducerCoordinator {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	return &SpaceFileProducerCoordinator{Store: producerTestStore{}, Files: &producerTestFile{}, Issuer: issuer}
}
func TestSpaceFileProducerSealsEmptyAndBoundedPagesWithExactReplay(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	space, operation := uuid.New(), uuid.New()
	for _, count := range []int{0, 1001} {
		scope := space.String()
		var refs []*filev1.FileReferenceKey
		for i := 0; i < count; i++ {
			refs = append(refs, &filev1.FileReferenceKey{FileId: uuid.NewString(), OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: uuid.NewString(), ScopeSpaceId: &scope})
		}
		files := &producerTestFile{}
		coordinator := &SpaceFileProducerCoordinator{Store: producerTestStore{refs: refs}, Files: files, Issuer: issuer}
		fence := &commonv1.SpaceLifecycleFenceRequest{SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 7}
		require.NoError(t, coordinator.Seal(context.Background(), fence))
		initial := append([]*filev1.RegisterSpaceDeletionReferenceChunkRequest(nil), files.requests...)
		require.NoError(t, coordinator.Seal(context.Background(), fence))
		require.True(t, proto.Equal(initial[0], files.requests[len(initial)]))
		require.True(t, initial[len(initial)-1].SealsProducer)
		for _, request := range initial {
			require.LessOrEqual(t, len(request.References), 1000)
			require.Equal(t, uint64(count), request.ExpectedTotalCount)
		}
		files.corrupt = true
		require.Error(t, coordinator.Seal(context.Background(), fence), "owner receipt mismatch must withhold Messaging fence ACK")
	}
}
