package webhook_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/webhook"
)

func TestDeliverAutocompletePOST_parsesChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload webhook.InteractionPayload
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, "autocomplete", payload.Type)
		require.Equal(t, "stats", payload.CommandName)
		require.Equal(t, "game", payload.OptionName)
		_ = json.NewEncoder(w).Encode(webhook.AutocompleteResponse{
			Choices: []webhook.AutocompleteChoice{
				{Name: "CS2", Value: "cs2"},
			},
		})
	}))
	t.Cleanup(srv.Close)

	choices, err := webhook.DeliverAutocompletePOST(
		context.Background(),
		srv.Client(),
		srv.URL,
		"secret",
		webhook.InteractionPayload{
			CommandName:  "stats",
			OptionName:   "game",
			FocusedValue: "cs",
		},
		2*time.Second,
	)
	require.NoError(t, err)
	require.Len(t, choices, 1)
	require.Equal(t, "CS2", choices[0].Name)
}

func TestDeliverAutocompletePOST_capsAt25(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var choices []webhook.AutocompleteChoice
		for i := 0; i < 30; i++ {
			choices = append(choices, webhook.AutocompleteChoice{Name: "n", Value: "v"})
		}
		_ = json.NewEncoder(w).Encode(webhook.AutocompleteResponse{Choices: choices})
	}))
	t.Cleanup(srv.Close)

	out, err := webhook.DeliverAutocompletePOST(
		context.Background(),
		srv.Client(),
		srv.URL,
		"secret",
		webhook.InteractionPayload{CommandName: "stats", OptionName: "game"},
		2*time.Second,
	)
	require.NoError(t, err)
	require.Len(t, out, 25)
}
