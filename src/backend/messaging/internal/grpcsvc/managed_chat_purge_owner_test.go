package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"
)

type purgeStoreStub struct {
	work      *store.ManagedChatPurgeWork
	completed *store.ManagedChatPurgeWork
	completeN int
}

func (s *purgeStoreStub) StartManagedChatPurge(_ context.Context, operationID, chatID uuid.UUID, cutoff time.Time, hash []byte) (*store.ManagedChatPurgeWork, error) {
	if s.completed != nil {
		replayed := *s.completed
		replayed.PurgeAfter = cutoff
		return &replayed, nil
	}
	if s.work == nil {
		s.work = &store.ManagedChatPurgeWork{OperationID: operationID, ChatID: chatID, PurgeAfter: cutoff, RequestSHA256: append([]byte(nil), hash...), State: "PENDING", MessageIDs: []uuid.UUID{uuid.MustParse("00000000-0000-4000-8000-000000000002")}, MessageAttachments: []store.ManagedChatPurgeMessage{{ID: uuid.MustParse("00000000-0000-4000-8000-000000000002"), FileIDs: []uuid.UUID{uuid.MustParse("00000000-0000-4000-8000-000000000003")}}}}
	}
	return s.work, nil
}

func (s *purgeStoreStub) RequireManagedChatPurgeEventsPublished(context.Context, uuid.UUID) error {
	return nil
}

func (s *purgeStoreStub) CompleteManagedChatPurge(_ context.Context, operationID uuid.UUID, fileHash, searchHash []byte) (*store.ManagedChatPurgeWork, error) {
	s.completeN++
	s.completed = &store.ManagedChatPurgeWork{OperationID: operationID, ChatID: s.work.ChatID, PurgeAfter: s.work.PurgeAfter, RequestSHA256: append([]byte(nil), s.work.RequestSHA256...), State: "COMPLETED", FileReceiptSHA256: append([]byte(nil), fileHash...), SearchReceiptSHA256: append([]byte(nil), searchHash...), CompletedAt: ptrTime(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)), MessageIDs: s.work.MessageIDs, MessageAttachments: s.work.MessageAttachments}
	// Model the timestamp precision of the real PostgreSQL completion read.
	s.completed.PurgeAfter = s.completed.PurgeAfter.Truncate(time.Microsecond)
	return s.completed, nil
}

type purgeFileOwnerStub struct {
	release *filev1.ReleaseFileReferencesRequest
	status  *filev1.GetFileReferenceGCStatusRequest
	pending bool
	statusN int
}

func (f *purgeFileOwnerStub) ReleaseFileReferences(_ context.Context, req *filev1.ReleaseFileReferencesRequest, _ ...grpc.CallOption) (*filev1.ReleaseFileReferencesResponse, error) {
	f.release = proto.Clone(req).(*filev1.ReleaseFileReferencesRequest)
	return &filev1.ReleaseFileReferencesResponse{Receipt: &filev1.ReleaseFileReferencesReceipt{ProtocolVersion: 1, ReceiptId: req.GetOperationId(), OperationId: req.GetOperationId(), ProducerId: req.GetProducerId(), ReleasedCount: uint64(len(req.GetReferences())), RequestSha256: lifecycleHash(req), CompletedAt: timestamppb.Now()}}, nil
}

func (f *purgeFileOwnerStub) GetFileReferenceGCStatus(_ context.Context, req *filev1.GetFileReferenceGCStatusRequest, _ ...grpc.CallOption) (*filev1.GetFileReferenceGCStatusResponse, error) {
	f.statusN++
	f.status = proto.Clone(req).(*filev1.GetFileReferenceGCStatusRequest)
	if f.pending {
		return &filev1.GetFileReferenceGCStatusResponse{OperationId: req.GetOperationId(), ProducerId: req.GetProducerId(), ReleaseRequestSha256: req.GetReleaseRequestSha256(), References: []*filev1.FileReferenceGCStatus{{Reference: req.GetReferences()[0], State: filev1.FileReferenceGCState_FILE_REFERENCE_GC_STATE_PENDING}}}, nil
	}
	return &filev1.GetFileReferenceGCStatusResponse{ReceiptId: "file-gc-receipt", OperationId: req.GetOperationId(), ProducerId: req.GetProducerId(), ReleaseRequestSha256: req.GetReleaseRequestSha256(), RequestSha256: lifecycleHash(req), References: []*filev1.FileReferenceGCStatus{{Reference: req.GetReferences()[0], State: filev1.FileReferenceGCState_FILE_REFERENCE_GC_STATE_GC_COMPLETE}}, CompletedAt: timestamppb.Now()}, nil
}

