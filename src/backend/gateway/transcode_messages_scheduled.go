package main

import (
	"context"
	"net/http"
	"strings"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// serveScheduledMessages exposes the documented owner-scoped Messaging lifecycle
// RPCs through the same authenticated Gateway context as ordinary messages.
func (t *transcoder) serveScheduledMessages(w http.ResponseWriter, r *http.Request, ctx context.Context, rest string) bool {
	switch {
	case r.Method == http.MethodGet && rest == "scheduled":
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		resp, err := t.clients.messaging.ListScheduledMessages(ctx, &messagingv1.ListScheduledMessagesRequest{
			Chat: &chatv1.ChatRef{Id: queryFirst(r, "chat_id")},
			Page: page,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case strings.HasPrefix(rest, "scheduled/"):
		idAndAction := strings.TrimPrefix(rest, "scheduled/")
		if r.Method == http.MethodPatch && idAndAction != "" && !strings.Contains(idAndAction, "/") {
			req := &messagingv1.UpdateScheduledMessageRequest{}
			if err := readProtoJSON(r, req); err != nil {
				writeGRPCError(w, err)
				return true
			}
			// The path is the sole source of the owner-scoped mutation ID.
			req.ScheduledMessageId = idAndAction
			resp, err := t.clients.messaging.UpdateScheduledMessage(ctx, req)
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		}
		if r.Method == http.MethodDelete && idAndAction != "" && !strings.Contains(idAndAction, "/") {
			_, err := t.clients.messaging.CancelScheduledMessage(ctx, &messagingv1.CancelScheduledMessageRequest{
				ScheduledMessageId: idAndAction,
			})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true
		}
		const sendNowSuffix = "/send-now"
		if r.Method == http.MethodPost && strings.HasSuffix(idAndAction, sendNowSuffix) {
			id := strings.TrimSuffix(idAndAction, sendNowSuffix)
			if id == "" || strings.Contains(id, "/") {
				return false
			}
			resp, err := t.clients.messaging.SendScheduledMessageNow(ctx, &messagingv1.SendScheduledMessageNowRequest{
				ScheduledMessageId: id,
			})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		}
	}
	return false
}
