package main

import (
	"net/http"
	"strings"

	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

func (t *transcoder) serveSpacesMembers(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	if !strings.Contains(rest, "/members") {
		return false
	}
	parts := strings.SplitN(rest, "/members", 2)
	spaceID := parts[0]
	memberRest := strings.TrimPrefix(parts[1], "/")

	switch {
	case r.Method == http.MethodGet && memberRest == "":
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		resp, err := t.clients.space.ListMembers(ctx, &spacev1.ListMembersRequest{
			SpaceId: spaceID,
			Page:    page,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && memberRest != "":
		resp, err := t.clients.space.KickMember(ctx, &spacev1.KickMemberRequest{
			SpaceId:   spaceID,
			ProfileId: memberRest,
		})
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
