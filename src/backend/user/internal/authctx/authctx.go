package authctx

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

// Metadata keys aligned with Gateway downstream headers (see gateway applyClaims).
const (
	HeaderUserID           = "x-voice-user-id"    // JWT claim user_id == account_id
	HeaderProfileID        = "x-voice-profile-id" // active profile_id (optional for some RPCs)
	HeaderSubscriptionTier = "x-voice-subscription-tier"
	HeaderInternalCaller   = "x-voice-internal-caller" // S2S: auth, subscription, …
)

// AccountID returns the caller's account UUID from incoming gRPC metadata, if present and valid.
func AccountID(ctx context.Context) (uuid.UUID, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	vals := md.Get(HeaderUserID)
	if len(vals) == 0 || vals[0] == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(vals[0])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// SubscriptionTier returns the caller subscription tier from gRPC metadata (free when absent).
func SubscriptionTier(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "free"
	}
	vals := md.Get(HeaderSubscriptionTier)
	if len(vals) == 0 {
		return "free"
	}
	tier := strings.TrimSpace(strings.ToLower(vals[0]))
	if tier == "" {
		return "free"
	}
	return tier
}

// ProfileID returns the caller's active profile UUID from incoming gRPC metadata, if present and valid.
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

// IsInternalService is true when a trusted peer service invokes S2S RPCs.
func IsInternalService(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	vals := md.Get(HeaderInternalCaller)
	return len(vals) > 0 && strings.TrimSpace(vals[0]) != ""
}
