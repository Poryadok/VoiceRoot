package main

import (
	"context"
	"net/http"
	"strings"

	commonv1 "voice.app/voice/common/v1"
	storyv1 "voice.app/voice/story/v1"
)

func (t *transcoder) serveStories(w http.ResponseWriter, r *http.Request, rest string) bool {
	if t.clients.story == nil {
		return false
	}
	rest = strings.TrimPrefix(rest, "/")
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case rest == "feed" && r.Method == http.MethodGet:
		page := &commonv1.CursorPageRequest{}
		if c := queryFirst(r, "cursor"); c != "" {
			page.Cursor = c
		}
		if n := queryFirst(r, "limit"); n != "" {
			page.PageSize = parseInt32Query(n)
		}
		if page.PageSize == 0 {
			if n := queryFirst(r, "page_size"); n != "" {
				page.PageSize = parseInt32Query(n)
			}
		}
		resp, err := t.clients.story.GetStoryFeed(ctx, &storyv1.GetStoryFeedRequest{Page: page})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case rest == "archive" && r.Method == http.MethodGet:
		req := &storyv1.GetArchiveRequest{}
		if pid := queryFirst(r, "profile_id"); pid != "" {
			req.ProfileId = pid
		}
		resp, err := t.clients.story.GetArchive(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case rest == "looking-for-party" && r.Method == http.MethodPost:
		req := &storyv1.CreateLookingForPartyRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.story.CreateLookingForParty(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case rest == "highlights" && r.Method == http.MethodGet:
		req := &storyv1.GetHighlightsRequest{ProfileId: queryFirst(r, "profile_id")}
		resp, err := t.clients.story.GetHighlights(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case rest == "highlights" && r.Method == http.MethodPost:
		req := &storyv1.CreateHighlightRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.story.CreateHighlight(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case strings.HasPrefix(rest, "highlights/"):
		return t.serveStoryHighlights(w, r, ctx, strings.TrimPrefix(rest, "highlights/"))

	case strings.HasPrefix(rest, "profiles/"):
		sub := strings.TrimPrefix(rest, "profiles/")
		parts := strings.Split(sub, "/")
		if len(parts) == 1 && r.Method == http.MethodGet {
			resp, err := t.clients.story.GetProfileStories(ctx, &storyv1.GetProfileStoriesRequest{
				ProfileId: parts[0],
			})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		}
		return false

	case rest == "" && r.Method == http.MethodPost:
		req := &storyv1.CreateStoryRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.story.CreateStory(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	default:
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || parts[0] == "" {
			return false
		}
		storyID := parts[0]
		switch {
		case len(parts) == 1 && r.Method == http.MethodGet:
			resp, err := t.clients.story.GetStory(ctx, &storyv1.GetStoryRequest{StoryId: storyID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true

		case len(parts) == 1 && r.Method == http.MethodDelete:
			_, err := t.clients.story.DeleteStory(ctx, &storyv1.DeleteStoryRequest{StoryId: storyID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true

		case len(parts) == 2 && parts[1] == "views" && r.Method == http.MethodPost:
			req := &storyv1.MarkViewedRequest{StoryId: storyID}
			_ = readProtoJSON(r, req)
			req.StoryId = storyID
			_, err := t.clients.story.MarkViewed(ctx, req)
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true

		case len(parts) == 2 && parts[1] == "viewers" && r.Method == http.MethodGet:
			resp, err := t.clients.story.GetViewers(ctx, &storyv1.GetViewersRequest{StoryId: storyID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true

		case len(parts) == 2 && parts[1] == "reactions" && r.Method == http.MethodPost:
			req := &storyv1.ReactToStoryRequest{StoryId: storyID}
			if err := readProtoJSON(r, req); err != nil {
				writeGRPCError(w, err)
				return true
			}
			req.StoryId = storyID
			_, err := t.clients.story.ReactToStory(ctx, req)
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true

		case len(parts) == 2 && parts[1] == "reply" && r.Method == http.MethodPost:
			req := &storyv1.ReplyToStoryRequest{StoryId: storyID}
			if err := readProtoJSON(r, req); err != nil {
				writeGRPCError(w, err)
				return true
			}
			req.StoryId = storyID
			resp, err := t.clients.story.ReplyToStory(ctx, req)
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

func (t *transcoder) serveStoryHighlights(w http.ResponseWriter, r *http.Request, ctx context.Context, rest string) bool {
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		return false
	}
	highlightID := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodPatch:
		req := &storyv1.UpdateHighlightRequest{HighlightId: highlightID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.HighlightId = highlightID
		resp, err := t.clients.story.UpdateHighlight(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case len(parts) == 1 && r.Method == http.MethodDelete:
		_, err := t.clients.story.DeleteHighlight(ctx, &storyv1.DeleteHighlightRequest{HighlightId: highlightID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case len(parts) == 2 && parts[1] == "stories" && r.Method == http.MethodPost:
		req := &storyv1.AddToHighlightRequest{HighlightId: highlightID}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.HighlightId = highlightID
		_, err := t.clients.story.AddToHighlight(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case len(parts) == 3 && parts[1] == "stories" && r.Method == http.MethodDelete:
		_, err := t.clients.story.RemoveFromHighlight(ctx, &storyv1.RemoveFromHighlightRequest{
			HighlightId: highlightID,
			StoryId:     parts[2],
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}
