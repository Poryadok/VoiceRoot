package memberadmission

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

const membershipTestID = "c759fa55-c2bd-43bd-9fa1-d960179f2220"

func TestMembershipBindingRejectsAmbiguousOrInventedIncarnations(t *testing.T) {
	ordinary := &OrdinaryMembershipBinding{MembershipGeneration: 1, PublicationIdentity: membershipTestID}
	community := &CommunityMembershipBinding{OwnerGeneration: 1, MembershipGeneration: 1, MembershipEpisodeID: membershipTestID}
	for name, binding := range map[string]MembershipBinding{
		"no-source":                             {},
		"both-sources":                          {Ordinary: ordinary, Community: community},
		"ordinary-zero-without-explicit-legacy": {Ordinary: &OrdinaryMembershipBinding{PublicationIdentity: membershipTestID}},
		"legacy-cannot-be-a-new-generation":     {Ordinary: &OrdinaryMembershipBinding{Legacy: true, MembershipGeneration: 1, PublicationIdentity: membershipTestID}},
		"ordinary-overflow":                     {Ordinary: &OrdinaryMembershipBinding{MembershipGeneration: math.MaxUint64, PublicationIdentity: membershipTestID}},
		"ordinary-missing-publication":          {Ordinary: &OrdinaryMembershipBinding{MembershipGeneration: 1}},
		"community-missing-owner":               {Community: &CommunityMembershipBinding{MembershipGeneration: 1, MembershipEpisodeID: membershipTestID}},
		"community-missing-member-generation":   {Community: &CommunityMembershipBinding{OwnerGeneration: 1, MembershipEpisodeID: membershipTestID}},
		"community-overflow":                    {Community: &CommunityMembershipBinding{OwnerGeneration: math.MaxUint64, MembershipGeneration: 1, MembershipEpisodeID: membershipTestID}},
		"community-missing-episode":             {Community: &CommunityMembershipBinding{OwnerGeneration: 1, MembershipGeneration: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, ValidateMembershipBinding(binding), ErrInvalidBinding)
			_, err := CanonicalMembershipBinding(binding)
			require.ErrorIs(t, err, ErrInvalidBinding)
		})
	}
}

func TestMembershipBindingCanonicalRoundTripPreservesExplicitSource(t *testing.T) {
	for _, binding := range []MembershipBinding{
		{Ordinary: &OrdinaryMembershipBinding{Legacy: true, PublicationIdentity: membershipTestID}},
		{Ordinary: &OrdinaryMembershipBinding{MembershipGeneration: 9, PublicationIdentity: membershipTestID}},
		{Community: &CommunityMembershipBinding{OwnerGeneration: 2, MembershipGeneration: 7, MembershipEpisodeID: membershipTestID}},
	} {
		raw, err := CanonicalMembershipBinding(binding)
		require.NoError(t, err)
		restored, err := ParseMembershipBinding(raw)
		require.NoError(t, err)
		require.Equal(t, binding, restored)
	}
	for _, raw := range []string{
		`{"ordinary":{"membership_generation":1,"publication_identity":"` + membershipTestID + `","legacy":false,"admin":true}}`,
		`{"community":{"owner_generation":1,"membership_generation":1,"membership_episode_id":"` + membershipTestID + `"},"source":"BOTH"}`,
		`{"ordinary":{"membership_generation":1,"publication_identity":"` + membershipTestID + `","legacy":false}} {}`,
	} {
		_, err := ParseMembershipBinding([]byte(raw))
		require.ErrorIs(t, err, ErrInvalidBinding, raw)
	}
}
