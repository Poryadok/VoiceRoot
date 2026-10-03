package squad

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/matchmaking/internal/authctx"
)

type squadIdentityServer struct {
	chatv1.UnimplementedChatServiceServer
	profile, account, other uuid.UUID
	calls                   atomic.Int32
}

type squadVoiceIdentityServer struct {
	callsv1.UnimplementedVoiceServiceServer
	owner *squadIdentityServer
}

func (s *squadIdentityServer) check(ctx context.Context) error {
	s.calls.Add(1)
	md, _ := metadata.FromIncomingContext(ctx)
	if len(md.Get(authctx.HeaderProfileID)) != 1 || len(md.Get(authctx.HeaderAccountID)) != 1 || md.Get(authctx.HeaderProfileID)[0] != s.profile.String() || md.Get(authctx.HeaderAccountID)[0] != s.account.String() {
		return status.Error(codes.PermissionDenied, "authenticated account does not own voice profile")
	}
	if len(md.Get("x-voice-session-epoch")) != 1 || md.Get("x-voice-session-epoch")[0] != "7" {
		return status.Error(codes.PermissionDenied, "session epoch changed")
	}
	return nil
}

func (s *squadIdentityServer) CreateChat(ctx context.Context, _ *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	if err := s.check(ctx); err != nil {
		return nil, err
	}
	return &chatv1.CreateChatResponse{Chat: &chatv1.Chat{Id: "match-chat"}}, nil
}

func (s *squadIdentityServer) AddMembers(ctx context.Context, request *chatv1.AddMembersRequest) (*chatv1.AddMembersResponse, error) {
	if err := s.check(ctx); err != nil {
		return nil, err
	}
	if len(request.ProfileIds) != 1 || request.ProfileIds[0] != s.other.String() {
		return nil, status.Error(codes.InvalidArgument, "all other match participants must be added exactly once")
	}
	return &chatv1.AddMembersResponse{}, nil
}

func (s *squadVoiceIdentityServer) StartCall(ctx context.Context, request *callsv1.StartCallRequest) (*callsv1.StartCallResponse, error) {
	if err := s.owner.check(ctx); err != nil {
		return nil, err
	}
	if request.GetLinkedChat().GetId() != "match-chat" || request.GetRoomType() != "group_voice" {
		return nil, status.Error(codes.InvalidArgument, "squad room must use its match chat")
	}
	return &callsv1.StartCallResponse{CallSession: &callsv1.CallSession{RoomId: "match-room"}}, nil
}

func TestSquadRPCsPreserveAuthenticatedParticipantIdentity(t *testing.T) {
	first, accepter, account := uuid.New(), uuid.New(), uuid.New()
	owner := &squadIdentityServer{profile: accepter, account: account, other: first}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	chatv1.RegisterChatServiceServer(server, owner)
	callsv1.RegisterVoiceServiceServer(server, &squadVoiceIdentityServer{owner: owner})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///squad", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	provisioner := &Provisioner{Chat: &GRPCChatClient{Client: chatv1.NewChatServiceClient(conn)}, Voice: &GRPCVoiceClient{Client: callsv1.NewVoiceServiceClient(conn)}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, accepter.String(), authctx.HeaderAccountID, account.String(), "x-voice-session-epoch", "7"))
	room, chat, err := provisioner.Provision(ctx, uuid.New(), []uuid.UUID{first, accepter})
	require.NoError(t, err)
	require.Equal(t, "match-room", room)
	require.Equal(t, "match-chat", chat)
	require.EqualValues(t, 3, owner.calls.Load())

	for _, invalid := range []context.Context{
		context.Background(),
		metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, uuid.NewString(), authctx.HeaderAccountID, account.String())),
		metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, accepter.String(), authctx.HeaderProfileID, first.String(), authctx.HeaderAccountID, account.String())),
	} {
		_, _, err := provisioner.Provision(invalid, uuid.New(), []uuid.UUID{first, accepter})
		require.Error(t, err)
		require.EqualValues(t, 3, owner.calls.Load(), "untrusted or nonparticipant callers must not create Chat/Voice resources")
	}
}
