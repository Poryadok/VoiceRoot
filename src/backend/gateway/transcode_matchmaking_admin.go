package main

import (
	"net/http"
	"strings"

	commonv1 "voice.app/voice/common/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// handleAdminMatchmaking serves staff game-request moderation (GC-03 / П.4).
func (g *gateway) handleAdminMatchmaking(w http.ResponseWriter, r *http.Request) {
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

	tc := g.config.transcoder
	if tc == nil || tc.clients.matchmaking == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "matchmaking_unavailable"})
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/matchmaking/")
	rest = strings.Trim(rest, "/")
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodGet && rest == "game-requests":
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		req := &matchmakingv1.ListGameRequestsRequest{Page: page}
		if st := queryFirst(r, "status"); st != "" {
			req.Status = &st
		}
		resp, err := tc.clients.matchmaking.ListGameRequests(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeProtoJSON(w, http.StatusOK, resp)

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/approve"):
		id := strings.TrimSuffix(rest, "/approve")
		id = strings.TrimSuffix(id, "/")
		id = strings.TrimPrefix(id, "game-requests/")
		if id == "" || strings.Contains(id, "/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		resp, err := tc.clients.matchmaking.ApproveGameRequest(ctx, &matchmakingv1.ApproveGameRequestRequest{
			GameId: id,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeProtoJSON(w, http.StatusOK, resp)

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/reject"):
		id := strings.TrimSuffix(rest, "/reject")
		id = strings.TrimSuffix(id, "/")
		id = strings.TrimPrefix(id, "game-requests/")
		if id == "" || strings.Contains(id, "/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		resp, err := tc.clients.matchmaking.RejectGameRequest(ctx, &matchmakingv1.RejectGameRequestRequest{
			GameId: id,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeProtoJSON(w, http.StatusOK, resp)

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	}
}
