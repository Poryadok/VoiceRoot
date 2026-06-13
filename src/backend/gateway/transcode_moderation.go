package main

import (
	"net/http"
	"strings"

	moderationv1 "voice.app/voice/moderation/v1"
)

func (t *transcoder) serveModeration(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	if rest != "reports" {
		return false
	}
	switch r.Method {
	case http.MethodPost:
		req := &moderationv1.CreateReportRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if strings.EqualFold(strings.TrimSpace(req.GetCategory()), "mm_toxic") {
			req.Category = "cheating"
		}
		resp, err := t.clients.moderation.CreateReport(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusAccepted, resp)
		return true
	case http.MethodGet:
		http.NotFound(w, r)
		return true
	default:
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return true
	}
}
