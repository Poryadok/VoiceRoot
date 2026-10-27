package dispatch

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
)

func TestAutocompleteCompletionIsBoundToBot(t *testing.T) {
	hub := NewHub()
	botID := uuid.NewString()
	session := store.Session{BotID: uuid.MustParse(botID), Epoch: 1}
	requestID := hub.NextAutocompleteRequestID()
	cacheKey := AutocompleteCacheKey(botID, "chat", "ping", "query", "a", 1)
	hub.RegisterAutocomplete(requestID, cacheKey, session)
	choices := []AutocompleteChoice{{Name: "A", Value: "a"}}
	require.False(t, hub.CompleteAutocomplete(requestID, uuid.NewString(), session, choices))
	_, cached := hub.GetAutocompleteChoices(cacheKey, session)
	require.False(t, cached)
	require.True(t, hub.CompleteAutocomplete(requestID, botID, session, choices))
}
