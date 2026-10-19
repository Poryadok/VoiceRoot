package automod

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"voice/backend/pkg/automodwire"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

// InstallLifecycleAuthority is called only by the P1/P2 coordinator after it
// verifies Auth authority and operational continuity. An RPC fact cannot install
// or replace this generation. A restore requires separate verified repair.
func (s *Store) InstallLifecycleAuthority(ctx context.Context, generation uuid.UUID, check BootCheck) error {
	if check == nil {
		return ErrBootClosed
	}
	if s == nil || s.Pool == nil || generation == uuid.Nil {
		return ErrSubject
	}
	if err := check(ctx); err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	tag, err := tx.Exec(ctx, `INSERT INTO automod_lifecycle_authority(singleton,authority_generation) VALUES(true,$1) ON CONFLICT(singleton) DO UPDATE SET authority_generation=EXCLUDED.authority_generation WHERE automod_lifecycle_authority.authority_generation=EXCLUDED.authority_generation`, generation)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	if err = check(ctx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ApplyLifecycleFact is a private Auth-authority operation, never an account
// lookup or a timer. The gate and installed generation must already be verified.
// Its response is returned only after the complete transaction commits.
func (s *Store) ApplyLifecycleFact(ctx context.Context, fact *automodv1.AccountLifecycleFact, check BootCheck) (*automodv1.ApplyLifecycleFactResponse, error) {
	if check == nil {
		return nil, ErrBootClosed
	}
	body, digest, err := automodwire.EncodeLifecycleFact(fact)
	if err != nil {
		return nil, err
	}
	subject := uuid.MustParse(fact.GetSubjectId())
	generation := uuid.MustParse(fact.GetAuthorityGeneration())
	response := &automodv1.ApplyLifecycleFactResponse{ProtocolVersion: 1, AuthorityGeneration: generation.String(), SubjectId: subject.String(), RequestedEventId: fact.GetEventId()}
	err = s.withLockedSubjectChecked(ctx, subject, check, func(tx pgx.Tx, account uuid.UUID) error {
		var currentGeneration uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT authority_generation FROM automod_lifecycle_authority WHERE singleton FOR SHARE`).Scan(&currentGeneration); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBootClosed
			}
			return err
		}
		if currentGeneration != generation {
			return ErrConflict
		}
		var terminal bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, subject).Scan(&terminal); err != nil {
			return err
		}
		if terminal {
			if err := s.purgeSubjectTx(ctx, tx, subject, account); err != nil {
				return err
			}
			response.Terminal = true
			return nil
		}
		if fact.GetState() == automodv1.SubjectState_SUBJECT_STATE_PURGED {
			if err := s.purgeSubjectTx(ctx, tx, subject, account); err != nil {
				return err
			}
			response.Terminal = true
			return nil
		}
		// A fresh lookup may legitimately precede its push. Never regress that
		// binding or invent the missing newer canonical event/hash in a receipt.
		var boundRevision int64
		var boundState string
		var boundCycle *uuid.UUID
		var boundDeadline *time.Time
		if account != uuid.Nil {
			if err := tx.QueryRow(ctx, `SELECT lifecycle_revision,lifecycle_state,deletion_cycle_id,purge_at FROM automod_subjects WHERE subject_id=$1 FOR UPDATE`, subject).Scan(&boundRevision, &boundState, &boundCycle, &boundDeadline); err != nil {
				return err
			}
			if boundRevision > int64(fact.GetLifecycleRevision()) {
				// An already persisted exact higher fact can supply the receipt;
				// the lower request itself never proves that fact's identity.
				var revision int64
				var hash []byte
				var gen uuid.UUID
				err := tx.QueryRow(ctx, `SELECT lifecycle_revision,fact_sha256,authority_generation FROM automod_lifecycle_current WHERE subject_id=$1`, subject).Scan(&revision, &hash, &gen)
				if err != nil || revision != boundRevision || gen != generation {
					return ErrConflict
				}
				response.LifecycleRevision = uint64(revision)
				response.CurrentFactSha256 = hash
				return nil
			}
		}
		var revision int64
		var hash []byte
		var oldGeneration uuid.UUID
		var oldState string
		var oldCycle *uuid.UUID
		var oldDeadline *time.Time
		err := tx.QueryRow(ctx, `SELECT lifecycle_revision,fact_sha256,authority_generation,lifecycle_state,deletion_cycle_id,purge_at FROM automod_lifecycle_current WHERE subject_id=$1 FOR UPDATE`, subject).Scan(&revision, &hash, &oldGeneration, &oldState, &oldCycle, &oldDeadline)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			if oldGeneration != generation {
				return ErrConflict
			}
			if revision >= int64(fact.GetLifecycleRevision()) {
				if revision == int64(fact.GetLifecycleRevision()) && !bytes.Equal(hash, digest[:]) {
					return ErrConflict
				}
				response.LifecycleRevision = uint64(revision)
				response.CurrentFactSha256 = hash
				return nil
			}
		}
		state := "LIVE"
		var cycle *uuid.UUID
		var deadline *time.Time
		if fact.GetState() == automodv1.SubjectState_SUBJECT_STATE_DELETED {
			state = "DELETED"
			id := uuid.MustParse(fact.GetDeletionCycleId())
			cycle = &id
			at := fact.GetPurgeAt().AsTime()
			deadline = &at
		}
		if oldState == "DELETED" && state == "DELETED" && oldCycle != nil && cycle != nil && *oldCycle == *cycle && !equalDeadline(oldDeadline, deadline) {
			return ErrConflict
		}
		if account != uuid.Nil && boundRevision == int64(fact.GetLifecycleRevision()) {
			// Only an absent legacy cycle may be repaired at the same revision,
			// by this complete fresh Auth push, without recreating detector state.
			if boundState != state || !equalDeadline(boundDeadline, deadline) || (boundCycle != nil && (cycle == nil || *boundCycle != *cycle)) {
				return ErrConflict
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO automod_lifecycle_current(subject_id,authority_generation,event_id,lifecycle_revision,lifecycle_state,deletion_cycle_id,purge_at,canonical_fact,fact_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(subject_id) DO UPDATE SET authority_generation=EXCLUDED.authority_generation,event_id=EXCLUDED.event_id,lifecycle_revision=EXCLUDED.lifecycle_revision,lifecycle_state=EXCLUDED.lifecycle_state,deletion_cycle_id=EXCLUDED.deletion_cycle_id,purge_at=EXCLUDED.purge_at,canonical_fact=EXCLUDED.canonical_fact,fact_sha256=EXCLUDED.fact_sha256`, subject, generation, uuid.MustParse(fact.GetEventId()), int64(fact.GetLifecycleRevision()), state, cycle, deadline, body, digest[:])
		if err != nil {
			return err
		}
		if account != uuid.Nil {
			_, err = tx.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=$2,lifecycle_state=$3,deletion_cycle_id=$4,purge_at=$5 WHERE subject_id=$1`, subject, int64(fact.GetLifecycleRevision()), state, cycle, deadline)
			if err != nil {
				return lifecycleError(err)
			}
		}
		response.LifecycleRevision = fact.GetLifecycleRevision()
		response.CurrentFactSha256 = append([]byte(nil), digest[:]...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func equalDeadline(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}
func lifecycleError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "40001" && pg.Message == "automod_lifecycle_projection_conflict" {
		return ErrConflict
	}
	return err
}

// Preserve account-before-subject ordering, including the absent-mapping race.
func (s *Store) withLockedSubjectChecked(ctx context.Context, subject uuid.UUID, check BootCheck, fn func(pgx.Tx, uuid.UUID) error) error {
	if s == nil || s.Pool == nil || subject == uuid.Nil {
		return ErrSubject
	}
	if check != nil {
		if err := check(ctx); err != nil {
			return err
		}
	}
	var account uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT account_id FROM automod_subjects WHERE subject_id=$1`, subject).Scan(&account)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if account != uuid.Nil {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, account.String()); err != nil {
			return err
		}
	}
	if err = lockSubject(ctx, tx, subject); err != nil {
		return err
	}
	var currentAccount uuid.UUID
	err = tx.QueryRow(ctx, `SELECT account_id FROM automod_subjects WHERE subject_id=$1`, subject).Scan(&currentAccount)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if currentAccount != uuid.Nil && currentAccount != account {
		return ErrConflict
	}
	if check != nil {
		if err = check(ctx); err != nil {
			return err
		}
	}
	if err = fn(tx, currentAccount); err != nil {
		return lifecycleError(err)
	}
	if check != nil {
		if err = check(ctx); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) purgeSubjectTx(ctx context.Context, tx pgx.Tx, subject, account uuid.UUID) error {
	if _, err := tx.Exec(ctx, `INSERT INTO automod_terminal_subjects(subject_id) VALUES($1) ON CONFLICT(subject_id) DO UPDATE SET subject_id=EXCLUDED.subject_id`, subject); err != nil {
		return err
	}
	if account != uuid.Nil && s.BeforeTerminalPurge != nil {
		if err := s.BeforeTerminalPurge(ctx, tx, account); err != nil {
			return err
		}
	}
	if s.BeforeTerminalSubjectPurge != nil {
		if err := s.BeforeTerminalSubjectPurge(ctx, tx, subject); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `DELETE FROM automod_subjects WHERE subject_id=$1`, subject)
	return err
}
