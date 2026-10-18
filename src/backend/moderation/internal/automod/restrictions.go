package automod

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"time"
)

type Restriction struct {
	AccountID, SubjectID uuid.UUID
	Revision             uint64
	Banned               bool
	MutedUntil           *time.Time
}

// ReadRestriction is a side-effect-free send admission read. It never examines
// message content, counts a sample or advances a post-mute position.
func (s *Store) ReadRestriction(ctx context.Context, account, subject uuid.UUID) (Restriction, error) {
	result := Restriction{AccountID: account, SubjectID: subject}
	if s == nil || s.Pool == nil || account == uuid.Nil || subject == uuid.Nil {
		return result, ErrSubject
	}
	var state string
	var terminal bool
	var actual *uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(a.restriction_revision,0),a.muted_until,COALESCE(a.lifecycle_state,'LIVE'),
	EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=$2) OR COALESCE(a.purge_at<=clock_timestamp(),false),
	EXISTS(SELECT 1 FROM sanctions s WHERE s.target_account_id=$1 AND s.type IN ('perm_ban','temp_ban') AND s.revoked_at IS NULL AND (s.expires_at IS NULL OR s.expires_at>clock_timestamp())),a.subject_id
	FROM (SELECT 1) singleton LEFT JOIN automod_subjects a ON a.account_id=$1`, account, subject).Scan(&result.Revision, &result.MutedUntil, &state, &terminal, &result.Banned, &actual)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrSubject
	}
	if err != nil {
		return result, err
	}
	if terminal {
		return result, ErrPurged
	}
	if actual != nil && *actual != subject {
		return result, ErrSubject
	}
	if state != "LIVE" {
		result.Banned = true
	}
	return result, nil
}

// QueueRestrictionTx derives the complete effective ban set after any sanction
// mutation, under the shared account lock. Persisted value and revision remain
// stable while network delivery is retried. Shadow/mute never suspend Auth.
func (s *Store) QueueRestrictionTx(ctx context.Context, tx pgx.Tx, account uuid.UUID) error {
	if tx == nil || account == uuid.Nil {
		return ErrSubject
	}
	var subject uuid.UUID
	var banned bool
	err := tx.QueryRow(ctx, `SELECT a.subject_id,EXISTS(SELECT 1 FROM sanctions s WHERE s.target_account_id=a.account_id AND s.type IN ('perm_ban','temp_ban') AND s.revoked_at IS NULL AND (s.expires_at IS NULL OR s.expires_at>clock_timestamp())) FROM automod_subjects a WHERE a.account_id=$1 AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=a.subject_id) FOR UPDATE`, account).Scan(&subject, &banned)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSubject
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE automod_subjects SET restriction_revision=restriction_revision+1,desired_banned=$2 WHERE subject_id=$1 AND (restriction_revision=0 OR desired_banned IS DISTINCT FROM $2)`, subject, banned)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO automod_restriction_sync(subject_id) VALUES($1) ON CONFLICT(subject_id) DO UPDATE SET requested_at=clock_timestamp()`, subject)
	return err
}

func (s *Store) RestrictionSnapshot(ctx context.Context, account, subject uuid.UUID) (result Restriction, err error) {
	result = Restriction{AccountID: account, SubjectID: subject}
	err = s.accountTx(ctx, account, func(tx pgx.Tx) error {
		var actual uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT subject_id FROM automod_subjects WHERE account_id=$1`, account).Scan(&actual); err != nil {
			return err
		}
		if actual != subject {
			return ErrSubject
		}
		if err := s.QueueRestrictionTx(ctx, tx, account); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT restriction_revision,desired_banned FROM automod_subjects WHERE subject_id=$1`, subject).Scan(&result.Revision, &result.Banned)
	})
	return result, err
}

func (s *Store) PendingRestrictions(ctx context.Context) ([]Restriction, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrSubject
	}
	rows, err := s.Pool.Query(ctx, `SELECT a.account_id,a.subject_id,a.restriction_revision,a.desired_banned FROM automod_restriction_sync q JOIN automod_subjects a USING(subject_id) ORDER BY q.requested_at,a.subject_id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Restriction
	for rows.Next() {
		var r Restriction
		if err := rows.Scan(&r.AccountID, &r.SubjectID, &r.Revision, &r.Banned); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
func (s *Store) CompleteRestriction(ctx context.Context, r Restriction) error {
	return s.accountTx(ctx, r.AccountID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM automod_restriction_sync q USING automod_subjects a WHERE q.subject_id=a.subject_id AND a.subject_id=$1 AND a.account_id=$2 AND a.restriction_revision=$3 AND a.desired_banned=$4`, r.SubjectID, r.AccountID, r.Revision, r.Banned)
		return err
	})
}
