package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/memberadmission"
)

func memberAdmissionStore(t *testing.T) (context.Context, *RoleStore) {
	t.Helper()
	ctx := context.Background()
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	fixture.Run(true, "goto", "16")
	fixture.Run(true, "up")
	return ctx, &RoleStore{Pool: fixture.Pool}
}

func ensureMemberBinding(space, profile uuid.UUID, generation uint64) memberadmission.EnsureBinding {
	return memberadmission.EnsureBinding{ProtocolVersion: 1, SpaceID: space.String(), ProfileID: profile.String(), MembershipGeneration: generation, AdmissionOperationID: uuid.NewString(), AdmissionBindingSHA256: make([]byte, 32)}
}

func removeMemberBinding(space, profile uuid.UUID, floor uint64) memberadmission.RemoveBinding {
	return memberadmission.RemoveBinding{ProtocolVersion: 1, SpaceID: space.String(), ProfileID: profile.String(), RemovalGenerationFloor: floor, RemovalOperationID: uuid.NewString(), RemovalBindingSHA256: make([]byte, 32)}
}

func TestMemberAdmissionPreparationIsInertAndReceiptHistorical(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile, owner := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	first := ensureMemberBinding(space, profile, 1)
	before := r22RoleEpoch(t, ctx, st, space)
	receipt, err := st.PrepareDefaultMemberRole(ctx, first)
	require.NoError(t, err)
	require.Greater(t, r22RoleEpoch(t, ctx, st, space), before)
	roles, err := st.GetRawMemberRoleInventory(ctx, space, profile)
	require.NoError(t, err)
	require.Empty(t, roles, "READY preparation is not an effective assignment")
	known := qualifiedMemberContext(t, ctx, space, profile, owner, "", 0, nil)
	mask, err := st.GetEffectiveMask(known, space, profile, nil, nil)
	require.NoError(t, err)
	require.Zero(t, mask)
	custom, err := st.CreateCustomRole(ctx, space, "new default", 1, 1, nil)
	require.NoError(t, err)
	require.NoError(t, st.SetDefaultJoinRole(ctx, space, custom.ID))
	revision := r22RoleEpoch(t, ctx, st, space)
	replay, err := st.PrepareDefaultMemberRole(ctx, first)
	require.NoError(t, err)
	require.Equal(t, receipt, replay, "replay must not choose the new default")
	require.Equal(t, revision, r22RoleEpoch(t, ctx, st, space), "replay is not a mutation")
	changed := first
	changed.AdmissionBindingSHA256 = append([]byte(nil), first.AdmissionBindingSHA256...)
	changed.AdmissionBindingSHA256[0] = 1
	_, err = st.PrepareDefaultMemberRole(ctx, changed)
	require.ErrorIs(t, err, ErrMemberAdmissionConflict)
	_, err = st.PrepareDefaultMemberRole(ctx, ensureMemberBinding(space, profile, 1))
	require.ErrorIs(t, err, ErrMemberAdmissionConflict, "same generation with another operation conflicts with recorded identity")
	removal := removeMemberBinding(space, profile, 1)
	_, err = st.RemoveOrdinaryMemberRoles(ctx, removal)
	require.NoError(t, err)
	replay, err = st.PrepareDefaultMemberRole(ctx, first)
	require.NoError(t, err)
	require.Equal(t, receipt, replay)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM role_member_admission_preparations WHERE space_id=$1`, space).Scan(&count))
	require.Zero(t, count, "historical Ensure cannot recreate removed preparation")
	next, err := st.PrepareDefaultMemberRole(ctx, ensureMemberBinding(space, profile, 2))
	require.NoError(t, err)
	require.Equal(t, custom.ID.String(), next.RoleID)
	_, err = st.RemoveOrdinaryMemberRoles(ctx, removal)
	require.NoError(t, err)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM role_member_admission_preparations WHERE space_id=$1`, space).Scan(&count))
	require.Equal(t, 1, count, "historical Remove cannot delete rejoin")
}

