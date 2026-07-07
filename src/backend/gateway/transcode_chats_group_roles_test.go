package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
)

type recordingChatsListMembers struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.ListMembersRequest
}

func (s *recordingChatsListMembers) ListMembers(_ context.Context, req *chatv1.ListMembersRequest) (*chatv1.ListMembersResponse, error) {
	s.last = req
	return &chatv1.ListMembersResponse{
		MemberList: &chatv1.MemberList{
			Members: []*chatv1.ChatMember{
				{
					ProfileId: "profile-owner",
					Role:      "owner",
					JoinedAt:  timestamppb.Now(),
				},
				{
					ProfileId: "profile-b",
					Role:      "member",
					JoinedAt:  timestamppb.Now(),
				},
			},
		},
	}, nil
}

type recordingChatsLeaveChat struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.LeaveChatRequest
}

func (s *recordingChatsLeaveChat) LeaveChat(_ context.Context, req *chatv1.LeaveChatRequest) (*chatv1.LeaveChatResponse, error) {
	s.last = req
	return &chatv1.LeaveChatResponse{}, nil
}

// TestTranscodeChatsListMembers documents roles.md: GET /api/v1/chats/{chatId}/members exposes role.
func TestTranscodeChatsListMembers(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingChatsListMembers{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/chats/group-1/members", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if grpcRec.last == nil || grpcRec.last.GetChatId() != "group-1" {
		t.Fatalf("ListMembers chat_id = %+v", grpcRec.last)
	}
	if grpcRec.last.GetPage() == nil {
		t.Fatal("ListMembers page must be forwarded from query")
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"role":"owner"`) || !strings.Contains(body, `"role":"member"`) {
		t.Fatalf("response must include member roles, got %q", body)
	}
}

// TestTranscodeChatsLeaveChat documents voluntary leave: POST /api/v1/chats/{chatId}/leave.
func TestTranscodeChatsLeaveChat(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingChatsLeaveChat{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodPost, "/api/v1/chats/group-1/leave", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if grpcRec.last == nil || grpcRec.last.GetChatId() != "group-1" {
		t.Fatalf("LeaveChat chat_id = %+v", grpcRec.last)
	}
}
