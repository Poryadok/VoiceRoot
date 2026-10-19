package authctx

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"voice/backend/pkg/principal"
)

const (
	HeaderUserID    = "x-voice-user-id"
	HeaderProfileID = "x-voice-profile-id"
	HeaderBotToken  = "x-voice-bot-token"
)

func AccountID(ctx context.Context) (uuid.UUID, bool) {
	if verified, present := principal.FromContext(ctx); present {
		return verifiedUserID(verified, verified.AccountID)
	}
	return parseUUIDHeader(ctx, HeaderUserID)
}

func ProfileID(ctx context.Context) (uuid.UUID, bool) {
	if verified, present := principal.FromContext(ctx); present {
		return verifiedUserID(verified, verified.ProfileID)
	}
	return parseUUIDHeader(ctx, HeaderProfileID)
}

func verifiedUserID(p principal.Principal, value string) (uuid.UUID, bool) {
	if p.Kind != "delegated_user" || p.Issuer != "gateway" || p.Audience != "bot" || p.Subject != p.AccountID || p.SessionEpoch <= 0 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(value)
	return id, err == nil && id != uuid.Nil && id.String() == value
}

func BotToken(ctx context.Context) (string, bool) {
	for _, from := range []func(context.Context) (metadata.MD, bool){
		metadata.FromIncomingContext,
		metadata.FromOutgoingContext,
	} {
		if md, ok := from(ctx); ok {
			if vals := md.Get(HeaderBotToken); len(vals) > 0 && vals[0] != "" {
				return vals[0], true
			}
		}
	}
	return "", false
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
