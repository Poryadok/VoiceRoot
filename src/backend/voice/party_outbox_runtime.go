package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"voice/backend/voice/internal/rosteroutbox"
)

func runRosterOutbox(ctx context.Context, pool *pgxpool.Pool, publisher rosteroutbox.Publisher, logger *slog.Logger) {
	relay := &rosteroutbox.Relay{Store: &rosteroutbox.Store{Pool: pool}, Publisher: publisher, OnError: func(err error) { logger.Warn("Voice roster outbox delivery failed", slog.String("error", err.Error())) }}
	if err := relay.Run(ctx); err != nil && ctx.Err() == nil {
		logger.Error("Voice roster outbox stopped", slog.String("error", err.Error()))
	}
}
