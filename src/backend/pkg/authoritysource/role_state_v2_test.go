package authoritysource

import (
	"bytes"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sort"
	"testing"
	"time"
	"voice/backend/pkg/memberadmission"
)

type roleObservationFixture struct {
	space, profile, owner string
	bindings              []memberadmission.MembershipBinding
	receipt               []byte
	expires               time.Time
}

func TestRoleV2StrictCodecAndOldVersionBoundary(t *testing.T) {
	state, observation := roleV2Fixture(t)
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	decoded, err := DecodeRoleStateV2(raw)
	require.NoError(t, err)
	_, err = QualifyRoleMember(decoded, observation, observation.profile, "", "", time.Now())
	require.NoError(t, err)
	_, err = DecodeRoleState(raw)
	require.Error(t, err, "state2 cannot masquerade as legacy state1")
	for name, changed := range map[string][]byte{
		"unknown":   append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`,"unknown":true}`)...),
		"missing":   bytes.Replace(raw, []byte(`"floors":[],`), nil, 1),
		"null":      bytes.Replace(raw, []byte(`"floors":[]`), []byte(`"floors":null`), 1),
		"duplicate": bytes.Replace(raw, []byte(`"schema_version":2`), []byte(`"schema_version":2,"schema_version":2`), 1),
		"legacy":    bytes.Replace(raw, []byte(`"schema_version":2`), []byte(`"schema_version":1`), 1),
	} {
		t.Run(name, func(t *testing.T) { _, err := DecodeRoleStateV2(changed); require.Error(t, err) })
	}
	state.Contributions[0].Admission.ReceiptSHA256[0] ^= 1
	_, err = QualifyRoleMember(state, observation, observation.profile, "", "", time.Now())
	require.Error(t, err, "a canonical binding alone cannot authenticate altered READY")
}

func TestRoleV2IndependentSourcesFloorsAndUnresolvedDeny(t *testing.T) {
	state, o := roleV2Fixture(t)
	ordinary := state.Contributions[0].Binding
	community := memberadmission.MembershipBinding{Community: &memberadmission.CommunityMembershipBinding{OwnerGeneration: 3, MembershipGeneration: 8, MembershipEpisodeID: uuid.NewString()}}
	o.bindings = append(o.bindings, community)
	state.Contributions = append(state.Contributions, RoleContributionV2{ID: uuid.NewString(), AuthorizationID: uuid.NewString(), ProfileID: o.profile, RoleID: state.Roles[0].ID, Origin: "INTERACTIVE", Binding: community})
	SortRoleStateV2(&state)
	state.Floors = []RoleMembershipFloor{{ProfileID: o.profile, Source: "ORDINARY", RetiredThrough: ordinary.Ordinary.MembershipGeneration}}
	qualified, err := QualifyRoleMember(state, o, o.profile, "", "", time.Now())
	require.NoError(t, err)
	require.Len(t, qualified.Roles, 1)
	require.Len(t, qualified.Contributions, 1)
	require.Equal(t, "INTERACTIVE", qualified.Contributions[0].Origin)
	state.Floors = append(state.Floors, RoleMembershipFloor{ProfileID: o.profile, Source: "COMMUNITY", RetiredThrough: 8})
	SortRoleStateV2(&state)
	qualified, err = QualifyRoleMember(state, o, o.profile, "", "", time.Now())
	require.NoError(t, err)
	require.Empty(t, qualified.Roles)
	state.Unresolved = []RoleAssignment{{ProfileID: o.profile, RoleID: state.Roles[0].ID}}
	_, err = QualifyRoleMember(state, o, o.profile, "", "", time.Now())
	require.ErrorIs(t, err, ErrUnresolvedRoleEvidence, "unknown legacy deny is not a known empty qualified view")
	o.expires = time.Now()
	_, err = QualifyRoleMember(state, o, o.profile, "", "", time.Now())
	require.ErrorIs(t, err, ErrQualificationUnavailable)
}

func TestRoleV2CommunityBaselineIsManagedMemberNotConfiguredDefault(t *testing.T) {
	state, o := roleV2Fixture(t)
	state.Contributions = []RoleContributionV2{}
	member := RoleDefinitionV2{ID: uuid.NewString(), Name: "Member", Managed: true, Position: 0, Permissions: 2}
	state.Roles = append(state.Roles, member)
	state.CommunityMemberRoleID = member.ID
	state.Roles[0].DefaultJoin = true
	o.bindings = []memberadmission.MembershipBinding{{Community: &memberadmission.CommunityMembershipBinding{OwnerGeneration: 1, MembershipGeneration: 2, MembershipEpisodeID: uuid.NewString()}}}
	SortRoleStateV2(&state)
	qualified, err := QualifyRoleMember(state, o, o.profile, "", "", time.Now())
	require.NoError(t, err)
	require.EqualValues(t, 2, qualified.EffectiveMask)
	require.Equal(t, member.ID, qualified.Roles[0].ID)
	state.CommunityMemberRoleID = ""
	_, err = QualifyRoleMember(state, o, o.profile, "", "", time.Now())
	require.Error(t, err, "missing fixed baseline policy is not complete authority")
}

