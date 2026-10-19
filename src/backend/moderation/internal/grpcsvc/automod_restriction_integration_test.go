package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
)

func TestAutoModRestriction_CompleteSetAndStaleAckCannotDropNewIntent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	s := automod.NewStore(pool)
	sanctions := &store.SanctionStore{Pool: pool}
	subject := automod.Subject{ID: uuid.New(), AccountID: uuid.New(), Epoch: uuid.New(), Revision: 1, State: "LIVE"}
	require.NoError(t, s.Provision(ctx, subject))
	initial, err := s.ReadRestriction(ctx, subject.AccountID, subject.ID)
	require.NoError(t, err)
	require.False(t, initial.Banned)
	first, second := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{first, second} {
		require.NoError(t, sanctions.WithAccountLock(ctx, subject.AccountID, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `INSERT INTO sanctions(id,target_account_id,type,reason,issued_by) VALUES($1,$2,'perm_ban','manual test',$3)`, id, subject.AccountID, uuid.New()); err != nil {
				return err
			}
			return s.QueueRestrictionTx(ctx, tx, subject.AccountID)
		}))
	}
	pending, err := s.PendingRestrictions(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.True(t, pending[0].Banned)
	old := pending[0]
	for _, id := range []uuid.UUID{first, second} {
		require.NoError(t, sanctions.WithAccountLock(ctx, subject.AccountID, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE sanctions SET revoked_at=clock_timestamp(),revoked_by=$2 WHERE id=$1`, id, uuid.New()); err != nil {
				return err
			}
			return s.QueueRestrictionTx(ctx, tx, subject.AccountID)
		}))
		snapshot, err := s.ReadRestriction(ctx, subject.AccountID, subject.ID)
		require.NoError(t, err)
		require.Equal(t, id == first, snapshot.Banned, "one remaining effective ban must keep the account restricted")
	}
	require.NoError(t, s.CompleteRestriction(ctx, old))
	pending, err = s.PendingRestrictions(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.False(t, pending[0].Banned)
	require.Greater(t, pending[0].Revision, old.Revision)
	require.NoError(t, s.CompleteRestriction(ctx, pending[0]))
	pending, err = s.PendingRestrictions(ctx)
	require.NoError(t, err)
	require.Empty(t, pending)
	var through, count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT through_sequence,post_mute_count FROM automod_subjects WHERE subject_id=$1`, subject.ID).Scan(&through, &count))
	require.Zero(t, through)
	require.Zero(t, count, "admission/reconciliation never count uncommitted sends")
}
