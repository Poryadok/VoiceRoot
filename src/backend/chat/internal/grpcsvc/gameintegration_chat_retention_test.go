package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
)

type retentionManagedChatTestStore struct {
	added  *store.ManagedChatMemberAddition
	synced *store.ManagedChatMemberSync
}

func (retentionManagedChatTestStore) ProvisionManagedChat(context.Context, store.ManagedChatCreate) (store.ManagedChatCreateResult, error) {
	return store.ManagedChatCreateResult{}, nil
}

func (s *retentionManagedChatTestStore) SyncManagedChatMembers(_ context.Context, request store.ManagedChatMemberSync) (store.ManagedChatMemberSyncResult, error) {
	s.synced = &request
	return store.ManagedChatMemberSyncResult{}, nil
}

func (s *retentionManagedChatTestStore) AddManagedChatMembers(_ context.Context, request store.ManagedChatMemberAddition) (store.ManagedChatMemberSyncResult, error) {
	s.added = &request
	return store.ManagedChatMemberSyncResult{ProfileIDs: request.ProfileIDs, ReceiptID: request.OperationID, RequestHash: request.RequestHash}, nil
}

func (retentionManagedChatTestStore) SetManagedChatRetention(_ context.Context, request store.ManagedChatRetention) (store.ManagedChatRetentionResult, error) {
	return store.ManagedChatRetentionResult{
		ChatID: uuid.MustParse("00000000-0000-4000-8000-000000000001"), ReceiptID: request.OperationID,
		RequestHash: request.RequestHash, PurgeAfter: request.PurgeAfter, CompletedAt: time.Now().UTC(),
	}, nil
}

func TestSetManagedChatRetentionRequiresBoundGISPrincipal(t *testing.T) {
	request := &chatv1.SetManagedChatRetentionRequest{
		ApplicationId:   uuid.NewString(),
		EnvironmentId:   uuid.NewString(),
		OperationId:     uuid.NewString(),
		ExternalChatKey: "match:retention-test",
		PurgeAfter:      timestamppb.New(time.Now().UTC().Add(30 * 24 * time.Hour)),
	}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration",
		Audience: "chat", RPC: "/voice.chat.v1.GameIntegrationChatService/SetManagedChatRetention",
		RequestID: request.GetOperationId(), RequestHash: hash,
	})

	service := &GameIntegrationChatGRPC{Store: &retentionManagedChatTestStore{}}
	response, err := service.SetManagedChatRetention(ctx, request)
	require.NoError(t, err)
	require.NotEmpty(t, response.GetChatId())
	require.Equal(t, request.GetOperationId(), response.GetReceiptId())
	require.Equal(t, hash, response.GetRequestHash())
	require.True(t, response.GetPurgeAfter().AsTime().Equal(request.GetPurgeAfter().AsTime()))

	wrongRPC := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration",
		Audience: "chat", RPC: "/voice.chat.v1.GameIntegrationChatService/ProvisionManagedChat",
		RequestID: request.GetOperationId(), RequestHash: hash,
	})
	_, err = service.SetManagedChatRetention(wrongRPC, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestSyncManagedChatMembersAddOnlyPreservesOperationHashAndDispatchesAdd(t *testing.T) {
	request := &chatv1.SyncManagedChatMembersRequest{
		ApplicationId: uuid.NewString(), EnvironmentId: uuid.NewString(), OperationId: uuid.NewString(),
		ChatId: uuid.NewString(), ProfileIds: []string{uuid.NewString()}, AddOnly: true,
	}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration",
		Audience: "chat", RPC: "/voice.chat.v1.GameIntegrationChatService/SyncManagedChatMembers",
		RequestID: request.GetOperationId(), RequestHash: hash,
	})
	storeStub := &retentionManagedChatTestStore{}
	response, err := (&GameIntegrationChatGRPC{Store: storeStub}).SyncManagedChatMembers(ctx, request)
	require.NoError(t, err)
	require.Nil(t, storeStub.synced)
	require.NotNil(t, storeStub.added)
	require.Equal(t, hash, storeStub.added.RequestHash)
	require.Equal(t, request.GetOperationId(), response.GetReceiptId())
	require.Equal(t, hash, response.GetRequestHash())

	request.AddOnly = false
	hash, err = principal.RequestHash(request)
	require.NoError(t, err)
	ctx = principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration",
		Audience: "chat", RPC: "/voice.chat.v1.GameIntegrationChatService/SyncManagedChatMembers",
		RequestID: request.GetOperationId(), RequestHash: hash,
	})
	_, err = (&GameIntegrationChatGRPC{Store: storeStub}).SyncManagedChatMembers(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, storeStub.synced)
}
