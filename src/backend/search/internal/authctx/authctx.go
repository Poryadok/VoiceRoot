package authctx

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

const (
	HeaderProfileID = "x-voice-profile-id"
	HeaderAccountID = "x-voice-user-id"
)

// ProfileID returns the caller active profile UUID from incoming gRPC metadata.
func ProfileID(ctx context.Context) (uuid.UUID, bool) {
	return parseUUIDHeader(ctx, HeaderProfileID)
}

// AccountID returns the caller account UUID from incoming gRPC metadata.
func AccountID(ctx context.Context) (uuid.UUID, bool) {
	return parseUUIDHeader(ctx, HeaderAccountID)
}

func parseUUIDHeader(ctx context.Context, key string) (uuid.UUID, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	vals := md.Get(key)
	if len(vals) == 0 || vals[0] == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(vals[0])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
