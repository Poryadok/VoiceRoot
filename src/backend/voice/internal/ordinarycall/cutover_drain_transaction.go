package ordinarycall

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"math"
	"reflect"
	"time"
	"voice/backend/voice/internal/rosteroutbox"
)

// mutateTx keeps the native room/identity/fence/outbox lock order in the
// caller's transaction. It never commits or performs external IO. The caller
// must roll back on any error; ordinary Mutate retains its original boundary.
func (s *PostgresStore) mutateTx(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, change func(*Room) error) (*Room, error) {
	if s == nil || s.encode == nil || tx == nil || change == nil {
		return nil, ErrUnavailable
	}
	var body []byte
	if err := tx.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1 FOR UPDATE`, roomID).Scan(&body); err != nil {
		return nil, mapStoreError(err)
	}
	before, err := decodeRoom(body, roomID)
	if err != nil {
		return nil, err
	}
	room, err := decodeRoom(body, roomID)
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&room.TransactionTime); err != nil {
		return nil, err
	}
	if err := change(room); err != nil {
		return nil, err
	}
	if room.Intent.RoomID != before.Intent.RoomID || room.Intent.OperationID != before.Intent.OperationID ||
		room.Intent.ChatID != before.Intent.ChatID || room.Intent.Group != before.Intent.Group ||
		!reflect.DeepEqual(room.Intent, before.Intent) ||
		room.LiveKitRoomName != before.LiveKitRoomName || room.Version < before.Version || room.RosterVersion < before.RosterVersion || len(room.Events) < len(before.Events) {
		return nil, ErrInvalidIntent
	}
	for index := range before.Events {
		if !reflect.DeepEqual(room.Events[index], before.Events[index]) {
			return nil, ErrInvalidIntent
		}
	}
	if err := lockAdmissions(ctx, tx, before, room); err != nil {
		return nil, err
	}
	// Identity-lock waits can cross the ring deadline even when the request's
	// earlier clock sample passed it. The final transaction must check again
	// with the database clock after the barrier, never accept on that old sample.
	var decisionAt time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&decisionAt); err != nil {
		return nil, err
	}
	if before.Stage == StageRinging && room.Stage == StageActive && !decisionAt.Before(before.ExpiresAt) {
		return nil, ErrExpired
	}
	if before.Stage == StagePreparing && (room.Stage == StageRinging || room.Stage == StageActive) {
		room.ReadyAt = decisionAt
		if room.Stage == StageRinging {
			room.ExpiresAt = decisionAt.Add(30 * time.Second)
		}
	}
	room.TransactionTime = decisionAt
	for index := len(before.Events); index < len(room.Events); index++ {
		room.Events[index].OccurredAt = decisionAt
	}
	for id, admission := range room.Admissions {
		if _, existed := before.Admissions[id]; !existed {
			admission.CreatedAt = decisionAt
			room.Admissions[id] = admission
		}
	}
	for profile, member := range room.Members {
		prior := before.Members[profile]
		if member.State == MemberJoined && (prior.State != MemberJoined || prior.Generation != member.Generation) {
			member.JoinedAt = decisionAt
		}
		if !member.LeftAt.IsZero() && prior.LeftAt.IsZero() {
			member.LeftAt = decisionAt
		}
		room.Members[profile] = member
	}
	if !room.EndedAt.IsZero() && before.EndedAt.IsZero() {
		room.EndedAt = decisionAt
	}
	if err := validateRoom(room); err != nil {
		return nil, err
	}
	if err := syncFences(ctx, tx, before, room); err != nil {
		return nil, err
	}
	if err := syncMemberships(ctx, tx, room); err != nil {
		return nil, err
	}
	body, err = json.Marshal(room)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE voice_ordinary_rooms SET state=$2,version=$3,document=$4,roster_version=$5,
next_attempt_at=CASE WHEN $2='PREPARING' THEN clock_timestamp()+interval '30 seconds' ELSE clock_timestamp() END,
updated_at=clock_timestamp() WHERE room_id=$1`, roomID, string(room.Stage), room.Version, body, room.RosterVersion)
	if err != nil {
		return nil, err
	}
	for index := len(before.Events); index < len(room.Events); index++ {
		event, err := s.encode(room, room.Events[index])
		if err != nil {
			return nil, err
		}
		if event.ID != room.Events[index].ID || event.ID == uuid.Nil || len(event.Payload) == 0 {
			return nil, ErrInvalidIntent
		}
		var rosterVersion any
		if room.Events[index].RosterVersion > 0 {
			rosterVersion = room.Events[index].RosterVersion
		}
		_, err = tx.Exec(ctx, `INSERT INTO voice_ordinary_outbox(event_id,room_id,room_version,ordinal,subject,payload,roster_version)
VALUES($1,$2,$3,$4,$5,$6,$7)`, event.ID, roomID, room.Version, index+1, event.Subject, event.Payload, rosterVersion)
		if err != nil {
			return nil, err
		}
		fact := room.Events[index]
		if fact.RosterVersion > 0 {
			if fact.RosterVersion > math.MaxInt64 || (fact.Kind != EventParticipantJoined && fact.Kind != EventParticipantLeft) {
				return nil, ErrInvalidIntent
			}
			if err := rosteroutbox.Append(ctx, tx, roomID, fact.ProfileID, int64(fact.RosterVersion), fact.Kind == EventParticipantJoined); err != nil {
				return nil, err
			}
		}
	}
	return room, nil
}
