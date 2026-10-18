package grpcsvc

import (
	"context"
	"time"

	"google.golang.org/grpc/tap"
	botv1 "voice.app/voice/bot/v1"
)

// PollEventsTransportLimit bounds one complete polling RPC, including native
// HTTP/2 flow-control waits. It matches the message webhook attempt budget.
const PollEventsTransportLimit = 25 * time.Second

func PollEventsTransportTap() tap.ServerInHandle {
	return pollEventsTransportTap(PollEventsTransportLimit)
}

type pollingTransportCancelKey struct{}

// In pinned grpc-go v1.70, the native HTTP/2 stream captures this tap context
// before constructing ctxDone/writeQuota. A stream-interceptor context wrapper
// would only bound application/DB work and would not interrupt native Send.
func pollEventsTransportTap(limit time.Duration) tap.ServerInHandle {
	return func(ctx context.Context, info *tap.Info) (context.Context, error) {
		if info.FullMethodName != botv1.BotService_PollEvents_FullMethodName {
			return ctx, nil
		}
		bounded, cancel := context.WithTimeout(ctx, limit)
		return context.WithValue(bounded, pollingTransportCancelKey{}, cancel), nil
	}
}

func releasePollingTransport(ctx context.Context) {
	if cancel, ok := ctx.Value(pollingTransportCancelKey{}).(context.CancelFunc); ok {
		cancel()
	}
}
