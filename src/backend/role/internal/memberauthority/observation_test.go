package memberauthority

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
)

func observationFixture() (*spacev1.GetMemberAdmissionAuthorityRequest, *spacev1.GetMemberAdmissionAuthorityResponse, memberadmission.MembershipBinding) {
	space, profile, operation := uuid.NewString(), uuid.NewString(), uuid.NewString()
	binding := memberadmission.MembershipBinding{Ordinary: &memberadmission.OrdinaryMembershipBinding{MembershipGeneration: 3, PublicationIdentity: operation}}
	return &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space, ProfileId: profile}, &spacev1.GetMemberAdmissionAuthorityResponse{
		ProtocolVersion: 1, SpaceId: space, ProfileId: profile, SpaceAuthorityRevision: 5,
		Lifecycle: &spacev1.MemberAdmissionLifecycleAuthority{Generation: 1, State: spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE},
		Ownership: &spacev1.MemberAdmissionOwnershipAuthority{Generation: 1, State: spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE, CurrentOwnerProfileId: uuid.NewString()},
		Ordinary:  &spacev1.MemberAdmissionOrdinaryAuthority{HeadGeneration: 3, State: spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE, PublishedBinding: &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{MembershipGeneration: 3, PublicationIdentity: operation}}}, RoleReceiptSha256: make([]byte, 32), PublicationKind: spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_ADMISSION_ROLE_READY},
		Community: &spacev1.MemberAdmissionCommunityAuthority{State: spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE},
		SampledAt: timestamppb.Now(), ValidityRemaining: durationpb.New(5 * time.Second),
	}, binding
}

func TestObservationExactPublishedBindingAndMonotonicBudget(t *testing.T) {
	req, reply, binding := observationFixture()
	start := time.Now()
	observed, err := Parse(req, reply, start, start.Add(time.Second))
	require.NoError(t, err)
	require.True(t, observed.Eligible(binding, make([]byte, 32), start.Add(4*time.Second)))
	selectors, err := observed.Bindings(start)
	require.NoError(t, err)
	require.Equal(t, []memberadmission.MembershipBinding{binding}, selectors)
	selectors[0].Ordinary.MembershipGeneration = 99
	require.True(t, observed.BindingEligible(binding, start), "returned bindings cannot mutate the saved observation")
	_, err = observed.Bindings(start.Add(5 * time.Second))
	require.ErrorIs(t, err, ErrUnavailable)
	require.False(t, observed.CanPrepare(3, binding.Ordinary.PublicationIdentity, make([]byte, 32), start), "published membership permits historical receipt replay, never new preparation")
	require.False(t, observed.Eligible(binding, make([]byte, 32), start.Add(5*time.Second)), "equality is expired")
	require.False(t, observed.Eligible(binding, []byte("wrong"), start.Add(time.Second)))
	other := binding
	other.Ordinary = &memberadmission.OrdinaryMembershipBinding{MembershipGeneration: 4, PublicationIdentity: binding.Ordinary.PublicationIdentity}
	require.False(t, observed.Eligible(other, make([]byte, 32), start.Add(time.Second)))
	// Owner service clocks may be arbitrarily skewed: validity is relative and
	// its entire transport/lock wait is subtracted by starting before dispatch.
	reply.SampledAt = timestamppb.New(start.Add(72 * time.Hour))
	_, err = Parse(req, reply, start, start.Add(5*time.Second))
	require.Error(t, err)
}

func TestPreparingMembershipIsNeverEffective(t *testing.T) {
	req, reply, binding := observationFixture()
	reply.Ordinary.State = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_PREPARING
	reply.Ordinary.PreparingBinding = reply.Ordinary.PublishedBinding
	reply.Ordinary.PreparingAdmissionBindingSha256 = make([]byte, 32)
	reply.Ordinary.PublishedBinding = nil
	reply.Ordinary.RoleReceiptSha256 = nil
	g, op := binding.Ordinary.MembershipGeneration, binding.Ordinary.PublicationIdentity
	req.ExpectedGeneration, req.ExpectedAdmissionOperationId = &g, &op
	now := time.Now()
	observed, err := Parse(req, reply, now, now)
	require.NoError(t, err)
	require.True(t, observed.CanPrepare(g, op, make([]byte, 32), now))
	require.False(t, observed.Eligible(binding, make([]byte, 32), now))
	selectors, err := observed.Bindings(now)
	require.NoError(t, err)
	require.Empty(t, selectors, "PREPARING never appears as an eligible source")
	bad := append([]byte(nil), make([]byte, 32)...)
	bad[0] = 1
	require.False(t, observed.CanPrepare(g, op, bad, now))
}

func TestRequestedChatNegativeRelationCannotAuthorize(t *testing.T) {
	req, reply, binding := observationFixture()
	chat := uuid.NewString()
	req.ChatId = &chat
	reply.TreeRelation = &spacev1.MemberAdmissionTreeRelationAuthority{ChatId: chat, Present: false, Revision: 1}
	now := time.Now()
	observation, err := Parse(req, reply, now, now)
	require.NoError(t, err, "authenticated negative is not unknown")
	require.False(t, observation.Available(now))
	require.False(t, observation.BindingEligible(binding, now))
}

