package grpcsvc

import (
	"context"
	"log/slog"

	"voice/backend/pkg/correlation"
	"voice/backend/pkg/natslog"
)

func (s *ChatGRPC) logPublishError(ctx context.Context, subject string, err error, attrs ...slog.Attr) {
	if err == nil || s == nil || s.Logger == nil {
		return
	}
	natslog.LogPublishError(s.Logger, subject, correlation.FromGRPC(ctx), err, attrs...)
}
