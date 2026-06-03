package main

import (
	"net/http"
	"strings"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

func (t *transcoder) serveChats(w http.ResponseWriter, r *http.Request, rest string) bool {
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
		req := &chatv1.ListChatsRequest{Page: page}
		if inbox := queryFirst(r, "inbox"); inbox != "" {
			req.Inbox = &inbox
		}
		resp, err := t.clients.chat.ListChats(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest != "" && !strings.Contains(rest, "/"):
		resp, err := t.clients.chat.GetChat(ctx, &chatv1.GetChatRequest{ChatId: rest})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/accept-request"):
		chatID := strings.TrimSuffix(rest, "/accept-request")
		chatID = strings.Trim(chatID, "/")
		_, err := t.clients.chat.AcceptDMRequest(ctx, &chatv1.AcceptDMRequestRequest{ChatId: chatID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/decline-request"):
		chatID := strings.TrimSuffix(rest, "/decline-request")
		chatID = strings.Trim(chatID, "/")
		_, err := t.clients.chat.DeclineDMRequest(ctx, &chatv1.DeclineDMRequestRequest{ChatId: chatID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodPost && rest == "dm":
		req := &chatv1.CreateDMRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.chat.CreateDM(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && strings.HasPrefix(rest, "dm/"):
		other := strings.TrimPrefix(rest, "dm/")
		resp, err := t.clients.chat.GetDM(ctx, &chatv1.GetDMRequest{OtherProfileId: other})
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
