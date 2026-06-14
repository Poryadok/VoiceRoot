package main

import (
	"net/http"
	"strings"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

func (t *transcoder) serveMessages(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodPost && rest == "forward":
		req := &messagingv1.ForwardMessageRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.messaging.ForwardMessage(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "send":
		req := &messagingv1.SendMessageRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.messaging.SendMessage(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "thread":
		req := &messagingv1.GetThreadMessagesRequest{
			Chat:           &chatv1.ChatRef{Id: queryFirst(r, "chat_id")},
			ThreadParentId: queryFirst(r, "thread_parent_id"),
		}
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		req.Page = page
		resp, err := t.clients.messaging.GetThreadMessages(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "":
		req := &messagingv1.GetMessagesRequest{
			Chat: &chatv1.ChatRef{Id: queryFirst(r, "chat_id")},
		}
		if v := queryFirst(r, "after_message_id"); v != "" {
			req.AfterMessageId = &v
		}
		if v := queryFirst(r, "before_message_id"); v != "" {
			req.BeforeMessageId = &v
		}
		if v := queryFirst(r, "last_message_id"); v != "" {
			req.LastMessageId = &v
		}
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		req.Page = page
		resp, err := t.clients.messaging.GetMessages(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPatch && rest != "" && !strings.Contains(rest, "/"):
		req := &messagingv1.EditMessageRequest{MessageId: rest}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.MessageId == "" {
			req.MessageId = rest
		}
		resp, err := t.clients.messaging.EditMessage(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && rest != "" && !strings.Contains(rest, "/"):
		req := &messagingv1.DeleteMessageRequest{
			MessageId: rest,
		}
		switch strings.ToLower(queryFirst(r, "scope")) {
		case "me", "for_me":
			scope := messagingv1.DeleteScope_DELETE_SCOPE_FOR_ME
			req.Scope = &scope
		case "everyone", "for_everyone":
			scope := messagingv1.DeleteScope_DELETE_SCOPE_FOR_EVERYONE
			req.Scope = &scope
		}
		_, err := t.clients.messaging.DeleteMessage(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodPost && rest == "read":
		req := &messagingv1.MarkReadRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.messaging.MarkRead(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/reactions"):
		msgID := strings.TrimSuffix(rest, "/reactions")
		msgID = strings.Trim(msgID, "/")
		req := &messagingv1.AddReactionRequest{MessageId: msgID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.MessageId == "" {
			req.MessageId = msgID
		}
		_, err := t.clients.messaging.AddReaction(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/pin"):
		msgID := strings.TrimSuffix(rest, "/pin")
		msgID = strings.Trim(msgID, "/")
		req := &messagingv1.PinMessageRequest{MessageId: msgID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.MessageId == "" {
			req.MessageId = msgID
		}
		if req.GetChat() == nil || req.GetChat().GetId() == "" {
			req.Chat = &chatv1.ChatRef{Id: queryFirst(r, "chat_id")}
		}
		_, err := t.clients.messaging.PinMessage(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodDelete && strings.HasSuffix(rest, "/pin"):
		msgID := strings.TrimSuffix(rest, "/pin")
		msgID = strings.Trim(msgID, "/")
		chatID := queryFirst(r, "chat_id")
		_, err := t.clients.messaging.UnpinMessage(ctx, &messagingv1.UnpinMessageRequest{
			Chat:      &chatv1.ChatRef{Id: chatID},
			MessageId: msgID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodDelete && strings.HasSuffix(rest, "/reactions"):
		msgID := strings.TrimSuffix(rest, "/reactions")
		msgID = strings.Trim(msgID, "/")
		_, err := t.clients.messaging.RemoveReaction(ctx, &messagingv1.RemoveReactionRequest{
			MessageId: msgID,
			Emoji:     queryFirst(r, "emoji"),
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodGet && rest == "read-state":
		resp, err := t.clients.messaging.GetReadState(ctx, &messagingv1.GetReadStateRequest{
			Chat: &chatv1.ChatRef{Id: queryFirst(r, "chat_id")},
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "prekeys":
		req := &messagingv1.UploadPreKeyBundleRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		_, err := t.clients.messaging.UploadPreKeyBundle(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodGet && rest == "prekeys":
		resp, err := t.clients.messaging.GetPreKeyBundle(ctx, &messagingv1.GetPreKeyBundleRequest{
			ProfileId: queryFirst(r, "profile_id"),
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
