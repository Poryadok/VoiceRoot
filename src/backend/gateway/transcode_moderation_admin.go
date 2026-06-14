package main

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"

	moderationv1 "voice.app/voice/moderation/v1"
)

func (g *gateway) handleAdminModeration(w http.ResponseWriter, r *http.Request) {
	claims, code := g.authenticate(r)
	if code != "" {
		status := http.StatusUnauthorized
		if code == "auth_unavailable" {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": code})
		return
	}
	if !hasRole(claims, "staff") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	applyClaims(r, claims)
	if g.config.transcoder != nil && g.config.transcoder.serveAdminModeration(w, r) {
		return
	}
	http.NotFound(w, r)
}

func withInternalGRPCMetadata(ctx context.Context, r *http.Request) context.Context {
	md := grpcMetadataFromRequest(r)
	md.Set("x-voice-internal", "true")
	return metadata.NewOutgoingContext(ctx, md)
}

func (t *transcoder) serveAdminModeration(w http.ResponseWriter, r *http.Request) bool {
	if t == nil || t.clients.moderation == nil {
		return false
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/moderation/")
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return false
	}
	ctx := withInternalGRPCMetadata(r.Context(), r)

	switch {
	case rest == "reports" && r.Method == http.MethodGet:
		req := &moderationv1.ListReportsRequest{
			StatusFilter: r.URL.Query().Get("status"),
			QueueFilter:  r.URL.Query().Get("queue"),
		}
		resp, err := t.clients.moderation.ListReports(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	case rest == "sanctions" && r.Method == http.MethodPost:
		req := &moderationv1.ApplySanctionRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.moderation.ApplySanction(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	case strings.HasPrefix(rest, "reports/") && strings.HasSuffix(rest, "/resolve") && r.Method == http.MethodPost:
		parts := strings.Split(rest, "/")
		if len(parts) != 3 {
			return false
		}
		req := &moderationv1.ResolveReportRequest{ReportId: parts[1]}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.ReportId == "" {
			req.ReportId = parts[1]
		}
		resp, err := t.clients.moderation.ResolveReport(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	case rest == "audit/export" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"entries":[]}`))
		return true
	default:
		return false
	}
}
