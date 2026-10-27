package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
)

func (t *transcoder) serveGIFRoutes(w http.ResponseWriter, r *http.Request, rest string) bool {
	if isGIFSelectionRequest(r) {
		t.serveGIFSelection(w, r)
		return true
	}
	if r.Method != http.MethodGet || (rest != "search" && rest != "trending") {
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	q, parseErr := url.ParseQuery(r.URL.RawQuery)
	if parseErr != nil {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "malformed GIF search parameters"))
		return true
	}
	for _, name := range []string{"q", "limit", "cursor"} {
		if len(q[name]) > 1 {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "duplicate GIF search parameter"))
			return true
		}
	}
	var limit int64
	var err error
	if raw, ok := q["limit"]; ok {
		limit, err = strconv.ParseInt(raw[0], 10, 32)
		if err != nil || limit < 0 || limit > 50 {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "GIF limit must be between 0 and 50"))
			return true
		}
	}
	if rest == "search" && strings.TrimSpace(q.Get("q")) == "" {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "GIF search query is required"))
		return true
	}
	if t.clients.chat == nil {
		writeGRPCError(w, status.Error(codes.Unavailable, "GIF search unavailable"))
		return true
	}
	ctx := withGRPCMetadata(r.Context(), r)
	if rest == "search" {
		resp, err := t.clients.chat.SearchGifs(ctx, &chatv1.SearchGifsRequest{Query: q.Get("q"), Limit: int32(limit), Cursor: q.Get("cursor")})
		if err != nil {
			writeGRPCError(w, err)
		} else {
			writeProtoJSON(w, http.StatusOK, resp)
		}
	} else {
		resp, err := t.clients.chat.GetTrendingGifs(ctx, &chatv1.GetTrendingGifsRequest{Limit: int32(limit), Cursor: q.Get("cursor")})
		if err != nil {
			writeGRPCError(w, err)
		} else {
			writeProtoJSON(w, http.StatusOK, resp)
		}
	}
	return true
}
