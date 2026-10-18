// Package memberbinding converts the shared Space eligibility selector without
// granting authority. Each consumer must independently obtain fresh Space proof.
package memberbinding

import (
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/memberadmission"
)

func FromProto(wire *rolev1.MembershipBinding) (memberadmission.MembershipBinding, error) {
	var out memberadmission.MembershipBinding
	if wire == nil || len(wire.ProtoReflect().GetUnknown()) != 0 {
		return out, memberadmission.ErrInvalidBinding
	}
	switch source := wire.Source.(type) {
	case *rolev1.MembershipBinding_Ordinary:
		if source == nil || source.Ordinary == nil {
			return out, memberadmission.ErrInvalidBinding
		}
		ordinary := source.Ordinary
		if len(ordinary.ProtoReflect().GetUnknown()) != 0 {
			return out, memberadmission.ErrInvalidBinding
		}
		out.Ordinary = &memberadmission.OrdinaryMembershipBinding{MembershipGeneration: ordinary.MembershipGeneration, PublicationIdentity: ordinary.PublicationIdentity, Legacy: ordinary.Legacy}
	case *rolev1.MembershipBinding_Community:
		if source == nil || source.Community == nil {
			return out, memberadmission.ErrInvalidBinding
		}
		community := source.Community
		if len(community.ProtoReflect().GetUnknown()) != 0 {
			return out, memberadmission.ErrInvalidBinding
		}
		out.Community = &memberadmission.CommunityMembershipBinding{OwnerGeneration: community.OwnerGeneration, MembershipGeneration: community.MembershipGeneration, MembershipEpisodeID: community.MembershipEpisodeId}
	default:
		return out, memberadmission.ErrInvalidBinding
	}
	if err := memberadmission.ValidateMembershipBinding(out); err != nil {
		return memberadmission.MembershipBinding{}, err
	}
	return out, nil
}

func ToProto(binding memberadmission.MembershipBinding) (*rolev1.MembershipBinding, error) {
	if err := memberadmission.ValidateMembershipBinding(binding); err != nil {
		return nil, err
	}
	if ordinary := binding.Ordinary; ordinary != nil {
		return &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{MembershipGeneration: ordinary.MembershipGeneration, PublicationIdentity: ordinary.PublicationIdentity, Legacy: ordinary.Legacy}}}, nil
	}
	community := binding.Community
	return &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Community{Community: &rolev1.CommunityMembershipBinding{OwnerGeneration: community.OwnerGeneration, MembershipGeneration: community.MembershipGeneration, MembershipEpisodeId: community.MembershipEpisodeID}}}, nil
}
