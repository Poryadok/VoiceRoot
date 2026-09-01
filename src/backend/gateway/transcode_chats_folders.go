package main

import (
	"context"
	"net/http"
	"strings"

	chatv1 "voice.app/voice/chat/v1"
)

func (t *transcoder) serveChatFolderRoutes(w http.ResponseWriter, r *http.Request, ctx context.Context, rest string) bool {
	rest = strings.TrimPrefix(rest, "folders")
	rest = strings.TrimPrefix(rest, "/")

	switch {
	case r.Method == http.MethodGet && rest == "":
		resp, err := t.clients.chat.ListFolders(ctx, &chatv1.ListFoldersRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "":
		req := &chatv1.CreateFolderRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.chat.CreateFolder(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case (r.Method == http.MethodPatch || r.Method == http.MethodPut) && rest != "" && !strings.Contains(rest, "/"):
		req := &chatv1.UpdateFolderRequest{FolderId: rest}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.FolderId == "" {
			req.FolderId = rest
		}
		resp, err := t.clients.chat.UpdateFolder(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && rest != "" && !strings.Contains(rest, "/"):
		_, err := t.clients.chat.DeleteFolder(ctx, &chatv1.DeleteFolderRequest{FolderId: rest})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodPut && strings.HasSuffix(rest, "/chats/order"):
		folderID := strings.TrimSuffix(rest, "/chats/order")
		folderID = strings.Trim(folderID, "/")
		req := &chatv1.ReorderFolderChatsRequest{FolderId: folderID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.FolderId == "" {
			req.FolderId = folderID
		}
		_, err := t.clients.chat.ReorderFolderChats(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/pin"):
		base := strings.TrimSuffix(rest, "/pin")
		base = strings.Trim(base, "/")
		parts := strings.SplitN(base, "/chats/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return false
		}
		req := &chatv1.PinChatInFolderRequest{FolderId: parts[0], ChatId: parts[1]}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.FolderId == "" {
			req.FolderId = parts[0]
		}
		if req.ChatId == "" {
			req.ChatId = parts[1]
		}
		_, err := t.clients.chat.PinChatInFolder(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodDelete && strings.HasSuffix(rest, "/pin"):
		base := strings.TrimSuffix(rest, "/pin")
		base = strings.Trim(base, "/")
		parts := strings.SplitN(base, "/chats/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return false
		}
		_, err := t.clients.chat.UnpinChatInFolder(ctx, &chatv1.UnpinChatInFolderRequest{
			FolderId: parts[0],
			ChatId:   parts[1],
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/chats"):
		folderID := strings.TrimSuffix(rest, "/chats")
		folderID = strings.Trim(folderID, "/")
		req := &chatv1.AddChatToFolderRequest{FolderId: folderID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.FolderId == "" {
			req.FolderId = folderID
		}
		_, err := t.clients.chat.AddChatToFolder(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodDelete && strings.Contains(rest, "/chats/"):
		parts := strings.SplitN(rest, "/chats/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(parts[1], "/") {
			return false
		}
		_, err := t.clients.chat.RemoveChatFromFolder(ctx, &chatv1.RemoveChatFromFolderRequest{
			FolderId: parts[0],
			ChatId:   parts[1],
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	default:
		return false
	}
}
