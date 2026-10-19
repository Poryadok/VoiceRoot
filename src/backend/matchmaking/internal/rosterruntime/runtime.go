// Package rosterruntime binds the fixed roster durable separately from starting
// domain processing. Its lifecycle never establishes installation admission.
package rosterruntime

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/rosterconsume"
	"voice/backend/matchmaking/internal/store"
)

var ErrUnavailable = errors.New("voice roster runtime unavailable")

type cancellationPublisher interface {
	PublishRosterCancellation(context.Context, uuid.UUID, []byte) error
}

type Runtime struct {
	mu              sync.Mutex
	started, closed bool
	closeOnce       sync.Once
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
	consumer        *rosterconsume.Consumer
	store           *store.MatchStore
	publisher       cancellationPublisher
	logger          *slog.Logger
	metrics         *prometheus.Registry
	gauge           prometheus.Gauge
}

// Prepare validates schema and binds the pre-provisioned durable. It starts no
// goroutine, Fetch, ACK, SQL mutation or cancellation dispatch. The fixed native
// root must separately qualify installation admission before calling Start.
func Prepare(parent context.Context, pool *pgxpool.Pool, natsURL string, events mmevents.Publisher, metrics *prometheus.Registry, logger *slog.Logger) (*Runtime, error) {
	publisher, ok := events.(cancellationPublisher)
	if parent == nil || parent.Err() != nil || pool == nil || metrics == nil || strings.TrimSpace(natsURL) == "" || !ok || nilPublisher(publisher) {
		return nil, ErrUnavailable
	}
	var schema bool
	if err := pool.QueryRow(parent, `SELECT to_regclass('matchmaking_voice_roster_inbox') IS NOT NULL AND to_regclass('matchmaking_voice_roster_contract_conflicts') IS NOT NULL AND to_regclass('matchmaking_voice_roster_cancellation_outbox') IS NOT NULL AND to_regclass('matchmaking_party_search_queue_intents') IS NOT NULL`).Scan(&schema); err != nil || !schema {
		return nil, ErrUnavailable
	}
	if logger == nil {
		logger = slog.Default()
	}
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "matchmaking_voice_roster_contract_mismatches", Help: "Persisted Voice roster event identity or payload contract conflicts requiring operator intervention."})
	if err := metrics.Register(gauge); err != nil {
		return nil, ErrUnavailable
	}
	st := &store.MatchStore{Pool: pool}
	consumer, err := rosterconsume.Start(natsURL, st, logger)
	if err != nil {
		metrics.Unregister(gauge)
		return nil, ErrUnavailable
	}
	if parent.Err() != nil {
		consumer.Close()
		metrics.Unregister(gauge)
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithCancel(parent)
	r := &Runtime{ctx: ctx, cancel: cancel, consumer: consumer, store: st, publisher: publisher, logger: logger, metrics: metrics, gauge: gauge}
	consumer.OnContractMismatch = r.refresh
	return r, nil
}

func nilPublisher(p cancellationPublisher) bool {
	if p == nil {
		return true
	}
	v := reflect.ValueOf(p)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// Start is idempotent within this prepared lifetime. Close is irreversible.
// Processing itself is not a native admission/readiness certificate.
func (r *Runtime) Start() error {
	if r == nil {
		return ErrUnavailable
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.ctx == nil || r.ctx.Err() != nil || r.consumer == nil {
		return ErrUnavailable
	}
	if r.started {
		return nil
	}
	r.started = true
	r.wg.Add(3)
	go func() {
		defer r.wg.Done()
		if err := r.consumer.Run(r.ctx); err != nil && r.ctx.Err() == nil {
			r.logger.Error("Voice roster durable consumer stopped", slog.Any("error", err))
		}
	}()
	go func() {
		defer r.wg.Done()
		r.refresh()
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-r.ctx.Done():
				return
			case <-tick.C:
				r.refresh()
			}
		}
	}()
	go func() {
		defer r.wg.Done()
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-r.ctx.Done():
				return
			case <-tick.C:
				attempt, done := context.WithTimeout(r.ctx, 8*time.Second)
				err := dispatchCancellation(attempt, r.store, r.publisher)
				done()
				if err != nil && r.ctx.Err() == nil {
					r.logger.Warn("Voice roster cancellation delivery pending", slog.Any("error", err))
				}
			}
		}
	}()
	return nil
}

func (r *Runtime) refresh() {
	read, done := context.WithTimeout(r.ctx, 3*time.Second)
	count, err := r.store.VoiceRosterContractMismatchCount(read)
	done()
	if err == nil {
		r.gauge.Set(float64(count))
	} else if r.ctx.Err() == nil {
		r.logger.Warn("Voice roster contract alert query failed", slog.Any("error", err))
	}
}

// Close cancels processing and joins owned goroutines before returning, while
// preserving durable cancellation completion/readback semantics.
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed = true
		if r.cancel != nil {
			r.cancel()
		}
		r.mu.Unlock()
		if r.consumer != nil {
			r.consumer.Close()
		}
		r.wg.Wait()
		if r.metrics != nil && r.gauge != nil {
			r.metrics.Unregister(r.gauge)
		}
	})
}

func dispatchCancellation(ctx context.Context, st *store.MatchStore, publisher cancellationPublisher) error {
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
