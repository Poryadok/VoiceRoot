// Package reportintake activates opaque receipts using protected owner authority.
package reportintake

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	analyticsv1 "voice.app/voice/analytics/v1"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/moderation/internal/store"
)

type TargetResolver interface {
	Resolve(context.Context, uuid.UUID, string, uuid.UUID) (string, uuid.UUID, error)
}

type Worker struct {
	Reports   *store.ReportStore
	Targets   TargetResolver
	Validated store.ReportValidatedHook
	Publish   func(context.Context, store.IntakeEvent) error
}

// ValidateOne performs no remote calls while holding a SQL transaction.
func (w *Worker) ValidateOne(ctx context.Context) (bool, error) {
	if w == nil || w.Reports == nil || w.Targets == nil {
		return false, errors.New("report validation unavailable")
	}
	claim, err := w.Reports.ClaimValidation(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	ownerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	state, account, ownerErr := w.Targets.Resolve(ownerCtx, claim.ReportID, claim.TargetType, claim.TargetID)
	cancel()
	if ownerErr != nil || (state != "LIVE" && state != "MISSING" && state != "DELETED") || (state != "LIVE" && account != uuid.Nil) || (state == "LIVE" && ((claim.TargetType == "user" && account == uuid.Nil) || (claim.TargetType != "user" && account != uuid.Nil))) {
		return true, w.Reports.RetryValidation(ctx, claim)
	}
	var events []store.IntakeEvent
	if state == "LIVE" && !claim.LegacyPublication {
		events, err = createdEvents(claim)
		if err != nil {
			_ = w.Reports.RetryValidation(ctx, claim)
			return true, err
		}
	}
	_, err = w.Reports.CompleteValidation(ctx, claim, state, account, events, w.Validated)
	if err != nil {
		_ = w.Reports.RetryValidation(ctx, claim)
	}
	return true, err
}

func createdEvents(claim *store.ValidationClaim) ([]store.IntakeEvent, error) {
	domainID, analyticsID := uuid.New(), uuid.New()
	domain, err := proto.Marshal(&eventsv1.ModerationStreamEvent{EventId: domainID.String(), OccurredAt: timestamppb.New(claim.CreatedAt), Payload: &eventsv1.ModerationStreamEvent_ReportCreated{ReportCreated: &eventsv1.ReportCreated{ReportId: claim.ReportID.String(), ReporterProfileId: claim.ReporterProfileID.String()}}})
	if err != nil {
		return nil, err
	}
	props, err := json.Marshal(map[string]string{"target_type": claim.TargetType, "category": claim.Category})
	if err != nil {
		return nil, err
	}
	analytics, err := protojson.Marshal(&analyticsv1.AnalyticsEvent{EventId: analyticsID.String(), EventType: "report_created", SourceService: "moderation", Timestamp: timestamppb.New(claim.CreatedAt), PropertiesJson: string(props)})
	if err != nil {
		return nil, err
	}
	return []store.IntakeEvent{{ID: domainID, Kind: "domain.report_created", Subject: "moderation.report_created", Payload: domain}, {ID: analyticsID, Kind: "analytics.report_created", Subject: "analytics.moderation.report_created", Payload: analytics}}, nil
}

// Run stops before its caller closes the database and publisher connections.
func (w *Worker) Run(ctx context.Context, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		worked, err := w.ValidateOne(ctx)
		if err != nil && ctx.Err() == nil {
			logger.Warn("report validation attempt unavailable")
		}
		delivered, publishErr := w.Reports.DeliverIntakeEvent(ctx, w.Publish)
		if publishErr != nil && ctx.Err() == nil {
			logger.Warn("report intake publication unavailable")
		}
		pause := time.Second
		if worked || delivered {
			pause = 10 * time.Millisecond
		}
		timer.Reset(pause)
	}
}
