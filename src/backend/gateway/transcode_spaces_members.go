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

	if strings.HasSuffix(memberRest, "/timeout") {
		profileID := strings.TrimSuffix(memberRest, "/timeout")
		switch r.Method {
		case http.MethodPost:
			req := &spacev1.TimeoutMemberRequest{SpaceId: spaceID, ProfileId: profileID}
			if err := readProtoJSON(r, req); err != nil {
				writeGRPCError(w, err)
				return true
			}
			req.SpaceId = spaceID
			req.ProfileId = profileID
			_, err := t.clients.space.TimeoutMember(ctx, req)
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true
		case http.MethodDelete:
			_, err := t.clients.space.RemoveMemberTimeout(ctx, &spacev1.RemoveMemberTimeoutRequest{
				SpaceId:   spaceID,
				ProfileId: profileID,
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
