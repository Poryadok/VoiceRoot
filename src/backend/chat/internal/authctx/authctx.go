package authctx

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"voice/backend/pkg/guestguard"
)

// Metadata keys aligned with Gateway downstream headers (see gateway applyClaims).
const (
	HeaderUserID    = "x-voice-user-id"    // JWT claim user_id == account_id
	HeaderProfileID = "x-voice-profile-id" // active profile_id
	HeaderAccountType = guestguard.HeaderAccountType
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

// AccountType returns caller account type (regular when absent).
func AccountType(ctx context.Context) string {
	return guestguard.AccountType(ctx)
}

// RequireRegular returns PermissionDenied for guest callers.
func RequireRegular(ctx context.Context) error {
	return guestguard.RequireRegular(ctx)
}
