package authctx

import (
	"context"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"
)

const (
	HeaderAccountID    = "x-voice-user-id"
	HeaderProfileID    = "x-voice-profile-id"
	HeaderSessionEpoch = "x-voice-session-epoch"
	HeaderActiveChatID = "x-voice-active-chat-id"
)

func AccountID(ctx context.Context) (string, bool) {
	return metadataValue(ctx, HeaderAccountID)
}

func ProfileID(ctx context.Context) (string, bool) {
	return metadataValue(ctx, HeaderProfileID)
}

// SessionEpoch reads the signed access-token epoch forwarded by Gateway. It
// rejects missing, malformed, non-positive, or duplicate metadata so callers
// cannot silently authorize against an ambiguous identity generation.
func SessionEpoch(ctx context.Context) (int64, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, false
	}
	values := md.Get(HeaderSessionEpoch)
	if len(values) != 1 {
		return 0, false
	}
	epoch, err := strconv.ParseInt(strings.TrimSpace(values[0]), 10, 64)
	if err != nil || epoch <= 0 {
		return 0, false
	}
	return epoch, true
}

func metadataValue(ctx context.Context, key string) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	values := md.Get(key)
	if len(values) == 0 {
		return "", false
	}
	id := strings.TrimSpace(values[0])
	return id, id != ""
}

func ActiveChatID(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	values := md.Get(HeaderActiveChatID)
	if len(values) == 0 {
		return "", false
	}
	id := strings.TrimSpace(values[0])
	return id, id != ""
}
