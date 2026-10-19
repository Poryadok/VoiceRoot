package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestNative44RetireCandidateWhileFrozen(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	owner, account := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "R23 durable lifecycle", "", "public")
	require.NoError(t, err)
	db.Run(true, "goto", "41")
	input := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	intent, err := st.PrepareMemberAdmission(ctx, input)
	require.NoError(t, err)
	policy, err := st.GetEntryPolicy(ctx, space.ID)
	require.NoError(t, err)
	changed := policy.Policy
	changed.RequireCaptcha = true
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: owner, OperationID: uuid.New(), ExpectedVersion: policy.Version, Policy: changed})
	require.NoError(t, err)
	_, err = st.ReserveLifecycleSchedule(ctx, account, owner, 7, lifecycleScheduleRequest(space.ID, uuid.New()))
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, st.InitializeNativeMembership(ctx, space.ID), "admin UNKNOWN cutover supports authoritative frozen maintenance")
	login := nativeReadinessLogin(t, ctx, db.Pool)
	cfg := login.Config().Copy()
	cfg.ConnConfig.RuntimeParams["default_transaction_isolation"] = "repeatable read"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	serving := &SpaceStore{Pool: pool}
	var isolation string
	require.NoError(t, pool.QueryRow(ctx, `SHOW default_transaction_isolation`).Scan(&isolation))
	require.Equal(t, "repeatable read", isolation, "owned maintenance must explicitly select RC despite caller default")
	_, err = serving.RetireCandidate(ctx, input.OperationID, intent.Generation+1, MemberAdmissionRetirePolicyChanged)
	require.ErrorIs(t, err, ErrMemberAdmissionStale)
	_, err = serving.RetireCandidate(ctx, input.OperationID, intent.Generation, MemberAdmissionRetireInviteInvalid)
	require.ErrorIs(t, err, ErrMemberAdmissionConflict, "no caller assertion can replace the actual invalidation predicate")
	retired, err := serving.RetireCandidate(ctx, input.OperationID, intent.Generation, MemberAdmissionRetirePolicyChanged)
	require.NoError(t, err, "actual frozen retirement must complete its DB mutation frame")
	require.Equal(t, "RETIRED", retired.State)
	_, err = serving.RetireCandidate(ctx, input.OperationID, intent.Generation, MemberAdmissionRetirePolicyChanged)
	require.NoError(t, err, "saved retirement replay is inert")
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames WHERE space_id=$1`, space.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_members WHERE space_id=$1`, space.ID).Scan(&count))
	require.Equal(t, 1, count, "maintenance preserves the actual owner member")
	_, err = serving.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()})
	require.Error(t, err, "negative maintenance never permits a positive frozen admission")
}
