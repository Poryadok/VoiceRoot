package buffer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/analytics/internal/store"
)

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
