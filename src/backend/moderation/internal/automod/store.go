package automod

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrConflict = errors.New("immutable moderation evidence conflict")
	ErrPurged   = errors.New("moderation subject permanently purged")
	ErrSubject  = errors.New("moderation subject is not provisioned")
)

type Subject struct {
	ID, AccountID, Epoch, CycleID uuid.UUID
	Revision                      int64
	State                         string
	PurgeAt                       *time.Time
}
type Digest struct {
	KeyID string
	Hash  [32]byte
}
type Envelope struct {
	Subject      Subject
	EventID      uuid.UUID
	Sequence     int64
	RecordedAt   time.Time
	Eligibility  Eligibility
	Digests      []Digest
	LinkCount    uint32
	Hash         [32]byte
	BeforeCommit BootCheck `json:"-"`
}
type Receipt struct {
	Through            int64
	Duplicate, Expired bool
}
type Store struct {
	Pool *pgxpool.Pool
	// All terminal participants run in the account-serialized transaction before
	// the raw account mapping disappears. A failure cannot commit partial erasure.
	BeforeTerminalPurge func(context.Context, pgx.Tx, uuid.UUID) error
	// Subject-only participants also run when a restored account mapping is
	// absent. They may not infer an account identity from this opaque UUID.
	BeforeTerminalSubjectPurge func(context.Context, pgx.Tx, uuid.UUID) error
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{Pool: pool} }

// BindLiveSubjectTx records a freshly verified Auth LIVE binding under the
// caller's account lock. It never invents a Messaging source epoch or cursor.
func (s *Store) BindLiveSubjectTx(ctx context.Context, tx pgx.Tx, account, subject uuid.UUID, revision int64) error {
	return s.BindSubjectTx(ctx, tx, Subject{ID: subject, AccountID: account, Revision: revision, State: "LIVE"})
}

