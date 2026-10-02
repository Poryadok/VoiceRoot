package grpcsvc

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

type spaceLifecyclePurgeStoreFake struct {
	pageCalls     []uint64
	chatIDs       []uuid.UUID
	completed     *messagingv1.PurgeSpaceRequest
	completionErr error
}

func (f *spaceLifecyclePurgeStoreFake) SpaceManifestChatPage(_ context.Context, spaceID, operationID uuid.UUID, scheduleGeneration uint64, manifestID string, manifestHash []byte, itemCount, pageIndex uint64) ([]uuid.UUID, error) {
	f.pageCalls = append(f.pageCalls, pageIndex)
	if spaceID == uuid.Nil || operationID == uuid.Nil || scheduleGeneration != 8 || manifestID == "" || len(manifestHash) != 32 || itemCount != 2 || pageIndex != 0 {
		return nil, errors.New("unexpected manifest page binding")
	}
	return append([]uuid.UUID(nil), f.chatIDs...), nil
}

func (f *spaceLifecyclePurgeStoreFake) CompleteSpaceLifecyclePurge(_ context.Context, req *messagingv1.PurgeSpaceRequest, _ string) ([]byte, error) {
	f.completed = proto.Clone(req).(*messagingv1.PurgeSpaceRequest)
	if f.completionErr != nil {
		return nil, f.completionErr
	}
	return proto.MarshalOptions{Deterministic: true}.Marshal(&messagingv1.PurgeSpaceResponse{Receipt: &commonv1.SpacePurgeReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: req.GetPurge().GetSpaceId(),
		DeletionOperationId: req.GetPurge().GetDeletionOperationId(), Generation: req.GetPurge().GetGeneration(),
		ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
		State:         commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED,
		RequestSha256: domainSHA(string(req.ProtoReflect().Descriptor().FullName()), mustMarshal(req)),
		CompletedAt:   timestamppb.New(time.Now().UTC()),
	}})
}

type spaceLifecycleChatPurgerFake struct {
	requests []*messagingv1.PurgeManagedChatContentRequest
	err      error
}

func (f *spaceLifecycleChatPurgerFake) PurgeManagedChatContent(_ context.Context, req *messagingv1.PurgeManagedChatContentRequest) (*messagingv1.PurgeManagedChatContentResponse, error) {
	f.requests = append(f.requests, proto.Clone(req).(*messagingv1.PurgeManagedChatContentRequest))
	if f.err != nil {
		return nil, f.err
	}
	wire, _ := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	digest := sha256.Sum256(wire)
	return &messagingv1.PurgeManagedChatContentResponse{
		ReceiptId: uuid.NewString(), OperationId: req.GetOperationId(), ChatId: req.GetChatId(), PurgeAfter: req.GetPurgeAfter(),
		FileReceiptSha256: make([]byte, 32), SearchReceiptSha256: make([]byte, 32),
		RequestSha256: digest[:], CompletedAt: timestamppb.Now(),
	}, nil
}

func TestPurgeSpaceRunsBoundedManifestChatsBeforeDurableCompletion(t *testing.T) {
	chatIDs := []uuid.UUID{uuid.MustParse("20000000-0000-4000-8000-000000000101"), uuid.MustParse("20000000-0000-4000-8000-000000000102")}
	request := validMessagingSpacePurgeRequest()
	store := &spaceLifecyclePurgeStoreFake{chatIDs: chatIDs}
	chatPurger := &spaceLifecycleChatPurgerFake{}
	service := &MessagingGRPC{SpaceLifecyclePurger: store, ManagedChatPurger: chatPurger, SpaceFileProducer: testSpaceFileProducer(t)}
	ctx := messagingSpacePurgeContext(t, request)

	response, err := service.PurgeSpace(ctx, request)
	require.NoError(t, err)
	require.Len(t, store.pageCalls, 1)
	require.Len(t, chatPurger.requests, 2)
	require.True(t, proto.Equal(request.GetPurge().GetPurgeDecidedAt(), chatPurger.requests[0].GetPurgeAfter()))
	require.Equal(t, chatIDs[0].String(), chatPurger.requests[0].GetChatId())
	require.Equal(t, spaceChatPurgeOperationID(uuid.MustParse(request.GetPurge().GetSpaceId()), uuid.MustParse(request.GetPurge().GetDeletionOperationId()), chatIDs[0]).String(), chatPurger.requests[0].GetOperationId())
	require.NotNil(t, store.completed)
	require.Equal(t, commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, response.GetReceipt().GetParticipantId())
}

