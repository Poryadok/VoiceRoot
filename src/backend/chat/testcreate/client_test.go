package testcreate

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/chatcreation"
)

type creatorService struct {
	chatv1.UnimplementedChatServiceServer
	calls atomic.Int32
}

func (s *creatorService) CreateChat(ctx context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	actor, err := chatcreation.Creator(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "no verified fixture actor")
	}
	s.calls.Add(1)
	return &chatv1.CreateChatResponse{Chat: &chatv1.Chat{CreatorProfileId: actor.ProfileID.String()}}, nil
}

func TestCreateFixtureUsesRealVerifierAndBoundCredential(t *testing.T) {
	service := &creatorService{}
	client, raw, signer, cleanup := newClient(t, service)
	t.Cleanup(cleanup)
	account, profile := uuid.NewString(), uuid.NewString()
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(authctx.HeaderUserID, account, authctx.HeaderProfileID, profile))
	req := &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_CHANNEL, Name: proto.String("signed fixture")}
	response, err := client.CreateChat(ctx, req)
	require.NoError(t, err)
	require.Equal(t, profile, response.GetChat().GetCreatorProfileId())
	require.Equal(t, int32(1), service.calls.Load())

	// Raw fixture headers on the real protected transport do not become authority.
	rawClient := chatv1.NewChatServiceClient(raw)
	_, err = rawClient.CreateChat(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, int32(1), service.calls.Load())

	signed, err := signer.context(ctx, req)
	require.NoError(t, err)
	changed := proto.Clone(req).(*chatv1.CreateChatRequest)
	changed.Name = proto.String("altered after signing")
	_, err = rawClient.CreateChat(signed, changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, int32(1), service.calls.Load())
	_, err = rawClient.CreateChat(signed, req)
	require.NoError(t, err, "body mismatch must not consume the valid credential")
	_, err = rawClient.CreateChat(signed, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, int32(2), service.calls.Load())
}
