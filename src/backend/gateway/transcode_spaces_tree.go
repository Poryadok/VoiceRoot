package main

import (
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

func (t *transcoder) serveSpacesTree(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return false
	}
	spaceID := parts[0]
	sub := strings.Join(parts[1:], "/")

	switch {
	case r.Method == http.MethodGet && sub == "tree":
		resp, err := t.clients.space.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && sub == "categories":
		req := &spacev1.CreateCategoryRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.SpaceId = spaceID
		resp, err := t.clients.space.CreateCategory(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && sub == "voice-rooms":
		req := &spacev1.CreateVoiceRoomRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.SpaceId = spaceID
		resp, err := t.clients.space.CreateVoiceRoom(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && sub == "tree/nodes":
		req := &spacev1.UpsertTreeNodeRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.SpaceId = spaceID
		resp, err := t.clients.space.UpsertTreeNode(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && sub == "tree/reorder":
		req := &spacev1.ReorderSpaceTreeRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.SpaceId = spaceID
		_, err := t.clients.space.ReorderSpaceTree(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodDelete && strings.HasPrefix(sub, "tree/nodes/"):
		nodeID := strings.TrimPrefix(sub, "tree/nodes/")
		_, err := t.clients.space.RemoveTreeNode(ctx, &spacev1.RemoveTreeNodeRequest{
			SpaceId: spaceID,
			NodeId:  nodeID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodPost && sub == "chats":
		chatReq := &chatv1.CreateChatRequest{}
		if err := readProtoJSON(r, chatReq); err != nil {
			writeGRPCError(w, err)
			return true
		}
		chatReq.SpaceId = &spaceID
		chatResp, err := t.clients.chat.CreateChat(ctx, chatReq)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		chat := chatResp.GetChat()
		if chat == nil {
			writeGRPCError(w, status.Error(codes.Internal, "chat missing from create response"))
			return true
		}
		chatType := chat.GetType()
		nodeReq := &spacev1.UpsertTreeNodeRequest{
			SpaceId: spaceID,
			Kind:    "text_chat",
			LinkedChat: &chatv1.ChatRef{
				Id:   chat.GetId(),
				Type: &chatType,
			},
		}
		nodeResp, err := t.clients.space.UpsertTreeNode(ctx, nodeReq)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, nodeResp)
		return true

	default:
		return false
	}
}