func TestPurgeSpaceEmptyManifestStillReturnsExactCompletion(t *testing.T) {
	request := validMessagingSpacePurgeRequest()
	request.Purge.Manifest.ItemCount = 0
	store := &spaceLifecyclePurgeStoreFake{}
	chatPurger := &spaceLifecycleChatPurgerFake{}
	service := &MessagingGRPC{SpaceLifecyclePurger: store, ManagedChatPurger: chatPurger, SpaceFileProducer: testSpaceFileProducer(t)}
	response, err := service.PurgeSpace(messagingSpacePurgeContext(t, request), request)
	require.NoError(t, err)
	require.NotNil(t, response.Receipt)
	require.Empty(t, store.pageCalls)
	require.Empty(t, chatPurger.requests)
	require.True(t, proto.Equal(request, store.completed))
	require.Equal(t, 1, service.SpaceFileProducer.Files.(*producerTestFile).releaseCalls, "empty Space still requires aggregate MESSAGING release")
}

func TestPurgeSpaceCannotCompleteAfterFileProducerReleaseFailure(t *testing.T) {
	request := validMessagingSpacePurgeRequest()
	request.Purge.Manifest.ItemCount = 0
	saved := &spaceLifecyclePurgeStoreFake{}
	producer := testSpaceFileProducer(t)
	producer.Files.(*producerTestFile).releaseErr = errors.New("fixture File unavailable")
	service := &MessagingGRPC{SpaceLifecyclePurger: saved, ManagedChatPurger: &spaceLifecycleChatPurgerFake{}, SpaceFileProducer: producer}
	_, err := service.PurgeSpace(messagingSpacePurgeContext(t, request), request)
	require.Error(t, err)
	require.Nil(t, saved.completed)
	producer.Files.(*producerTestFile).releaseErr = nil
	_, err = service.PurgeSpace(messagingSpacePurgeContext(t, request), request)
	require.NoError(t, err)
	require.NotNil(t, saved.completed)
}

func TestPurgeSpaceDoesNotCompleteWhenAChatPurgeFails(t *testing.T) {
	request := validMessagingSpacePurgeRequest()
	store := &spaceLifecyclePurgeStoreFake{chatIDs: []uuid.UUID{uuid.New(), uuid.New()}}
	chatPurger := &spaceLifecycleChatPurgerFake{err: errors.New("File release pending")}
	service := &MessagingGRPC{SpaceLifecyclePurger: store, ManagedChatPurger: chatPurger, SpaceFileProducer: testSpaceFileProducer(t)}
	_, err := service.PurgeSpace(messagingSpacePurgeContext(t, request), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Len(t, chatPurger.requests, 1)
	require.Nil(t, store.completed)
}

func TestPurgeSpaceRequiresRequestBoundSpacePrincipal(t *testing.T) {
	request := validMessagingSpacePurgeRequest()
	store := &spaceLifecyclePurgeStoreFake{chatIDs: []uuid.UUID{uuid.New(), uuid.New()}}
	chatPurger := &spaceLifecycleChatPurgerFake{}
	service := &MessagingGRPC{SpaceLifecyclePurger: store, ManagedChatPurger: chatPurger, SpaceFileProducer: testSpaceFileProducer(t)}
	wrong := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging",
		RPC: messagingv1.MessagingService_PurgeSpace_FullMethodName,
	})
	_, err := service.PurgeSpace(wrong, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Empty(t, store.pageCalls)
	require.Empty(t, chatPurger.requests)
}

func validMessagingSpacePurgeRequest() *messagingv1.PurgeSpaceRequest {
	return &messagingv1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: "20000000-0000-4000-8000-000000000001",
		DeletionOperationId: "20000000-0000-4000-8000-000000000002", Generation: 9,
		PurgeDecidedAt: timestamppb.New(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)),
		ParticipantId:  commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
		Manifest:       &commonv1.ManifestBinding{ManifestId: "20000000-0000-4000-8000-000000000003", ManifestSha256: make([]byte, 32), ItemCount: 2},
	}}
}

func messagingSpacePurgeContext(t *testing.T, request *messagingv1.PurgeSpaceRequest) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "messaging",
		RPC:       messagingv1.MessagingService_PurgeSpace_FullMethodName,
		RequestID: request.GetPurge().GetDeletionOperationId(), RequestHash: hash,
	})
}

func mustMarshal(message proto.Message) []byte {
	wire, _ := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	return wire
}