func TestChatObservationRetainsExactScopeAndRevision(t *testing.T) {
	req, reply, _ := observationFixture()
	chat, other := uuid.NewString(), uuid.NewString()
	req.ChatId = &chat
	now := time.Now()
	_, err := Parse(req, reply, now, now)
	require.ErrorIs(t, err, ErrUnavailable, "missing requested relation is unknown")
	reply.TreeRelation = &spacev1.MemberAdmissionTreeRelationAuthority{ChatId: other, Present: true, Revision: 1}
	_, err = Parse(req, reply, now, now)
	require.ErrorIs(t, err, ErrUnavailable, "wrong chat cannot substitute")
	reply.TreeRelation.ChatId = chat
	first, err := Parse(req, reply, now, now)
	require.NoError(t, err)
	require.True(t, first.BoundToChat(chat, now))
	require.False(t, first.BoundToChat(other, now))
	require.EqualValues(t, 1, first.TreeRelationRevision())
	ctx, err := WithObservations(context.Background(), first)
	require.NoError(t, err)
	_, ok := FromContext(ctx, req.SpaceId, req.ProfileId)
	require.False(t, ok, "membership-only lookup cannot silently use scoped evidence")
	_, ok = FromChatContext(ctx, req.SpaceId, req.ProfileId, other)
	require.False(t, ok)
	got, ok := FromChatContext(ctx, req.SpaceId, req.ProfileId, chat)
	require.True(t, ok)
	require.True(t, got.SameSnapshot(first, now))
	reply.TreeRelation.Revision = 2
	second, err := Parse(req, reply, now, now)
	require.NoError(t, err)
	require.False(t, first.SameSnapshot(second, now), "relation revision must survive double collection")
}

func TestExplicitInitialLiveZeroPreservesFirstP3Generation(t *testing.T) {
	req, reply, _ := observationFixture()
	reply.Lifecycle.Generation = 0
	now := time.Now()
	_, err := Parse(req, reply, now, now)
	require.NoError(t, err, "Space attests known initial LIVE0; no minted LIVE1")
	reply.Lifecycle.State = spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_FROZEN
	_, err = Parse(req, reply, now, now)
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestObservationMalformedUnknownAndWrongScopeAreUnavailable(t *testing.T) {
	for name, mutate := range map[string]func(*spacev1.GetMemberAdmissionAuthorityResponse){
		"space":         func(v *spacev1.GetMemberAdmissionAuthorityResponse) { v.SpaceId = uuid.NewString() },
		"profile":       func(v *spacev1.GetMemberAdmissionAuthorityResponse) { v.ProfileId = uuid.NewString() },
		"revision":      func(v *spacev1.GetMemberAdmissionAuthorityResponse) { v.SpaceAuthorityRevision = 0 },
		"lifecycle":     func(v *spacev1.GetMemberAdmissionAuthorityResponse) { v.Lifecycle = nil },
		"unknown state": func(v *spacev1.GetMemberAdmissionAuthorityResponse) { v.Lifecycle.State = 99 },
		"no bound":      func(v *spacev1.GetMemberAdmissionAuthorityResponse) { v.ValidityRemaining = nil },
		"too long": func(v *spacev1.GetMemberAdmissionAuthorityResponse) {
			v.ValidityRemaining = durationpb.New(6 * time.Second)
		},
		"retired generation": func(v *spacev1.GetMemberAdmissionAuthorityResponse) { v.Ordinary.RetiredThrough = 3 },
		"missing digest":     func(v *spacev1.GetMemberAdmissionAuthorityResponse) { v.Ordinary.RoleReceiptSha256 = nil },
		"retired legacy": func(v *spacev1.GetMemberAdmissionAuthorityResponse) {
			v.Ordinary.HeadGeneration = 0
			v.Ordinary.RetiredThrough = 1
			v.Ordinary.ExplicitLegacy = true
			v.Ordinary.RoleReceiptSha256 = nil
			v.Ordinary.PublicationKind = spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_LEGACY_MIGRATED
			v.Ordinary.PublishedBinding.GetOrdinary().MembershipGeneration = 0
			v.Ordinary.PublishedBinding.GetOrdinary().Legacy = true
		},
		"unknown wrapper": func(v *spacev1.GetMemberAdmissionAuthorityResponse) { v.ProtoReflect().SetUnknown([]byte{0x78, 1}) },
		"unknown nested": func(v *spacev1.GetMemberAdmissionAuthorityResponse) {
			v.Ordinary.PublishedBinding.GetOrdinary().ProtoReflect().SetUnknown([]byte{0x78, 1})
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, reply, _ := observationFixture()
			mutate(reply)
			now := time.Now()
			_, err := Parse(req, reply, now, now)
			require.Error(t, err)
		})
	}
	req, reply, _ := observationFixture()
	copy := proto.Clone(reply).(*spacev1.GetMemberAdmissionAuthorityResponse)
	copy.Lifecycle.State = spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_FROZEN
	now := time.Now()
	observed, err := Parse(req, copy, now, now)
	require.NoError(t, err, "authenticated inactive is distinct from malformed/unavailable")
	require.False(t, observed.Available(now))
}

