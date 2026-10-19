package ordinarycall

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrCutoverOwned means generic execution is excluded; native owner work is
// still pending. It is neither a retired-terminal receipt nor SDK success.
var ErrCutoverOwned = errors.New("ordinary call execution owned by cutover")

// CheckExecution is only the native exclusion gate. Passing it does not prove
// an SDK route's historical resource binding or retirement continuity. Those
// mandatory composition predicates remain separate from this first slice.
func (s *PostgresStore) CheckExecution(ctx context.Context, roomID, incarnation uuid.UUID) error {
	if s == nil || s.pool == nil {
		return ErrUnavailable
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(tx)
	var body []byte
	if err = tx.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1 FOR SHARE`, roomID).Scan(&body); err != nil {
		return mapStoreError(err)
	}
	room, err := decodeRoom(body, roomID)
	if err != nil {
		return err
	}
	if room.Intent.OperationID != incarnation {
		return ErrConflict
	}
	if err = checkCutoverExecution(ctx, tx, room); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// The caller holds the native room lock. Missing protected schema or invalid
// work is an error, never permission to execute a fallback current client.
func checkCutoverExecution(ctx context.Context, tx pgx.Tx, room *Room) error {
	rows, err := tx.Query(ctx, `SELECT work_id FROM voice_ordinary_cutover_works WHERE room_id=$1 ORDER BY work_id`, room.Intent.RoomID)
	if err != nil {
		return errors.Join(ErrUnavailable, err)
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return errors.Join(ErrUnavailable, err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return errors.Join(ErrUnavailable, err)
	}
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		work, e := readCutoverWork(ctx, tx, id)
		if e != nil {
			return errors.Join(ErrUnavailable, e)
		}
		var request cutoverRequest
		if json.Unmarshal(work.OwnerRequestBytes, &request) != nil || request.Expected.RoomID != room.Intent.RoomID || request.Expected.Incarnation != room.Intent.OperationID {
			return ErrConflict
		}
	}
	// No existing work state is a permission to resume generic execution, even
	// after a future final receipt. Retired-terminal continuity is not wired yet.
	return ErrCutoverOwned
}
