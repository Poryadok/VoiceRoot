// Package lifecycle owns durable metadata fencing the local enqueue boundary.
// It stores neither message payloads nor recipients. Pending work never expires.
package lifecycle

import (
	"context"
	"errors"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDenied   = errors.New("realtime chat lifecycle does not permit delivery")
	ErrPending  = errors.New("realtime admissions remain unresolved")
	ErrConflict = errors.New("realtime lifecycle binding conflict")
)

type Store struct{ Pool *pgxpool.Pool }

// Ready observes the actual metadata surface without creating domain records
// or claiming schema/old-writer/restore qualification for public P3 activation.
func (s *Store) Ready(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `
SELECT id,runtime_identity,state,opened_at,closed_at FROM realtime_incarnations LIMIT 0;
SELECT chat_id,space_id,scope_known,generation,state,operation_id,updated_at FROM realtime_chat_gates LIMIT 0;
SELECT id,incarnation_id,chat_id,generation,state,admitted_at,settled_at FROM realtime_admissions LIMIT 0;
SELECT space_id,operation_id,generation,kind,request_bytes,request_sha256,response_bytes,completed_at FROM realtime_lifecycle_operations LIMIT 0;
SELECT space_id,operation_id,schedule_generation,generation,state,manifest_id,manifest_sha256,item_count,received_pages,received_items,sealed,freeze_complete FROM realtime_manifest_heads LIMIT 0;
SELECT space_id,operation_id,chat_id FROM realtime_manifest_items LIMIT 0`)
	return err
}

func (s *Store) StartClosing(ctx context.Context, id uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `UPDATE realtime_incarnations SET state='CLOSING' WHERE id=$1 AND state='OPEN'`, id)
	return err
}

// CloseIncarnation is called only after the original local executor has closed
// irreversibly and drained all scopes/ambiguous transactions. It cannot clear
// tickets or attest a different process's termination.
func (s *Store) CloseIncarnation(ctx context.Context, id uuid.UUID) error {
	tx, err := begin(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer rollback(tx)
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM realtime_incarnations WHERE id=$1 FOR UPDATE`, id).Scan(&state); err != nil {
		return err
	}
	if state != "CLOSING" {
		return ErrConflict
	}
	var pending bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM realtime_admissions WHERE incarnation_id=$1 AND state='PENDING')`, id).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return ErrPending
	}
	if _, err = tx.Exec(ctx, `UPDATE realtime_incarnations SET state='CLOSED',closed_at=clock_timestamp() WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Open creates a never-reused process incarnation. Reopening an existing UUID,
// even with the same runtime identity, is forbidden.
func (s *Store) Open(ctx context.Context, id uuid.UUID, identity string) error {
	if id == uuid.Nil || identity == "" {
		return ErrConflict
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO realtime_incarnations(id,runtime_identity,state) VALUES($1,$2,'OPEN')`, id, identity)
	return err
}

func begin(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	return pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
}
func rollback(tx pgx.Tx) { _ = tx.Rollback(context.Background()) }

type gate struct {
	space      uuid.UUID
	known      bool
	generation uint64
	state      string
	operation  uuid.UUID
}

