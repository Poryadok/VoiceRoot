package reportaudience

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresSource reads only User-owned data. Auth account state is classified separately.
type PostgresSource struct{ Pool *pgxpool.Pool }

type bindingReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readBinding(ctx context.Context, reader bindingReader, profile uuid.UUID) (Binding, error) {
	out := Binding{ProfileID: profile}
	var revision int64
	var deleted bool
	err := reader.QueryRow(ctx, `SELECT account_id,sdk_eligibility_revision,deleted_at IS NOT NULL FROM profiles WHERE id=$1`, profile).Scan(&out.AccountID, &revision, &deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		out.State = Missing
		return out, nil
	}
	if err != nil {
		return Binding{}, err
	}
	if revision <= 0 || out.AccountID == uuid.Nil {
		return Binding{}, ErrUnavailable
	}
	out.Revision = uint64(revision)
	out.State = Live
	if deleted {
		out.State = Deleted
	}
	return out, nil
}

func (s PostgresSource) Snapshot(ctx context.Context, profile uuid.UUID) (Snapshot, error) {
	if s.Pool == nil {
		return Snapshot{}, ErrUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	binding, err := readBinding(ctx, tx, profile)
	if err != nil {
		return Snapshot{}, err
	}
	out := Snapshot{Binding: binding}
	if binding.State == Live {
		// Frozen and guest profiles deliberately remain candidates. Multiple profiles
		// cannot inflate an account, and the complete target account is excluded.
		rows, queryErr := tx.Query(ctx, `SELECT DISTINCT account_id FROM profiles WHERE deleted_at IS NULL AND account_id<>$1 ORDER BY account_id`, binding.AccountID)
		if queryErr != nil {
			return Snapshot{}, queryErr
		}
		for rows.Next() {
			var id uuid.UUID
			if scanErr := rows.Scan(&id); scanErr != nil {
				rows.Close()
				return Snapshot{}, scanErr
			}
			out.Candidates = append(out.Candidates, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return Snapshot{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Snapshot{}, err
	}
	return out, nil
}

func (s PostgresSource) Recheck(ctx context.Context, expected Binding) error {
	if s.Pool == nil {
		return ErrUnavailable
	}
	current, err := readBinding(ctx, s.Pool, expected.ProfileID)
	if err != nil {
		return err
	}
	if current != expected {
		return ErrUnavailable
	}
	return nil
}
