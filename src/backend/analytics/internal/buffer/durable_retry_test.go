package buffer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/analytics/internal/store"
)

func TestAccumulatorRetainsAcklessEntriesWhenMixedBatchFlushFails(t *testing.T) {
	jetStreamAck := &stubAck{}
	acc := New(2, time.Hour, func(context.Context, []store.EventRow) error {
		return errors.New("clickhouse unavailable")
	}, nil)

	acc.AppendProto(&analyticsv1.AnalyticsEvent{EventId: "grpc-event", EventType: "test", Timestamp: timestamppb.Now()})
	acc.AppendWithAck(&analyticsv1.AnalyticsEvent{EventId: "stream-event", EventType: "test", Timestamp: timestamppb.Now()}, jetStreamAck)

	require.Eventually(t, func() bool { return jetStreamAck.naked.Load() == 1 }, time.Second, 10*time.Millisecond)
	require.Equal(t, 1, acc.PendingCount(), "ack-less gRPC event must be retried after mixed-batch failure")
}
