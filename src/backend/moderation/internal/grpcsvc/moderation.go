package grpcsvc

import (
	moderationv1 "voice.app/voice/moderation/v1"

	"voice/backend/moderation/internal/store"
)

// ModerationGRPC implements ModerationService (Phase 11 — RPC handlers pending).
type ModerationGRPC struct {
	moderationv1.UnimplementedModerationServiceServer
	Reports *store.ReportStore
}
