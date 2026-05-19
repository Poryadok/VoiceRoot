package main

import (
	"net/http"
	"strings"

	socialv1 "voice.app/voice/social/v1"
	commonv1 "voice.app/voice/common/v1"
)

func (t *transcoder) serveFriends(w http.ResponseWriter, r *http.Request, rest string) bool {
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
		resp, err := t.clients.social.ListFriends(ctx, &socialv1.ListFriendsRequest{Page: page})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "requests":
		resp, err := t.clients.social.ListFriendRequests(ctx, &socialv1.ListFriendRequestsRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "invitations":
		req := &socialv1.SendFriendInvitationRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.social.SendFriendInvitation(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && strings.HasPrefix(rest, "invitations/") && strings.HasSuffix(rest, "/accept"):
		target := strings.TrimSuffix(strings.TrimPrefix(rest, "invitations/"), "/accept")
		resp, err := t.clients.social.AcceptFriendInvitation(ctx, &socialv1.AcceptFriendInvitationRequest{
			RequesterProfileId: target,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && strings.HasPrefix(rest, "invitations/") && strings.HasSuffix(rest, "/decline"):
		target := strings.TrimSuffix(strings.TrimPrefix(rest, "invitations/"), "/decline")
		resp, err := t.clients.social.DeclineFriendInvitation(ctx, &socialv1.DeclineFriendInvitationRequest{
			RequesterProfileId: target,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && rest != "" && !strings.Contains(rest, "/"):
		resp, err := t.clients.social.RemoveFriend(ctx, &socialv1.RemoveFriendRequest{
			FriendProfileId: rest,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "blocks":
		req := &socialv1.BlockAccountRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.social.BlockAccount(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && strings.HasPrefix(rest, "blocks/"):
		target := strings.TrimPrefix(rest, "blocks/")
		resp, err := t.clients.social.UnblockAccount(ctx, &socialv1.UnblockAccountRequest{
			BlockedAccountId: target,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "blocks":
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		resp, err := t.clients.social.ListBlocked(ctx, &socialv1.ListBlockedRequest{Page: page})
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
