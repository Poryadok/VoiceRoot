package rosterruntime

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"log/slog"
	"reflect"
	"voice/backend/matchmaking/internal/mmevents"
)

type capturedCancellationPublisher interface {
	PublishCapturedCancellation(context.Context, uuid.UUID, []byte) error
}
type nativeCancellationAdapter struct {
	mmevents.Publisher
	captured capturedCancellationPublisher
}

func (p nativeCancellationAdapter) PublishRosterCancellation(ctx context.Context, id uuid.UUID, raw []byte) error {
	return p.captured.PublishCapturedCancellation(ctx, id, raw)
}
func nativeCancellationEvents(events mmevents.Publisher) (mmevents.Publisher, error) {
	captured, ok := events.(capturedCancellationPublisher)
	if !ok || captured == nil {
		return nil, ErrUnavailable
	}
	value := reflect.ValueOf(captured)
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Chan, reflect.Func, reflect.Map, reflect.Slice:
		if value.IsNil() {
			return nil, ErrUnavailable
		}
	}
	return nativeCancellationAdapter{Publisher: events, captured: captured}, nil
}

// PrepareNative requires the concrete durable publisher for both roster reset
// and ordinary cancellation facts. It preserves Prepare's no-delivery/no-domain
// mutation phase; Start still belongs after native admission qualification.
func PrepareNative(parent context.Context, pool *pgxpool.Pool, natsURL string, events mmevents.Publisher, metrics *prometheus.Registry, logger *slog.Logger, options ...nats.Option) (*Runtime, error) {
	native, err := nativeCancellationEvents(events)
	if err != nil {
		return nil, err
	}
	return Prepare(parent, pool, natsURL, native, metrics, logger, options...)
}
