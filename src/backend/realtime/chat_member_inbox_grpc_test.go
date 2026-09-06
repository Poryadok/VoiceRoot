package main

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	chatv1 "voice.app/voice/chat/v1"
)

type pagedMemberServer struct {
	chatv1.UnimplementedChatServiceServer
	requests []*chatv1.ListMembersRequest
}

func (s *pagedMemberServer) ListMembers(_ context.Context, request *chatv1.ListMembersRequest) (*chatv1.ListMembersResponse, error) {
	s.requests = append(s.requests, request)
	members := make([]*chatv1.ChatMember, 0, 100)
	if request.GetPage().GetCursor() == "next" {
		members = append(members, &chatv1.ChatMember{ProfileId: "archived", IsArchived: true})
		return &chatv1.ListMembersResponse{MemberList: &chatv1.MemberList{Members: members}}, nil
	}
	for i := 0; i < 100; i++ {
		members = append(members, &chatv1.ChatMember{ProfileId: fmt.Sprintf("member-%03d", i)})
	}
	return &chatv1.ListMembersResponse{MemberList: &chatv1.MemberList{Members: members, NextCursor: "next"}}, nil
}

func TestGRPCChatMemberInboxListerPagesAllMembers(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	server := &pagedMemberServer{}
	grpcServer := grpc.NewServer()
	chatv1.RegisterChatServiceServer(grpcServer, server)
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.NewClient("bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithInsecure())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	lister := newGRPCChatMemberInboxLister(conn)
	states, err := lister.RecipientDeliveryStates(context.Background(), "chat-1")
	require.NoError(t, err)
	require.Len(t, server.requests, 2)
	require.EqualValues(t, 100, server.requests[0].GetPage().GetPageSize())
	require.Empty(t, server.requests[0].GetPage().GetCursor())
	require.Equal(t, "next", server.requests[1].GetPage().GetCursor())
	require.True(t, states["archived"].IsArchived)
	require.Len(t, states, 101)
}
