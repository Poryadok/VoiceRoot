package s2s

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
)

type chatPermissionServer struct {
	chatv1.UnimplementedChatServiceServer
	seen    metadata.MD
	chat    string
	allowed bool
	calls   int
}

func (s *chatPermissionServer) GetChatPermissions(ctx context.Context, r *chatv1.GetChatPermissionsRequest) (*chatv1.GetChatPermissionsResponse, error) {
	s.seen, _ = metadata.FromIncomingContext(ctx)
	s.chat = r.GetChatId()
	s.calls++
	return &chatv1.GetChatPermissionsResponse{CanPostAsChat: s.allowed}, nil
}

func TestOfficialPermissionAdapterBindsAuthenticatedSubjectAndRequeries(t *testing.T) {
	subject, chat := uuid.New(), uuid.New()
	server := &chatPermissionServer{allowed: true}
	conn, cleanup := startBufconnChat(t, server)
	t.Cleanup(cleanup)
	g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-profile-id", subject.String(), "x-voice-user-id", uuid.New().String(),
		"x-voice-internal-caller", "forged", "authorization", "must-not-forward", "x-unrelated", "private"))
	allowed, err := g.CanPostAsChat(ctx, chat, subject)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, chat.String(), server.chat)
	require.Equal(t, []string{subject.String()}, server.seen.Get("x-voice-profile-id"))
	require.Equal(t, []string{"messaging"}, server.seen.Get("x-voice-internal-caller"))
	require.Empty(t, server.seen.Get("authorization"))
	require.Empty(t, server.seen.Get("x-unrelated"))
	server.allowed = false
	allowed, err = g.CanPostAsChat(ctx, chat, subject)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Equal(t, 2, server.calls)
	_, err = g.CanPostAsChat(ctx, chat, uuid.New())
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, 2, server.calls, "wrong actor must not reach Chat")
	_, err = g.CanPostAsChat(context.Background(), chat, subject)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	duplicate := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", subject.String(), "x-voice-profile-id", subject.String()))
	_, err = g.CanPostAsChat(duplicate, chat, subject)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, 2, server.calls)
}
