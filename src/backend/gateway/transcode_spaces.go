package main

import (
	"net/http"
	"strings"

	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

func (t *transcoder) serveSpaces(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodGet && rest == "":
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		resp, err := t.clients.space.ListMySpaces(ctx, &spacev1.ListMySpacesRequest{Page: page})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest != "" && !strings.Contains(rest, "/"):
		resp, err := t.clients.space.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: rest})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "":
		req := &spacev1.CreateSpaceRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.space.CreateSpace(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPatch && rest != "" && !strings.Contains(rest, "/"):
		req := &spacev1.UpdateSpaceRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.SpaceId = rest
		resp, err := t.clients.space.UpdateSpace(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	default:
		if strings.Contains(rest, "/") {
			if t.serveSpacesInvites(w, r, rest) {
				return true
			}
			if t.serveSpacesBans(w, r, rest) {
				return true
			}
			if t.serveSpacesMembers(w, r, rest) {
				return true
			}
			return t.serveSpacesTree(w, r, rest)
		}
		return false
	}
}