func lockGate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (gate, error) {
	if id == uuid.Nil {
		return gate{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, `INSERT INTO realtime_chat_gates(chat_id) VALUES($1) ON CONFLICT DO NOTHING`, id); err != nil {
		return gate{}, err
	}
	var g gate
	var space, operation *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT space_id,scope_known,generation,state,operation_id FROM realtime_chat_gates WHERE chat_id=$1 FOR UPDATE`, id).Scan(&space, &g.known, &g.generation, &g.state, &operation)
	if space != nil {
		g.space = *space
	}
	if operation != nil {
		g.operation = *operation
	}
	return g, err
}

// Hydrate consumes an authenticated Chat-native baseline outside SQL locks.
// It initializes only UNKNOWN; it cannot restore or advance an existing gate.
func (s *Store) Hydrate(ctx context.Context, chat, space uuid.UUID, generation uint64, state string) error {
	if generation > math.MaxInt64 || (state != "LIVE" && state != "FROZEN" && state != "PURGE_DECIDED" && state != "PURGED") {
		return ErrConflict
	}
	tx, err := begin(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer rollback(tx)
	g, err := lockGate(ctx, tx, chat)
	if err != nil {
		return err
	}
	if g.known && g.space != space {
		return ErrConflict
	}
	if g.state == "UNKNOWN" {
		_, err = tx.Exec(ctx, `UPDATE realtime_chat_gates SET space_id=NULLIF($2,'00000000-0000-0000-0000-000000000000'::uuid),scope_known=true,generation=$3,state=$4,updated_at=clock_timestamp() WHERE chat_id=$1`, chat, space, generation, state)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Begin commits outstanding work before the caller may touch any local sink.
// Any commit error denies enqueue. The durable pending row remains conservative
// if commit succeeded but its acknowledgement was lost; no replay occurs here.
func (s *Store) Begin(ctx context.Context, incarnation, chat uuid.UUID) (uuid.UUID, error) {
	tx, err := begin(ctx, s.Pool)
	if err != nil {
		return uuid.Nil, err
	}
	defer rollback(tx)
	var state string
	if err = tx.QueryRow(ctx, `SELECT state FROM realtime_incarnations WHERE id=$1 FOR UPDATE`, incarnation).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrDenied
	} else if err != nil {
		return uuid.Nil, err
	}
	if state != "OPEN" {
		return uuid.Nil, ErrDenied
	}
	g, err := lockGate(ctx, tx, chat)
	if err != nil {
		return uuid.Nil, err
	}
	if !g.known || g.state != "LIVE" {
		return uuid.Nil, ErrDenied
	}
	id := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO realtime_admissions(id,incarnation_id,chat_id,generation,state) VALUES($1,$2,$3,$4,'PENDING')`, id, incarnation, chat, g.generation); err != nil {
		return uuid.Nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// Settle runs only after the last possible synchronous enqueue has returned.
// It is idempotent for the exact process/chat/ticket, never for a replacement.
func (s *Store) Settle(ctx context.Context, incarnation, chat, id uuid.UUID) error {
	tx, err := begin(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if _, err = lockGate(ctx, tx, chat); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE realtime_admissions SET state='SETTLED',settled_at=COALESCE(settled_at,clock_timestamp()) WHERE id=$1 AND incarnation_id=$2 AND chat_id=$3`, id, incarnation, chat)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return tx.Commit(ctx)
}

// Restrict installs a deny independently from completion. Only participant
// operation code may call it after validating exact immutable manifest coverage.
func (s *Store) Restrict(ctx context.Context, chat, space, operation uuid.UUID, generation uint64, state string) error {
	if space == uuid.Nil || operation == uuid.Nil || generation == 0 || generation > math.MaxInt64 || (state != "FROZEN" && state != "PURGE_DECIDED" && state != "PURGED") {
		return ErrConflict
	}
	tx, err := begin(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer rollback(tx)
	g, err := lockGate(ctx, tx, chat)
	if err != nil {
		return err
	}
	if g.known && g.space != space {
		return ErrConflict
	}
	if g.generation > generation || g.state == "PURGED" && state != "PURGED" || g.generation == generation && g.operation != uuid.Nil && g.operation != operation {
		return ErrConflict
	}
	if g.generation == generation && g.state != "UNKNOWN" && g.state != "LIVE" && g.state != state {
		return ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE realtime_chat_gates SET space_id=$2,scope_known=true,operation_id=$3,generation=$4,state=$5,updated_at=clock_timestamp() WHERE chat_id=$1`, chat, space, operation, generation, state)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func canonicalChats(ids []uuid.UUID) ([]uuid.UUID, error) {
	out := append([]uuid.UUID(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	for i, id := range out {
		if id == uuid.Nil || i > 0 && out[i-1] == id {
			return nil, ErrConflict
		}
	}
	return out, nil
}

func drained(ctx context.Context, tx pgx.Tx, ids []uuid.UUID) error {
	for _, id := range ids {
		g, err := lockGate(ctx, tx, id)
		if err != nil {
			return err
		}
		if !g.known || g.state == "UNKNOWN" || g.state == "LIVE" {
			return ErrDenied
		}
	}
	var pending bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM realtime_admissions WHERE chat_id=ANY($1::uuid[]) AND state='PENDING')`, ids).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return ErrPending
	}
	return nil
}

// Drained is an observation only. Receipt finalization must invoke drained in
// the same transaction that persists its immutable completed response.
func (s *Store) Drained(ctx context.Context, chats []uuid.UUID) error {
	ids, err := canonicalChats(chats)
	if err != nil {
		return err
	}
	tx, err := begin(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer rollback(tx)
	if err = drained(ctx, tx, ids); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
