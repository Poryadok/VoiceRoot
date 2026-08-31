package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"

	commonv1 "voice.app/voice/common/v1"
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
		page := &commonv1.CursorPageRequest{}
		if c := strings.TrimSpace(r.URL.Query().Get("cursor")); c != "" {
			page.Cursor = c
		}
		if ps := parseInt32Query(r.URL.Query().Get("page_size")); ps > 0 {
			page.PageSize = ps
		}
		req.Page = page
		resp, err := t.clients.moderation.ListReports(ctx, req)
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
	case strings.HasPrefix(rest, "reports/") && r.Method == http.MethodGet:
		parts := strings.Split(rest, "/")
		if len(parts) != 2 || parts[1] == "" {
			return false
		}
		resp, err := t.clients.moderation.GetReport(ctx, &moderationv1.GetReportRequest{ReportId: parts[1]})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	case rest == "sanctions" && r.Method == http.MethodGet:
		accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))
		if accountID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id is required"})
			return true
		}
		resp, err := t.clients.moderation.GetAccountSanctions(ctx, &moderationv1.GetAccountSanctionsRequest{AccountId: accountID})
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
	case strings.HasPrefix(rest, "sanctions/") && strings.HasSuffix(rest, "/revoke") && r.Method == http.MethodPost:
		sanctionID := strings.TrimSuffix(strings.TrimPrefix(rest, "sanctions/"), "/revoke")
		sanctionID = strings.Trim(sanctionID, "/")
		if sanctionID == "" {
			return false
		}
		_, err := t.clients.moderation.RevokeSanction(ctx, &moderationv1.RevokeSanctionRequest{SanctionId: sanctionID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	case strings.HasPrefix(rest, "appeals/") && strings.HasSuffix(rest, "/review") && r.Method == http.MethodPost:
		appealID := strings.TrimSuffix(strings.TrimPrefix(rest, "appeals/"), "/review")
		appealID = strings.Trim(appealID, "/")
		if appealID == "" {
			return false
		}
		req := &moderationv1.ReviewAppealRequest{AppealId: appealID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.AppealId == "" {
			req.AppealId = appealID
		}
		resp, err := t.clients.moderation.ReviewAppeal(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	case rest == "audit/export" && r.Method == http.MethodGet:
		resp, err := t.clients.moderation.ExportAuditLog(ctx, &moderationv1.ExportAuditLogRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeModerationAuditExportJSON(w, http.StatusOK, resp)
		return true
	default:
		return false
	}
}

func writeModerationAuditExportJSON(w http.ResponseWriter, httpStatus int, resp *moderationv1.ExportAuditLogResponse) {
	entries := make([]map[string]string, 0)
	if export := resp.GetAuditLogExport(); export != nil {
		for _, row := range export.GetEntries() {
			entry := map[string]string{
				"id":                row.GetId(),
				"actor_profile_id":  row.GetActorProfileId(),
				"action":            row.GetAction(),
				"target_type":       row.GetTargetType(),
				"target_id":         row.GetTargetId(),
				"details":           row.GetDetails(),
			}
			if ts := row.GetCreatedAt(); ts != nil {
				entry["created_at"] = ts.AsTime().UTC().Format(time.RFC3339Nano)
			}
			entries = append(entries, entry)
		}
	}
	writeJSON(w, httpStatus, map[string]any{"entries": entries})
}
