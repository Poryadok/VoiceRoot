package grpcsvc

import (
	"context"

	"github.com/google/uuid"

	moderationv1 "voice.app/voice/moderation/v1"

	"voice/backend/moderation/internal/store"
)

// ModerationGRPC implements ModerationService.
type ModerationGRPC struct {
	moderationv1.UnimplementedModerationServiceServer
	Reports              *store.ReportStore
	Sanctions            *store.SanctionStore
	Appeals              *store.AppealStore
	AuditLog             *store.AuditLogStore
	AutoMod              *store.AutoModStore
	PlatformAudienceSize int64
	Auth                 AccountStatusClient
	Users                ProfileAccountLookup
	Analytics            interface {
		Publish(ctx context.Context, subject, sourceService, eventType string, props map[string]any) error
	}
}

// ProfileAccountLookup resolves profile_id to account_id (User Service).
type ProfileAccountLookup interface {
	AccountIDForProfile(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error)
}
