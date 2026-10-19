package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/role/internal/memberauthority"
	"voice/backend/role/permissions"
)

func qualifiedMemberContext(t *testing.T, ctx context.Context, space, profile, owner uuid.UUID, operation string, generation uint64, digest []byte, chat ...uuid.UUID) context.Context {
	t.Helper()
	state := spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE
	var binding *rolev1.MembershipBinding
	if operation != "" {
		state = spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE
		binding = &rolev1.MembershipBinding{Source: &rolev1.MembershipBinding_Ordinary{Ordinary: &rolev1.OrdinaryMembershipBinding{MembershipGeneration: generation, PublicationIdentity: operation}}}
	}
	reply := &spacev1.GetMemberAdmissionAuthorityResponse{ProtocolVersion: 1, SpaceId: space.String(), ProfileId: profile.String(), SpaceAuthorityRevision: 5,
		Lifecycle: &spacev1.MemberAdmissionLifecycleAuthority{Generation: 1, State: spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE},
		Ownership: &spacev1.MemberAdmissionOwnershipAuthority{Generation: 1, State: spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE, CurrentOwnerProfileId: owner.String()},
		Ordinary:  &spacev1.MemberAdmissionOrdinaryAuthority{HeadGeneration: generation, State: state, PublishedBinding: binding, RoleReceiptSha256: digest},
		Community: &spacev1.MemberAdmissionCommunityAuthority{State: spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_NONE}, SampledAt: timestamppb.Now(), ValidityRemaining: durationpb.New(5 * time.Second)}
	now := time.Now()
	req := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.String(), ProfileId: profile.String()}
	if operation != "" {
		reply.Ordinary.PublicationKind = spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_ADMISSION_ROLE_READY
	}
	if len(chat) > 0 {
		id := chat[0].String()
		req.ChatId = &id
		reply.TreeRelation = &spacev1.MemberAdmissionTreeRelationAuthority{ChatId: id, Present: true, Revision: 1}
	}
	observation, err := memberauthority.Parse(req, reply, now, now)
	require.NoError(t, err)
	result, err := memberauthority.WithObservations(ctx, observation)
	require.NoError(t, err)
	return result
}

func TestQualifiedMemberRolesExcludeUnknownLegacyAndInertPreparation(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile, owner := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	admin, err := st.RoleIDByName(ctx, space, permissions.RoleAdmin)
	require.NoError(t, err)
	require.NoError(t, st.AssignMemberRole(ctx, space, profile, admin, owner))
	_, err = st.GetQualifiedMemberRoles(ctx, space, profile)
	require.ErrorIs(t, err, ErrScopeUnavailable, "missing owner authority is unknown, never legacy fallback")
	known := qualifiedMemberContext(t, ctx, space, profile, owner, "", 0, nil)
	roles, err := st.GetQualifiedMemberRoles(known, space, profile)
	require.ErrorIs(t, err, ErrScopeUnavailable, "unresolved roles cannot be presented as a complete no-deny view")
	require.NoError(t, st.RevokeMemberRole(ctx, space, profile, admin))
	binding := ensureMemberBinding(space, profile, 1)
	receipt, err := st.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	roles, err = st.GetQualifiedMemberRoles(known, space, profile)
	require.NoError(t, err)
	require.Empty(t, roles, "inert preparation cannot grant access")
	published := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	roles, err = st.GetQualifiedMemberRoles(published, space, profile)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	require.Equal(t, receipt.RoleID, roles[0].ID.String())
	for _, role := range roles {
		require.NotEqual(t, admin, role.ID, "unresolved Admin remains inert even when another contribution proves membership")
	}
	wrong := append([]byte(nil), receipt.ReceiptSHA256...)
	wrong[0] ^= 1
	wrongReceipt := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, wrong)
	roles, err = st.GetQualifiedMemberRoles(wrongReceipt, space, profile)
	require.NoError(t, err)
	require.Empty(t, roles)
	_, err = st.RemoveOrdinaryMemberRoles(ctx, removeMemberBinding(space, profile, 1))
	require.NoError(t, err)
	roles, err = st.GetQualifiedMemberRoles(published, space, profile)
	require.NoError(t, err)
	require.Empty(t, roles, "local removal floor wins over prior observation")
}

