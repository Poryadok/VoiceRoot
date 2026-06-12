package apns

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

// NoopSender logs push attempts when APNs credentials are not configured.
type NoopSender struct {
	Logger *slog.Logger
}

func (n *NoopSender) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, p push.Payload) error {
	_ = ctx
	if n != nil && n.Logger != nil {
		n.Logger.Debug("apns noop send",
			slog.String("profile_id", profileID.String()),
			slog.String("type", p.Data["type"]),
			slog.Bool("has_token", token.Token != ""),
		)
	}
	return nil
}
