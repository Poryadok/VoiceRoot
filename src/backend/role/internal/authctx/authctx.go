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
)

// ProfileID returns the caller profile UUID from gRPC metadata.
func ProfileID(ctx context.Context) (uuid.UUID, bool) {
	if p, verified := principal.FromContext(ctx); verified {
		if p.Kind != "delegated_user" || p.Issuer != "gateway" {
			return uuid.Nil, false
		}
		id, err := uuid.Parse(p.ProfileID)
		return id, err == nil && id != uuid.Nil && id.String() == p.ProfileID
	}
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
