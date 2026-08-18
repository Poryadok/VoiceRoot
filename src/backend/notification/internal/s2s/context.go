package s2s

import (
	"context"

	"google.golang.org/grpc/metadata"
)

const internalCallerHeader = "x-voice-internal-caller"

// Context marks outbound gRPC as Notification Service S2S.
func Context(ctx context.Context) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs(internalCallerHeader, "notification"))
}
