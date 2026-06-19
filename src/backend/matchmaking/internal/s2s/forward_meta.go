package s2s

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// ForwardIncomingMetadata attaches incoming gRPC metadata to an outgoing child call.
func ForwardIncomingMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md) == 0 {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, md.Copy())
}
