package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"

	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type spacePurgeReceiptLookupFunc func(context.Context, store.SpacePurgeReceiptKey) (*commonv1.SpacePurgeReceipt, error)

func (f spacePurgeReceiptLookupFunc) GetSpacePurgeReceipt(ctx context.Context, key store.SpacePurgeReceiptKey) (*commonv1.SpacePurgeReceipt, error) {
	return f(ctx, key)
}

func TestGetSpacePurgeReceiptRequiresExactChatPrincipalAndReturnsCommittedReceipt(t *testing.T) {
	request := validSpacePurgeReceiptRequest()
	requestHash, err := principal.RequestHash(request)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging",
		RPC:       messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName,
		RequestID: uuid.NewString(), RequestHash: requestHash,
	})
	called := false
	service := &MessagingGRPC{SpacePurgeReceipts: spacePurgeReceiptLookupFunc(func(_ context.Context, key store.SpacePurgeReceiptKey) (*commonv1.SpacePurgeReceipt, error) {
		called = true
		require.Equal(t, uuid.MustParse(request.GetSpaceId()), key.SpaceID)
		require.Equal(t, uuid.MustParse(request.GetDeletionOperationId()), key.DeletionOperationID)
		require.EqualValues(t, 8, key.PurgeGeneration)
		require.EqualValues(t, 7, key.SourceScheduleGeneration)
		require.Equal(t, request.GetMessagingRequestSha256(), key.MessagingRequestSHA256)
		return &commonv1.SpacePurgeReceipt{
			ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: request.GetSpaceId(),
			DeletionOperationId: request.GetDeletionOperationId(), Generation: request.GetPurgeGeneration(),
			ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
			State:         commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED,
			RequestSha256: request.GetMessagingRequestSha256(), CompletedAt: timestamppb.New(time.Now().UTC()),
		}, nil
	})}
	response, err := service.GetSpacePurgeReceipt(ctx, request)
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, request.GetSpaceId(), response.GetReceipt().GetSpaceId())
}

func TestGetSpacePurgeReceiptRejectsUntrustedAndMismatchedEvidence(t *testing.T) {
	request := validSpacePurgeReceiptRequest()
	requestHash, err := principal.RequestHash(request)
	require.NoError(t, err)
	valid := principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging", RPC: messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName, RequestID: "lookup-1", RequestHash: requestHash}
	for name, identity := range map[string]principal.Principal{
		"wrong issuer":   {Kind: "service", Issuer: "space", Subject: "service:space", Audience: "messaging", RPC: valid.RPC, RequestID: valid.RequestID, RequestHash: requestHash},
		"wrong audience": {Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "file", RPC: valid.RPC, RequestID: valid.RequestID, RequestHash: requestHash},
		"wrong RPC":      {Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging", RPC: messagingv1.MessagingService_PurgeSpace_FullMethodName, RequestID: valid.RequestID, RequestHash: requestHash},
		"wrong hash":     {Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging", RPC: valid.RPC, RequestID: valid.RequestID, RequestHash: "sha256:" + string(make([]byte, 64))},
	} {
		t.Run(name, func(t *testing.T) {
			service := &MessagingGRPC{SpacePurgeReceipts: spacePurgeReceiptLookupFunc(func(context.Context, store.SpacePurgeReceiptKey) (*commonv1.SpacePurgeReceipt, error) {
				t.Fatal("untrusted call reached receipt store")
				return nil, nil
			})}
			_, err := service.GetSpacePurgeReceipt(principal.WithVerified(context.Background(), identity), request)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}

	service := &MessagingGRPC{SpacePurgeReceipts: spacePurgeReceiptLookupFunc(func(context.Context, store.SpacePurgeReceiptKey) (*commonv1.SpacePurgeReceipt, error) {
		return nil, store.ErrSpacePurgeReceiptBinding
	})}
	_, err = service.GetSpacePurgeReceipt(principal.WithVerified(context.Background(), valid), request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestGetSpacePurgeReceiptValidatesRequestAndStoredParticipant(t *testing.T) {
	request := validSpacePurgeReceiptRequest()
	requestHash, err := principal.RequestHash(request)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging", RPC: messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName, RequestID: "lookup-2", RequestHash: requestHash})
	service := &MessagingGRPC{SpacePurgeReceipts: spacePurgeReceiptLookupFunc(func(context.Context, store.SpacePurgeReceiptKey) (*commonv1.SpacePurgeReceipt, error) {
		return &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, SpaceId: request.GetSpaceId(), DeletionOperationId: request.GetDeletionOperationId(), Generation: 8, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_FILE, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: request.GetMessagingRequestSha256(), CompletedAt: timestamppb.Now()}, nil
	})}
	_, err = service.GetSpacePurgeReceipt(ctx, request)
	require.Equal(t, codes.Unavailable, status.Code(err))

	badGeneration := validSpacePurgeReceiptRequest()
	badGeneration.PurgeGeneration = 7
	_, err = service.GetSpacePurgeReceipt(ctx, badGeneration)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "principal is bound to the original deterministic request")

	validRequest := validSpacePurgeReceiptRequest()
	validRequest.SpaceId = "not-a-uuid"
	validHash, err := principal.RequestHash(validRequest)
	require.NoError(t, err)
	validCtx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging", RPC: messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName, RequestID: "lookup-3", RequestHash: validHash})
	_, err = service.GetSpacePurgeReceipt(validCtx, validRequest)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	service.SpacePurgeReceipts = spacePurgeReceiptLookupFunc(func(context.Context, store.SpacePurgeReceiptKey) (*commonv1.SpacePurgeReceipt, error) {
		return nil, errors.New("db down")
	})
	validRequest = validSpacePurgeReceiptRequest()
	validHash, err = principal.RequestHash(validRequest)
	require.NoError(t, err)
	validCtx = principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "messaging", RPC: messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName, RequestID: "lookup-4", RequestHash: validHash})
	_, err = service.GetSpacePurgeReceipt(validCtx, validRequest)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func validSpacePurgeReceiptRequest() *messagingv1.GetSpacePurgeReceiptRequest {
	return &messagingv1.GetSpacePurgeReceiptRequest{
		SpaceId: "20000000-0000-4000-8000-000000000001", DeletionOperationId: "20000000-0000-4000-8000-000000000002",
		PurgeGeneration: 8, SourceScheduleGeneration: 7, MessagingRequestSha256: make([]byte, 32),
	}
}
