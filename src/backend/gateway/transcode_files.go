package main

import (
	"net/http"
	"strings"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
)

func (t *transcoder) serveFiles(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := t.withFileGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodPost && rest == "upload":
		req := &filev1.RequestUploadRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.file.RequestUpload(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "bulk-metadata":
		req := &filev1.GetBulkMetadataRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.file.GetBulkMetadata(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/confirm"):
		fileID := strings.TrimSuffix(rest, "/confirm")
		fileID = strings.Trim(fileID, "/")
		if fileID == "" || strings.Contains(fileID, "/") {
			return false
		}
		req := &filev1.ConfirmUploadRequest{FileId: fileID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.FileId == "" {
			req.FileId = fileID
		}
		resp, err := t.clients.file.ConfirmUpload(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "quota":
		req := &filev1.CheckQuotaRequest{}
		if profileID := strings.TrimSpace(queryFirst(r, "profile_id")); profileID != "" {
			req.ProfileId = profileID
		}
		resp, err := t.clients.file.CheckQuota(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "":
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		req := &filev1.ListFilesRequest{Page: page}
		if chatID := strings.TrimSpace(queryFirst(r, "chat_id")); chatID != "" {
			ref := &chatv1.ChatRef{Id: chatID}
			if chatType := strings.TrimSpace(queryFirst(r, "chat_type")); chatType != "" {
				if enum, ok := chatTypeFromQuery(chatType); ok {
					ref.Type = &enum
				}
			}
			req.FilterChat = ref
		}
		resp, err := t.clients.file.ListFiles(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && strings.HasSuffix(rest, "/url"):
		fileID := strings.TrimSuffix(rest, "/url")
		fileID = strings.Trim(fileID, "/")
		if fileID == "" || strings.Contains(fileID, "/") {
			return false
		}
		resp, err := t.clients.file.GetFileURL(ctx, &filev1.GetFileURLRequest{FileId: fileID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest != "" && !strings.Contains(rest, "/"):
		resp, err := t.clients.file.GetFileMetadata(ctx, &filev1.GetFileMetadataRequest{FileId: rest})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && rest != "" && !strings.Contains(rest, "/"):
		_, err := t.clients.file.DeleteFile(ctx, &filev1.DeleteFileRequest{FileId: rest})
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

func chatTypeFromQuery(raw string) (chatv1.ChatType, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "dm":
		return chatv1.ChatType_CHAT_TYPE_DM, true
	case "group":
		return chatv1.ChatType_CHAT_TYPE_GROUP, true
	case "channel":
		return chatv1.ChatType_CHAT_TYPE_CHANNEL, true
	default:
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, false
	}
}
