package authctx

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

const (
	HeaderUserID    = "x-voice-user-id"
	HeaderProfileID = "x-voice-profile-id"
)

// ProfileID returns the caller's active profile UUID from incoming gRPC metadata.
func ProfileID(ctx context.Context) (uuid.UUID, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	vals := md.Get(HeaderProfileID)
	if len(vals) == 0 || vals[0] == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(vals[0])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