func TestNativeOrdinaryPublicationNeverBecomesDefaultRoleReadiness(t *testing.T) {
	for _, kind := range []spacev1.MemberAdmissionPublicationKind{
		spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE,
		spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE,
		spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_RECOVERY,
		spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL,
	} {
		t.Run(kind.String(), func(t *testing.T) {
			req, reply, binding := observationFixture()
			reply.Ordinary.PublicationKind = kind
			reply.Ordinary.RoleReceiptSha256 = nil
			reply.Ordinary.NativePublication = &spacev1.MemberAdmissionNativePublicationProof{Kind: kind, OperationId: binding.Ordinary.PublicationIdentity, BindingSha256: make([]byte, 32)}
			now := time.Now()
			observation, err := Parse(req, reply, now, now)
			require.NoError(t, err)
			require.True(t, observation.BindingEligible(binding, now), "native publication is a membership source")
			require.False(t, observation.Eligible(binding, nil, now), "native publication is never an implicit default role receipt")
			require.False(t, observation.CanPrepare(3, binding.Ordinary.PublicationIdentity, make([]byte, 32), now))
		})
	}
}

func TestPublicationProofKindsNeverSubstituteForEachOther(t *testing.T) {
	for name, mutate := range map[string]func(*spacev1.MemberAdmissionOrdinaryAuthority){
		"missing kind": func(v *spacev1.MemberAdmissionOrdinaryAuthority) { v.PublicationKind = 0 },
		"unknown kind": func(v *spacev1.MemberAdmissionOrdinaryAuthority) { v.PublicationKind = 99 },
		"native with READY digest": func(v *spacev1.MemberAdmissionOrdinaryAuthority) {
			v.PublicationKind = spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE
			v.NativePublication = &spacev1.MemberAdmissionNativePublicationProof{Kind: v.PublicationKind, OperationId: v.PublishedBinding.GetOrdinary().PublicationIdentity, BindingSha256: make([]byte, 32)}
		},
		"READY with native proof": func(v *spacev1.MemberAdmissionOrdinaryAuthority) {
			v.NativePublication = &spacev1.MemberAdmissionNativePublicationProof{Kind: spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE, OperationId: v.PublishedBinding.GetOrdinary().PublicationIdentity, BindingSha256: make([]byte, 32)}
		},
		"wrong native operation": func(v *spacev1.MemberAdmissionOrdinaryAuthority) {
			v.PublicationKind = spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE
			v.RoleReceiptSha256 = nil
			v.NativePublication = &spacev1.MemberAdmissionNativePublicationProof{Kind: v.PublicationKind, OperationId: uuid.NewString(), BindingSha256: make([]byte, 32)}
		},
		"wrong native kind": func(v *spacev1.MemberAdmissionOrdinaryAuthority) {
			v.PublicationKind = spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE
			v.RoleReceiptSha256 = nil
			v.NativePublication = &spacev1.MemberAdmissionNativePublicationProof{Kind: spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL, OperationId: v.PublishedBinding.GetOrdinary().PublicationIdentity, BindingSha256: make([]byte, 32)}
		},
		"legacy kind positive generation": func(v *spacev1.MemberAdmissionOrdinaryAuthority) {
			v.PublicationKind = spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_LEGACY_MIGRATED
			v.RoleReceiptSha256 = nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, reply, _ := observationFixture()
			mutate(reply.Ordinary)
			now := time.Now()
			_, err := Parse(req, reply, now, now)
			require.ErrorIs(t, err, ErrUnavailable)
		})
	}
	// Explicit migrated generation zero remains a membership source, without a
	// fabricated admission receipt or implicit default-role entitlement.
	req, reply, binding := observationFixture()
	reply.Ordinary.HeadGeneration = 0
	reply.Ordinary.ExplicitLegacy = true
	reply.Ordinary.RoleReceiptSha256 = nil
	reply.Ordinary.PublicationKind = spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_LEGACY_MIGRATED
	reply.Ordinary.PublishedBinding.GetOrdinary().MembershipGeneration = 0
	reply.Ordinary.PublishedBinding.GetOrdinary().Legacy = true
	binding.Ordinary.MembershipGeneration, binding.Ordinary.Legacy = 0, true
	now := time.Now()
	observation, err := Parse(req, reply, now, now)
	require.NoError(t, err)
	require.True(t, observation.BindingEligible(binding, now))
	require.False(t, observation.Eligible(binding, nil, now))
}
