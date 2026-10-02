package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
)

func TestSpacePurgeLooksUpMessagingThenReleasesChatReferencesBeforeLocalPurge(t *testing.T) {
	spaceID, operationID := "cfc6d381-7b5a-4f18-8721-67d5beaf4010", "738e8ea3-1d37-4fa7-9fa1-bdafd6f53c52"
	purge := &commonv1.SpacePurgeRequest{
		ProtocolVersion: 1, SpaceId: spaceID, DeletionOperationId: operationID, Generation: 2,
		PurgeDecidedAt: timestamppb.New(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)),
		ParticipantId:  commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
		Manifest:       &commonv1.ManifestBinding{ManifestId: operationID, ManifestSha256: make([]byte, 32), ItemCount: 3},
	}
	messagingPurge := proto.Clone(purge).(*commonv1.SpacePurgeRequest)
	messagingPurge.ParticipantId = commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING
	messagingRequest := &messagingv1.PurgeSpaceRequest{Purge: messagingPurge}
	messageEvents := []string{}
	messaging := &spacePurgeMessagingFake{events: &messageEvents, receipt: &commonv1.SpacePurgeReceipt{
		ProtocolVersion: 1, ReceiptId: "message-receipt", SpaceId: spaceID, DeletionOperationId: operationID, Generation: 2,
		ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
		State:         commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED,
		RequestSha256: lifecycleDomainDigest("voice.messaging.v1.PurgeSpaceRequest", messagingRequest), CompletedAt: timestamppb.Now(),
	}}
	files := &spacePurgeFileFake{events: &messageEvents}
	local := &spacePurgeStoreFake{events: &messageEvents}
	issuer, publicKey := testSpacePurgeIssuer(t)
	service := &SpaceLifecycleGRPC{Store: local, Issuer: issuer, Messaging: messaging, File: files}
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "chat", RPC: chatv1.ChatService_PurgeSpace_FullMethodName})

	response, err := service.PurgeSpace(ctx, &chatv1.PurgeSpaceRequest{Purge: purge})
	require.NoError(t, err)
	require.NotNil(t, response.GetReceipt())
	require.Equal(t, []string{"chat-preflight", "messaging", "file", "chat-store"}, messageEvents)
	require.Equal(t, operationID, messaging.request.GetDeletionOperationId())
	require.Equal(t, uint64(1), messaging.request.GetSourceScheduleGeneration())
	require.Equal(t, filev1.FileReferenceProducerId_FILE_REFERENCE_PRODUCER_ID_CHAT, files.request.GetProducerId())
	require.Zero(t, files.request.GetPurgeGeneration()-files.request.GetSourceScheduleGeneration()-1)
	require.NotNil(t, local.evidence.MessagingReceipt)
	require.NotNil(t, local.evidence.FileReleaseReceipt)
	for _, outbound := range []struct {
		ctx context.Context
		audience, method string
		request proto.Message
	}{{messaging.ctx, "messaging", messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName, messaging.request},
		{files.ctx, "file", filev1.FileService_ReleaseSpaceDeletionProducerReferences_FullMethodName, files.request}} {
		callContext := outbound.ctx
		deadline, ok := callContext.Deadline()
		require.True(t, ok)
		require.True(t, time.Until(deadline) <= 10*time.Second)
		md, ok := metadata.FromOutgoingContext(callContext)
		require.True(t, ok)
		require.Equal(t, []string{operationID}, md.Get("x-request-id"))
		require.Len(t, md.Get("authorization"), 1)
		require.True(t, strings.HasPrefix(md.Get("authorization")[0], "Bearer "))
		requestHash, err := principal.RequestHash(outbound.request)
		require.NoError(t, err)
		_, err = principal.VerifyService(context.Background(), strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{
			ExpectedIssuer: "chat", ExpectedAudience: outbound.audience, ExpectedRPC: outbound.method,
			ExpectedRequestID: operationID, ExpectedRequestHash: requestHash,
			KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &publicKey, nil },
		})
		require.NoError(t, err)
	}
}

func TestSpacePurgeStopsBeforeFileWhenMessagingReceiptIsMissing(t *testing.T) {
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: "cfc6d381-7b5a-4f18-8721-67d5beaf4010", DeletionOperationId: "738e8ea3-1d37-4fa7-9fa1-bdafd6f53c52", Generation: 2,
		PurgeDecidedAt: timestamppb.Now(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
		Manifest: &commonv1.ManifestBinding{ManifestId: "738e8ea3-1d37-4fa7-9fa1-bdafd6f53c52", ManifestSha256: make([]byte, 32)}}
	events := []string{}
	files := &spacePurgeFileFake{events: &events}
	local := &spacePurgeStoreFake{events: &events}
	service := &SpaceLifecycleGRPC{Store: local, Issuer: mustTestSpacePurgeIssuer(t), Messaging: &spacePurgeMessagingFake{events: &events}, File: files}
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "chat", RPC: chatv1.ChatService_PurgeSpace_FullMethodName})
	_, err := service.PurgeSpace(ctx, &chatv1.PurgeSpaceRequest{Purge: purge})
	require.Error(t, err)
	require.Equal(t, []string{"chat-preflight", "messaging"}, events)
	require.False(t, local.called)
}

