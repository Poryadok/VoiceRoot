package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/messaging/internal/store"
)

type filePreviewStore interface {
	PersistFilePreview(context.Context, *eventsv1.FileProcessed) error
	FilePreviewPending(context.Context) ([]store.FilePreviewWork, error)
	RefreshFilePreview(context.Context, store.FilePreviewWork, *uuid.UUID) (int, error)
	DeferFilePreview(context.Context, store.FilePreviewWork, bool) error
}

func runFilePreviewPending(ctx context.Context, messages filePreviewStore, policy musicChatPolicy, logger *slog.Logger) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		work, err := messages.FilePreviewPending(ctx)
		if err != nil && ctx.Err() == nil {
			logger.WarnContext(ctx, "file preview pending pass unavailable")
		}
		for _, w := range work {
			attempt, cancel := context.WithTimeout(ctx, 10*time.Second)
			scope, err := policy.Load(attempt, w.ChatID)
			if errors.Is(err, store.ErrChatNotFound) {
				err = messages.DeferFilePreview(attempt, w, true)
			} else if err == nil && scope != nil {
				_, err = messages.RefreshFilePreview(attempt, w, scope.SpaceID)
			} else if err == nil {
				err = errors.New("destination policy unavailable")
			}
			if err != nil {
				_ = messages.DeferFilePreview(attempt, w, false)
				if ctx.Err() == nil {
					logger.WarnContext(ctx, "file preview destination refresh pending")
				}
			}
			cancel()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
