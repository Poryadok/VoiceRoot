package profileprojection

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
)

// StoreAdapter atomically persists the inbox/revision fence and selected Search projection.
type StoreAdapter struct {
	Pool            *pgxpool.Pool
	Generation      uint64
	BeforeFenceLock func(*userv1.SearchProfileProjectionEvent)
	AfterFenceLock  func(*userv1.SearchProfileProjectionEvent)
}

func (s *StoreAdapter) generation() uint64 {
	if s != nil && s.Generation != 0 {
		return s.Generation
	}
	return 1
}

type SnapshotState struct {
	Phase         string
	HighWatermark uint64
	Cursor        string
}

func (s *StoreAdapter) SnapshotState(ctx context.Context) (SnapshotState, error) {
	var state SnapshotState
	err := s.Pool.QueryRow(ctx, `SELECT snapshot_phase,snapshot_high_watermark,snapshot_cursor
		FROM search_user_profile_generation_checkpoint WHERE generation=$1`, s.generation()).Scan(&state.Phase, &state.HighWatermark, &state.Cursor)
	return state, err
}

func (s *StoreAdapter) StartSnapshot(ctx context.Context, high uint64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE search_user_profile_generation_checkpoint
		SET snapshot_phase='snapshot',snapshot_high_watermark=$1,snapshot_cursor='',updated_at=now()
		WHERE generation=$2`, high, s.generation())
	return err
}

func (s *StoreAdapter) AdvanceSnapshotCursor(ctx context.Context, cursor string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE search_user_profile_generation_checkpoint
		SET snapshot_cursor=$1,updated_at=now() WHERE generation=$2 AND snapshot_phase='snapshot'`, cursor, s.generation())
	return err
}

func (s *StoreAdapter) FinishSnapshot(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `UPDATE search_user_profile_generation_checkpoint
		SET snapshot_phase='replay',snapshot_cursor='',updated_at=now() WHERE generation=$1 AND snapshot_phase='snapshot'`, s.generation())
	return err
}

// ResetSnapshot abandons an expired or rejected opaque cursor. Previously
// applied immutable events remain protected by the inbox revision fence; the
// next bootstrap obtains a fresh H-bound snapshot session.
func (s *StoreAdapter) ResetSnapshot(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `UPDATE search_user_profile_generation_checkpoint
		SET snapshot_phase='idle',snapshot_high_watermark=0,snapshot_cursor='',updated_at=now()
		WHERE generation=$1`, s.generation())
	return err
}

func (s *StoreAdapter) Checkpoint(ctx context.Context) (uint64, error) {
	var offset uint64
	err := s.Pool.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_generation_checkpoint WHERE generation=$1`, s.generation()).Scan(&offset)
	return offset, err
}

func (s *StoreAdapter) StoreCheckpoint(ctx context.Context, offset uint64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE search_user_profile_generation_checkpoint SET journal_offset=GREATEST(journal_offset,$1),updated_at=now() WHERE generation=$2`, offset, s.generation())
	return err
}

func (s *StoreAdapter) Apply(ctx context.Context, envelope *userv1.SearchProfileProjectionEvent) (ApplyResult, error) {
	return s.apply(ctx, envelope, 0)
}

// ApplyAndCheckpoint commits a journal event and the durable replay offset in
// one database transaction. A restart therefore either sees neither change or
// replays an already-applied event through the inbox/revision fence.
func (s *StoreAdapter) ApplyAndCheckpoint(ctx context.Context, envelope *userv1.SearchProfileProjectionEvent, checkpoint uint64) (ApplyResult, error) {
	if checkpoint == 0 {
		return Quarantined, fmt.Errorf("journal checkpoint is required")
	}
	return s.apply(ctx, envelope, checkpoint)
}

