package main

import (
	"net/http"
	"strings"

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
