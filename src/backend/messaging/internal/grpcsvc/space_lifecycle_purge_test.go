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
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

type spaceLifecyclePurgeStoreFake struct {
	pageCalls     []uint64
	chatIDs       []uuid.UUID
	completed     *messagingv1.PurgeSpaceRequest
	completionErr error
	lookupBytes   []byte
	lookupErr     error
	lookupCalls   []store.SpacePurgeReceiptKey
}

func (f *spaceLifecyclePurgeStoreFake) GetSpacePurgeResponse(_ context.Context, key store.SpacePurgeReceiptKey) ([]byte, error) {
	f.lookupCalls = append(f.lookupCalls, key)
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	if f.lookupBytes == nil {
		return nil, store.ErrSpacePurgeReceiptNotFound
	}
	return append([]byte(nil), f.lookupBytes...), nil
}

func TestPurgeSpaceReplaysCommittedResponseWithoutExternalDependencies(t *testing.T) {
	request := validMessagingSpacePurgeRequest()
	saved := &spaceLifecyclePurgeStoreFake{}
	wire, err := saved.CompleteSpaceLifecyclePurge(context.Background(), request, uuid.NewString())
	require.NoError(t, err)
	want := &messagingv1.PurgeSpaceResponse{}
	require.NoError(t, proto.Unmarshal(wire, want))
	want.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 1})
	want.Receipt.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 2})
	saved.lookupBytes = mustMarshal(want)
	saved.completed = nil
	service := &MessagingGRPC{SpaceLifecyclePurger: saved}

	response, err := service.PurgeSpace(messagingSpacePurgeContext(t, request), request)
	require.NoError(t, err, "lost-response replay must not depend on File or child purgers")
	require.Equal(t, saved.lookupBytes, mustMarshal(response), "retain original receipt and unknown response fields")
	require.Len(t, saved.lookupCalls, 1)
	key := saved.lookupCalls[0]
	require.Equal(t, uuid.MustParse(request.Purge.SpaceId), key.SpaceID)
	require.Equal(t, uuid.MustParse(request.Purge.DeletionOperationId), key.DeletionOperationID)
	require.Equal(t, request.Purge.Generation, key.PurgeGeneration)
	require.Equal(t, request.Purge.Generation-1, key.SourceScheduleGeneration)
	require.Equal(t, want.Receipt.RequestSha256, key.MessagingRequestSHA256)
	require.Empty(t, saved.pageCalls)
	require.Nil(t, saved.completed)
}

func TestPurgeSpaceLookupFailureCannotRestartDestructiveWork(t *testing.T) {
	request := validMessagingSpacePurgeRequest()
	fixture := &spaceLifecyclePurgeStoreFake{}
	wrongBinding, err := fixture.CompleteSpaceLifecyclePurge(context.Background(), request, uuid.NewString())
	require.NoError(t, err)
	response := &messagingv1.PurgeSpaceResponse{}
	require.NoError(t, proto.Unmarshal(wrongBinding, response))
	response.Receipt.RequestSha256[0] ^= 1
	for _, tc := range []struct {
		name string
		err  error
		wire []byte
		code codes.Code
	}{
		{name: "changed binding", err: store.ErrSpacePurgeReceiptBinding, code: codes.FailedPrecondition},
		{name: "storage unavailable", err: errors.New("database unavailable"), code: codes.Unavailable},
		{name: "corrupt receipt", wire: []byte{0xff}, code: codes.Internal},
		{name: "missing receipt", wire: []byte{}, code: codes.Internal},
		{name: "wrong receipt request hash", wire: mustMarshal(response), code: codes.Internal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := validMessagingSpacePurgeRequest()
			saved := &spaceLifecyclePurgeStoreFake{lookupBytes: tc.wire, lookupErr: tc.err}
			producer := testSpaceFileProducer(t)
			chats := &spaceLifecycleChatPurgerFake{}
			service := &MessagingGRPC{SpaceLifecyclePurger: saved, ManagedChatPurger: chats, SpaceFileProducer: producer}
			response, err := service.PurgeSpace(messagingSpacePurgeContext(t, request), request)
			require.Equal(t, tc.code, status.Code(err))
			require.Nil(t, response)
			require.Equal(t, 0, producer.Files.(*producerTestFile).releaseCalls)
			require.Empty(t, chats.requests)
			require.Empty(t, saved.pageCalls)
			require.Nil(t, saved.completed)
		})
	}
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
	require.Empty(t, store.lookupCalls, "untrusted caller must not reach committed receipt lookup")
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
