package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

func presenceIdentityUUID(value string) (string, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || id == uuid.Nil {
		return "", fmt.Errorf("presence requires a valid identity UUID")
	}
	return id.String(), nil
}

// presenceIdentityContext preserves tracing metadata, but never inherits
// credentials or privilege claims from another call. Values are copied so the
// caller's context remains immutable across concurrent viewer lookups.
func presenceIdentityContext(ctx context.Context, accountID, profileID, accountType string) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	for key := range md {
		if strings.HasPrefix(key, "x-voice-") || key == "authorization" ||
			key == "x-profile-id" || key == "x-account-id" || key == "x-user-id" ||
			key == "x-actor-id" || key == "x-internal-caller" {
			md.Delete(key)
		}
	}
	md.Set(grpcMDVoiceUserID, accountID)
	md.Set(grpcMDVoiceProfileID, profileID)
	if accountType != "" {
		md.Set("x-voice-account-type", accountType)
	}
	return metadata.NewOutgoingContext(ctx, md)
}
