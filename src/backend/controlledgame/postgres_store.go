package controlledgame

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errCommandConflict = errors.New("command conflicts with durable receiver state")

const receiverSchema = `
CREATE TABLE IF NOT EXISTS command_inbox (
	command_id UUID PRIMARY KEY,
	operation_id UUID NOT NULL,
	body_sha256 BYTEA NOT NULL CHECK (octet_length(body_sha256) = 32),
	body_bytes BYTEA NOT NULL,
	receipt_body BYTEA NOT NULL DEFAULT ''::bytea,
	accepted_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS effect_ledger (
	operation_id UUID PRIMARY KEY,
	effect_key BYTEA NOT NULL UNIQUE CHECK (octet_length(effect_key) = 32),
	command_id UUID NOT NULL UNIQUE REFERENCES command_inbox(command_id),
	consumed_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS command_results (
	result_id UUID PRIMARY KEY,
	command_id UUID NOT NULL UNIQUE REFERENCES command_inbox(command_id),
	result_body BYTEA NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS result_outbox (
	outbox_id UUID PRIMARY KEY,
	result_id UUID NOT NULL UNIQUE REFERENCES command_results(result_id),
	command_id UUID NOT NULL UNIQUE REFERENCES command_inbox(command_id),
	result_body BYTEA NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	delivered_at TIMESTAMPTZ
);`

// PostgresStore owns only the controlled receiver schema in its dedicated test DB.
type PostgresStore struct {
	pool  *pgxpool.Pool
	clock func() time.Time
}

// OpenPostgresStore creates the receiver tables idempotently in the supplied,
// isolated database. It must never be pointed at GIS or Voice databases.
func OpenPostgresStore(ctx context.Context, pool *pgxpool.Pool, clock func() time.Time) (*PostgresStore, error) {
	if ctx == nil || pool == nil || clock == nil {
		return nil, errors.New("controlled game store dependencies are required")
	}
	if _, err := pool.Exec(ctx, receiverSchema); err != nil {
		return nil, fmt.Errorf("initialize controlled game receiver schema: %w", err)
	}
	return &PostgresStore{pool: pool, clock: clock}, nil
}

// accept commits inbox, one-shot effect, immutable result and outbox together.
// The bool reports a same-ID, same-body replay whose original receipt is reused.
func (store *PostgresStore) accept(
	ctx context.Context,
	commandID, operationID string,
	effectKey []byte,
	body []byte,
	apply EffectApplier,
) (receipt []byte, replay bool, err error) {
	if store == nil || store.pool == nil || store.clock == nil || apply == nil {
		return nil, false, errors.New("controlled game store is not configured")
	}
	bodyHash := sha256.Sum256(body)
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	insert, err := tx.Exec(ctx, `
		INSERT INTO command_inbox(command_id, operation_id, body_sha256, body_bytes, accepted_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (command_id) DO NOTHING`, commandID, operationID, bodyHash[:], body, store.clock().UTC())
	if err != nil {
		return nil, false, err
	}
	if insert.RowsAffected() == 0 {
		var storedHash, storedReceipt []byte
		if err := tx.QueryRow(ctx, `SELECT body_sha256, receipt_body FROM command_inbox WHERE command_id = $1 FOR SHARE`, commandID).
			Scan(&storedHash, &storedReceipt); err != nil {
			return nil, false, err
		}
		if !equalBytes(storedHash, bodyHash[:]) || len(storedReceipt) == 0 {
			return nil, false, errCommandConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		return storedReceipt, true, nil
	}

	_, err = tx.Exec(ctx, `INSERT INTO effect_ledger(operation_id, effect_key, command_id, consumed_at) VALUES ($1, $2, $3, $4)`,
		operationID, effectKey, commandID, store.clock().UTC())
	if err != nil {
		if isUniqueViolation(err) {
			return nil, false, errCommandConflict
		}
		return nil, false, err
	}

	resultBody, err := apply(ctx, tx, append([]byte(nil), body...))
	if err != nil {
		return nil, false, err
	}
	if len(resultBody) == 0 {
		return nil, false, errors.New("effect returned an empty result")
	}
	var resultEnvelope struct {
		ResultID string `json:"result_id"`
	}
	if err := json.Unmarshal(resultBody, &resultEnvelope); err != nil || !canonicalUUID(resultEnvelope.ResultID) {
		return nil, false, errors.New("effect returned result without a canonical result ID")
	}
	resultID := resultEnvelope.ResultID
	outboxID, err := newUUID()
	if err != nil {
		return nil, false, err
	}
	createdAt := store.clock().UTC()
	if _, err := tx.Exec(ctx, `INSERT INTO command_results(result_id, command_id, result_body, created_at)
		VALUES ($1, $2, $3, $4)`, resultID, commandID, resultBody, createdAt); err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO result_outbox(outbox_id, result_id, command_id, result_body, created_at)
		VALUES ($1, $2, $3, $4, $5)`, outboxID, resultID, commandID, resultBody, createdAt); err != nil {
		return nil, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE command_inbox SET receipt_body = $2 WHERE command_id = $1`, commandID, resultBody); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return append([]byte(nil), resultBody...), false, nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}

func isUniqueViolation(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