// BindSubjectTx accepts only fresh Auth-owned lifecycle authority. It does not
// create or modify a Messaging source epoch, cursor or detector count.
func (s *Store) BindSubjectTx(ctx context.Context, tx pgx.Tx, binding Subject) error {
	if tx == nil || !validLifecycle(binding) {
		return ErrSubject
	}
	if err := lockSubject(ctx, tx, binding.ID); err != nil {
		return err
	}
	var terminal bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, binding.ID).Scan(&terminal); err != nil {
		return err
	}
	if terminal {
		return ErrPurged
	}
	tag, err := tx.Exec(ctx, `INSERT INTO automod_subjects(subject_id,account_id,lifecycle_revision,lifecycle_state,purge_at,deletion_cycle_id) VALUES($1,$2,$3,$4,$5,$6)
	ON CONFLICT(subject_id) DO UPDATE SET lifecycle_revision=EXCLUDED.lifecycle_revision,lifecycle_state=EXCLUDED.lifecycle_state,purge_at=EXCLUDED.purge_at,deletion_cycle_id=EXCLUDED.deletion_cycle_id
 WHERE automod_subjects.account_id=EXCLUDED.account_id
 AND (automod_subjects.lifecycle_state <> 'DELETED' OR EXCLUDED.lifecycle_state <> 'DELETED' OR automod_subjects.deletion_cycle_id IS DISTINCT FROM EXCLUDED.deletion_cycle_id OR automod_subjects.purge_at IS NOT DISTINCT FROM EXCLUDED.purge_at)
 AND (automod_subjects.lifecycle_revision < EXCLUDED.lifecycle_revision OR
	(automod_subjects.lifecycle_revision=EXCLUDED.lifecycle_revision AND automod_subjects.lifecycle_state=EXCLUDED.lifecycle_state AND automod_subjects.purge_at IS NOT DISTINCT FROM EXCLUDED.purge_at AND automod_subjects.deletion_cycle_id IS NOT DISTINCT FROM EXCLUDED.deletion_cycle_id))`, binding.ID, binding.AccountID, binding.Revision, binding.State, binding.PurgeAt, nullableCycle(binding.CycleID))
	if err != nil {
		return lifecycleError(err)
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (s *Store) BindSubject(ctx context.Context, binding Subject) error {
	return s.accountTx(ctx, binding.AccountID, func(tx pgx.Tx) error { return s.BindSubjectTx(ctx, tx, binding) })
}

// RequireLiveSubjectTx consumes a fresh protected Auth binding. The caller must
// hold WithAccountLock for account; this does not provision an unknown subject.
func (s *Store) RequireLiveSubjectTx(ctx context.Context, tx pgx.Tx, account, subject uuid.UUID, revision int64) error {
	if tx == nil || account == uuid.Nil || subject == uuid.Nil || revision <= 0 {
		return ErrSubject
	}
	var live bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_subjects a WHERE a.subject_id=$1 AND a.account_id=$2 AND a.lifecycle_revision=$3 AND a.lifecycle_state='LIVE' AND a.purge_at IS NULL AND NOT EXISTS(SELECT 1 FROM automod_terminal_subjects t WHERE t.subject_id=a.subject_id))`, subject, account, revision).Scan(&live)
	if err != nil {
		return err
	}
	if !live {
		return ErrSubject
	}
	return nil
}

func (s *Store) accountTx(ctx context.Context, account uuid.UUID, fn func(pgx.Tx) error) error {
	return s.accountTxChecked(ctx, account, nil, fn)
}

func (s *Store) accountTxChecked(ctx context.Context, account uuid.UUID, check BootCheck, fn func(pgx.Tx) error) error {
	if s == nil || s.Pool == nil || account == uuid.Nil {
		return ErrSubject
	}
	if check != nil {
		if err := check(ctx); err != nil {
			return err
		}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, account.String()); err != nil {
		return err
	}
	if check != nil {
		if err = check(ctx); err != nil {
			return err
		}
	}
	if err = fn(tx); err != nil {
		return lifecycleError(err)
	}
	if check != nil {
		if err = check(ctx); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func validSubject(s Subject) bool {
	return s.Epoch != uuid.Nil && validLifecycle(s)
}
func validLifecycle(s Subject) bool {
	return s.ID != uuid.Nil && s.AccountID != uuid.Nil && s.Revision > 0 &&
		((s.State == "LIVE" && s.PurgeAt == nil && s.CycleID == uuid.Nil) ||
			(s.State == "DELETED" && s.PurgeAt != nil && s.CycleID != uuid.Nil && !s.PurgeAt.IsZero() && s.PurgeAt.Nanosecond()%1000 == 0))
}

func nullableCycle(cycle uuid.UUID) any {
	if cycle == uuid.Nil {
		return nil
	}
	return cycle
}

// Provision is called only with freshly verified Auth authority, never an inbox
// event's claimed identity. An existing source epoch or cursor cannot be reset.
func (s *Store) Provision(ctx context.Context, subject Subject) error {
	return s.provision(ctx, subject, nil)
}

func (s *Store) provision(ctx context.Context, subject Subject, check BootCheck) error {
	if !validSubject(subject) {
		return ErrEvidence
	}
	return s.accountTxChecked(ctx, subject.AccountID, check, func(tx pgx.Tx) error {
		if err := lockSubject(ctx, tx, subject.ID); err != nil {
			return err
		}
		var purged bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, subject.ID).Scan(&purged); err != nil {
			return err
		}
		if purged {
			return ErrPurged
		}
		var existing Subject
		err := tx.QueryRow(ctx, `SELECT subject_id,account_id,source_epoch,lifecycle_revision,lifecycle_state,purge_at,deletion_cycle_id FROM automod_subjects WHERE subject_id=$1 OR account_id=$2 FOR UPDATE`, subject.ID, subject.AccountID).Scan(&existing.ID, &existing.AccountID, &existing.Epoch, &existing.Revision, &existing.State, &existing.PurgeAt, &existing.CycleID)
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `INSERT INTO automod_subjects(subject_id,account_id,source_epoch,lifecycle_revision,lifecycle_state,purge_at,deletion_cycle_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, subject.ID, subject.AccountID, subject.Epoch, subject.Revision, subject.State, subject.PurgeAt, nullableCycle(subject.CycleID))
			return err
		}
		if err != nil {
			return err
		}
		if existing.ID != subject.ID || existing.AccountID != subject.AccountID || existing.Epoch != uuid.Nil && existing.Epoch != subject.Epoch || existing.Revision > subject.Revision {
			return ErrConflict
		}
		if existing.State == "DELETED" && subject.State == "DELETED" && existing.CycleID == subject.CycleID && !equalTime(existing.PurgeAt, subject.PurgeAt) {
			return ErrConflict
		}
		if existing.Revision == subject.Revision && (existing.State != subject.State || existing.CycleID != subject.CycleID || !equalTime(existing.PurgeAt, subject.PurgeAt)) {
			return ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE automod_subjects SET lifecycle_revision=$2,lifecycle_state=$3,purge_at=$4,source_epoch=COALESCE(source_epoch,$5),deletion_cycle_id=$6 WHERE subject_id=$1`, subject.ID, subject.Revision, subject.State, subject.PurgeAt, subject.Epoch, nullableCycle(subject.CycleID))
		return err
	})
}
func equalTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func validEnvelope(e Envelope) bool {
	if !validSubject(e.Subject) || e.Sequence <= 0 || e.EventID == uuid.Nil || e.RecordedAt.IsZero() || e.Hash == [32]byte{} {
		return false
	}
	if e.Eligibility == Eligible {
		if len(e.Digests) == 0 || len(e.Digests) > 4 {
			return false
		}
		keys := map[string]bool{}
		for _, d := range e.Digests {
			if d.KeyID == "" || len(d.KeyID) > 64 || keys[d.KeyID] || d.Hash == [32]byte{} {
				return false
			}
			keys[d.KeyID] = true
		}
		return true
	}
	return (e.Eligibility == NoText || e.Eligibility == E2E || e.Eligibility == ExpiredEvidence) && len(e.Digests) == 0 && e.LinkCount == 0
}

// Ingest records out-of-order events durably, but advances only the contiguous
// provisioned account stream. Every effect is committed under the same account
// advisory lock used by manual sanctions, expiry and appeal processing.
func (s *Store) Ingest(ctx context.Context, e Envelope) (receipt Receipt, err error) {
	if !validEnvelope(e) {
		return receipt, ErrEvidence
	}
	err = s.accountTxChecked(ctx, e.Subject.AccountID, e.BeforeCommit, func(tx pgx.Tx) error {
		var terminal bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, e.Subject.ID).Scan(&terminal); err != nil {
			return err
		}
		if terminal {
			return ErrPurged
		}
		var account Account
		var epoch uuid.UUID
		var revision int64
		var sourceAt, mute *time.Time
		var purgeAt *time.Time
		var now time.Time
		err := tx.QueryRow(ctx, `SELECT through_sequence,last_source_at,muted_until,post_mute_count,source_epoch,lifecycle_revision,purge_at,clock_timestamp() FROM automod_subjects WHERE subject_id=$1 AND account_id=$2 FOR UPDATE`, e.Subject.ID, e.Subject.AccountID).Scan(&account.LastSequence, &sourceAt, &mute, &account.PostMuteCount, &epoch, &revision, &purgeAt, &now)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSubject
		}
		if err != nil {
			return err
		}
		if epoch != e.Subject.Epoch || e.Subject.Revision > revision {
			return ErrConflict
		}
		if purgeAt != nil && !now.Before(*purgeAt) {
			return ErrPurged
		}
		if e.RecordedAt.After(now.Add(5*time.Second)) || e.Eligibility == ExpiredEvidence && now.Before(e.RecordedAt.Add(EvidenceLifetime)) {
			return ErrEvidence
		}
		if sourceAt != nil {
			account.LastSourceAt = *sourceAt
		}
		if mute != nil {
			account.MuteUntil = *mute
		}
		receipt.Through = int64(account.LastSequence)
		var priorHash []byte
		var priorEvent uuid.UUID
		var priorAt time.Time
		err = tx.QueryRow(ctx, `SELECT original_hash,event_id,recorded_at FROM automod_inbox WHERE subject_id=$1 AND sequence=$2`, e.Subject.ID, e.Sequence).Scan(&priorHash, &priorEvent, &priorAt)
		if err == nil {
			if !bytes.Equal(priorHash, e.Hash[:]) || priorEvent != e.EventID || !priorAt.Equal(e.RecordedAt) {
				return ErrConflict
			}
			if e.Sequence <= receipt.Through {
				receipt.Duplicate = true
				return nil
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		} else {
			if e.Sequence <= receipt.Through {
				receipt.Duplicate = true
				return nil
			}
			payload, err := json.Marshal(e)
			if err != nil {
				return err
			}
			if e.Eligibility == ExpiredEvidence || !now.Before(e.RecordedAt.Add(EvidenceLifetime)) {
				payload = nil
				receipt.Expired = true
			}
			if _, err = tx.Exec(ctx, `INSERT INTO automod_inbox(subject_id,sequence,event_id,recorded_at,original_hash,payload) VALUES($1,$2,$3,$4,$5,$6)`, e.Subject.ID, e.Sequence, e.EventID, e.RecordedAt, e.Hash[:], payload); err != nil {
				return err
			}
		}
		for account.LastSequence < math.MaxInt64 {
			var payload []byte
			var seq int64
			var at time.Time
			err = tx.QueryRow(ctx, `SELECT sequence,recorded_at,payload FROM automod_inbox WHERE subject_id=$1 AND sequence=$2 AND applied_at IS NULL`, e.Subject.ID, int64(account.LastSequence)+1).Scan(&seq, &at, &payload)
			if errors.Is(err, pgx.ErrNoRows) {
				break
			}
			if err != nil {
				return err
			}
			current := Envelope{Subject: e.Subject, Sequence: seq, RecordedAt: at, Eligibility: ExpiredEvidence}
			if len(payload) > 0 && now.Before(at.Add(EvidenceLifetime)) {
				if err = json.Unmarshal(payload, &current); err != nil {
					return err
				}
			}
			var pattern Pattern
			var patternID uuid.UUID
			if current.Eligibility == Eligible {
				patternID, pattern, err = loadPattern(ctx, tx, e.Subject.ID, current, now)
				if err != nil {
					return err
				}
			}
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sanctions WHERE target_account_id=$1 AND type='perm_ban' AND issuer_service='service:moderation/automod' AND revoked_at IS NULL)`, e.Subject.AccountID).Scan(&account.PermanentActive); err != nil {
				return err
			}
			decision, err := Observe(account, pattern, Event{Sequence: uint64(seq), RecordedAt: at, Eligibility: current.Eligibility, LinkCount: current.LinkCount}, now)
			if err != nil {
				return err
			}
			account = decision.Account
			if patternID != uuid.Nil {
				state, err := json.Marshal(decision.Pattern)
				if err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE automod_patterns SET state=$3,last_seen_at=$4 WHERE subject_id=$1 AND pattern_id=$2`, e.Subject.ID, patternID, state, at); err != nil {
					return err
				}
			}
			for _, action := range decision.Actions {
				if err = writeAction(ctx, tx, e.Subject, action); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(ctx, `UPDATE automod_inbox SET payload=NULL,applied_at=$3 WHERE subject_id=$1 AND sequence=$2`, e.Subject.ID, seq, now); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE automod_subjects SET through_sequence=$2,last_source_at=$3,muted_until=$4,post_mute_count=$5 WHERE subject_id=$1`, e.Subject.ID, account.LastSequence, nullableTime(account.LastSourceAt), nullableTime(account.MuteUntil), account.PostMuteCount)
		receipt.Through = int64(account.LastSequence)
		return err
	})
	return receipt, err
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func lockSubject(ctx context.Context, tx pgx.Tx, subject uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-subject-lifecycle:' || $1::text,0))`, subject.String())
	return err
}
func loadPattern(ctx context.Context, tx pgx.Tx, subject uuid.UUID, e Envelope, now time.Time) (uuid.UUID, Pattern, error) {
	var id uuid.UUID
	var pattern Pattern
	for _, d := range e.Digests {
		var found uuid.UUID
		err := tx.QueryRow(ctx, `SELECT pattern_id FROM automod_pattern_aliases WHERE subject_id=$1 AND key_id=$2 AND digest=$3`, subject, d.KeyID, d.Hash[:]).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return id, pattern, err
		}
		if id != uuid.Nil && id != found {
			return id, pattern, ErrConflict
		}
		id = found
	}
	if id == uuid.Nil {
		id = uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO automod_patterns(subject_id,pattern_id,state,last_seen_at) VALUES($1,$2,'{}',$3)`, subject, id, now); err != nil {
			return id, pattern, err
		}
	} else {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT state FROM automod_patterns WHERE subject_id=$1 AND pattern_id=$2`, subject, id).Scan(&raw); err != nil {
			return id, pattern, err
		}
		if err := json.Unmarshal(raw, &pattern); err != nil {
			return id, pattern, err
		}
	}
	for _, d := range e.Digests {
		if _, err := tx.Exec(ctx, `INSERT INTO automod_pattern_aliases(subject_id,key_id,digest,pattern_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, subject, d.KeyID, d.Hash[:], id); err != nil {
			return id, pattern, err
		}
	}
	return id, pattern, nil
}

func writeAction(ctx context.Context, tx pgx.Tx, subject Subject, a Action) error {
	var sanction any
	if a.Kind == PermanentBan {
		id := uuid.New()
		sanction = id
		if _, err := tx.Exec(ctx, `INSERT INTO sanctions(id,target_account_id,type,reason,issued_by,issuer_service) VALUES($1,$2,'perm_ban','automod: repeated committed spam',NULL,'service:moderation/automod')`, id, subject.AccountID); err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]any{"event_type": "moderation.sanction_applied", "sanction_id": id, "target_account_id": subject.AccountID, "sanction_type": "perm_ban", "issuer_service": "service:moderation/automod"})
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO automod_event_outbox(event_id,subject_id,payload) VALUES($1,$2,$3)`, uuid.New(), subject.ID, payload); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE automod_subjects SET restriction_revision=restriction_revision+1,desired_banned=true WHERE subject_id=$1`, subject.ID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO automod_restriction_sync(subject_id) VALUES($1) ON CONFLICT(subject_id) DO UPDATE SET requested_at=clock_timestamp()`, subject.ID); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO automod_actions(subject_id,sequence,kind,source_at,applied_at,expires_at,sanction_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, subject.ID, a.Sequence, a.Kind, a.SourceAt, a.AppliedAt, nullableTime(a.ExpiresAt), sanction)
	return err
}

// Purge consumes positive permanent Auth terminal authority. A captured deletion
// deadline alone is never sufficient; an unseen restore may have superseded it.
func (s *Store) Purge(ctx context.Context, subject uuid.UUID) error {
	return s.withLockedSubjectChecked(ctx, subject, nil, func(tx pgx.Tx, account uuid.UUID) error {
		return s.purgeSubjectTx(ctx, tx, subject, account)
	})
}
