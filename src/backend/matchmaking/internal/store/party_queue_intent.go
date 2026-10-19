package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrPartyQueueLeaseLost = errors.New("party queue projection lease changed")
var ErrPartyQueueProjectionChanged = errors.New("party queue projection source changed")

type PartyQueueLease struct {
	PartyID, LeaseToken uuid.UUID
	Revision            int64
	Action              string
	projectionHash      [32]byte
	nativeStampHash     [32]byte
	terminalProjection  bool
}

type PartyQueueProjection struct {
	Lease    PartyQueueLease
	Enqueue  bool
	Sessions []SearchSession
}

func (s *PartyStore) ClaimQueueIntent(ctx context.Context, lease time.Duration) (*PartyQueueLease, error) {
	if s == nil || s.Pool == nil || lease <= 0 || lease > time.Minute {
		return nil, ErrSourcePartyUnavailable
	}
	out := &PartyQueueLease{LeaseToken: uuid.New()}
	err := s.Pool.QueryRow(ctx, `WITH candidate AS (
 SELECT party_id FROM matchmaking_party_search_queue_intents
 WHERE next_attempt_at<=clock_timestamp() AND (lease_until IS NULL OR lease_until<=clock_timestamp())
 ORDER BY next_attempt_at,party_id FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE matchmaking_party_search_queue_intents i SET lease_token=$1,
 lease_until=clock_timestamp()+make_interval(secs=>$2),
 attempts=CASE WHEN attempts<9223372036854775807 THEN attempts+1 ELSE attempts END
FROM candidate c WHERE i.party_id=c.party_id RETURNING i.party_id,i.revision,i.action`, out.LeaseToken, lease.Seconds()).Scan(&out.PartyID, &out.Revision, &out.Action)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Join(ErrSourcePartyUnavailable, err)
	}
	return out, nil
}
func (s *PartyStore) ReadQueueIntent(ctx context.Context, lease PartyQueueLease) (PartyQueueProjection, error) {
	if s == nil || s.Pool == nil {
		return PartyQueueProjection{}, ErrSourcePartyUnavailable
	}
	if !validPartyQueueLease(lease) {
		return PartyQueueProjection{}, ErrPartyQueueLeaseLost
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PartyQueueProjection{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	defer rollbackPartySearch(tx)
	var current bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM matchmaking_party_search_queue_intents
WHERE party_id=$1 AND revision=$2 AND lease_token=$3 AND action=$4 AND lease_until>clock_timestamp())`, lease.PartyID, lease.Revision, lease.LeaseToken, lease.Action).Scan(&current)
	if err != nil {
		return PartyQueueProjection{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	if !current {
		return PartyQueueProjection{}, ErrPartyQueueLeaseLost
	}
	var disbanded *time.Time
	if err := tx.QueryRow(ctx, `SELECT disbanded_at FROM parties WHERE id=$1`, lease.PartyID).Scan(&disbanded); err != nil {
		return PartyQueueProjection{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	rows, err := tx.Query(ctx, `SELECT `+sessionSelectCols+` FROM search_sessions WHERE party_id=$1 ORDER BY profile_id,id`, lease.PartyID)
	if err != nil {
		return PartyQueueProjection{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	out := PartyQueueProjection{Lease: lease, Enqueue: lease.Action == "ENQUEUE" && disbanded == nil, Sessions: []SearchSession{}}
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			rows.Close()
			return PartyQueueProjection{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		out.Sessions = append(out.Sessions, session)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return PartyQueueProjection{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	if len(out.Sessions) == 0 {
		return PartyQueueProjection{}, ErrSourcePartyUnavailable
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return PartyQueueProjection{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	for _, session := range out.Sessions {
		if session.Status != SessionStatusSearching || session.MatchID != nil || session.TimeoutAt == nil || !session.TimeoutAt.After(now) {
			out.Enqueue = false
		}
	}
	if out.Enqueue {
		// Only the installed native capture source is qualified here. A future
		// MM_LFP_CONSENT_V1 adapter must validate its distinct owner receipt; NULL
		// and arbitrary legacy parties cannot become a native authority fallback.
		party, err := scanSourceParty(tx.QueryRow(ctx, `SELECT `+sourcePartyCols+` FROM parties WHERE id=$1 AND search_operation_id IS NOT NULL`, lease.PartyID))
		if err != nil || len(party.MemberProfileIDs) != len(out.Sessions) {
			return PartyQueueProjection{}, errors.Join(ErrSourcePartyUnavailable, err)
		}
		for i, session := range out.Sessions {
			if session.ProfileID != party.MemberProfileIDs[i] || session.GameID != party.GameID || session.Mode != party.Mode || session.Criteria != party.Criteria || !samePartyUUID(session.PartyID, &party.ID) {
				return PartyQueueProjection{}, ErrSourcePartyUnavailable
			}
		}
	}
	// RELEASE must survive erased/unknown private identity evidence. It uses
	// retained session projection identities, never a fresh ownership lookup.
	out.Lease.projectionHash, err = partyQueueProjectionHash(ctx, tx, lease.PartyID)
	if err != nil {
		return PartyQueueProjection{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PartyQueueProjection{}, errors.Join(ErrSourcePartyUnavailable, err)
	}
	out.Lease.terminalProjection = !out.Enqueue
	return out, nil
}
func (s *PartyStore) FinishQueueIntent(ctx context.Context, lease PartyQueueLease, delay time.Duration, success bool) error {
	if s == nil || s.Pool == nil || delay < 0 || delay > time.Minute {
		return ErrSourcePartyUnavailable
	}
	if !validPartyQueueLease(lease) {
		return ErrPartyQueueLeaseLost
	}
	if success && s.NativeAdmission != nil && !lease.terminalProjection && lease.nativeStampHash == [32]byte{} {
		return ErrSourcePartyUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return errors.Join(ErrSourcePartyUnavailable, err)
	}
	defer rollbackPartySearch(tx)
	changed := false
	if success {
		// Projection acknowledgment is not forward admission. Lock only the
		// local party -> sessions -> intent order, with no Redis IO in this tx.
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM parties WHERE id=$1 FOR UPDATE`, lease.PartyID).Scan(&id); err != nil {
			return errors.Join(ErrSourcePartyUnavailable, err)
		}
		rows, err := tx.Query(ctx, `SELECT id FROM search_sessions WHERE party_id=$1 ORDER BY id FOR UPDATE`, lease.PartyID)
		if err != nil {
			return errors.Join(ErrSourcePartyUnavailable, err)
		}
		for rows.Next() {
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return errors.Join(ErrSourcePartyUnavailable, err)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return errors.Join(ErrSourcePartyUnavailable, err)
		}
		// Use a fresh statement after lock waits; an earlier snapshot must not
		// certify sessions changed by a writer that committed while we waited.
		current, err := partyQueueProjectionHash(ctx, tx, lease.PartyID)
		if err != nil {
			return errors.Join(ErrSourcePartyUnavailable, err)
		}
		changed = lease.projectionHash == [32]byte{} || current != lease.projectionHash
		if changed {
			success = false
			delay = time.Second
		}
	}
	statement := `UPDATE matchmaking_party_search_queue_intents
SET lease_token=NULL,lease_until=NULL,next_attempt_at=clock_timestamp()+make_interval(secs=>$5),
 projected_at=CASE WHEN $6 THEN clock_timestamp() ELSE projected_at END`
	args := []any{lease.PartyID, lease.Revision, lease.LeaseToken, lease.Action, delay.Seconds(), success}
	if s.NativeAdmission != nil {
		statement += `,projected_state_sha256=CASE WHEN $6 THEN $7 ELSE projected_state_sha256 END,
 projected_admission_stamp_sha256=CASE WHEN $6 THEN $8 ELSE projected_admission_stamp_sha256 END`
		var stamp []byte
		if !lease.terminalProjection {
			stamp = lease.nativeStampHash[:]
		}
		args = append(args, lease.projectionHash[:], stamp)
	}
	statement += ` WHERE party_id=$1 AND revision=$2 AND lease_token=$3 AND action=$4 AND lease_until>clock_timestamp()`
	tag, err := tx.Exec(ctx, statement, args...)
	if err != nil {
		return errors.Join(ErrSourcePartyUnavailable, err)
	}
	if tag.RowsAffected() != 1 {
		return ErrPartyQueueLeaseLost
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.Join(ErrSourcePartyUnavailable, err)
	}
	if changed {
		return ErrPartyQueueProjectionChanged
	}
	return nil
}

func partyQueueProjectionHash(ctx context.Context, tx pgx.Tx, partyID uuid.UUID) ([32]byte, error) {
	var raw string
	err := tx.QueryRow(ctx, `SELECT jsonb_build_object(
'party',(SELECT to_jsonb(p) FROM parties p WHERE id=$1),
'intent',(SELECT jsonb_build_object('revision',i.revision,'action',i.action) FROM matchmaking_party_search_queue_intents i WHERE i.party_id=$1),
'sessions',COALESCE((SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM search_sessions s WHERE party_id=$1),'[]'::jsonb))::text`, partyID).Scan(&raw)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(append([]byte("voice.mm-party.queue-cut.v2\x00"), []byte(raw)...)), nil
}

func validPartyQueueLease(l PartyQueueLease) bool {
	return l.PartyID != uuid.Nil && l.LeaseToken != uuid.Nil && l.Revision > 0 && (l.Action == "ENQUEUE" || l.Action == "RELEASE")
}
