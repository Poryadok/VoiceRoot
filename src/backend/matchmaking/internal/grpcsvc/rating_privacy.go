package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/authctx"
	"voice/backend/pkg/guestguard"
	"voice/backend/pkg/privacy"
)

// MmRatingPrivacyChecker loads show_mm_rating audience from User Service.
type MmRatingPrivacyChecker interface {
	ShowMmRatingAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
}

// MmRatingProfileFriendChecker verifies friendship for MM rating visibility.
type MmRatingProfileFriendChecker interface {
	AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
	AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
}

// MmRatingSpaceCoMembershipChecker checks shared space membership for rating visibility.
type MmRatingSpaceCoMembershipChecker interface {
	AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error)
}

func (s *MatchmakingGRPC) ensureMmRatingVisible(ctx context.Context, ownerProfile uuid.UUID) error {
	if s == nil || s.RatingPrivacy == nil {
		return nil
	}
	viewerProfile, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil
	}
	if viewerProfile == ownerProfile {
		return nil
	}
	audience, err := s.RatingPrivacy.ShowMmRatingAudience(ctx, ownerProfile)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	matcher := privacy.Matcher{Social: s.RatingFriends, Space: s.RatingSpaceCoMembership}
	if err := privacy.CheckAllowed(matcher, ctx, ownerProfile, viewerProfile, audience, guestguard.IsGuest(ctx)); err != nil {
		if errors.Is(err, privacy.ErrDenied) {
			return status.Error(codes.PermissionDenied, "mm rating hidden by privacy settings")
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}
