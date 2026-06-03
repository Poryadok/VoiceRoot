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

func AccountID(ctx context.Context) (uuid.UUID, bool) {
	return idFromMetadata(ctx, HeaderUserID)
}

func ProfileID(ctx context.Context) (uuid.UUID, bool) {
	return idFromMetadata(ctx, HeaderProfileID)
}

func idFromMetadata(ctx context.Context, key string) (uuid.UUID, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	vals := md.Get(key)
	if len(vals) == 0 || vals[0] == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(vals[0])
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}
