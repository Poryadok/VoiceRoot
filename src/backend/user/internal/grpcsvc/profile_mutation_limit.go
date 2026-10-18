package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProfileMutationLimiter is required for profile creation/deletion. Its account
// comes from the existing authenticated caller, never the requested profile ID.
type ProfileMutationLimiter interface {
	Allow(context.Context, uuid.UUID, string) (bool, error)
}

func (s *UserGRPC) allowProfileMutation(ctx context.Context, accountID uuid.UUID, operation string) error {
	if err := ctx.Err(); err != nil {
		return status.FromContextError(err).Err()
	}
	if s.ProfileMutations == nil {
		return status.Error(codes.Unavailable, "profile mutation limiter unavailable")
	}
	allowed, err := s.ProfileMutations.Allow(ctx, accountID, operation)
	// A reply after cancellation must not authorize a database mutation.
	if contextErr := ctx.Err(); contextErr != nil {
		return status.FromContextError(contextErr).Err()
	}
	if err != nil {
		return status.Error(codes.Unavailable, "profile mutation limiter unavailable")
	}
	if !allowed {
		return status.Error(codes.ResourceExhausted, "profile mutation rate limit exceeded")
	}
	return nil
}
