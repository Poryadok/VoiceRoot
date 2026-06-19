package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/guestguard"
	"voice/backend/pkg/privacy"
)

// CallPrivacyChecker reads callee allow_calls audience.
type CallPrivacyChecker interface {
	AllowCallsAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
}

// CallProfileFriendChecker verifies friendship for call privacy.
type CallProfileFriendChecker interface {
	AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
	AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
}

// CallSpaceCoMembershipChecker checks shared space membership for call privacy.
type CallSpaceCoMembershipChecker interface {
	AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error)
}

func (s *VoiceGRPC) ensureCallPrivacy(ctx context.Context, callerID, calleeID string) error {
	if s == nil || s.Privacy == nil {
		return nil
	}
	caller, err := uuid.Parse(callerID)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid caller profile")
	}
	callee, err := uuid.Parse(calleeID)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid callee profile")
	}
	audience, err := s.Privacy.AllowCallsAudience(ctx, callee)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	matcher := privacy.Matcher{Social: s.Friends, Space: s.SpaceCoMembership}
	if err := privacy.CheckAllowed(matcher, ctx, callee, caller, audience, guestguard.IsGuest(ctx)); err != nil {
		if errors.Is(err, privacy.ErrDenied) {
			return status.Error(codes.PermissionDenied, "call blocked by recipient privacy settings")
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}
