package buffer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/analytics/internal/store"
)

type stubAck struct {
	acked atomic.Int32
	naked atomic.Int32
}

func (s *stubAck) Ack(...nats.AckOpt) error {
	s.acked.Add(1)
	return nil
}

func (s *stubAck) Nak(...nats.AckOpt) error {
	s.naked.Add(1)
	return nil
}

func TestAccumulatorFlushOnMaxEvents(t *testing.T) {
	var flushed atomic.Int32
	acc := New(2, time.Hour, func(ctx context.Context, rows []store.EventRow) error {
		flushed.Add(int32(len(rows)))
		return nil
	}, nil)

	ev := &analyticsv1.AnalyticsEvent{
		EventId:   "e1",
		EventType: "test",
		Timestamp: timestamppb.Now(),
	}
	acc.AppendProto(ev)
	require.Equal(t, 0, int(flushed.Load()))
	acc.AppendProto(&analyticsv1.AnalyticsEvent{EventId: "e2", EventType: "test", Timestamp: timestamppb.Now()})
	require.Eventually(t, func() bool { return flushed.Load() == 2 }, time.Second, 10*time.Millisecond)
}

func TestAccumulatorFlushOnInterval(t *testing.T) {
	var flushed atomic.Int32
	acc := New(1000, 20*time.Millisecond, func(ctx context.Context, rows []store.EventRow) error {
		flushed.Add(int32(len(rows)))
		return nil
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	acc.Start(ctx)
	defer acc.Stop()

	acc.AppendProto(&analyticsv1.AnalyticsEvent{EventId: "e1", EventType: "test", Timestamp: timestamppb.Now()})
	require.Eventually(t, func() bool { return flushed.Load() == 1 }, time.Second, 10*time.Millisecond)
}

func TestAccumulatorAckAfterFlush(t *testing.T) {
	ack := &stubAck{}
	acc := New(1, time.Hour, func(ctx context.Context, rows []store.EventRow) error {
		return nil
	}, nil)
	acc.AppendWithAck(&analyticsv1.AnalyticsEvent{EventId: "e1", EventType: "test", Timestamp: timestamppb.Now()}, ack)
	require.Eventually(t, func() bool { return ack.acked.Load() == 1 }, time.Second, 10*time.Millisecond)
}

func TestAccumulatorNakOnFlushFailure(t *testing.T) {
	ack := &stubAck{}
	acc := New(1, time.Hour, func(ctx context.Context, rows []store.EventRow) error {
		return context.DeadlineExceeded
	}, nil)
	acc.AppendWithAck(&analyticsv1.AnalyticsEvent{EventId: "e1", EventType: "test", Timestamp: timestamppb.Now()}, ack)
	require.Eventually(t, func() bool { return ack.naked.Load() == 1 }, time.Second, 10*time.Millisecond)
	require.Equal(t, int32(0), ack.acked.Load())
}
