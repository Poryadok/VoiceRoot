package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/role/internal/memberauthority"
)

func TestRole25NativeAndSource2CleanupParity(t *testing.T) {
	ctx := context.Background()
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	f.Run(true, "goto", "21")
	s := &RoleStore{Pool: f.Pool}
	space, owner, profile := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, s.BootstrapSpaceRoles(ctx, space, owner))
	binding := ensureMemberBinding(space, profile, 1)
	receipt, err := s.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	f.Run(true, "goto", "25")
	fresh := func() context.Context {
		return qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	}
	native, err := s.GetQualifiedMemberRoles(fresh(), space, profile)
	require.NoError(t, err, "actual21->25 must retain only exact qualified native rights")
	require.Len(t, native, 1)
	scope := &authorityv1.SourceScope{SchemaVersion: 2, SpaceId: space.String()}
	before, err := s.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	facts, err := authoritysource.DecodeRoleStateV2(before.CanonicalState)
	require.NoError(t, err)
	observed, ok := memberauthority.FromContext(fresh(), space.String(), profile.String())
	require.True(t, ok)
	folded, err := authoritysource.QualifyRoleMember(facts, observed, profile.String(), "", "", time.Now())
	require.NoError(t, err)
	require.Equal(t, native[0].ID.String(), folded.Roles[0].ID)
	mask, err := s.GetQualifiedEffectiveMask(fresh(), space, profile, nil, nil)
	require.NoError(t, err)
	require.Equal(t, folded.EffectiveMask, mask, "native mask and canonical state2 use the same qualified fold")
	req := removalRequest()
	req.SpaceId = space.String()
	insertRemovalRole(t, f.Pool, req.SpaceId, req.ActorProfileId, "Bot custom", false)
	preCleanup, err := s.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	preFacts, err := authoritysource.DecodeRoleStateV2(preCleanup.CanonicalState)
	require.NoError(t, err)
	_, err = s.ApplyBotInstallationRemoval(ctx, req)
	require.NoError(t, err)
	after, err := s.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	afterFacts, err := authoritysource.DecodeRoleStateV2(after.CanonicalState)
	require.NoError(t, err)
	require.Greater(t, afterFacts.RoleRevision, preFacts.RoleRevision, "real cleanup effects invalidate Role facts")
	require.Equal(t, preFacts.SDKRevision, afterFacts.SDKRevision, "independent SDK clock is unchanged")
	remaining, err := s.GetQualifiedMemberRoles(fresh(), space, profile)
	require.NoError(t, err)
	require.Equal(t, native, remaining)
	_, err = s.ReadAuthoritySnapshot(ctx, &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String()})
	require.Error(t, err, "state1 is never widened to25")
	for _, change := range []struct{ name, sql, undo string }{
		{"dirty", `UPDATE schema_migrations SET dirty=true`, `UPDATE schema_migrations SET dirty=false`},
		{"future", `UPDATE schema_migrations SET version=26`, `UPDATE schema_migrations SET version=25`},
		{"bot_guard", `ALTER TABLE role_bot_global_removals DISABLE TRIGGER role_bot_global_removals_guard`, `ALTER TABLE role_bot_global_removals ENABLE TRIGGER role_bot_global_removals_guard`},
		{"clock", `ALTER TABLE role_member_contributions DISABLE TRIGGER role_authority_revision_member_contributions`, `ALTER TABLE role_member_contributions ENABLE TRIGGER role_authority_revision_member_contributions`},
	} {
		t.Run(change.name, func(t *testing.T) {
			_, err := f.Pool.Exec(ctx, change.sql)
			require.NoError(t, err)
			defer func() { _, err := f.Pool.Exec(ctx, change.undo); require.NoError(t, err) }()
			_, err = s.ReadAuthoritySnapshot(ctx, scope)
			require.Error(t, err)
			_, err = s.GetQualifiedMemberRoles(fresh(), space, profile)
			require.ErrorIs(t, err, ErrScopeUnavailable)
		})
	}
}

func TestQualifiedAdmissionCatalogBeforeReceipt(t *testing.T) {
	for _, kind := range []string{"ensure", "remove"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			f := removalFixture(t)
			s := &RoleStore{Pool: f.Pool}
			space, owner := uuid.New(), uuid.New()
			require.NoError(t, s.BootstrapSpaceRoles(ctx, space, owner))
			call := func() error {
				binding := ensureMemberBinding(space, uuid.New(), 1)
				o := preparingObservation(t, binding, owner, 5*time.Second)
				if kind == "ensure" {
					_, err := s.PrepareQualifiedDefaultMemberRole(ctx, binding, o)
					return err
				}
				_, err := s.RemoveQualifiedOrdinaryMemberRoles(ctx, removeMemberBinding(space, uuid.MustParse(binding.ProfileID), 1), o)
				return err
			}
			require.NoError(t, call(), "clean25 positive is the same qualified path")
			_, err := f.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=true`)
			require.NoError(t, err)
			require.ErrorIs(t, call(), ErrScopeUnavailable)
			var receipts int
			require.NoError(t, f.Pool.QueryRow(ctx, `SELECT count(*) FROM role_member_admission_receipts`).Scan(&receipts))
			require.Equal(t, 1, receipts, "unqualified schema may not mint even an inert receipt")
		})
	}
}
