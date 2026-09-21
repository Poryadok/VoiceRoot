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

// ListSearchProjectionSnapshot returns the complete current authority state in
// stable UUID order. Deleted profiles are represented as tombstones so a
// retained Search projection cannot resurrect them after bootstrap.
func (s *ProfileStore) ListSearchProjectionSnapshot(ctx context.Context, after *uuid.UUID, limit int) ([]*userv1.SearchProfileProjectionEvent, *uuid.UUID, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	args := []any{limit + 1}
	where := ""
	if after != nil {
		where = "WHERE id > $2"
		args = append(args, *after)
	}
	rows, err := s.pool.Query(ctx, `SELECT `+profileSelectCols+` FROM profiles `+where+` ORDER BY id LIMIT $1`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	result := make([]*userv1.SearchProfileProjectionEvent, 0, limit)
	var next *uuid.UUID
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, nil, err
		}
		if len(result) == limit {
			id := result[len(result)-1].GetProfileId()
			parsed, _ := uuid.Parse(id)
			next = &parsed
			break
		}
		if p.DeletedAt != nil {
			result = append(result, &userv1.SearchProfileProjectionEvent{ProtocolVersion: 1, EventId: uuid.NewString(), ProfileId: p.ID.String(), SourceRevision: p.SearchProjectionRevision, Payload: &userv1.SearchProfileProjectionEvent_Delete{Delete: &userv1.SearchProfileDelete{}}})
		} else {
			result = append(result, searchProjectionUpsert(p))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
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
	err = tx.QueryRow(ctx, `INSERT INTO user_profile_search_journal
        (event_id, profile_id, source_revision, protocol_version, payload, payload_sha256)
        VALUES ($1,$2,$3,1,$4,$5) RETURNING journal_offset`,
		eventID, profileID, event.GetSourceRevision(), payload, digest[:]).Scan(&offset)
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
