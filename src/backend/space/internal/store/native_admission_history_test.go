package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/memberadmission"
)

func TestNativeAdmissionWriterHistoryAndFirstPublicationHook(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL admission publication history")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Native admission history", "", "public")
	require.NoError(t, err)
	db.Run(true, "up")
	input := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	intent, err := st.PrepareMemberAdmission(ctx, input)
	require.NoError(t, err)
	var state string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, input.ProfileID).Scan(&state), "actual Prepare must finish NONE history in the same transaction")
	require.Equal(t, "NONE", state)
	role := uuid.New()
	receipt, err := memberadmission.EnsureReceiptFingerprint(intent.EnsureBinding(), role.String())
	require.NoError(t, err)
	calls := 0
	hook := func(ctx context.Context, scoped *SpaceStore, i *MemberAdmissionIntent, m *MembershipRow) error {
		calls++
		require.NotNil(t, scoped.tx)
		require.Equal(t, input.OperationID, i.Input.OperationID)
		require.Equal(t, input.ProfileID, m.ProfileID)
		var published bool
		require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_member_admission_intents i JOIN space_members m ON m.space_id=i.space_id AND m.profile_id=i.profile_id AND m.admission_operation_id=i.operation_id AND m.membership_generation=i.generation JOIN space_member_admission_outbox o ON o.operation_id=i.operation_id WHERE i.operation_id=$1 AND i.state='PUBLISHED')`, input.OperationID).Scan(&published))
		require.True(t, published, "hook must observe the actual atomic published outcome")
		return nil
	}
	member, err := st.publishMemberAdmission(ctx, input.OperationID, role, receipt[:], nil, nil, hook)
	require.NoError(t, err)
	require.NotNil(t, member)
	require.Equal(t, 1, calls)
	var known bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state,boundary_state='KNOWN' FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, input.ProfileID).Scan(&state, &known))
	require.Equal(t, "ACTIVE", state)
	require.True(t, known)
	replay, err := st.publishMemberAdmission(ctx, input.OperationID, role, receipt[:], nil, nil, func(context.Context, *SpaceStore, *MemberAdmissionIntent, *MembershipRow) error {
		t.Fatal("historical replay invoked new completion callback")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, member.JoinedAt, replay.JoinedAt)
	require.Equal(t, 1, calls)
	failedInput := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	failed, err := st.PrepareMemberAdmission(ctx, failedInput)
	require.NoError(t, err)
	failedReceipt, err := memberadmission.EnsureReceiptFingerprint(failed.EnsureBinding(), role.String())
	require.NoError(t, err)
	sentinel := errors.New("local publication journal rejected")
	_, err = st.publishMemberAdmission(ctx, failedInput.OperationID, role, failedReceipt[:], nil, nil, func(context.Context, *SpaceStore, *MemberAdmissionIntent, *MembershipRow) error { return sentinel })
	require.ErrorIs(t, err, sentinel)
	saved, err := st.LookupMemberAdmission(ctx, failedInput.OperationID)
	require.NoError(t, err)
	require.Equal(t, "PREPARING", saved.State)
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, failedInput.ProfileID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_member_admission_outbox WHERE operation_id=$1`, failedInput.OperationID).Scan(&count))
	require.Zero(t, count)
	// A bookkeeping callback cannot turn successful publication into a removal
	// and still return a positive saved membership response.
	_, err = st.publishMemberAdmission(ctx, failedInput.OperationID, role, failedReceipt[:], nil, nil, func(ctx context.Context, scoped *SpaceStore, i *MemberAdmissionIntent, _ *MembershipRow) error {
		_, err := scoped.tx.Exec(ctx, `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2`, i.Input.SpaceID, i.Input.ProfileID)
		return err
	})
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
	saved, err = st.LookupMemberAdmission(ctx, failedInput.OperationID)
	require.NoError(t, err)
	require.Equal(t, "PREPARING", saved.State)
	_, err = st.publishMemberAdmission(ctx, failedInput.OperationID, role, failedReceipt[:], nil, nil, func(ctx context.Context, scoped *SpaceStore, _ *MemberAdmissionIntent, _ *MembershipRow) error {
		_, err := scoped.tx.Exec(ctx, `SELECT pg_sleep(5.05)`)
		return err
	})
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable, "final common clock follows every postpublication hook SQL wait")
	saved, err = st.LookupMemberAdmission(ctx, failedInput.OperationID)
	require.NoError(t, err)
	require.Equal(t, "PREPARING", saved.State)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, failedInput.ProfileID).Scan(&state))
	require.Equal(t, "NONE", state)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_member_admission_outbox WHERE operation_id=$1`, failedInput.OperationID).Scan(&count))
	require.Zero(t, count)
}
