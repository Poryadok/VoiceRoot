package main

import (
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
)

func (t *transcoder) serveRoles(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	if t.clients.role == nil {
		return false
	}

	switch {
	case r.Method == http.MethodGet && rest == "members":
		req := &rolev1.GetMemberRolesRequest{
			SpaceId:   queryFirst(r, "space_id"),
			ProfileId: queryFirst(r, "profile_id"),
		}
		resp, err := t.clients.role.GetMemberRoles(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "":
		spaceID := queryFirst(r, "space_id")
		if spaceID == "" {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "space_id is required"))
			return true
		}
		resp, err := t.clients.role.ListRoles(ctx, &rolev1.ListRolesRequest{SpaceId: spaceID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "":
		req := &rolev1.CreateRoleRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.role.CreateRole(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "revoke":
		req := &rolev1.RevokeRoleRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.role.RevokeRole(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "assign":
		req := &rolev1.AssignRoleRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.role.AssignRole(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "check":
		req := &rolev1.CheckPermissionRequest{
			SpaceId:        queryFirst(r, "space_id"),
			ProfileId:      queryFirst(r, "profile_id"),
			PermissionName: queryFirst(r, "permission_name"),
		}
		if chatID := queryFirst(r, "chat_id"); chatID != "" {
			req.Chat = &chatv1.ChatRef{Id: chatID}
		}
		resp, err := t.clients.role.CheckPermission(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "chat-overrides":
		req := &rolev1.SetChatOverrideRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.role.SetChatOverride(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	default:
		if rest != "" && !strings.Contains(rest, "/") {
			return false
		}
		return false
	}
}
