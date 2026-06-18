package privacy

import (
	"context"

	"github.com/google/uuid"
)

// SocialGraph supplies friendship facts for audience matching.
type SocialGraph interface {
	AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
	AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error)
}

// SpaceCoMembership checks shared space membership for space_members audience.
type SpaceCoMembership interface {
	AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error)
}

// Matcher evaluates union audience semantics (privacy.md).
type Matcher struct {
	Social SocialGraph
	Space  SpaceCoMembership
}

func (m Matcher) Allowed(ctx context.Context, ownerProfile, viewerProfile uuid.UUID, audience Audience, viewerIsGuest bool) (bool, error) {
	if ownerProfile == viewerProfile {
		return true, nil
	}
	if audience.IsNobody() {
		return false, nil
	}
	if viewerIsGuest {
		return audience.IncludeGuests || audience.IsEveryoneShortcut(), nil
	}
	if audience.Friends && m.Social != nil {
		ok, err := m.Social.AreFriends(ctx, viewerProfile, ownerProfile)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	if audience.FriendsOfFriends && m.Social != nil {
		ok, err := m.Social.AreFriends(ctx, viewerProfile, ownerProfile)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
		fof, err := m.Social.AreFriendsOfFriends(ctx, viewerProfile, ownerProfile)
		if err != nil {
			return false, err
		}
		if fof {
			return true, nil
		}
	}
	if audience.SpaceMembers {
		if m.Space == nil {
			return false, nil
		}
		ok, err := m.Space.AreCoMembers(ctx, viewerProfile, ownerProfile, audience.SpaceIDs)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
