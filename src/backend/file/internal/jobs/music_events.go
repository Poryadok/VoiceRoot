package jobs

import (
	"context"
	"log/slog"
	"time"
)

type musicOutbox interface {
	DispatchMusicEvent(context.Context, func(context.Context, []byte) error) (bool, error)
}
type musicPublisher interface {
	PublishMusicEvent(context.Context, []byte) error
}

func StartMusicEventWorker(ctx context.Context, store musicOutbox, publisher musicPublisher, logger *slog.Logger) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			for i := 0; i < 16; i++ {
				published, err := store.DispatchMusicEvent(ctx, publisher.PublishMusicEvent)
				if err != nil {
					logger.WarnContext(ctx, "music processing event publication pending")
					break
				}
				if !published {
					break
				}
			}
		}
	}()
}
