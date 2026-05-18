// Package testchat hosts Chat gRPC over bufconn for cross-module tests (e.g. Messaging).
// It lives outside chat/internal so other modules under voice/backend may import it.
package testchat

import (
	"context"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/chat/internal/chatevents"
	chatgrpc "voice/backend/chat/internal/grpcsvc"
	chatstore "voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

const defaultBufSize = 1 << 20

// ChatDeps configures optional ChatService collaborators (same as production ChatGRPC wiring).
// Zero value is valid for S2S paths that only need DM rows already present (e.g. ListMembers).
type ChatDeps struct {
	Profiles   chatgrpc.UserProfileLookup
	Blocks     chatgrpc.AccountBlockChecker
	ListEnrich chatgrpc.ListChatsEnrichment
	ChatEvents chatevents.Publisher
}

// NewBufconnChatClient returns a ChatService client backed by an in-process server using pool (chat_db migrations applied).
func NewBufconnChatClient(t *testing.T, pool *pgxpool.Pool) (chatv1.ChatServiceClient, func()) {
	t.Helper()
	return NewBufconnChatClientWith(t, pool, ChatDeps{})
}

// NewBufconnChatClientWith is like [NewBufconnChatClient] but wires profiles/blocks/enrichment for CreateDM/GetDM/ListChats.
func NewBufconnChatClientWith(t *testing.T, pool *pgxpool.Pool, deps ChatDeps) (chatv1.ChatServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(defaultBufSize)
	srv := grpc.NewServer()
	chatv1.RegisterChatServiceServer(srv, &chatgrpc.ChatGRPC{
		DM:         &chatstore.DMStore{Pool: pool},
		Profiles:   deps.Profiles,
		Blocks:     deps.Blocks,
		ListEnrich: deps.ListEnrich,
		ChatEvents: deps.ChatEvents,
	})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("chat grpc serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///bufchat",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	cleanup := func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
	return chatv1.NewChatServiceClient(conn), cleanup
}
