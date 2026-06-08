package s2s

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/messaging/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

type stubChatListMembers struct {
	chatv1.UnimplementedChatServiceServer
	members []*chatv1.ChatMember
	err     error
}

func (s *stubChatListMembers) ListMembers(_ context.Context, _ *chatv1.ListMembersRequest) (*chatv1.ListMembersResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &chatv1.ListMembersResponse{
		MemberList: &chatv1.MemberList{Members: s.members},
	}, nil
}

func startBufconnChat(t *testing.T, impl chatv1.ChatServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	chatv1.RegisterChatServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

func TestGRPCChatGuard_EnsureMember(t *testing.T) {
	t.Parallel()

	chatID := uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	self := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	peer := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")

	t.Run("nil client", func(t *testing.T) {
		t.Parallel()
		var g *GRPCChatGuard
		err := g.EnsureMember(context.Background(), chatID, self)
		require.Error(t, err)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("member ok", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnChat(t, &stubChatListMembers{members: []*chatv1.ChatMember{
			{ProfileId: self.String()},
			{ProfileId: peer.String()},
		}})
		t.Cleanup(cleanup)
		g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))
		require.NoError(t, g.EnsureMember(context.Background(), chatID, self))
	})

	t.Run("not member", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnChat(t, &stubChatListMembers{members: []*chatv1.ChatMember{
			{ProfileId: peer.String()},
		}})
		t.Cleanup(cleanup)
		g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))
		err := g.EnsureMember(context.Background(), chatID, self)
		require.ErrorIs(t, err, store.ErrNotChatMember)
	})

	t.Run("permission denied maps to not member", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnChat(t, &stubChatListMembers{
			err: status.Error(codes.PermissionDenied, "denied"),
		})
		t.Cleanup(cleanup)
		g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))
		err := g.EnsureMember(context.Background(), chatID, self)
		require.ErrorIs(t, err, store.ErrNotChatMember)
	})
}

func TestGRPCChatGuard_DMOtherProfileID(t *testing.T) {
	t.Parallel()

	chatID := uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	self := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	peer := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")

	conn, cleanup := startBufconnChat(t, &stubChatListMembers{members: []*chatv1.ChatMember{
		{ProfileId: self.String()},
		{ProfileId: peer.String()},
	}})
	t.Cleanup(cleanup)
	g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))

	got, err := g.DMOtherProfileID(context.Background(), chatID, self)
	require.NoError(t, err)
	require.Equal(t, peer, got)

	_, err = g.DMOtherProfileID(context.Background(), chatID, peer)
	require.NoError(t, err)

	err = g.EnsureMember(context.Background(), chatID, uuid.New())
	require.ErrorIs(t, err, store.ErrNotChatMember)
}

func TestGrpcMemberErr_nonStatus(t *testing.T) {
	t.Parallel()
	require.Equal(t, errors.New("plain"), grpcMemberErr(errors.New("plain")))
}

func TestGRPCChatGuard_DMOtherProfileID_edgeCases(t *testing.T) {
	t.Parallel()

	chatID := uuid.MustParse("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	self := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	peer := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	extra := uuid.MustParse("dddddddd-dddd-4ddd-8ddd-dddddddddddd")

	t.Run("not found maps to not member", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnChat(t, &stubChatListMembers{
			err: status.Error(codes.NotFound, "missing chat"),
		})
		t.Cleanup(cleanup)
		g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))
		err := g.EnsureMember(context.Background(), chatID, self)
		require.ErrorIs(t, err, store.ErrNotChatMember)
	})

	t.Run("caller absent", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnChat(t, &stubChatListMembers{members: []*chatv1.ChatMember{
			{ProfileId: peer.String()},
		}})
		t.Cleanup(cleanup)
		g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))
		_, err := g.DMOtherProfileID(context.Background(), chatID, self)
		require.ErrorIs(t, err, store.ErrNotChatMember)
	})

	t.Run("single member", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnChat(t, &stubChatListMembers{members: []*chatv1.ChatMember{
			{ProfileId: self.String()},
		}})
		t.Cleanup(cleanup)
		g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))
		_, err := g.DMOtherProfileID(context.Background(), chatID, self)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("three members", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnChat(t, &stubChatListMembers{members: []*chatv1.ChatMember{
			{ProfileId: self.String()},
			{ProfileId: peer.String()},
			{ProfileId: extra.String()},
		}})
		t.Cleanup(cleanup)
		g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))
		_, err := g.DMOtherProfileID(context.Background(), chatID, self)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("invalid profile id", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnChat(t, &stubChatListMembers{members: []*chatv1.ChatMember{
			{ProfileId: self.String()},
			{ProfileId: "not-a-uuid"},
		}})
		t.Cleanup(cleanup)
		g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))
		_, err := g.DMOtherProfileID(context.Background(), chatID, self)
		require.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("list members transport error", func(t *testing.T) {
		t.Parallel()
		conn, cleanup := startBufconnChat(t, &stubChatListMembers{
			err: status.Error(codes.Unavailable, "chat down"),
		})
		t.Cleanup(cleanup)
		g := NewGRPCChatGuard(chatv1.NewChatServiceClient(conn))
		_, err := g.DMOtherProfileID(context.Background(), chatID, self)
		require.Equal(t, codes.Unavailable, status.Code(err))
	})
}
