package delivery

import (
	"fmt"

	"github.com/google/uuid"
)

// GroupingKey returns a stable collapse tag for chat-grouped pushes.
func GroupingKey(profileID uuid.UUID, chatID string) string {
	return fmt.Sprintf("push:group:%s:%s", profileID.String(), chatID)
}

// NextGroupingState updates counter/body for subsequent messages in the same chat.
// collapseTag must be the stable GroupingKey for the chat; pass empty when continuing from prev.
func NextGroupingState(collapseTag string, prev *GroupingState, body string) GroupingState {
	if prev == nil {
		return GroupingState{
			CollapseTag: collapseTag,
			Counter:     1,
			LastBody:    body,
		}
	}
	tag := prev.CollapseTag
	if tag == "" {
		tag = collapseTag
	}
	return GroupingState{
		CollapseTag: tag,
		Counter:     prev.Counter + 1,
		LastBody:    body,
	}
}
