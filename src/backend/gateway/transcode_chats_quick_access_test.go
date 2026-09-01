package main

import (
	"context"
	"net/http"
	"testing"

	chatv1 "voice.app/voice/chat/v1"
)

type recordingQuickAccess struct {
	chatv1.UnimplementedChatServiceServer
	listLast   bool
	addLast    *chatv1.AddQuickAccessRequest
	removeLast *chatv1.RemoveQuickAccessRequest
	reorderLast *chatv1.ReorderQuickAccessRequest
}

func (s *recordingQuickAccess) ListQuickAccess(context.Context, *chatv1.ListQuickAccessRequest) (*chatv1.ListQuickAccessResponse, error) {
	s.listLast = true
	return &chatv1.ListQuickAccessResponse{}, nil
}

func (s *recordingQuickAccess) AddQuickAccess(_ context.Context, req *chatv1.AddQuickAccessRequest) (*chatv1.AddQuickAccessResponse, error) {
	s.addLast = req
	return &chatv1.AddQuickAccessResponse{}, nil
}

func (s *recordingQuickAccess) RemoveQuickAccess(_ context.Context, req *chatv1.RemoveQuickAccessRequest) (*chatv1.RemoveQuickAccessResponse, error) {
	s.removeLast = req
	return &chatv1.RemoveQuickAccessResponse{}, nil
}

func (s *recordingQuickAccess) ReorderQuickAccess(_ context.Context, req *chatv1.ReorderQuickAccessRequest) (*chatv1.ReorderQuickAccessResponse, error) {
	s.reorderLast = req
	return &chatv1.ReorderQuickAccessResponse{}, nil
}

func TestTranscodeChatsQuickAccess(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingQuickAccess{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})

	listResp := performRequest(h, http.MethodGet, "/api/v1/chats/quick-access", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%q", listResp.Code, http.StatusOK, listResp.Body.String())
	}
	if !grpcRec.listLast {
		t.Fatal("ListQuickAccess not called")
	}

	addResp := performRequest(h, http.MethodPost, "/api/v1/chats/quick-access", `{"chat_id":"chat-1"}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if addResp.Code != http.StatusNoContent {
		t.Fatalf("add status = %d, want %d; body=%q", addResp.Code, http.StatusNoContent, addResp.Body.String())
	}
	if grpcRec.addLast == nil || grpcRec.addLast.GetChatId() != "chat-1" {
		t.Fatalf("AddQuickAccess request = %+v", grpcRec.addLast)
	}

	removeResp := performRequest(h, http.MethodDelete, "/api/v1/chats/quick-access/chat-1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if removeResp.Code != http.StatusNoContent {
		t.Fatalf("remove status = %d, want %d; body=%q", removeResp.Code, http.StatusNoContent, removeResp.Body.String())
	}
	if grpcRec.removeLast == nil || grpcRec.removeLast.GetChatId() != "chat-1" {
		t.Fatalf("RemoveQuickAccess request = %+v", grpcRec.removeLast)
	}

	reorderResp := performRequest(h, http.MethodPut, "/api/v1/chats/quick-access/order", `{"chat_ids":["c2","c1"]}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if reorderResp.Code != http.StatusNoContent {
		t.Fatalf("reorder status = %d, want %d; body=%q", reorderResp.Code, http.StatusNoContent, reorderResp.Body.String())
	}
	if grpcRec.reorderLast == nil || len(grpcRec.reorderLast.GetChatIds()) != 2 {
		t.Fatalf("ReorderQuickAccess request = %+v", grpcRec.reorderLast)
	}
}
