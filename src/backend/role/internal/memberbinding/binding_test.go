package memberbinding

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/memberadmission"
)

const episode = "ab98baed-9e2e-4caa-8a50-53ec83c767d0"

func TestMembershipBindingRoundTrip(t *testing.T) {
	for name, value := range map[string]memberadmission.MembershipBinding{
		"ordinary":  {Ordinary: &memberadmission.OrdinaryMembershipBinding{MembershipGeneration: 3, PublicationIdentity: episode}},
		"legacy":    {Ordinary: &memberadmission.OrdinaryMembershipBinding{PublicationIdentity: episode, Legacy: true}},
		"community": {Community: &memberadmission.CommunityMembershipBinding{OwnerGeneration: 7, MembershipGeneration: 11, MembershipEpisodeID: episode}},
	} {
		t.Run(name, func(t *testing.T) {
			wire, err := ToProto(value)
			require.NoError(t, err)
			encoded, err := proto.Marshal(wire)
			require.NoError(t, err)
			var decoded rolev1.MembershipBinding
			require.NoError(t, proto.Unmarshal(encoded, &decoded))
			got, err := FromProto(&decoded)
			require.NoError(t, err)
			require.Equal(t, value, got)
		})
	}
}

func TestMembershipBindingRejectsInvalidOrUnknownSource(t *testing.T) {
	validOrdinary := func() *rolev1.MembershipBinding {
		return &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{MembershipGeneration: 1, PublicationIdentity: episode}}}
	}
	outerUnknown := validOrdinary()
	outerUnknown.ProtoReflect().SetUnknown([]byte{0x78, 0x01})
	nestedUnknown := validOrdinary()
	nestedUnknown.GetOrdinary().ProtoReflect().SetUnknown([]byte{0x78, 0x01})
	communityUnknown := &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Community{Community: &rolev1.CommunityMembershipBinding{OwnerGeneration: 1, MembershipGeneration: 2, MembershipEpisodeId: episode}}}
	communityUnknown.GetCommunity().ProtoReflect().SetUnknown([]byte{0x78, 0x01})
	for name, value := range map[string]*rolev1.MembershipBinding{
		"nil":               nil,
		"missing":           {},
		"nil ordinary":      {Source: &rolev1.MembershipBinding_Ordinary{}},
		"nil community":     {Source: &rolev1.MembershipBinding_Community{}},
		"untagged zero":     {Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{PublicationIdentity: episode}}},
		"legacy positive":   {Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{MembershipGeneration: 1, PublicationIdentity: episode, Legacy: true}}},
		"overflow":          {Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{MembershipGeneration: math.MaxUint64, PublicationIdentity: episode}}},
		"unknown wrapper":   outerUnknown,
		"unknown ordinary":  nestedUnknown,
		"unknown community": communityUnknown,
		"noncanonical":      {Source: &rolev1.MembershipBinding_Community{Community: &rolev1.CommunityMembershipBinding{OwnerGeneration: 1, MembershipGeneration: 2, MembershipEpisodeId: "AB98BAED-9E2E-4CAA-8A50-53EC83C767D0"}}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := FromProto(value)
			require.ErrorIs(t, err, memberadmission.ErrInvalidBinding)
			require.Equal(t, memberadmission.MembershipBinding{}, got)
		})
	}
	_, err := ToProto(memberadmission.MembershipBinding{})
	require.ErrorIs(t, err, memberadmission.ErrInvalidBinding)
	_, err = ToProto(memberadmission.MembershipBinding{Ordinary: &memberadmission.OrdinaryMembershipBinding{MembershipGeneration: 1, PublicationIdentity: episode}, Community: &memberadmission.CommunityMembershipBinding{OwnerGeneration: 1, MembershipGeneration: 2, MembershipEpisodeID: episode}})
	require.ErrorIs(t, err, memberadmission.ErrInvalidBinding)
}
