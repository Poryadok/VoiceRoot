package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/moderation/internal/automod"
	"voice/backend/pkg/automodclient"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

type restrictionSyncFunc func(context.Context, uuid.UUID, uuid.UUID, uint64, bool) (*automodv1.ReconcileModerationRestrictionResponse, error)

func (f restrictionSyncFunc) Reconcile(ctx context.Context, a, s uuid.UUID, r uint64, b bool) (*automodv1.ReconcileModerationRestrictionResponse, error) {
	return f(ctx, a, s, r, b)
}
func repairGate(t *testing.T) *automod.BootGate {
	t.Helper()
	g := automod.NewBootGate()
	id, generation := g.Identity()
	for phase := automod.BootLocalIssuer; phase <= automod.BootTerminalImported; phase++ {
		require.NoError(t, g.Advance(automod.BootReceipt{BootID: id, Generation: generation, Phase: phase, ProofID: uuid.New()}))
	}
	return g
}
func exactRestrictionAck(a, s uuid.UUID, r uint64, b bool) *automodv1.ReconcileModerationRestrictionResponse {
	return &automodv1.ReconcileModerationRestrictionResponse{ProtocolVersion: 1, AccountId: a.String(), SubjectId: s.String(), Revision: r, ModerationBanned: b, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}
}

func TestRestrictionSync_CurrentBootAndExactAckRequired(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	s := automod.NewStore(pool)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
	require.NoError(t, s.Provision(ctx, subject))
	_, err := s.RestrictionSnapshot(ctx, subject.AccountID, subject.ID)
	require.NoError(t, err)
	called := false
	closed := automod.NewBootGate()
	client := restrictionSyncFunc(func(_ context.Context, a, b uuid.UUID, r uint64, ban bool) (*automodv1.ReconcileModerationRestrictionResponse, error) {
		called = true
		return exactRestrictionAck(a, b, r, ban), nil
	})
	require.ErrorIs(t, s.SyncRestrictions(ctx, closed, client), automod.ErrBootClosed)
	require.False(t, called)
	g := repairGate(t)
	client = restrictionSyncFunc(func(_ context.Context, a, b uuid.UUID, r uint64, ban bool) (*automodv1.ReconcileModerationRestrictionResponse, error) {
		return exactRestrictionAck(a, uuid.New(), r, ban), nil
	})
	require.ErrorIs(t, s.SyncRestrictions(ctx, g, client), automod.ErrRestrictionAck)
	client = restrictionSyncFunc(func(_ context.Context, a, b uuid.UUID, r uint64, ban bool) (*automodv1.ReconcileModerationRestrictionResponse, error) {
		g.Close()
		return exactRestrictionAck(a, b, r, ban), nil
	})
	require.ErrorIs(t, s.SyncRestrictions(ctx, g, client), automod.ErrBootClosed)
	pending, err := s.PendingRestrictions(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.EqualValues(t, 1, pending[0].Revision)
}

func TestRestrictionSync_StaleAckPreservesNewerIntent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	s := automod.NewStore(pool)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
	require.NoError(t, s.Provision(ctx, subject))
	_, err := s.RestrictionSnapshot(ctx, subject.AccountID, subject.ID)
	require.NoError(t, err)
	g := repairGate(t)
	client := restrictionSyncFunc(func(callCtx context.Context, a, b uuid.UUID, r uint64, ban bool) (*automodv1.ReconcileModerationRestrictionResponse, error) {
		tx, err := pool.Begin(callCtx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		_, err = tx.Exec(callCtx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, a.String())
		require.NoError(t, err)
		_, err = tx.Exec(callCtx, `UPDATE automod_subjects SET restriction_revision=restriction_revision+1,desired_banned=true WHERE account_id=$1`, a)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(callCtx))
		return exactRestrictionAck(a, b, r, ban), nil
	})
	require.NoError(t, s.SyncRestrictions(ctx, g, client))
	pending, err := s.PendingRestrictions(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.True(t, pending[0].Banned)
	require.EqualValues(t, 2, pending[0].Revision)
	client = restrictionSyncFunc(func(_ context.Context, a, b uuid.UUID, r uint64, ban bool) (*automodv1.ReconcileModerationRestrictionResponse, error) {
		return exactRestrictionAck(a, b, r, ban), nil
	})
	require.NoError(t, s.SyncRestrictions(ctx, g, client))
	pending, err = s.PendingRestrictions(ctx)
	require.NoError(t, err)
	require.Empty(t, pending)
}

func TestRestrictionSync_AheadUsesCurrentLocalRevision(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	s := automod.NewStore(pool)
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
	require.NoError(t, s.Provision(ctx, subject))
	_, err := s.RestrictionSnapshot(ctx, subject.AccountID, subject.ID)
	require.NoError(t, err)
	g := repairGate(t)
	client := restrictionSyncFunc(func(callCtx context.Context, a, b uuid.UUID, r uint64, ban bool) (*automodv1.ReconcileModerationRestrictionResponse, error) {
		tx, err := pool.Begin(callCtx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		var acquired bool
		require.NoError(t, tx.QueryRow(callCtx, `SELECT pg_try_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, a.String()).Scan(&acquired))
		require.True(t, acquired, "remote call must not hold account lock")
		_, err = tx.Exec(callCtx, `UPDATE automod_subjects SET restriction_revision=restriction_revision+1 WHERE account_id=$1`, a)
		require.NoError(t, err)
		require.NoError(t, tx.Commit(callCtx))
		return nil, &automodclient.RestrictionAheadError{Requested: r, Accepted: r + 1}
	})
	require.NoError(t, s.SyncRestrictions(ctx, g, client))
	_, err = g.AcquirePrivateRepair(ctx)
	require.NoError(t, err, "newer committed local request explains ahead reply")
	client = restrictionSyncFunc(func(_ context.Context, a, b uuid.UUID, r uint64, ban bool) (*automodv1.ReconcileModerationRestrictionResponse, error) {
		return nil, &automodclient.RestrictionAheadError{Requested: r, Accepted: r + 1}
	})
	require.ErrorIs(t, s.SyncRestrictions(ctx, g, client), automod.ErrRestoreDivergence)
	_, err = g.AcquirePrivateRepair(ctx)
	require.ErrorIs(t, err, automod.ErrBootClosed)
	pending, err := s.PendingRestrictions(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.EqualValues(t, 2, pending[0].Revision, "restore divergence must not bump local history to win")
}
