package main

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"log/slog"
	"time"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/rosterconsume"
	"voice/backend/matchmaking/internal/store"
)

type rosterCancellationPublisher interface {
	PublishRosterCancellation(context.Context, uuid.UUID, []byte) error
}

// startVoiceRosterRuntime is called by the jointly assembled A3 startup. Neither
// a missing schema, a noop publisher nor a missing fixed durable is readiness.
func startVoiceRosterRuntime(parent context.Context, pool *pgxpool.Pool, natsURL string, events mmevents.Publisher, metrics *prometheus.Registry, logger *slog.Logger) (func(), error) {
	publisher, ok := events.(rosterCancellationPublisher)
	if !ok || pool == nil || metrics == nil {
		return nil, errors.New("durable Voice roster dependencies unavailable")
	}
	var ready bool
	if err := pool.QueryRow(parent, `SELECT to_regclass('matchmaking_voice_roster_inbox') IS NOT NULL AND to_regclass('matchmaking_voice_roster_contract_conflicts') IS NOT NULL AND to_regclass('matchmaking_voice_roster_cancellation_outbox') IS NOT NULL AND to_regclass('matchmaking_party_search_queue_intents') IS NOT NULL`).Scan(&ready); err != nil || !ready {
		return nil, errors.New("Voice roster inbox schema unavailable")
	}
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "matchmaking_voice_roster_contract_mismatches", Help: "Persisted Voice roster event identity or payload contract conflicts requiring operator intervention."})
	if err := metrics.Register(gauge); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	st := &store.MatchStore{Pool: pool}
	refresh := func() {
		read, done := context.WithTimeout(ctx, 3*time.Second)
		count, err := st.VoiceRosterContractMismatchCount(read)
		done()
		if err == nil {
			gauge.Set(float64(count))
		} else if ctx.Err() == nil {
			logger.Warn("Voice roster contract alert query failed", slog.Any("error", err))
		}
	}
	consumer, err := rosterconsume.Start(natsURL, st, logger)
	if err != nil {
		cancel()
		return nil, err
	}
	consumer.OnContractMismatch = refresh
	go func() {
		if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("Voice roster durable consumer stopped", slog.Any("error", err))
		}
	}()
	go func() {
		refresh()
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				refresh()
			}
		}
	}()
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				attempt, done := context.WithTimeout(ctx, 8*time.Second)
				err := dispatchRosterCancellation(attempt, st, publisher)
				done()
				if err != nil && ctx.Err() == nil {
					logger.Warn("Voice roster cancellation delivery pending", slog.Any("error", err))
				}
			}
		}
	}()
	return func() { cancel(); consumer.Close() }, nil
}

func dispatchRosterCancellation(ctx context.Context, st *store.MatchStore, publisher rosterCancellationPublisher) error {
	item, err := st.ClaimVoiceRosterCancellation(ctx)
	if err != nil || item == nil {
		return err
	}
	send, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = publisher.PublishRosterCancellation(send, item.EventID, item.Payload)
	cancel()
	finish, done := context.WithTimeout(context.Background(), 2*time.Second)
	defer done()
	return errors.Join(err, st.FinishVoiceRosterCancellation(finish, *item, err == nil))
}
