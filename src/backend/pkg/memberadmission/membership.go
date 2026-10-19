package memberadmission

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
)

// MembershipBinding selects ONE eligibility source. It is not a role origin,
// a membership assertion, or a portable permission. Space owns its authority.
type MembershipBinding struct {
	Ordinary  *OrdinaryMembershipBinding  `json:"ordinary,omitempty"`
	Community *CommunityMembershipBinding `json:"community,omitempty"`
}

type OrdinaryMembershipBinding struct {
	MembershipGeneration uint64 `json:"membership_generation"`
	PublicationIdentity  string `json:"publication_identity"`
	Legacy               bool   `json:"legacy"`
}

type CommunityMembershipBinding struct {
	OwnerGeneration      uint64 `json:"owner_generation"`
	MembershipGeneration uint64 `json:"membership_generation"`
	MembershipEpisodeID  string `json:"membership_episode_id"`
}

// ValidateMembershipBinding is the shared BE010/Space-chat source selector.
func ValidateMembershipBinding(binding MembershipBinding) error {
	if (binding.Ordinary == nil) == (binding.Community == nil) {
		return ErrInvalidBinding
	}
	if ordinary := binding.Ordinary; ordinary != nil {
		if !canonicalID(ordinary.PublicationIdentity) || ordinary.MembershipGeneration > math.MaxInt64 {
			return ErrInvalidBinding
		}
		if ordinary.Legacy != (ordinary.MembershipGeneration == 0) {
			return ErrInvalidBinding
		}
		return nil
	}
	community := binding.Community
	if community.OwnerGeneration == 0 || community.OwnerGeneration > math.MaxInt64 ||
		community.MembershipGeneration == 0 || community.MembershipGeneration > math.MaxInt64 ||
		!canonicalID(community.MembershipEpisodeID) {
		return ErrInvalidBinding
	}
	return nil
}

func ParseMembershipBinding(raw []byte) (MembershipBinding, error) {
	var binding MembershipBinding
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&binding); err != nil {
		return MembershipBinding{}, ErrInvalidBinding
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return MembershipBinding{}, ErrInvalidBinding
	}
	if err := ValidateMembershipBinding(binding); err != nil {
		return MembershipBinding{}, err
	}
	return binding, nil
}

func CanonicalMembershipBinding(binding MembershipBinding) ([]byte, error) {
	if err := ValidateMembershipBinding(binding); err != nil {
		return nil, err
	}
	return json.Marshal(binding)
}
