package grpcsvc

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/search/internal/store"
)

// MessageStoreAdapter wraps MessageSearchStore for SearchGRPC.
type MessageStoreAdapter struct {
	*store.MessageSearchStore
}

func (a *MessageStoreAdapter) SearchInChat(ctx context.Context, chatID uuid.UUID, query string, cursor *string, limit int) ([]MessageHit, string, error) {
	hits, next, err := a.MessageSearchStore.SearchInChat(ctx, chatID, query, cursor, limit)
	return toMessageHits(hits), next, err
}

func (a *MessageStoreAdapter) SearchGlobalMessages(ctx context.Context, _ uuid.UUID, query string, cursor *string, limit int, accessibleChatIDs []uuid.UUID) ([]MessageHit, string, error) {
	hits, next, err := a.MessageSearchStore.SearchGlobalMessages(ctx, query, cursor, limit, accessibleChatIDs)
	return toMessageHits(hits), next, err
}

func toMessageHits(in []store.MessageHit) []MessageHit {
	out := make([]MessageHit, 0, len(in))
	for _, h := range in {
		out = append(out, MessageHit{
			MessageID: h.MessageID,
			ChatID:    h.ChatID,
			Snippet:   h.Snippet,
			Score:     h.Score,
		})
	}
	return out
}

// ProfileStoreAdapter wraps profile search methods.
type ProfileStoreAdapter struct {
	*store.ProfileSpaceSearchStore
}

func (a *ProfileStoreAdapter) SearchProfiles(ctx context.Context, viewer uuid.UUID, query string, excludeBlocked []uuid.UUID, limit int) ([]uuid.UUID, error) {
	hits, err := a.ProfileSpaceSearchStore.SearchProfiles(ctx, viewer, query, excludeBlocked, limit)
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.ProfileID)
	}
	return out, nil
}

// SpaceStoreAdapter wraps space search methods.
type SpaceStoreAdapter struct {
	*store.ProfileSpaceSearchStore
}

func (a *SpaceStoreAdapter) SearchSpaces(ctx context.Context, query string, cursor *string, limit int) ([]uuid.UUID, string, error) {
	hits, next, err := a.ProfileSpaceSearchStore.SearchSpaces(ctx, query, cursor, limit)
	if err != nil {
		return nil, "", err
	}
	out := make([]uuid.UUID, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.SpaceID)
	}
	return out, next, nil
}

// ProjectionChatAccess searches chat_search_documents.
type ProjectionChatAccess struct {
	Store      *store.ProfileSpaceSearchStore
	Accessible func(ctx context.Context, viewer uuid.UUID) ([]uuid.UUID, error)
}

func (a *ProjectionChatAccess) AccessibleChatIDs(ctx context.Context, viewer uuid.UUID) ([]uuid.UUID, error) {
	if a.Accessible == nil {
		return nil, nil
	}
	return a.Accessible(ctx, viewer)
}

func (a *ProjectionChatAccess) SearchChats(ctx context.Context, query string, limit int) ([]uuid.UUID, error) {
	if a.Store == nil {
		return nil, nil
	}
	return a.Store.SearchChats(ctx, query, limit)
}
