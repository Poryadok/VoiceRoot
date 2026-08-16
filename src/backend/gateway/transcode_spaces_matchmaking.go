package main

import (
	"net/http"
	"strings"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
	spacev1 "voice.app/voice/space/v1"
)

// serveSpacesMatchmaking handles:
//   POST   /api/v1/spaces/{id}/matchmaking/queue  → StartSpaceQueue
//   PATCH  /api/v1/spaces/{id}/matchmaking/config → UpdateSpaceMmConfig
func (t *transcoder) serveSpacesMatchmaking(w http.ResponseWriter, r *http.Request, rest string) bool {
	parts := strings.Split(rest, "/")
	if len(parts) < 3 || parts[1] != "matchmaking" {
		return false
	}
	spaceID := parts[0]
	if spaceID == "" {
		return false
	}
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case len(parts) == 3 && parts[2] == "queue" && r.Method == http.MethodPost:
		req := &matchmakingv1.StartSpaceQueueRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.SpaceId = spaceID
		resp, err := t.clients.matchmaking.StartSpaceQueue(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case len(parts) == 3 && parts[2] == "config" && r.Method == http.MethodPatch:
		req := &spacev1.UpdateSpaceMmConfigRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.SpaceId = spaceID
		resp, err := t.clients.space.UpdateSpaceMmConfig(ctx, req)
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
