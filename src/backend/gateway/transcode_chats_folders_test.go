package main

import (
	"context"
	"net/http"
	"testing"

	chatv1 "voice.app/voice/chat/v1"
)

type recordingFolderChat struct {
	chatv1.UnimplementedChatServiceServer
	listFoldersLast  bool
	createFolderLast *chatv1.CreateFolderRequest
	updateFolderLast *chatv1.UpdateFolderRequest
	deleteFolderLast *chatv1.DeleteFolderRequest
	addLast          *chatv1.AddChatToFolderRequest
	removeLast       *chatv1.RemoveChatFromFolderRequest
	reorderLast      *chatv1.ReorderFolderChatsRequest
	pinLast          *chatv1.PinChatInFolderRequest
	unpinLast        *chatv1.UnpinChatInFolderRequest
	listChatsLast    *chatv1.ListChatsRequest
}

func (s *recordingFolderChat) ListFolders(context.Context, *chatv1.ListFoldersRequest) (*chatv1.ListFoldersResponse, error) {
	s.listFoldersLast = true
	return &chatv1.ListFoldersResponse{}, nil
}

func (s *recordingFolderChat) CreateFolder(_ context.Context, req *chatv1.CreateFolderRequest) (*chatv1.CreateFolderResponse, error) {
	s.createFolderLast = req
	return &chatv1.CreateFolderResponse{}, nil
}

func (s *recordingFolderChat) UpdateFolder(_ context.Context, req *chatv1.UpdateFolderRequest) (*chatv1.UpdateFolderResponse, error) {
	s.updateFolderLast = req
	return &chatv1.UpdateFolderResponse{}, nil
}

func (s *recordingFolderChat) DeleteFolder(_ context.Context, req *chatv1.DeleteFolderRequest) (*chatv1.DeleteFolderResponse, error) {
	s.deleteFolderLast = req
	return &chatv1.DeleteFolderResponse{}, nil
}

func (s *recordingFolderChat) AddChatToFolder(_ context.Context, req *chatv1.AddChatToFolderRequest) (*chatv1.AddChatToFolderResponse, error) {
	s.addLast = req
	return &chatv1.AddChatToFolderResponse{}, nil
}

func (s *recordingFolderChat) RemoveChatFromFolder(_ context.Context, req *chatv1.RemoveChatFromFolderRequest) (*chatv1.RemoveChatFromFolderResponse, error) {
	s.removeLast = req
	return &chatv1.RemoveChatFromFolderResponse{}, nil
}

func (s *recordingFolderChat) ReorderFolderChats(_ context.Context, req *chatv1.ReorderFolderChatsRequest) (*chatv1.ReorderFolderChatsResponse, error) {
	s.reorderLast = req
	return &chatv1.ReorderFolderChatsResponse{}, nil
}

func (s *recordingFolderChat) PinChatInFolder(_ context.Context, req *chatv1.PinChatInFolderRequest) (*chatv1.PinChatInFolderResponse, error) {
	s.pinLast = req
	return &chatv1.PinChatInFolderResponse{}, nil
}

func (s *recordingFolderChat) UnpinChatInFolder(_ context.Context, req *chatv1.UnpinChatInFolderRequest) (*chatv1.UnpinChatInFolderResponse, error) {
	s.unpinLast = req
	return &chatv1.UnpinChatInFolderResponse{}, nil
}

func (s *recordingFolderChat) ListChats(_ context.Context, req *chatv1.ListChatsRequest) (*chatv1.ListChatsResponse, error) {
	s.listChatsLast = req
	return &chatv1.ListChatsResponse{}, nil
}

func TestTranscodeChatsFolders(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingFolderChat{}
	conn, cleanup := startBufconnChatConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{chat: chatv1.NewChatServiceClient(conn)}},
	})

	listFolders := performRequest(h, http.MethodGet, "/api/v1/chats/folders", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if listFolders.Code != http.StatusOK || !grpcRec.listFoldersLast {
		t.Fatalf("ListFolders status=%d listLast=%v", listFolders.Code, grpcRec.listFoldersLast)
	}

	createFolder := performRequest(h, http.MethodPost, "/api/v1/chats/folders", `{"name":"Work"}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if createFolder.Code != http.StatusOK || grpcRec.createFolderLast == nil || grpcRec.createFolderLast.GetName() != "Work" {
		t.Fatalf("CreateFolder status=%d req=%+v", createFolder.Code, grpcRec.createFolderLast)
	}

	updateFolder := performRequest(h, http.MethodPatch, "/api/v1/chats/folders/f1", `{"name":"Renamed","sort_order":7}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if updateFolder.Code != http.StatusOK || grpcRec.updateFolderLast == nil || grpcRec.updateFolderLast.GetFolderId() != "f1" || grpcRec.updateFolderLast.GetName() != "Renamed" {
		t.Fatalf("UpdateFolder status=%d req=%+v", updateFolder.Code, grpcRec.updateFolderLast)
	}

	deleteFolder := performRequest(h, http.MethodDelete, "/api/v1/chats/folders/f2", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if deleteFolder.Code != http.StatusNoContent || grpcRec.deleteFolderLast == nil || grpcRec.deleteFolderLast.GetFolderId() != "f2" {
		t.Fatalf("DeleteFolder status=%d req=%+v", deleteFolder.Code, grpcRec.deleteFolderLast)
	}

	add := performRequest(h, http.MethodPost, "/api/v1/chats/folders/f1/chats", `{"chat_id":"c1"}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if add.Code != http.StatusNoContent || grpcRec.addLast == nil || grpcRec.addLast.GetFolderId() != "f1" {
		t.Fatalf("AddChatToFolder status=%d req=%+v", add.Code, grpcRec.addLast)
	}

	remove := performRequest(h, http.MethodDelete, "/api/v1/chats/folders/f1/chats/c1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if remove.Code != http.StatusNoContent || grpcRec.removeLast == nil {
		t.Fatalf("RemoveChatFromFolder status=%d req=%+v", remove.Code, grpcRec.removeLast)
	}

	reorder := performRequest(h, http.MethodPut, "/api/v1/chats/folders/f1/chats/order", `{"chat_ids":["c2","c1"]}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if reorder.Code != http.StatusNoContent || grpcRec.reorderLast == nil {
		t.Fatalf("ReorderFolderChats status=%d req=%+v", reorder.Code, grpcRec.reorderLast)
	}

	pin := performRequest(h, http.MethodPost, "/api/v1/chats/folders/f1/chats/c1/pin", `{}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if pin.Code != http.StatusNoContent || grpcRec.pinLast == nil {
		t.Fatalf("PinChatInFolder status=%d req=%+v", pin.Code, grpcRec.pinLast)
	}

	unpin := performRequest(h, http.MethodDelete, "/api/v1/chats/folders/f1/chats/c1/pin", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if unpin.Code != http.StatusNoContent || grpcRec.unpinLast == nil {
		t.Fatalf("UnpinChatInFolder status=%d req=%+v", unpin.Code, grpcRec.unpinLast)
	}

	listChats := performRequest(h, http.MethodGet, "/api/v1/chats?folder_id=f1", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if listChats.Code != http.StatusOK || grpcRec.listChatsLast == nil || grpcRec.listChatsLast.GetFolderId() != "f1" {
		t.Fatalf("ListChats folder_id status=%d req=%+v", listChats.Code, grpcRec.listChatsLast)
	}
}
