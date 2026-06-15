package grpcclient

import (
	"context"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// DialTimeoutFromEnv returns GRPC_DIAL_TIMEOUT or 15s default.
func DialTimeoutFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("GRPC_DIAL_TIMEOUT"))
	if raw == "" {
		return 15 * time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 15 * time.Second
	}
	return d
}

// WaitForReady blocks until conn is Ready or timeout elapses.
func WaitForReady(ctx context.Context, conn *grpc.ClientConn) error {
	if conn == nil {
		return context.Canceled
	}
	for {
		st := conn.GetState()
		if st == connectivity.Ready {
			return nil
		}
		if st == connectivity.Shutdown {
			return context.Canceled
		}
		if !conn.WaitForStateChange(ctx, st) {
			return context.Cause(ctx)
		}
	}
}
