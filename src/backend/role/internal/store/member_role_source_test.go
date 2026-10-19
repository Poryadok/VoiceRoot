package store

import (
	"crypto/sha256"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"voice/backend/pkg/authoritysource"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/role/internal/memberauthority"
)

func TestRole21ExplicitSource2SharesNativeQualifier(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, owner, profile := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	binding := ensureMemberBinding(space, profile, 1)
	receipt, err := st.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	observed := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	native, err := st.GetQualifiedMemberRoles(observed, space, profile)
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 2, SpaceId: space.String()}
	snapshot, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err, "source21 requires explicit state2 rather than widening state1")
	facts, err := authoritysource.DecodeRoleStateV2(snapshot.CanonicalState)
	require.NoError(t, err)
	observation, ok := memberauthority.FromContext(observed, space.String(), profile.String())
	require.True(t, ok)
	folded, err := authoritysource.QualifyRoleMember(facts, observation, profile.String(), "", "", time.Now())
	require.NoError(t, err)
	require.Len(t, folded.Roles, len(native))
	require.Equal(t, native[0].ID.String(), folded.Roles[0].ID)
	revision, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, snapshot.Revision, revision)
	_, err = st.ReadAuthoritySnapshot(ctx, &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String()})
	require.Error(t, err, "version1 semantics stay exact16/17/20")
	_, err = authoritysource.DecodeRoleState(snapshot.CanonicalState)
	require.Error(t, err, "old decoder must not reinterpret state2")
	room := uuid.New()
	grant := roleGrantRequest(uuid.New(), uuid.New(), uuid.New(), room, uuid.New(), 1, []uuid.UUID{profile}, sha256.Sum256([]byte("qualified source2 SDK")))
	_, err = st.ApplyGameSessionGrants(ctx, grant)
	require.NoError(t, err)
	scope.VoiceRoomIds = []string{room.String()}
	sdkSnapshot, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	sdkFacts, err := authoritysource.DecodeRoleStateV2(sdkSnapshot.CanonicalState)
	require.NoError(t, err)
	require.Greater(t, sdkFacts.SDKRevision, facts.SDKRevision)
	require.Equal(t, facts.RoleRevision, sdkFacts.RoleRevision, "SDK and Role clocks remain independent")
	require.Len(t, sdkFacts.SDKSessionGrants, 1)
	var ownerDDL string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT pg_get_functiondef('public.role_member_owner_authority_guard()'::regprocedure)`).Scan(&ownerDDL))
	for _, mutation := range []struct{ name, change, restore string }{
		{"dirty", `UPDATE schema_migrations SET dirty=true`, `UPDATE schema_migrations SET dirty=false`},
		{"future", `UPDATE schema_migrations SET version=22`, `UPDATE schema_migrations SET version=21`},
		{"disabled_clock", `ALTER TABLE role_member_owner_authority DISABLE TRIGGER role_authority_revision_member_owner_authority`, `ALTER TABLE role_member_owner_authority ENABLE TRIGGER role_authority_revision_member_owner_authority`},
		{"disabled_receipt_guard", `ALTER TABLE role_member_admission_receipts DISABLE TRIGGER role_member_admission_receipt_change`, `ALTER TABLE role_member_admission_receipts ENABLE TRIGGER role_member_admission_receipt_change`},
		{"row_security", `ALTER TABLE role_member_contributions ENABLE ROW LEVEL SECURITY`, `ALTER TABLE role_member_contributions DISABLE ROW LEVEL SECURITY`},
		{"missing_current_contributions", `ALTER TABLE role_member_contributions RENAME TO missing_contributions`, `ALTER TABLE missing_contributions RENAME TO role_member_contributions`},
		{"changed_owner_guard_body", `CREATE OR REPLACE FUNCTION role_member_owner_authority_guard() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$`, ownerDDL},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			_, err := st.Pool.Exec(ctx, mutation.change)
			require.NoError(t, err)
			t.Cleanup(func() { _, err := st.Pool.Exec(ctx, mutation.restore); require.NoError(t, err) })
			_, err = st.ReadAuthoritySnapshot(ctx, scope)
			require.Error(t, err)
			_, err = st.ReadAuthorityRevision(ctx, scope)
			require.Error(t, err)
			fresh := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
			_, err = st.GetQualifiedMemberRoles(fresh, space, profile)
			require.ErrorIs(t, err, ErrScopeUnavailable, "native and source2 must reject the same unqualified catalog")
		})
	}
}
