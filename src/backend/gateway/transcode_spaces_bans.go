package main

import (
	"net/http"
	"strings"

	spacev1 "voice.app/voice/space/v1"
)

func (t *transcoder) serveSpacesBans(w http.ResponseWriter, r *http.Request, rest string) bool {
	if !strings.Contains(rest, "/bans") {
		return false
	}
	parts := strings.SplitN(rest, "/bans", 2)
	spaceID := parts[0]
	banRest := strings.TrimPrefix(parts[1], "/")
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodGet && banRest == "":
		resp, err := t.clients.space.ListBans(ctx, &spacev1.ListBansRequest{SpaceId: spaceID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && banRest == "":
		req := &spacev1.BanMemberRequest{SpaceId: spaceID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.SpaceId = spaceID
		_, err := t.clients.space.BanMember(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodDelete && banRest != "":
		_, err := t.clients.space.UnbanMember(ctx, &spacev1.UnbanMemberRequest{
			SpaceId:   spaceID,
			AccountId: banRest,
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
