package automod

import (
	"context"
	"github.com/jackc/pgx/v5"
)

// BindRetainedSubjectTx serves owner-authorized privacy recovery and retention
// rechecks of an already accepted reporter. New submission/target admission
// still requires LIVE authority; this helper grants no report/vote eligibility.
// The caller holds every relevant sorted account lock before this call.
func (s *Store) BindRetainedSubjectTx(ctx context.Context, tx pgx.Tx, binding Subject) error {
	if tx == nil || !validLifecycle(binding) {
		return ErrSubject
	}
	if err := lockSubject(ctx, tx, binding.ID); err != nil {
		return err
	}
	if binding.State == "DELETED" {
		var retained bool
		if err := tx.QueryRow(ctx, `SELECT $1::timestamptz>clock_timestamp()`, binding.PurgeAt).Scan(&retained); err != nil {
			return err
		}
		if !retained {
			// A stale captured deletion deadline is not a permanent terminal
			// assertion and must never override a newer local LIVE restore.
			return ErrSubject
		}
	}
	if err := s.BindSubjectTx(ctx, tx, binding); err != nil {
		return err
	}
	return s.RequireRetainedSubjectTx(ctx, tx, binding)
}

// RequireRetainedSubjectTx is a privacy-only recheck, including a fresh database
// deadline. It does not itself admit a new send, decide report validity/voting
// eligibility, or change counters. An accepted reporter may remain retained
// DELETED until its deadline while the target keeps its separate LIVE check.
// Call again before committing privacy recovery after any intervening work.
func (s *Store) RequireRetainedSubjectTx(ctx context.Context, tx pgx.Tx, binding Subject) error {
	if tx == nil || !validLifecycle(binding) {
		return ErrSubject
	}
	if err := lockSubject(ctx, tx, binding.ID); err != nil {
		return err
	}
	var retained bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_subjects a WHERE a.subject_id=$1 AND a.account_id=$2 AND a.lifecycle_revision=$3 AND a.lifecycle_state=$4 AND a.purge_at IS NOT DISTINCT FROM $5::timestamptz AND a.deletion_cycle_id IS NOT DISTINCT FROM $6::uuid AND (a.lifecycle_state='LIVE' AND a.purge_at IS NULL OR a.lifecycle_state='DELETED' AND a.purge_at>clock_timestamp()) AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=a.subject_id))`, binding.ID, binding.AccountID, binding.Revision, binding.State, binding.PurgeAt, nullableCycle(binding.CycleID)).Scan(&retained)
	if err != nil {
		return err
	}
	if !retained {
		return ErrSubject
	}
	return nil
}
