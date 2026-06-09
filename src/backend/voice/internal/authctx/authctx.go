package authctx

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"
)

const (
	HeaderProfileID    = "x-voice-profile-id"
	HeaderActiveChatID = "x-voice-active-chat-id"
)

func ProfileID(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	values := md.Get(HeaderProfileID)
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
