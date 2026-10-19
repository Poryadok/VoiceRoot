package store

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemberAdmissionContinuationDenialRetirementWaitPreservesCandidate(t *testing.T) {
	t.Run("current guest denial", func(t *testing.T) { testContinuationRetirementWait(t, false) })
	t.Run("known local guard denial", func(t *testing.T) { testContinuationRetirementWait(t, true) })
}

func testContinuationRetirementWait(t *testing.T, guardDenial bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st := nativeCreatorFixture(t)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	i, role, receipt := prepareContinuationFixture(t, st, false, guardDenial)
	now := time.Now()
	var clock atomic.Int64
	clock.Store(now.UnixNano())
	currentType := "guest"
	if guardDenial {
		currentType = "regular"
	}
	client := continuationClient(t, &now, currentType)
	client.Clock = func() time.Time { return time.Unix(0, clock.Load()) }
	proof, err := st.ObserveMemberAdmissionContinuation(ctx, i.Input.OperationID, client, nil)
	require.NoError(t, err)
	block, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer rollbackLifecycleTx(context.Background(), block)
	_, err = block.Exec(ctx, `LOCK TABLE space_member_role_cleanup IN SHARE MODE`)
	require.NoError(t, err)
	result := make(chan error, 1)
	var guard MemberAdmissionFinalGuard
	if guardDenial {
		// Controlled local guard denial tests the second retirement branch,
		// never a caller-supplied owner capability or production allow.
		guard = func(context.Context, *SpaceStore, *MemberAdmissionIntent) error { return ErrInviteRevoked }
	}
	go func() {
		_, err := st.PublishMemberAdmissionWithContinuation(ctx, i.Input.OperationID, role, receipt, proof, guard)
		result <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%INSERT INTO space_member_role_cleanup%' AND $1::integer=ANY(pg_blocking_pids(pid)))`, int(block.Conn().PgConn().PID())).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "retirement must actually reach blocked cleanup SQL after its initial proof check")
	clock.Add(int64(5 * time.Second))
	require.NoError(t, block.Rollback(ctx))
	select {
	case err = <-result:
		require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	case <-ctx.Done():
		t.Fatal("retirement did not finish after releasing owned SQL lock")
	}
	saved, err := st.LookupMemberAdmission(ctx, i.Input.OperationID)
	require.NoError(t, err)
	require.Equal(t, "PREPARING", saved.State)
	var floor, cleanup int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retired_through,(SELECT count(*) FROM space_member_role_cleanup WHERE space_id=$1 AND profile_id=$2) FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, i.Input.SpaceID, i.Input.ProfileID).Scan(&floor, &cleanup))
	require.Zero(t, floor)
	require.Zero(t, cleanup)
}
