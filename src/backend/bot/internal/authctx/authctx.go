package authctx

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

const (
	HeaderUserID    = "x-voice-user-id"
	HeaderProfileID = "x-voice-profile-id"
	HeaderBotToken  = "x-voice-bot-token"
)

func AccountID(ctx context.Context) (uuid.UUID, bool) {
	return parseUUIDHeader(ctx, HeaderUserID)
}

func ProfileID(ctx context.Context) (uuid.UUID, bool) {
	return parseUUIDHeader(ctx, HeaderProfileID)
}

func BotToken(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	vals := md.Get(HeaderBotToken)
	if len(vals) == 0 || vals[0] == "" {
		return "", false
	}
	return vals[0], true
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
