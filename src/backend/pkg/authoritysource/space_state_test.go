package authoritysource

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSpaceStateLeaseExpiryDoesNotInventPublicGuestMembership(t *testing.T) {
	manual, roster, outsider := uuid.NewString(), uuid.NewString(), uuid.NewString()
	state := SpaceState{Exists: true, Visibility: "public", AllowGuests: true, Members: []string{manual}, Community: &CommunityAuthority{Status: "active", OwnerGeneration: 2, LeaseUntilUnixMillis: 2000}, CommunityMembers: []CommunityMember{{ProfileID: roster, OwnerGeneration: 2, SourceRevision: 1, LeaseUntilUnixMillis: 1500}}}
	require.True(t, state.MemberAt(manual, 1499))
	require.True(t, state.MemberAt(roster, 1499))
	require.False(t, state.MemberAt(roster, 1500))
	require.False(t, state.MemberAt(outsider, 1499))
	require.Equal(t, int64(1500), state.ValidUntil(1499))
	require.Zero(t, state.ValidUntil(1500))
	require.True(t, state.MemberAt(manual, 2500))
	state.Community.LeaseUntilUnixMillis = 1400
	require.False(t, state.MemberAt(roster, 1400))
	state.DeletionPhase = "SCHEDULED"
	require.False(t, state.MemberAt(manual, 1499))
}

func TestSpaceStateDecoderRefusesPartialUnknownAndCrossScopeAuthority(t *testing.T) {
	owner, member, room, category := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	state := SpaceState{SchemaVersion: 1, SpaceID: uuid.NewString(), Revision: 1, Exists: true, OwnerProfileID: owner, Visibility: "private", Members: []string{member}, BannedAccounts: []string{}, Timeouts: []SpaceTimeout{}, VoiceRooms: []string{room}, Categories: []string{category}, Tree: []SpaceTreeResource{{NodeID: uuid.NewString(), Kind: "voice_room", ResourceID: room, CategoryID: category}}, CommunityMembers: []CommunityMember{}}
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	_, err = DecodeSpaceState(raw)
	require.NoError(t, err)
	for name, change := range map[string]func(*SpaceState){
		"unknown version":                    func(s *SpaceState) { s.SchemaVersion = 2 },
		"missing clock":                      func(s *SpaceState) { s.Revision = 0 },
		"partial memberships":                func(s *SpaceState) { s.Members = nil },
		"unowned category":                   func(s *SpaceState) { s.Tree[0].CategoryID = uuid.NewString() },
		"unowned voice room":                 func(s *SpaceState) { s.Tree[0].ResourceID = uuid.NewString() },
		"unknown resource kind":              func(s *SpaceState) { s.Tree[0].Kind = "new_kind" },
		"contradictory lifecycle":            func(s *SpaceState) { s.DeletionPhase = "PURGING" },
		"purged live space":                  func(s *SpaceState) { s.Purged = true },
		"missing space with retained grants": func(s *SpaceState) { s.Exists = false },
	} {
		t.Run(name, func(t *testing.T) {
			var changed SpaceState
			require.NoError(t, json.Unmarshal(raw, &changed))
			change(&changed)
			encoded, err := json.Marshal(changed)
			require.NoError(t, err)
			_, err = DecodeSpaceState(encoded)
			require.Error(t, err)
		})
	}
	_, err = DecodeSpaceState(append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"future_authority":true}`)...))
	require.Error(t, err)
}