func TestQualifiedCanonicalOwnerNeedsIndependentCurrentOwnerProof(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, owner := uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	current := qualifiedMemberContext(t, ctx, space, owner, owner, "", 0, nil)
	roles, err := st.GetQualifiedMemberRoles(current, space, owner)
	require.ErrorIs(t, err, ErrScopeUnavailable, "owner identity alone cannot manufacture an eligible membership source")
	binding := ensureMemberBinding(space, owner, 1)
	receipt, err := st.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	current = qualifiedMemberContext(t, ctx, space, owner, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	roles, err = st.GetQualifiedMemberRoles(current, space, owner)
	require.NoError(t, err)
	require.Len(t, roles, 2)
	var canonicalOwner bool
	for _, role := range roles {
		canonicalOwner = canonicalOwner || role.Managed && role.Name == permissions.RoleOwner
	}
	require.True(t, canonicalOwner)
	changed := qualifiedMemberContext(t, ctx, space, owner, uuid.New(), "", 0, nil)
	_, err = st.GetQualifiedMemberRoles(changed, space, owner)
	require.ErrorIs(t, err, ErrScopeUnavailable, "owner mismatch is unavailable, never guessed repair")
}

func TestQualifiedLegacyOwnerNeedsActualBootstrapWriterReconciliation(t *testing.T) {
	ctx := context.Background()
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	fixture.Run(true, "goto", "20")
	st := &RoleStore{Pool: fixture.Pool}
	space, owner := uuid.New(), uuid.New()
	// This is a real version20 historical bootstrap, not a forged origin row.
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	fixture.Run(true, "goto", "21")
	binding := ensureMemberBinding(space, owner, 1)
	receipt, err := st.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	observed := qualifiedMemberContext(t, ctx, space, owner, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	_, err = st.GetQualifiedMemberRoles(observed, space, owner)
	require.ErrorIs(t, err, ErrScopeUnavailable, "raw Owner + current Space owner cannot synthesize Role bootstrap evidence")
	before := r22RoleEpoch(t, ctx, st, space)
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	require.Greater(t, r22RoleEpoch(t, ctx, st, space), before)
	observed = qualifiedMemberContext(t, ctx, space, owner, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	roles, err := st.GetQualifiedMemberRoles(observed, space, owner)
	require.NoError(t, err)
	require.Len(t, roles, 2)
	before = r22RoleEpoch(t, ctx, st, space)
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	require.Equal(t, before, r22RoleEpoch(t, ctx, st, space), "replay of reconciliation is inert")
}

func TestQualifiedChatReaderRequiresExactScopedObservation(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile, owner, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	binding := ensureMemberBinding(space, profile, 1)
	receipt, err := st.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	unscoped := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	_, err = st.GetQualifiedMemberRolesForChat(unscoped, space, profile, chat)
	require.ErrorIs(t, err, ErrScopeUnavailable)
	scoped := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256, chat)
	roles, err := st.GetQualifiedMemberRolesForChat(scoped, space, profile, chat)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	_, err = st.GetQualifiedMemberRolesForChat(scoped, space, profile, uuid.New())
	require.ErrorIs(t, err, ErrScopeUnavailable)
	_, err = st.GetQualifiedMemberRoles(scoped, space, profile)
	require.ErrorIs(t, err, ErrScopeUnavailable, "scoped authority is not silently promoted to unscoped")
}

func TestQualifiedEffectiveMaskDoesNotAdoptChangedDefault(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile, owner, legacyProfile := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	join, err := permissions.MaskFor(permissions.VoiceJoin)
	require.NoError(t, err)
	speak, err := permissions.MaskFor(permissions.VoiceSpeak)
	require.NoError(t, err)
	roleA, err := st.CreateCustomRole(ctx, space, "qualified admitted A", join, 20, &owner)
	require.NoError(t, err)
	roleB, err := st.CreateCustomRole(ctx, space, "later default B", speak, 21, &owner)
	require.NoError(t, err)
	require.NoError(t, st.SetDefaultJoinRole(ctx, space, roleA.ID))
	binding := ensureMemberBinding(space, profile, 1)
	receipt, err := st.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	require.Equal(t, roleA.ID.String(), receipt.RoleID)
	require.NoError(t, st.SetDefaultJoinRole(ctx, space, roleB.ID))
	qualified := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	mask, err := st.GetQualifiedEffectiveMask(qualified, space, profile, nil, nil)
	require.NoError(t, err)
	require.Equal(t, join, mask, "changing default cannot mint a qualified B contribution or bypass the saved A receipt")
	// The actual reader has cut over at21: it must no longer borrow the old
	// baseline for raw tuples. Source1 compatibility is tested at exact20.
	require.NoError(t, st.AssignMemberRole(ctx, space, legacyProfile, roleA.ID, owner))
	legacy, err := st.GetEffectiveMask(ctx, space, legacyProfile, nil, nil)
	require.ErrorIs(t, err, ErrScopeUnavailable)
	require.Zero(t, legacy)
}

func TestQualifiedMaskOwnerNameAloneIsNotOwnerProof(t *testing.T) {
	join, err := permissions.MaskFor(permissions.VoiceJoin)
	require.NoError(t, err)
	mask, err := (&RoleStore{}).effectiveMaskForRoles(context.Background(), uuid.New(), []RoleRow{{Name: permissions.RoleOwner, PermissionsMask: join}}, nil, nil, false)
	require.NoError(t, err)
	require.Equal(t, join, mask, "an unmanaged same-name contribution folds normally and cannot bypass overrides as Owner")
}

func TestQualifiedUnresolvedRestrictionIsUnavailable(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile, owner := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	role, err := st.CreateCustomRole(ctx, space, "unresolved restriction", 0, 10, &owner)
	require.NoError(t, err)
	require.NoError(t, st.AssignMemberRole(ctx, space, profile, role.ID, owner))
	binding := ensureMemberBinding(space, profile, 1)
	receipt, err := st.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	observed := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	_, err = st.GetQualifiedEffectiveMask(observed, space, profile, nil, nil)
	require.ErrorIs(t, err, ErrScopeUnavailable, "excluding an unresolved role must not silently discard its possible deny overrides")
	require.ErrorContains(t, err, "unresolved legacy Role evidence", "dependency failure is not the completeness discriminator")
}