func TestRoleV2OwnerRequiresManagedLinkCurrentOwnerAndMembership(t *testing.T) {
	state, o := roleV2Fixture(t)
	owner := RoleDefinitionV2{ID: uuid.NewString(), Name: "Owner", Managed: true, Position: 99, Permissions: 7}
	state.Roles = append(state.Roles, owner)
	state.Owner = &RoleOwnerLink{ProfileID: o.profile, RoleID: owner.ID, Origin: "BOOTSTRAP", Generation: 1}
	SortRoleStateV2(&state)
	_, err := QualifyRoleMember(state, o, o.profile, "", "", time.Now())
	require.Error(t, err, "local Owner row does not prove current Space owner")
	o.owner = o.profile
	qualified, err := QualifyRoleMember(state, o, o.profile, "", "", time.Now())
	require.NoError(t, err)
	require.True(t, qualified.CanonicalOwner)
	require.EqualValues(t, 7, qualified.EffectiveMask)
	o.bindings = nil
	_, err = QualifyRoleMember(state, o, o.profile, "", "", time.Now())
	require.Error(t, err, "Owner shortcut cannot hide an unknown membership source")
	state.Owner = nil
	owner.Managed = false
	state.Roles = []RoleDefinitionV2{owner}
	state.Contributions = []RoleContributionV2{}
	qualified, err = QualifyRoleMember(state, o, o.profile, "", "", time.Now())
	require.NoError(t, err)
	require.Empty(t, qualified.Roles, "Owner name alone never contributes a grant")
}

func (o roleObservationFixture) SpaceID() string              { return o.space }
func (o roleObservationFixture) ProfileID() string            { return o.profile }
func (o roleObservationFixture) Revision() uint64             { return 1 }
func (o roleObservationFixture) Available(now time.Time) bool { return now.Before(o.expires) }
func (o roleObservationFixture) IsOwner(p string, now time.Time) bool {
	return o.Available(now) && p == o.owner
}
func (o roleObservationFixture) Bindings(now time.Time) ([]memberadmission.MembershipBinding, error) {
	if !o.Available(now) {
		return nil, ErrQualificationUnavailable
	}
	return o.bindings, nil
}
func (o roleObservationFixture) BindingEligible(b memberadmission.MembershipBinding, now time.Time) bool {
	if !o.Available(now) {
		return false
	}
	x, _ := memberadmission.CanonicalMembershipBinding(b)
	for _, candidate := range o.bindings {
		y, _ := memberadmission.CanonicalMembershipBinding(candidate)
		if string(x) == string(y) {
			return true
		}
	}
	return false
}
func (o roleObservationFixture) Eligible(b memberadmission.MembershipBinding, r []byte, now time.Time) bool {
	return o.BindingEligible(b, now) && len(r) == 32 && string(r) == string(o.receipt)
}
func roleV2Fixture(t *testing.T) (RoleStateV2, roleObservationFixture) {
	t.Helper()
	space, profile, role, op := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	binding := memberadmission.EnsureBinding{ProtocolVersion: 1, SpaceID: space, ProfileID: profile, MembershipGeneration: 1, AdmissionOperationID: op, AdmissionBindingSHA256: make([]byte, 32)}
	receipt, err := memberadmission.EnsureReceiptFingerprint(binding, role)
	require.NoError(t, err)
	selector := memberadmission.MembershipBinding{Ordinary: &memberadmission.OrdinaryMembershipBinding{MembershipGeneration: 1, PublicationIdentity: op}}
	state := RoleStateV2{SchemaVersion: 2, SpaceID: space, RoleRevision: 2, SDKRevision: 3, AllPermissions: 7, Roles: []RoleDefinitionV2{{ID: role, Name: "admitted", Position: 1, Permissions: 1}}, Contributions: []RoleContributionV2{{ID: op, ProfileID: profile, RoleID: role, Origin: "ORDINARY_DEFAULT", Binding: selector, AuthorizationID: op, Admission: &RoleAdmissionProof{Binding: binding, ReceiptSHA256: receipt[:]}}}, Floors: []RoleMembershipFloor{}, Unresolved: []RoleAssignment{}, ChatOverrides: []RoleOverride{}, VoiceOverrides: []RoleOverride{}, SDKSessionGrants: []SDKSessionGrant{}}
	return state, roleObservationFixture{space: space, profile: profile, bindings: []memberadmission.MembershipBinding{selector}, receipt: receipt[:], expires: time.Now().Add(time.Second)}
}
func TestRoleV2QualifiesExactREADYWithoutAmbientDefault(t *testing.T) {
	state, observation := roleV2Fixture(t)
	state.Roles = append(state.Roles, RoleDefinitionV2{ID: uuid.NewString(), Name: "later default", Position: 2, Permissions: 2, DefaultJoin: true})
	sort.Slice(state.Roles, func(i, j int) bool { return state.Roles[i].ID < state.Roles[j].ID })
	actual, err := QualifyRoleMember(state, observation, observation.profile, "", "", time.Now())
	require.NoError(t, err)
	require.EqualValues(t, 1, actual.EffectiveMask)
	require.Len(t, actual.Roles, 1)
}
