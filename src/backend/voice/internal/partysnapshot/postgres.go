package partysnapshot

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// NativeCutReader reads all native authorities, mappings and fences using this
// exact transaction. It must not start a second transaction or perform Auth IO.
// Source completeness is owning durable evidence, never a caller-provided flag.
type NativeCutReader interface {
	ReadNativeCut(context.Context, pgx.Tx, Subject) (Candidate, error)
}

type TransactionStarter interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// PostgresReader makes the native source boundary explicit. Every pass is a new
// short read-only MVCC transaction; the final clock sample observes deadlines
// after all source reads rather than transaction_timestamp at BEGIN.
type PostgresReader struct {
	Database TransactionStarter
	Source   NativeCutReader
}

func (r *PostgresReader) Read(ctx context.Context, subject Subject) (Candidate, error) {
	if r == nil || r.Database == nil || r.Source == nil {
		return Candidate{}, ErrUnavailable
	}
	tx, err := r.Database.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Candidate{}, ErrUnavailable
	}
	defer func() {
		// Cleanup is bounded even when the request's context is already done.
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	candidate, err := r.Source.ReadNativeCut(ctx, tx, subject)
	if err != nil {
		return Candidate{}, ErrUnavailable
	}
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&candidate.DatabaseTime); err != nil {
		return Candidate{}, ErrUnavailable
	}
	if err := tx.Commit(ctx); err != nil {
		return Candidate{}, ErrUnavailable
	}
	return candidate, nil
}
