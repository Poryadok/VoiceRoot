package identityassociation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestNativeDueRawErasePreparationCannotSurviveTransactionOrClaimNoop(t *testing.T) {
	pool, profile, account := nativeDatabase(t)
	store, _ := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, store.ActivateGuards(ctx))
	var before uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&before))
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, PrepareDueRawEraseTx(ctx, tx, profile, dueParentForTest(account)))
	require.NoError(t, tx.Commit(ctx))
	_, err = pool.Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
	require.Error(t, err, "a previous transaction's preparation cannot authorize deletion")
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, PrepareDueRawEraseTx(ctx, tx, profile, dueParentForTest(account)))
	tag, err := tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, uuid.New())
	require.NoError(t, err)
	require.Zero(t, tag.RowsAffected())
	require.NoError(t, tx.Commit(ctx))
	var head uint64
	var gaps, remaining, markers int
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head,(SELECT count(*) FROM identity_journal WHERE kind='UNRESOLVED_RAW_ERASURE'),(SELECT count(*) FROM profiles),(SELECT count(*) FROM pg_class WHERE relname='user_identity_due_erasure') FROM identity_meta WHERE singleton`).Scan(&head, &gaps, &remaining, &markers))
	require.Equal(t, before, head)
	require.Zero(t, gaps)
	require.Equal(t, 1, remaining)
	require.Zero(t, markers)
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, PrepareDueRawEraseTx(ctx, tx, profile, dueParentForTest(account)))
	tag, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
	require.NoError(t, err)
	require.EqualValues(t, 1, tag.RowsAffected())
	require.NoError(t, tx.Commit(ctx))
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	tag, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
	require.NoError(t, err)
	require.Zero(t, tag.RowsAffected())
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head,(SELECT count(*) FROM identity_journal WHERE kind='UNRESOLVED_RAW_ERASURE') FROM identity_meta WHERE singleton`).Scan(&head, &gaps))
	require.Equal(t, before+1, head)
	require.Equal(t, 1, gaps, "zero-row retry does not claim another erased identity")
}

func TestNativeBirthAndUnresolvedEraseSerializeBothOrders(t *testing.T) {
	for _, eraseFirst := range []bool{true, false} {
		name := "birth-first"
		if eraseFirst {
			name = "erase-first"
		}
		t.Run(name, func(t *testing.T) {
			pool, erasedProfile, erasedAccount := nativeDatabase(t)
			store, keys := nativeStore(t, pool)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			require.NoError(t, store.ActivateGuards(ctx))
			bornProfile, bornAccount := uuid.New(), uuid.New()
			prepared, err := keys.PrepareSelectors(bornProfile, bornAccount)
			require.NoError(t, err)
			birthProof := VerifiedParentProof{verified: true, observation: validParentForTest(bornAccount)}
			birthProof.observation.OwnerHandle = "birth-parent-" + bornAccount.String()
			eraseProof := dueParentForTest(erasedAccount)
			eraseProof.observation.OwnerHandle = "erase-parent-" + erasedAccount.String()
			fence := nativeFence(t, pool)
			holder, err := store.Begin(ctx)
			require.NoError(t, err)
			defer holder.Rollback(context.Background())
			birth := func(cap *IdentityTx) error {
				if _, err := store.CreateBirthTx(ctx, cap, prepared, birthProof); err != nil {
					return err
				}
				_, err := cap.SQL().Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'orderedbirth','4321','Ordered birth')`, bornProfile, bornAccount)
				return err
			}
			erase := func(cap *IdentityTx) error {
				if err := PrepareDueRawEraseTx(ctx, cap, erasedProfile, eraseProof); err != nil {
					return err
				}
				_, err := cap.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, erasedProfile)
				return err
			}
			if eraseFirst {
				require.NoError(t, erase(holder))
			} else {
				require.NoError(t, birth(holder))
			}
			waiter, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
			require.NoError(t, err)
			defer waiter.Rollback(context.Background())
			returned := make(chan error, 1)
			go func() {
				cap, acquireErr := acquireIdentityTx(ctx, waiter, fence)
				if acquireErr != nil {
					returned <- acquireErr
					return
				}
				var operationErr error
				if eraseFirst {
					operationErr = birth(cap)
				} else {
					operationErr = erase(cap)
				}
				if operationErr == nil {
					operationErr = cap.Commit(ctx)
				} else {
					_ = cap.Rollback(context.Background())
				}
				returned <- operationErr
			}()
			nativeWaitEdge(t, pool, waiter.Conn().PgConn().PID(), holder.SQL().Conn().PgConn().PID(), returned)
			require.NoError(t, holder.Commit(ctx))
			operationErr := <-returned
			wantBirth := 1
			if eraseFirst {
				require.ErrorIs(t, operationErr, ErrIncomplete, "freshly valid active proof cannot bypass a committed namespace gap")
				wantBirth = 0
			} else {
				require.NoError(t, operationErr)
			}
			var head, issued uint64
			var born, erased, reservations, associations, gaps int
			require.NoError(t, pool.QueryRow(ctx, `SELECT history_head,unresolved_erase_issued,(SELECT count(*) FROM profiles WHERE id=$1),(SELECT count(*) FROM profiles WHERE id=$2),(SELECT count(*) FROM identity_profile_reservations),(SELECT count(*) FROM identity_associations),(SELECT count(*) FROM identity_journal WHERE kind='UNRESOLVED_RAW_ERASURE') FROM identity_meta WHERE singleton`, bornProfile, erasedProfile).Scan(&head, &issued, &born, &erased, &reservations, &associations, &gaps))
			require.Equal(t, head, issued)
			require.Equal(t, wantBirth, born)
			require.Zero(t, erased)
			require.Equal(t, wantBirth, reservations)
			require.Equal(t, wantBirth, associations)
			require.Equal(t, 1, gaps)
			if !eraseFirst {
				observed, err := store.CaptureProfiles(ctx, []uuid.UUID{bornProfile})
				require.NoError(t, err)
				require.Equal(t, CaptureAssociated, observed.Observations[bornProfile].Status)
				require.Equal(t, OriginBirth, observed.Observations[bornProfile].Fact.Origin)
			}
		})
	}
}
