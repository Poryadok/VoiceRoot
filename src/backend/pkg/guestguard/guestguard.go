package guestguard

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	HeaderAccountType = "x-voice-account-type"
	AccountTypeGuest  = "guest"
	AccountTypeRegular = "regular"
)

// AccountType returns the caller account type from gRPC metadata (regular when absent).
func AccountType(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return AccountTypeRegular
	}
	vals := md.Get(HeaderAccountType)
	if len(vals) == 0 {
		return AccountTypeRegular
	}
	switch strings.ToLower(strings.TrimSpace(vals[0])) {
	case AccountTypeGuest:
		return AccountTypeGuest
	default:
		return AccountTypeRegular
	}
}

// IsGuest reports whether the caller is a guest account.
func IsGuest(ctx context.Context) bool {
	return AccountType(ctx) == AccountTypeGuest
}

// RequireRegular returns PermissionDenied when the caller is a guest account.
func RequireRegular(ctx context.Context) error {
	if IsGuest(ctx) {
		return status.Error(codes.PermissionDenied, "guest accounts cannot perform this action")
	}
	return nil
}
