package dispatch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAutocompleteCompletionIsBoundToBot(t *testing.T) {
	hub := NewHub()
	requestID := hub.NextAutocompleteRequestID()
	cacheKey := AutocompleteCacheKey("bot-a", "chat", "ping", "query", "a")
	hub.RegisterAutocomplete(requestID, cacheKey)
	choices := []AutocompleteChoice{{Name: "A", Value: "a"}}
	require.False(t, hub.CompleteAutocomplete(requestID, "bot-b", choices))
	_, cached := hub.GetAutocompleteChoices(cacheKey)
	require.False(t, cached)
	require.True(t, hub.CompleteAutocomplete(requestID, "bot-a", choices))
}
