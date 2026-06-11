package fcm

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"voice/backend/notification/internal/store"
)

// NoopSender logs push attempts when FCM credentials are not configured.
type NoopSender struct {
	Logger *slog.Logger
}

func (n *NoopSender) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload PushPayload) error {
	_ = ctx
	if n != nil && n.Logger != nil {
		n.Logger.Debug("fcm noop send",
			slog.String("profile_id", profileID.String()),
			slog.String("type", payload.Data["type"]),
			slog.Bool("has_token", token.Token != ""),
		)
	}
	return nil
}