func TestSpacePurgeRejectsMismatchedMessagingReceiptBeforeFileRelease(t *testing.T) {
	spaceID, operationID := "cfc6d381-7b5a-4f18-8721-67d5beaf4010", "738e8ea3-1d37-4fa7-9fa1-bdafd6f53c52"
	purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: spaceID, DeletionOperationId: operationID, Generation: 2,
		PurgeDecidedAt: timestamppb.Now(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_CHAT,
		Manifest: &commonv1.ManifestBinding{ManifestId: operationID, ManifestSha256: make([]byte, 32)}}
	events := []string{}
	files := &spacePurgeFileFake{events: &events}
	local := &spacePurgeStoreFake{events: &events}
	badReceipt := &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: "receipt", SpaceId: spaceID,
		DeletionOperationId: operationID, Generation: 3, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
		State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: make([]byte, 32), CompletedAt: timestamppb.Now()}
	service := &SpaceLifecycleGRPC{Store: local, Issuer: mustTestSpacePurgeIssuer(t), Messaging: &spacePurgeMessagingFake{events: &events, receipt: badReceipt}, File: files}
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "chat", RPC: chatv1.ChatService_PurgeSpace_FullMethodName})
	_, err := service.PurgeSpace(ctx, &chatv1.PurgeSpaceRequest{Purge: purge})
	require.Error(t, err)
	require.Equal(t, []string{"chat-preflight", "messaging"}, events)
	require.False(t, local.called)
}

type spacePurgeMessagingFake struct {
	events  *[]string
	receipt *commonv1.SpacePurgeReceipt
	request *messagingv1.GetSpacePurgeReceiptRequest
	ctx     context.Context
}

func (f *spacePurgeMessagingFake) GetSpacePurgeReceipt(ctx context.Context, req *messagingv1.GetSpacePurgeReceiptRequest, _ ...grpc.CallOption) (*messagingv1.GetSpacePurgeReceiptResponse, error) {
	*f.events = append(*f.events, "messaging")
	f.ctx, f.request = ctx, req
	if f.receipt == nil {
		return nil, nil
	}
	return &messagingv1.GetSpacePurgeReceiptResponse{Receipt: f.receipt}, nil
}

type spacePurgeFileFake struct {
	events  *[]string
	request *filev1.ReleaseSpaceDeletionProducerReferencesRequest
	ctx     context.Context
}

func (f *spacePurgeFileFake) ReleaseSpaceDeletionProducerReferences(ctx context.Context, req *filev1.ReleaseSpaceDeletionProducerReferencesRequest, _ ...grpc.CallOption) (*filev1.ReleaseSpaceDeletionProducerReferencesResponse, error) {
	*f.events = append(*f.events, "file")
	f.ctx, f.request = ctx, req
	return &filev1.ReleaseSpaceDeletionProducerReferencesResponse{Receipt: &filev1.ReleaseSpaceDeletionProducerReferencesReceipt{
		ProtocolVersion: 1, ReceiptId: "file-receipt", SpaceId: req.GetSpaceId(), DeletionOperationId: req.GetDeletionOperationId(),
		PurgeGeneration: req.GetPurgeGeneration(), SourceScheduleGeneration: req.GetSourceScheduleGeneration(), ProducerId: req.GetProducerId(),
		ExpectedReferencesSha256: req.GetExpectedReferencesSha256(), RequestSha256: lifecycleDomainDigest("voice.file.v1.ReleaseSpaceDeletionProducerReferencesRequest", req),
		CompletedAt: timestamppb.Now(),
	}}, nil
}

type spacePurgeStoreFake struct {
	events   *[]string
	called   bool
	evidence store.SpacePurgeOwnerEvidence
}

func (f *spacePurgeStoreFake) PrepareSpaceDeletionManifest(context.Context, *chatv1.PrepareSpaceDeletionManifestRequest) (*chatv1.PrepareSpaceDeletionManifestResponse, error) {
	panic("unused")
}
func (f *spacePurgeStoreFake) ApplySpaceLifecycleFence(context.Context, *chatv1.ApplySpaceLifecycleFenceRequest) (*chatv1.ApplySpaceLifecycleFenceResponse, error) {
	panic("unused")
}
func (f *spacePurgeStoreFake) GetSpacePurgeManifestPage(context.Context, *chatv1.GetSpacePurgeManifestPageRequest) (*chatv1.GetSpacePurgeManifestPageResponse, error) {
	panic("unused")
}
func (f *spacePurgeStoreFake) ValidateSpacePurgeRequest(context.Context, *chatv1.PurgeSpaceRequest) error {
	*f.events = append(*f.events, "chat-preflight")
	return nil
}
func (f *spacePurgeStoreFake) PurgeSpace(_ context.Context, _ *chatv1.PurgeSpaceRequest, evidence store.SpacePurgeOwnerEvidence) (*chatv1.PurgeSpaceResponse, error) {
	*f.events = append(*f.events, "chat-store")
	f.called, f.evidence = true, evidence
	return &chatv1.PurgeSpaceResponse{Receipt: &commonv1.SpacePurgeReceipt{ReceiptId: "chat-receipt"}}, nil
}

func mustTestSpacePurgeIssuer(t *testing.T) *principal.Issuer {
	issuer, _ := testSpacePurgeIssuer(t)
	return issuer
}

func testSpacePurgeIssuer(t *testing.T) (*principal.Issuer, rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	return issuer, key.PublicKey
}
