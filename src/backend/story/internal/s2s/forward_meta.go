package s2s

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// ForwardIncomingMetadata copies incoming gRPC metadata onto the outgoing context for downstream calls.
func ForwardIncomingMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md) == 0 {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, md.Copy())
}
