package main

import (
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonv1 "voice.app/voice/common/v1"
	searchv1 "voice.app/voice/search/v1"

	chatv1 "voice.app/voice/chat/v1"
)

func (t *transcoder) serveSearch(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	rest = strings.TrimPrefix(rest, "/")

	switch {
	case r.Method == http.MethodGet && rest == "in-chat":
		page := searchPageFromQuery(r)
		chatID := queryFirst(r, "chat_id")
		if chatID == "" {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "chat_id required"))
			return true
		}
		resp, err := t.clients.search.SearchInChat(ctx, &searchv1.SearchInChatRequest{
			Chat:  &chatv1.ChatRef{Id: chatID},
			Query: queryFirst(r, "q"),
			Page:  page,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "global":
		page := searchPageFromQuery(r)
		if page.PageSize == 0 {
			page.PageSize = 20
		}
		resp, err := t.clients.search.SearchGlobal(ctx, &searchv1.SearchGlobalRequest{
			Query: queryFirst(r, "q"),
			Page:  page,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "users":
		resp, err := t.clients.search.SearchUsers(ctx, &searchv1.SearchUsersRequest{
			Query: queryFirst(r, "q"),
			Limit: parseInt32Query(queryFirst(r, "limit")),
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "spaces":
		page := searchPageFromQuery(r)
		if page.PageSize == 0 {
			page.PageSize = 20
		}
		resp, err := t.clients.search.SearchSpaces(ctx, &searchv1.SearchSpacesRequest{
			Query: queryFirst(r, "q"),
			Page:  page,
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

func searchPageFromQuery(r *http.Request) *commonv1.CursorPageRequest {
	page := &commonv1.CursorPageRequest{}
	_ = decodeQueryJSON(page, queryFirst(r, "page"))
	if page.PageSize == 0 {
		if n := queryFirst(r, "page_size"); n != "" {
			page.PageSize = parseInt32Query(n)
		}
	}
	if page.Cursor == "" {
		page.Cursor = queryFirst(r, "cursor")
	}
	return page
}
