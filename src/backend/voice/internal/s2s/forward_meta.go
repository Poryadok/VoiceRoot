package s2s

import (
	"context"

	"google.golang.org/grpc/metadata"
)

const (
	headerUserID    = "x-voice-user-id"
	headerProfileID = "x-voice-profile-id"
	headerRequestID = "x-request-id"
)

func ForwardIncomingMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	out := metadata.MD{}
	for _, key := range []string{headerUserID, headerProfileID, headerRequestID} {
		if vals := md.Get(key); len(vals) > 0 {
			out.Set(key, vals...)
		}
	}
	if len(out) == 0 {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, out)
}
