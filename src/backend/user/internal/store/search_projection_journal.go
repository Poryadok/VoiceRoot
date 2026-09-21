package store

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
)

// MaterializeSearchProjectionBaseline durably turns legacy profile rows into
// normal journal/outbox records before a snapshot begins. Holding the same
// allocator row used by AppendSearchProjection serializes this with every live
// mutation, so the returned H is a real immutable boundary.
func (s *ProfileStore) MaterializeSearchProjectionBaseline(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT singleton FROM user_profile_search_offset WHERE singleton = true FOR UPDATE`); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT `+profileSelectCols+` FROM profiles p
		WHERE NOT EXISTS (SELECT 1 FROM user_profile_search_journal j WHERE j.profile_id = p.id)
		ORDER BY p.id FOR UPDATE`)
	if err != nil {
		return err
	}
	var profiles []*ProfileRow
	for rows.Next() {
		p, scanErr := scanProfile(rows)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		profiles = append(profiles, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, p := range profiles {
		if p.SearchProjectionRevision == 0 {
			row := tx.QueryRow(ctx, `UPDATE profiles SET search_projection_revision = 1 WHERE id = $1 RETURNING `+profileSelectCols, p.ID)
			var scanErr error
			p, scanErr = scanProfile(row)
			if scanErr != nil {
				return scanErr
			}
		}
		event := searchProjectionUpsert(p)
		if p.DeletedAt != nil {
			event.Payload = &userv1.SearchProfileProjectionEvent_Delete{Delete: &userv1.SearchProfileDelete{}}
		}
		if err := AppendSearchProjection(ctx, tx, event); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListSearchProjectionSnapshot returns one stable page of the immutable latest
// journal state at H. It deliberately reuses durable payload bytes (including
// event IDs), never reconstructing events from live profile rows.
func (s *ProfileStore) ListSearchProjectionSnapshot(ctx context.Context, high, after uint64, limit int) ([]*userv1.SearchProfileProjectionEvent, uint64, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `WITH latest AS (
		SELECT DISTINCT ON (profile_id) journal_offset, payload
		FROM user_profile_search_journal
		WHERE journal_offset <= $1
		ORDER BY profile_id, journal_offset DESC
	)
	SELECT journal_offset, payload FROM latest
	WHERE journal_offset > $2
	ORDER BY journal_offset
	LIMIT $3`, high, after, limit+1)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]*userv1.SearchProfileProjectionEvent, 0, limit)
	var next uint64
	for rows.Next() {
		var offset uint64
		var payload []byte
		err := rows.Scan(&offset, &payload)
		if err != nil {
			return nil, 0, err
		}
		if len(result) == limit {
			next = result[len(result)-1].GetJournalOffset()
			break
		}
		event := &userv1.SearchProfileProjectionEvent{}
		if err := proto.Unmarshal(payload, event); err != nil {
			return nil, 0, err
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return result, next, nil
}

func (s *ProfileStore) SearchProjectionCheckpoint(ctx context.Context) (uint64, error) {
	var high uint64
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(journal_offset), 0) FROM user_profile_search_journal`).Scan(&high); err != nil {
		return 0, err
	}
	return high, nil
}

func (s *ProfileStore) ListSearchProjectionJournal(ctx context.Context, after uint64, limit int) ([]*userv1.SearchProfileProjectionEvent, uint64, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `SELECT payload FROM user_profile_search_journal WHERE journal_offset > $1 ORDER BY journal_offset LIMIT $2`, after, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var events []*userv1.SearchProfileProjectionEvent
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, 0, err
		}
		event := &userv1.SearchProfileProjectionEvent{}
		if err := proto.Unmarshal(payload, event); err != nil {
			return nil, 0, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	high, err := s.SearchProjectionCheckpoint(ctx)
	return events, high, err
}

// AppendSearchProjection atomically records the immutable User event and its
// publishable outbox row in the caller's profile-mutation transaction.
func AppendSearchProjection(ctx context.Context, tx pgx.Tx, event *userv1.SearchProfileProjectionEvent) error {
	if tx == nil || event == nil || event.GetProtocolVersion() != 1 || event.GetSourceRevision() == 0 {
		return fmt.Errorf("valid v1 search projection event required")
	}
	eventID, err := uuid.Parse(event.GetEventId())
	if err != nil || eventID == uuid.Nil {
		return fmt.Errorf("valid event_id required")
	}
	profileID, err := uuid.Parse(event.GetProfileId())
	if err != nil || profileID == uuid.Nil {
		return fmt.Errorf("valid profile_id required")
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	var offset uint64
	err = tx.QueryRow(ctx, `UPDATE user_profile_search_offset
        SET next_offset = next_offset + 1 WHERE singleton = true
        RETURNING next_offset - 1`).Scan(&offset)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO user_profile_search_journal
		(journal_offset,event_id, profile_id, source_revision, protocol_version, payload, payload_sha256)
	        VALUES ($1,$2,$3,$4,1,$5,$6)`,
		offset, eventID, profileID, event.GetSourceRevision(), payload, digest[:])
	if err != nil {
		return err
	}
	event.JournalOffset = offset
	// Persist bytes with their final offset; the initial insert reserved the
	// gap-free identity value inside this same transaction.
	payload, err = proto.Marshal(event)
	if err != nil {
		return err
	}
	digest = sha256.Sum256(payload)
	if _, err = tx.Exec(ctx, `UPDATE user_profile_search_journal SET payload=$2,payload_sha256=$3 WHERE journal_offset=$1`, offset, payload, digest[:]); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO user_profile_search_outbox(event_id,journal_offset,payload) VALUES($1,$2,$3)`, eventID, offset, payload)
	return err
}
