package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/rosterruntime"
)

// The compatibility wrapper preserves the assembled caller signature. The
// separate native root uses Prepare and Start at its distinct admission steps.
// This wrapper remains dormant until its complete caller is qualified.
func startVoiceRosterRuntime(parent context.Context, pool *pgxpool.Pool, natsURL string, events mmevents.Publisher, metrics *prometheus.Registry, logger *slog.Logger) (func(), error) {
	prepared, err := rosterruntime.Prepare(parent, pool, natsURL, events, metrics, logger)
	if err != nil {
		return nil, err
	}
	if err := prepared.Start(); err != nil {
		prepared.Close()
		return nil, err
	}
	return prepared.Close, nil
}
