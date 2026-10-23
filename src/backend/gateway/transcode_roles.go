package main

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
)

func (t *transcoder) serveRoles(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodGet && rest == "members":
		req := &rolev1.GetMemberRolesRequest{
			SpaceId:   queryFirst(r, "space_id"),
			ProfileId: queryFirst(r, "profile_id"),
		}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_GetMemberRoles_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.GetMemberRoles(ctx, req)
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
		if voiceRoomID := queryFirst(r, "voice_room_id"); voiceRoomID != "" {
			req.VoiceRoomId = &voiceRoomID
		}
		if t.clients.role == nil {
			writeGRPCError(w, status.Error(codes.Unavailable, "Role decision transport unavailable"))
			return true
		}
		resp, err := t.clients.role.CheckPermission(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "effective":
		req := &rolev1.GetEffectivePermissionsRequest{
			SpaceId:   queryFirst(r, "space_id"),
			ProfileId: queryFirst(r, "profile_id"),
		}
		if chatID := queryFirst(r, "chat_id"); chatID != "" {
			req.Chat = &chatv1.ChatRef{Id: chatID}
		}
		if voiceRoomID := queryFirst(r, "voice_room_id"); voiceRoomID != "" {
			req.VoiceRoomId = &voiceRoomID
		}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_GetEffectivePermissions_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.GetEffectivePermissions(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "default-join":
		req := &rolev1.GetDefaultJoinRoleRequest{SpaceId: queryFirst(r, "space_id")}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_GetDefaultJoinRole_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.GetDefaultJoinRole(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "chat-overrides":
		req := &rolev1.GetChatOverridesRequest{SpaceId: queryFirst(r, "space_id")}
		if chatID := queryFirst(r, "chat_id"); chatID != "" {
			req.FilterChat = &chatv1.ChatRef{Id: chatID}
		}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_GetChatOverrides_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.GetChatOverrides(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "voice-overrides":
		req := &rolev1.GetVoiceRoomOverridesRequest{SpaceId: queryFirst(r, "space_id")}
		if voiceRoomID := queryFirst(r, "voice_room_id"); voiceRoomID != "" {
			req.VoiceRoomId = &voiceRoomID
		}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_GetVoiceRoomOverrides_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.GetVoiceRoomOverrides(ctx, req)
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
		req := &rolev1.ListRolesRequest{SpaceId: spaceID}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_ListRoles_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.ListRoles(ctx, req)
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
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_CreateRole_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.CreateRole(ctx, req)
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
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_RevokeRole_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.RevokeRole(ctx, req)
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
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_AssignRole_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.AssignRole(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "reorder":
		req := &rolev1.ReorderRolesRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_ReorderRoles_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.ReorderRoles(ctx, req)
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
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_SetChatOverride_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.SetChatOverride(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "voice-overrides":
		req := &rolev1.SetVoiceRoomOverrideRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_SetVoiceRoomOverride_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.SetVoiceRoomOverride(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPut && rest == "default-join":
		req := &rolev1.SetDefaultJoinRoleRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_SetDefaultJoinRole_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.SetDefaultJoinRole(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && rest == "chat-overrides":
		req := &rolev1.RemoveChatOverrideRequest{
			SpaceId: queryFirst(r, "space_id"),
			RoleId:  queryFirst(r, "role_id"),
		}
		if chatID := queryFirst(r, "chat_id"); chatID != "" {
			req.Chat = &chatv1.ChatRef{Id: chatID}
		}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_RemoveChatOverride_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.RemoveChatOverride(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && rest == "voice-overrides":
		voiceRoomID := queryFirst(r, "voice_room_id")
		req := &rolev1.RemoveVoiceRoomOverrideRequest{
			SpaceId: queryFirst(r, "space_id"),
			RoleId:  queryFirst(r, "role_id"),
		}
		req.VoiceRoomId = voiceRoomID
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_RemoveVoiceRoomOverride_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.RemoveVoiceRoomOverride(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPatch && rest != "" && !strings.Contains(rest, "/"):
		req := &rolev1.UpdateRoleRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.RoleId = rest
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_UpdateRole_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.UpdateRole(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && rest != "" && !strings.Contains(rest, "/"):
		req := &rolev1.DeleteRoleRequest{RoleId: rest}
		ctx, err := t.auditInitiatingContext(r, req, rolev1.RoleService_DeleteRole_FullMethodName, uuid.NewString())
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.roleAudit.DeleteRole(ctx, req)
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
