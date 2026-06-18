package privacy

import (
	"encoding/json"
	"fmt"

	userv1 "voice.app/voice/user/v1"
)

// Audience is the union multiselect model from privacy.md.
type Audience struct {
	Friends           bool     `json:"friends"`
	FriendsOfFriends  bool     `json:"friends_of_friends"`
	SpaceMembers      bool     `json:"space_members"`
	SpaceIDs          []string `json:"space_ids,omitempty"`
	IncludeGuests     bool     `json:"include_guests"`
}

func Nobody() Audience {
	return Audience{}
}

func FriendsOnly() Audience {
	return Audience{Friends: true}
}

func FriendsAndFoF() Audience {
	return Audience{Friends: true, FriendsOfFriends: true}
}

func SpaceMembersOnly() Audience {
	return Audience{SpaceMembers: true}
}

func SpaceMembersAndFriends() Audience {
	return Audience{SpaceMembers: true, Friends: true}
}

func EveryoneWithGuests() Audience {
	return Audience{
		Friends:          true,
		FriendsOfFriends: true,
		SpaceMembers:     true,
		IncludeGuests:    true,
	}
}

func (a Audience) IsNobody() bool {
	return !a.Friends && !a.FriendsOfFriends && !a.SpaceMembers && !a.IncludeGuests && len(a.SpaceIDs) == 0
}

func (a Audience) IsEveryoneShortcut() bool {
	return a.Friends && a.FriendsOfFriends && a.SpaceMembers && a.IncludeGuests && len(a.SpaceIDs) == 0
}

func FromProto(p *userv1.PrivacyAudience) Audience {
	if p == nil {
		return Audience{}
	}
	return Audience{
		Friends:          p.GetFriends(),
		FriendsOfFriends: p.GetFriendsOfFriends(),
		SpaceMembers:     p.GetSpaceMembers(),
		SpaceIDs:         append([]string(nil), p.GetSpaceIds()...),
		IncludeGuests:    p.GetIncludeGuests(),
	}
}

func ToProto(a Audience) *userv1.PrivacyAudience {
	return &userv1.PrivacyAudience{
		Friends:          a.Friends,
		FriendsOfFriends: a.FriendsOfFriends,
		SpaceMembers:     a.SpaceMembers,
		SpaceIds:         append([]string(nil), a.SpaceIDs...),
		IncludeGuests:    a.IncludeGuests,
	}
}

func MarshalJSON(a Audience) ([]byte, error) {
	return json.Marshal(a)
}

func UnmarshalJSON(data []byte) (Audience, error) {
	if len(data) == 0 {
		return Audience{}, nil
	}
	var a Audience
	if err := json.Unmarshal(data, &a); err != nil {
		return Audience{}, err
	}
	return a, nil
}

// ValidateAudience rejects invalid combinations (e.g. space_ids without space_members).
func ValidateAudience(name string, a Audience) error {
	if len(a.SpaceIDs) > 0 && !a.SpaceMembers {
		return fmt.Errorf("%w: %s", errInvalidAudience, name+": space_ids require space_members")
	}
	if a.IsNobody() {
		return nil
	}
	return nil
}