type purgeSearchOwnerStub struct {
	request *searchv1.PurgeManagedChatMessagesRequest
	calls   int
}

func (s *purgeSearchOwnerStub) PurgeManagedChatMessages(_ context.Context, req *searchv1.PurgeManagedChatMessagesRequest, _ ...grpc.CallOption) (*searchv1.PurgeManagedChatMessagesResponse, error) {
	s.calls++
	s.request = proto.Clone(req).(*searchv1.PurgeManagedChatMessagesRequest)
	hash, _ := principal.RequestHash(req)
	decoded, _ := hex.DecodeString(hash[7:])
	h := sha256.New()
	_, _ = h.Write([]byte("voice.search.managed_chat_message_ids.v1\x00"))
	for _, raw := range req.GetMessageIds() {
		id := uuid.MustParse(raw)
		_, _ = h.Write(id[:])
	}
	return &searchv1.PurgeManagedChatMessagesResponse{ReceiptId: "search-receipt", OperationId: req.GetOperationId(), ChatId: req.GetChatId(), DeletedCount: uint64(len(req.GetMessageIds())), MessageIdsSha256: h.Sum(nil), RequestSha256: decoded, CompletedAt: timestamppb.Now()}, nil
}

func TestManagedChatPurgeCoordinatorWaitsForFileGCThenVerifiesBothOwnerReceipts(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	request := &messagingv1.PurgeManagedChatContentRequest{OperationId: uuid.NewString(), ChatId: uuid.NewString(), PurgeAfter: timestamppb.New(time.Date(2026, 9, 29, 12, 0, 0, 123, time.UTC))}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration", Audience: "messaging", RPC: messagingv1.MessagingService_PurgeManagedChatContent_FullMethodName, RequestID: request.GetOperationId(), RequestHash: hash})
	files := &purgeFileOwnerStub{pending: true}
	search := &purgeSearchOwnerStub{}
	owner := &ManagedChatPurgeCoordinator{Store: &purgeStoreStub{}, Files: files, Search: search, Issuer: issuer}
	_, err = owner.PurgeManagedChatContent(ctx, request)
	require.Error(t, err, "pending File GC must keep the work item retryable")
	require.Zero(t, search.calls, "Search is not purged until File release has terminal evidence")
	require.Len(t, files.release.GetReferences(), 1)
	require.Equal(t, files.release.GetReferences()[0].GetOwnerId(), "00000000-0000-4000-8000-000000000002")
	require.Equal(t, filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, files.release.GetReferences()[0].GetOwnerType())

	files.pending = false
	response, err := owner.PurgeManagedChatContent(ctx, request)
	require.NoError(t, err)
	require.Equal(t, request.GetOperationId(), response.GetOperationId())
	require.True(t, proto.Equal(request.PurgeAfter, response.PurgeAfter), "first receipt retains the exact signed cutoff")
	require.True(t, validManagedChatPurgeReceipt(response, request), "Space adapter must accept the real T33 coordinator digest")
	wrongDigest := proto.Clone(response).(*messagingv1.PurgeManagedChatContentResponse)
	wrongDigest.RequestSha256 = domainSHA(string(request.ProtoReflect().Descriptor().FullName()), mustMarshal(request))
	require.False(t, validManagedChatPurgeReceipt(wrongDigest, request), "P3 parent digest cannot substitute for the T33 child digest")
	require.Len(t, response.GetFileReceiptSha256(), 32)
	require.Len(t, response.GetSearchReceiptSha256(), 32)
	require.Equal(t, []string{"00000000-0000-4000-8000-000000000002"}, search.request.GetMessageIds())
	require.Equal(t, 1, search.calls)
	_, err = owner.PurgeManagedChatContent(ctx, request)
	require.NoError(t, err)
	require.Equal(t, 1, search.calls, "completed operation replay uses immutable owner evidence")
}

func ptrTime(value time.Time) *time.Time { return &value }
