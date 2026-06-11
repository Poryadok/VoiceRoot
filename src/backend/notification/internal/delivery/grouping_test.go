package delivery_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/delivery"
)

func TestGrouping_SecondMessageIncrementsCounterSameCollapseTag(t *testing.T) {
	profileID := uuid.New()
	chatID := uuid.NewString()
	tag := delivery.GroupingKey(profileID, chatID)
	require.NotEmpty(t, tag)

	first := delivery.NextGroupingState(tag, nil, "hello")
	require.Equal(t, 1, first.Counter)
	require.Equal(t, tag, first.CollapseTag)
	require.Equal(t, "hello", first.LastBody)

	second := delivery.NextGroupingState(tag, &first, "world")
	require.Equal(t, tag, second.CollapseTag, "collapse tag must stay stable for the chat")
	require.Equal(t, 2, second.Counter)
	require.Equal(t, "world", second.LastBody)
}

func TestGrouping_EmptyPrevCollapseTagUsesNewTag(t *testing.T) {
	tag := delivery.GroupingKey(uuid.New(), uuid.NewString())
	prev := delivery.GroupingState{Counter: 2, LastBody: "prior"}
	next := delivery.NextGroupingState(tag, &prev, "next")
	require.Equal(t, tag, next.CollapseTag)
	require.Equal(t, 3, next.Counter)
}
