package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/guestguard"
	"voice/backend/pkg/privacy"
	"voice/backend/space/internal/store"
)

// InvitePrivacyChecker reads allow_chat_space_invites audience.
type InvitePrivacyChecker interface {
	AllowChatSpaceInvitesAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error)
}

// InviteProfileFriendChecker verifies friendship for invite privacy.
type InviteProfileFriendChecker interface {
	AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
	AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
}

// StoreCoMembership checks co-membership via space_db (production wiring).
type StoreCoMembership struct {
	Store *store.SpaceStore
}

func (s *StoreCoMembership) AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error) {
	if s == nil || s.Store == nil {
		return false, nil
	}
	var ids []uuid.UUID
	for _, raw := range spaceIDs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			return false, err
		}
		ids = append(ids, id)
	}
	return s.Store.AreCoMembers(ctx, profileA, profileB, ids)
}

// InviteSpaceCoMembershipChecker checks shared space membership for invite privacy.
type InviteSpaceCoMembershipChecker interface {
	AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error)
}

func (s *SpaceGRPC) ensureJoinInvitePrivacy(ctx context.Context, joiner, inviter uuid.UUID) error {
	if s == nil || s.Privacy == nil {
		return nil
	}
	audience, err := s.Privacy.AllowChatSpaceInvitesAudience(ctx, joiner)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	matcher := privacy.Matcher{Social: s.Friends, Space: s.SpaceCoMembership}
	if err := privacy.CheckAllowed(matcher, ctx, joiner, inviter, audience, guestguard.IsGuest(ctx)); err != nil {
		if errors.Is(err, privacy.ErrDenied) {
			return status.Error(codes.PermissionDenied, "invite blocked by recipient privacy settings")
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}
