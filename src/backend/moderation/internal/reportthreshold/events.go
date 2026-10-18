package reportthreshold

import (
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/moderation/internal/store"
)

// Events serializes the existing domain event once, inside the case transaction.
// It contains no reporter identity or audience member list.
func Events(c store.ReportThresholdCase) ([]store.ReportThresholdEvent, error) {
	id := uuid.New()
	envelope := &eventsv1.ModerationStreamEvent{EventId: id.String(), OccurredAt: timestamppb.New(c.CreatedAt), Payload: &eventsv1.ModerationStreamEvent_SanctionApplied{SanctionApplied: &eventsv1.SanctionApplied{SanctionId: c.SanctionID.String(), TargetAccountId: c.AccountID.String(), Type: "shadow_ban"}}}
	payload, err := (proto.MarshalOptions{Deterministic: true}).Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return []store.ReportThresholdEvent{{ID: id, Kind: "domain.sanction_applied", Subject: "moderation.sanction_applied", Payload: payload}}, nil
}
