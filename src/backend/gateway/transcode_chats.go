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

	case r.Method == http.MethodPost && rest == "":
		req := &chatv1.CreateChatRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.chat.CreateChat(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPatch && rest != "" && !strings.Contains(rest, "/"):
		req := &chatv1.UpdateChatRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.ChatId = rest
		resp, err := t.clients.chat.UpdateChat(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && strings.HasSuffix(rest, "/members"):
		chatID := strings.TrimSuffix(rest, "/members")
		chatID = strings.Trim(chatID, "/")
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		resp, err := t.clients.chat.ListMembers(ctx, &chatv1.ListMembersRequest{
			ChatId: chatID,
			Page:   page,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/leave"):
		chatID := strings.TrimSuffix(rest, "/leave")
		chatID = strings.Trim(chatID, "/")
		_, err := t.clients.chat.LeaveChat(ctx, &chatv1.LeaveChatRequest{ChatId: chatID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodPost && strings.HasSuffix(rest, "/members"):
		chatID := strings.TrimSuffix(rest, "/members")
		chatID = strings.Trim(chatID, "/")
		req := &chatv1.AddMembersRequest{ChatId: chatID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.ChatId == "" {
			req.ChatId = chatID
		}
		_, err := t.clients.chat.AddMembers(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case r.Method == http.MethodDelete && strings.Contains(rest, "/members/"):
		parts := strings.SplitN(rest, "/members/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return false
		}
		_, err := t.clients.chat.RemoveMember(ctx, &chatv1.RemoveMemberRequest{
			ChatId:    parts[0],
			ProfileId: parts[1],
		})
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
