package matcher

import (
	"context"

	"voice/backend/matchmaking/internal/mmevents"
)

// MMEventsAdapter bridges matcher events to the JetStream publisher.
type MMEventsAdapter struct {
	Pub mmevents.Publisher
}

// PublishMatchFound implements EventPublisher.
func (a MMEventsAdapter) PublishMatchFound(ctx context.Context, ev MatchFoundEvent) error {
	if a.Pub == nil {
		return nil
	}
	profileIDs := make([]string, len(ev.ProfileIDs))
	for i, id := range ev.ProfileIDs {
		profileIDs[i] = id.String()
	}
	sessionIDs := make([]string, len(ev.SessionIDs))
	for i, id := range ev.SessionIDs {
		sessionIDs[i] = id.String()
	}
	return a.Pub.PublishMatchFound(ctx, mmevents.MatchFoundEvent{
		MatchID:    ev.MatchID.String(),
		GameID:     ev.GameID.String(),
		Mode:       ev.Mode,
		Region:     ev.Region,
		ProfileIDs: profileIDs,
		SessionIDs: sessionIDs,
	})
}
