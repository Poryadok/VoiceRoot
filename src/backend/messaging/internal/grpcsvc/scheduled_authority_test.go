package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/authctx"
	"voice/backend/messaging/internal/s2s"
	"voice/backend/messaging/internal/store"
)

type scheduledChatAuthorityClient struct {
	chatv1.ChatServiceClient
	err error
}

func (c scheduledChatAuthorityClient) GetChat(context.Context, *chatv1.GetChatRequest, ...grpc.CallOption) (*chatv1.GetChatResponse, error) {
	return nil, c.err
}

type scheduledAllowGuard struct{ ChatGuard }

func (scheduledAllowGuard) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type scheduledErrorResolver struct{ err error }

func (r scheduledErrorResolver) ResolveChatType(context.Context, uuid.UUID, uuid.UUID) (chatv1.ChatType, error) {
	return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, r.err
}

func TestScheduledListPreservesAuthoritativeChatErrors(t *testing.T) {
	tests := []struct {
		name     string
		resolver AuthoritativeChatTypeResolver
		code     codes.Code
	}{
		{"grpc non-member", s2s.NewGRPCChatTypeResolver(scheduledChatAuthorityClient{err: status.Error(codes.PermissionDenied, "private detail")}), codes.PermissionDenied},
		{"grpc missing", s2s.NewGRPCChatTypeResolver(scheduledChatAuthorityClient{err: status.Error(codes.NotFound, "private detail")}), codes.NotFound},
		{"grpc outage", s2s.NewGRPCChatTypeResolver(scheduledChatAuthorityClient{err: status.Error(codes.Unavailable, "private detail")}), codes.Unavailable},
		{"SQL missing", scheduledErrorResolver{pgx.ErrNoRows}, codes.NotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &MessagingGRPC{Messages: &store.MessagesStore{Pool: &pgxpool.Pool{}}, ChatGuard: scheduledAllowGuard{}, ChatTypeResolver: tt.resolver}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, uuid.NewString()))
			_, err := svc.ListScheduledMessages(ctx, &messagingv1.ListScheduledMessagesRequest{Chat: chatDMRef(uuid.New())})
			if status.Code(err) != tt.code {
				t.Fatalf("got %v want %v", err, tt.code)
			}
		})
	}
}
