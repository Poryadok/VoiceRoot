package grouping_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/push"
)

func TestApplyToPayload_NilStoreStillSetsCounter(t *testing.T) {
	profileID := uuid.New()
	payload := push.Payload{Body: "Only"}
	require.NoError(t, grouping.ApplyToPayload(context.Background(), nil, profileID, "chat-x", "Only", &payload))
	require.Equal(t, 1, payload.Counter)
}

func TestApplyToPayload_IncrementsCounter(t *testing.T) {
	store := grouping.NewMemoryStore()
	profileID := uuid.New()
	chatID := uuid.NewString()
	payload := push.Payload{Body: "First"}
	require.NoError(t, grouping.ApplyToPayload(context.Background(), store, profileID, chatID, "First", &payload))
	require.Equal(t, 1, payload.Counter)
	require.NotEmpty(t, payload.CollapseTag)

	payload2 := push.Payload{Body: "Second"}
	require.NoError(t, grouping.ApplyToPayload(context.Background(), store, profileID, chatID, "Second", &payload2))
	require.Equal(t, 2, payload2.Counter)
	require.Equal(t, "Second", payload2.Body)
}
