package main

import (
	"net/http"
	"strings"

	filev1 "voice.app/voice/file/v1"
)

func (t *transcoder) serveFiles(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

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

	default:
		return false
	}
}
