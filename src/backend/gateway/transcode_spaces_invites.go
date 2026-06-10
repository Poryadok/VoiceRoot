package main

import (
	"net/http"
	"strings"

	spacev1 "voice.app/voice/space/v1"
)

func (t *transcoder) serveSpacesInvites(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[1] != "invites" {
		return false
	}
	spaceID := parts[0]
	inviteSub := strings.Join(parts[2:], "/")

	switch {
	case r.Method == http.MethodPost && inviteSub == "":
		req := &spacev1.CreateInviteRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.SpaceId = spaceID
		resp, err := t.clients.space.CreateInvite(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && inviteSub == "":
		resp, err := t.clients.space.ListInvites(ctx, &spacev1.ListInvitesRequest{SpaceId: spaceID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && inviteSub != "":
		inviteID := inviteSub
		_, err := t.clients.space.RevokeInvite(ctx, &spacev1.RevokeInviteRequest{InviteId: inviteID})
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

func (t *transcoder) serveInvites(w http.ResponseWriter, r *http.Request, rest string) bool {
	if t == nil || t.clients.space == nil {
		return false
	}
	ctx := withGRPCMetadata(r.Context(), r)
	parts := strings.Split(rest, "/")
	if len(parts) < 1 || parts[0] == "" {
		return false
	}
	code := parts[0]
	sub := strings.Join(parts[1:], "/")

	switch {
	case r.Method == http.MethodGet && sub == "":
		resp, err := t.clients.space.GetInvite(ctx, &spacev1.GetInviteRequest{Code: code})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && sub == "join":
		resp, err := t.clients.space.JoinByInvite(ctx, &spacev1.JoinByInviteRequest{Code: code})
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
