package email

import (
	"context"
	"log/slog"
)

// NoopSender logs email sends without contacting a provider.
type NoopSender struct {
	Logger *slog.Logger
}

func (n *NoopSender) Send(ctx context.Context, msg Message) error {
	_ = ctx
	if n != nil && n.Logger != nil {
		n.Logger.Debug("email noop send", slog.String("to", msg.To), slog.String("subject", msg.Subject))
	}
	return nil
}
