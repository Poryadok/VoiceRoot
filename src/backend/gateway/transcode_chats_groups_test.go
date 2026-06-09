package main

import (
	"context"
	"net/http"
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
)

type recordingChatsCreateGroup struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.CreateChatRequest
}

func (s *recordingChatsCreateGroup) CreateChat(_ context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	s.last = req
	now := timestamppb.Now()
	return &chatv1.CreateChatResponse{
		Chat: &chatv1.Chat{
			Id:               "group-1",
			Type:             chatv1.ChatType_CHAT_TYPE_GROUP,
			Name:             req.Name,
			CreatorProfileId: "profile-1",
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, nil
}

type recordingChatsAddMembers struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.AddMembersRequest
}

func (s *recordingChatsAddMembers) AddMembers(_ context.Context, req *chatv1.AddMembersRequest) (*chatv1.AddMembersResponse, error) {
	s.last = req
	return &chatv1.AddMembersResponse{}, nil
}

type recordingChatsRemoveMember struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.RemoveMemberRequest
}

func (s *recordingChatsRemoveMember) RemoveMember(_ context.Context, req *chatv1.RemoveMemberRequest) (*chatv1.RemoveMemberResponse, error) {
	s.last = req
	return &chatv1.RemoveMemberResponse{}, nil
}

type recordingChatsUpdateGroup struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.UpdateChatRequest
}

func (s *recordingChatsUpdateGroup) UpdateChat(_ context.Context, req *chatv1.UpdateChatRequest) (*chatv1.UpdateChatResponse, error) {
	s.last = req
	now := timestamppb.Now()
	return &chatv1.UpdateChatResponse{
		Chat: &chatv1.Chat{
			Id:               req.GetChatId(),
			Type:             chatv1.ChatType_CHAT_TYPE_GROUP,
			AvatarUrl:        req.AvatarUrl,
			CreatorProfileId: "profile-1",
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, nil
}

// TestTranscodeChatsCreateGroup documents PLAN Phase 4 REST: POST /api/v1/chats with type=group.
func TestTranscodeChatsCreateGroup(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingChatsCreateGroup{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"chats": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	body := `{"type":"CHAT_TYPE_GROUP","name":"Friday squad"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/chats", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if proxyCalled {
		t.Fatal("REST proxy must not run when gRPC transcoder handles POST /api/v1/chats")
	}
	if grpcRec.last == nil || grpcRec.last.GetType() != chatv1.ChatType_CHAT_TYPE_GROUP {
		t.Fatalf("CreateChat request = %+v", grpcRec.last)
	}
	if grpcRec.last.GetName() != "Friday squad" {
		t.Fatalf("CreateChat name = %q", grpcRec.last.GetName())
	}
}

// TestTranscodeChatsAddMembers documents invite: POST /api/v1/chats/{chatId}/members.
func TestTranscodeChatsAddMembers(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingChatsAddMembers{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})

	body := `{"profile_ids":["profile-b","profile-c"]}`
	resp := performRequest(h, http.MethodPost, "/api/v1/chats/group-1/members", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if grpcRec.last == nil || grpcRec.last.GetChatId() != "group-1" {
		t.Fatalf("AddMembers chat_id = %+v", grpcRec.last)
	}
	if len(grpcRec.last.GetProfileIds()) != 2 {
		t.Fatalf("AddMembers profile_ids = %+v", grpcRec.last.GetProfileIds())
	}
}

// TestTranscodeChatsRemoveMember documents kick: DELETE /api/v1/chats/{chatId}/members/{profileId}.
func TestTranscodeChatsRemoveMember(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingChatsRemoveMember{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodDelete, "/api/v1/chats/group-1/members/profile-b", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if grpcRec.last == nil || grpcRec.last.GetChatId() != "group-1" || grpcRec.last.GetProfileId() != "profile-b" {
		t.Fatalf("RemoveMember request = %+v", grpcRec.last)
	}
}

// TestTranscodeChatsUpdateGroupAvatar documents PATCH /api/v1/chats/{chatId} with avatar_url.
func TestTranscodeChatsUpdateGroupAvatar(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingChatsUpdateGroup{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})

	body := `{"avatar_url":"https://cdn.voice.gg/groups/party.webp"}`
	resp := performRequest(h, http.MethodPatch, "/api/v1/chats/group-1", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if grpcRec.last == nil || grpcRec.last.GetChatId() != "group-1" {
		t.Fatalf("UpdateChat chat_id = %+v", grpcRec.last)
	}
	if grpcRec.last.GetAvatarUrl() != "https://cdn.voice.gg/groups/party.webp" {
		t.Fatalf("UpdateChat avatar_url = %q", grpcRec.last.GetAvatarUrl())
	}
}
