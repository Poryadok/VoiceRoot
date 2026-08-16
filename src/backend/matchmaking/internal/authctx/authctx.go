package authctx

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

const HeaderProfileID = "x-voice-profile-id"
const HeaderAccountID = "x-voice-user-id"
const HeaderRoles = "x-voice-roles"

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

// AccountID reads the authenticated account id from gateway metadata.
func AccountID(ctx context.Context) (uuid.UUID, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	vals := md.Get(HeaderAccountID)
	if len(vals) == 0 || vals[0] == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(vals[0])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// HasRole reports whether the gateway-forwarded roles claim includes role.
func HasRole(ctx context.Context, role string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return false
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	vals := md.Get(HeaderRoles)
	if len(vals) == 0 || vals[0] == "" {
		return false
	}
	for _, part := range strings.Split(vals[0], ",") {
		if strings.EqualFold(strings.TrimSpace(part), role) {
			return true
		}
	}
	return false
}

// IsStaff is true when JWT roles include platform staff (admin panel).
func IsStaff(ctx context.Context) bool {
	return HasRole(ctx, "staff")
}