func TestMemberAdmissionEmptyRemovalFloorAndIndependentCommunity(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile := uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, uuid.New()))
	_, err := st.Pool.Exec(ctx, `INSERT INTO role_member_source_floors(space_id,profile_id,source_kind,retired_through) VALUES($1,$2,'COMMUNITY',100)`, space, profile)
	require.NoError(t, err)
	_, err = st.RemoveOrdinaryMemberRoles(ctx, removeMemberBinding(space, profile, 5))
	require.NoError(t, err)
	_, err = st.PrepareDefaultMemberRole(ctx, ensureMemberBinding(space, profile, 4))
	require.ErrorIs(t, err, ErrMemberAdmissionStale)
	_, err = st.PrepareDefaultMemberRole(ctx, ensureMemberBinding(space, profile, 6))
	require.NoError(t, err, "community floor is not an ordinary floor")
	var floor int64
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retired_through FROM role_member_source_floors WHERE space_id=$1 AND profile_id=$2 AND source_kind='COMMUNITY'`, space, profile).Scan(&floor))
	require.EqualValues(t, 100, floor)
	_, err = st.Pool.Exec(ctx, `UPDATE role_member_source_floors SET retired_through=1 WHERE space_id=$1`, space)
	require.Error(t, err)
	_, err = st.Pool.Exec(ctx, `DELETE FROM role_member_source_floors WHERE space_id=$1`, space)
	require.Error(t, err)
	_, err = st.Pool.Exec(ctx, `TRUNCATE role_member_source_floors`)
	require.Error(t, err)
}

func TestMemberAdmissionConcurrentReplayOneReceipt(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile := uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, uuid.New()))
	binding := ensureMemberBinding(space, profile, 1)
	start := make(chan struct{})
	results := make(chan MemberAdmissionReceipt, 2)
	errors := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			r, e := st.PrepareDefaultMemberRole(ctx, binding)
			results <- r
			errors <- e
		}()
	}
	close(start)
	workers.Wait()
	require.NoError(t, <-errors)
	require.NoError(t, <-errors)
	require.Equal(t, <-results, <-results)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM role_member_admission_receipts WHERE space_id=$1`, space).Scan(&count))
	require.Equal(t, 1, count)
	_, err := st.Pool.Exec(ctx, `UPDATE role_member_admission_receipts SET binding_sha256=$2 WHERE space_id=$1`, space, make([]byte, 32))
	require.Error(t, err)
	_, err = st.Pool.Exec(ctx, role14SQL(t, "000021_member_admission_provenance.down.sql"))
	require.Error(t, err, "saved evidence must prevent downgrade")
}

func TestMemberAdmissionCannotMintCanonicalOwnerThroughDefault(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile := uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, uuid.New()))
	ownerRole, err := st.RoleIDByName(ctx, space, "Owner")
	require.NoError(t, err)
	require.NoError(t, st.SetDefaultJoinRole(ctx, space, ownerRole))
	_, err = st.PrepareDefaultMemberRole(ctx, ensureMemberBinding(space, profile, 1))
	require.Error(t, err, "Owner is exclusively bootstrap/ownership transfer, not admission")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM role_member_admission_receipts WHERE space_id=$1`, space).Scan(&count))
	require.Zero(t, count)
}

func TestMemberAdmissionP3RetentionNeverAllowsIdentityReuse(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile := uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, uuid.New()))
	_, err := st.PrepareDefaultMemberRole(ctx, ensureMemberBinding(space, profile, 1))
	require.NoError(t, err)
	_, err = st.RemoveOrdinaryMemberRoles(ctx, removeMemberBinding(space, profile, 1))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `DELETE FROM role_member_admission_receipts WHERE space_id=$1`, space)
	require.Error(t, err, "live evidence cannot be deleted")
	freshSpace := uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, freshSpace, uuid.New()))
	_, err = st.PrepareDefaultMemberRole(ctx, ensureMemberBinding(freshSpace, profile, 1))
	require.NoError(t, err)
	_, err = st.RemoveOrdinaryMemberRoles(ctx, removeMemberBinding(freshSpace, profile, 1))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,clock_timestamp()-interval '29 days')`, freshSpace)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,clock_timestamp()-interval '31 days')`, space)
	require.NoError(t, err)
	require.NoError(t, st.CleanupSpaceDeletionFenceEvidence(ctx), "existing P3 worker must cover admission evidence")
	for _, table := range []string{"role_member_admission_receipts", "role_member_admission_preparations", "role_member_source_floors"} {
		var count int
		require.NoError(t, st.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE space_id=$1", space).Scan(&count))
		require.Zero(t, count, "private member evidence must not survive P30D after retirement")
	}
	var retained int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM role_member_admission_receipts WHERE space_id=$1`, freshSpace).Scan(&retained))
	require.Equal(t, 2, retained, "retirement alone must not erase unexpired historical evidence")
	_, err = st.Pool.Exec(ctx, `DELETE FROM role_member_source_floors WHERE space_id=$1`, freshSpace)
	require.Error(t, err, "floor remains immutable until P30D")
	_, err = st.PrepareDefaultMemberRole(ctx, ensureMemberBinding(space, profile, 2))
	require.ErrorIs(t, err, ErrSpaceRetired, "permanent retirement dominates after private floors are erased")
}
