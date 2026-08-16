package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	chatv1 "voice.app/voice/chat/v1"
)

type recordingChatsArchive struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.ArchiveChatRequest
}

func (s *recordingChatsArchive) ArchiveChat(_ context.Context, req *chatv1.ArchiveChatRequest) (*chatv1.ArchiveChatResponse, error) {
	s.last = req
	return &chatv1.ArchiveChatResponse{}, nil
}

type recordingChatsMute struct {
	chatv1.UnimplementedChatServiceServer
	last *chatv1.MuteChatRequest
}

func (s *recordingChatsMute) MuteChat(_ context.Context, req *chatv1.MuteChatRequest) (*chatv1.MuteChatResponse, error) {
	s.last = req
	return &chatv1.MuteChatResponse{}, nil
}

// TestTranscodeChatsArchive documents POST /api/v1/chats/{chatId}/archive → ArchiveChat.
func TestTranscodeChatsArchive(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingChatsArchive{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodPost, "/api/v1/chats/chat-1/archive", `{"archived":true}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if grpcRec.last == nil || grpcRec.last.GetChatId() != "chat-1" || !grpcRec.last.GetArchived() {
		t.Fatalf("ArchiveChat request = %+v", grpcRec.last)
	}
}

// TestTranscodeChatsMute documents POST /api/v1/chats/{chatId}/mute → MuteChat.
func TestTranscodeChatsMute(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingChatsMute{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodPost, "/api/v1/chats/chat-1/mute", `{"muted_until":"2030-01-02T03:04:05Z"}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if grpcRec.last == nil || grpcRec.last.GetChatId() != "chat-1" {
		t.Fatalf("MuteChat request = %+v", grpcRec.last)
	}
	want, err := time.Parse(time.RFC3339, "2030-01-02T03:04:05Z")
	if err != nil {
		t.Fatal(err)
	}
	if !grpcRec.last.GetMutedUntil().AsTime().Equal(want) {
		t.Fatalf("MuteChat muted_until = %v, want %v", grpcRec.last.GetMutedUntil().AsTime(), want)
	}
}