func (s *StoreAdapter) apply(ctx context.Context, envelope *userv1.SearchProfileProjectionEvent, checkpoint uint64) (ApplyResult, error) {
	if s == nil || s.Pool == nil {
		return Quarantined, fmt.Errorf("profile projection store unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Quarantined, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := s.applyInTransaction(ctx, tx, envelope, checkpoint)
	if err != nil {
		if result == Quarantined {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return Quarantined, commitErr
			}
		}
		return result, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Quarantined, err
	}
	return result, nil
}

// applyInTransaction is used by mirrored delivery so every live generation
// sees one authority record or none. The caller owns commit/rollback.
func (s *StoreAdapter) applyInTransaction(ctx context.Context, tx pgx.Tx, envelope *userv1.SearchProfileProjectionEvent, checkpoint uint64) (ApplyResult, error) {
	if s == nil || s.Pool == nil {
		return Quarantined, fmt.Errorf("profile projection store unavailable")
	}
	payload, err := proto.Marshal(envelope)
	if err != nil {
		return Quarantined, err
	}
	digest := sha256.Sum256(payload)
	eventID, err := uuid.Parse(envelope.GetEventId())
	if err != nil {
		return Quarantined, err
	}
	profileID, err := uuid.Parse(envelope.GetProfileId())
	if err != nil {
		return Quarantined, err
	}
	// Event identity is recognized before readiness evidence. A redelivery after
	// a crash is therefore a true no-op; reusing an ID with different authority
	// bytes is durable quarantine rather than a second digest frame.
	var storedOffset uint64
	var storedHash []byte
	err = tx.QueryRow(ctx, `SELECT source_revision,payload_sha256 FROM search_user_profile_generation_inbox WHERE generation=$1 AND event_id=$2`, s.generation(), eventID).Scan(&storedOffset, &storedHash)
	if err == nil {
		if storedOffset == envelope.GetSourceRevision() && string(storedHash) == string(digest[:]) {
			return NoopDuplicate, nil
		}
		_, quarantineErr := tx.Exec(ctx, `INSERT INTO search_user_profile_generation_inbox(generation,event_id,profile_id,source_revision,payload_sha256,quarantined_at,quarantine_reason) VALUES($1,$2,$3,$4,$5,now(),'event id payload mismatch') ON CONFLICT(generation,event_id) DO UPDATE SET quarantined_at=now(),quarantine_reason='event id payload mismatch'`, s.generation(), eventID, profileID, envelope.GetSourceRevision(), digest[:])
		if quarantineErr != nil {
			return Quarantined, quarantineErr
		}
		return Quarantined, fmt.Errorf("event id payload mismatch")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Quarantined, err
	}
	event, err := eventFromProto(envelope)
	if err != nil {
		_, quarantineErr := tx.Exec(ctx, `INSERT INTO search_user_profile_generation_inbox(generation,event_id,profile_id,source_revision,payload_sha256,quarantined_at,quarantine_reason) VALUES($1,$2,$3,$4,$5,now(),'invalid profile projection event')`, s.generation(), eventID, profileID, envelope.GetSourceRevision(), digest[:])
		if quarantineErr != nil {
			return Quarantined, quarantineErr
		}
		return Quarantined, err
	}
	var highWatermark, evidenceCount, evidenceFirst, evidenceLast uint64
	var evidenceDigest []byte
	if err = tx.QueryRow(ctx, `SELECT snapshot_high_watermark,evidence_count,evidence_first_offset,evidence_last_offset,evidence_digest FROM search_user_profile_generation_checkpoint WHERE generation=$1 FOR UPDATE`, s.generation()).Scan(&highWatermark, &evidenceCount, &evidenceFirst, &evidenceLast, &evidenceDigest); err != nil {
		return Quarantined, err
	}
	evidence, err := NewReadinessEvidence(s.generation(), highWatermark)
	if err != nil {
		return Quarantined, err
	}
	if evidenceCount != 0 {
		if len(evidenceDigest) != 32 {
			return Quarantined, fmt.Errorf("invalid readiness evidence")
		}
		evidence.Count, evidence.First, evidence.Last = evidenceCount, evidenceFirst, evidenceLast
		copy(evidence.Digest[:], evidenceDigest)
	}
	deterministicPayload, err := (proto.MarshalOptions{Deterministic: true}).Marshal(envelope)
	if err != nil {
		return Quarantined, err
	}
	if err := evidence.Add(envelope.GetJournalOffset(), deterministicPayload); err != nil {
		return Quarantined, err
	}
	state := State{ProfileID: event.ProfileID}
	if s.BeforeFenceLock != nil {
		s.BeforeFenceLock(envelope)
	}
	generation := s.generation()
	if _, err = tx.Exec(ctx, `INSERT INTO search_user_profile_generation_fence(generation,profile_id,source_revision,payload_sha256) VALUES($1,$2,0,decode(repeat('00',32),'hex')) ON CONFLICT DO NOTHING`, generation, profileID); err != nil {
		return Quarantined, err
	}
	var revision int64
	var hash []byte
	var tombstoned bool
	if err = tx.QueryRow(ctx, `SELECT source_revision,payload_sha256,tombstoned_at IS NOT NULL FROM search_user_profile_generation_fence WHERE generation=$1 AND profile_id=$2 FOR UPDATE`, generation, profileID).Scan(&revision, &hash, &tombstoned); err != nil {
		return Quarantined, err
	}
	state.SourceRevision = uint64(revision)
	state.PayloadHash = fmt.Sprintf("%x", hash)
	state.Tombstoned = tombstoned
	if s.AfterFenceLock != nil {
		s.AfterFenceLock(envelope)
	}
	result, applyErr := Apply(&state, event, fmt.Sprintf("%x", digest[:]))
	if applyErr != nil {
		_, quarantineErr := tx.Exec(ctx, `INSERT INTO search_user_profile_generation_inbox(generation,event_id,profile_id,source_revision,payload_sha256,quarantined_at,quarantine_reason)
			VALUES($1,$2,$3,$4,$5,now(),$6)
			ON CONFLICT(generation,event_id) DO UPDATE SET quarantined_at=EXCLUDED.quarantined_at, quarantine_reason=EXCLUDED.quarantine_reason`, generation, eventID, profileID, event.SourceRevision, digest[:], applyErr.Error())
		if quarantineErr != nil {
			return Quarantined, quarantineErr
		}
		return result, applyErr
	}
	_, err = tx.Exec(ctx, `INSERT INTO search_user_profile_generation_inbox(generation,event_id,profile_id,source_revision,payload_sha256) VALUES($1,$2,$3,$4,$5) ON CONFLICT(generation,event_id) DO NOTHING`, generation, eventID, profileID, event.SourceRevision, digest[:])
	if err != nil {
		return Quarantined, err
	}
	if result == Applied {
		if event.Kind == Delete {
			_, err = tx.Exec(ctx, `UPDATE search_user_profile_generation_documents SET source_revision=$3,tombstoned_at=now(),updated_at=now() WHERE generation=$1 AND profile_id=$2`, generation, profileID, event.SourceRevision)
		} else {
			u := envelope.GetUpsert()
			accountID, parseErr := uuid.Parse(u.GetAccountId())
			if parseErr != nil {
				return Quarantined, parseErr
			}
			_, err = tx.Exec(ctx, `INSERT INTO search_user_profile_generation_documents(generation,profile_id,account_id,username,discriminator,display_name,username_lower,verification_type,username_search_key,display_name_search_key,normalization_version,source_revision,tombstoned_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,lower($4),$7,$8,$9,$10,$11,NULL,now()) ON CONFLICT(generation,profile_id) DO UPDATE SET account_id=EXCLUDED.account_id,username=EXCLUDED.username,discriminator=EXCLUDED.discriminator,display_name=EXCLUDED.display_name,username_lower=EXCLUDED.username_lower,verification_type=EXCLUDED.verification_type,username_search_key=EXCLUDED.username_search_key,display_name_search_key=EXCLUDED.display_name_search_key,normalization_version=EXCLUDED.normalization_version,source_revision=EXCLUDED.source_revision,tombstoned_at=NULL,updated_at=now()`, generation, profileID, accountID, event.Username, u.GetDiscriminator(), event.DisplayName, u.GetVerificationType(), event.UsernameSearchKey, event.DisplayNameSearchKey, event.NormalizationVersion, event.SourceRevision)
		}
		if err != nil {
			return Quarantined, err
		}
		if _, err = tx.Exec(ctx, `UPDATE search_user_profile_generation_fence SET source_revision=$3,payload_sha256=$4,tombstoned_at=CASE WHEN $5 THEN now() ELSE NULL END,updated_at=now() WHERE generation=$1 AND profile_id=$2`, generation, profileID, event.SourceRevision, digest[:], event.Kind == Delete); err != nil {
			return Quarantined, err
		}
	}
	if checkpoint != 0 {
		if _, err = tx.Exec(ctx, `UPDATE search_user_profile_generation_checkpoint SET journal_offset=GREATEST(journal_offset,$1),evidence_count=$3,evidence_first_offset=$4,evidence_last_offset=$5,evidence_digest=$6,updated_at=now() WHERE generation=$2`, checkpoint, generation, evidence.Count, evidence.First, evidence.Last, evidence.Digest[:]); err != nil {
			return Quarantined, err
		}
	}
	return result, nil
}
func eventFromProto(event *userv1.SearchProfileProjectionEvent) (Event, error) {
	if event == nil {
		return Event{}, fmt.Errorf("missing profile projection event")
	}
	out := Event{ProtocolVersion: int(event.GetProtocolVersion()), EventID: event.GetEventId(), ProfileID: event.GetProfileId(), SourceRevision: event.GetSourceRevision()}
	if u := event.GetUpsert(); u != nil {
		out.Kind = Upsert
		out.Username = u.GetUsername()
		out.DisplayName = u.GetDisplayName()
		out.UsernameSearchKey = u.GetUsernameSearchKey()
		out.DisplayNameSearchKey = u.GetDisplayNameSearchKey()
		out.NormalizationVersion = int(u.GetNormalizationVersion())
	} else if event.GetDelete() != nil {
		out.Kind = Delete
	}
	return out, ValidateEvent(out)
}
