package registry

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrSelfApproval      = errors.New("application owner cannot approve")
	ErrAdmissionConflict = errors.New("application cannot be admitted")
)

type ApproveSandboxInput struct {
	ApplicationID     uuid.UUID
	OperatorAccountID uuid.UUID
	IdempotencyKey    string
}

type Environment struct {
	ID            uuid.UUID
	ApplicationID uuid.UUID
	Kind          string
	Status        string
	Revision      int64
	CreatedAt     time.Time
}

// ApproveSandbox is an operator-only state transition. The HTTP layer must
// authenticate the configured operator account before invoking this method.
func (s *Store) ApproveSandbox(ctx context.Context, in ApproveSandboxInput) (Environment, error) {
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.ApplicationID == uuid.Nil || in.OperatorAccountID == uuid.Nil || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 128 {
		return Environment{}, ErrInvalidApplication
	}
	if s == nil || s.Pool == nil {
		return Environment{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Environment{}, fmt.Errorf("begin sandbox approval: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var ownerID uuid.UUID
	var status string
	err = tx.QueryRow(ctx, `SELECT owner_account_id, status FROM applications WHERE id=$1 FOR UPDATE`, in.ApplicationID).Scan(&ownerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Environment{}, ErrAdmissionConflict
	}
	if err != nil {
		return Environment{}, fmt.Errorf("lock application: %w", err)
	}
	if ownerID == in.OperatorAccountID {
		return Environment{}, ErrSelfApproval
	}
	const route = "applications.approve_sandbox"
	hash := sha256.Sum256([]byte(in.ApplicationID.String() + ":sandbox"))
	command, err := tx.Exec(ctx, `
		INSERT INTO registry_operations (actor_kind, actor_id, route, idempotency_key, request_hash, status)
		VALUES ('operator', $1, $2, $3, $4, 'pending') ON CONFLICT DO NOTHING`,
		in.OperatorAccountID, route, in.IdempotencyKey, hash[:])
	if err != nil {
		return Environment{}, fmt.Errorf("insert sandbox approval operation: %w", err)
	}
	if command.RowsAffected() == 0 {
		var savedHash []byte
		var resultID pgtype.UUID
		var operationStatus string
		err = tx.QueryRow(ctx, `SELECT request_hash, result_id, status FROM registry_operations
			WHERE actor_kind='operator' AND actor_id=$1 AND route=$2 AND idempotency_key=$3`,
			in.OperatorAccountID, route, in.IdempotencyKey).Scan(&savedHash, &resultID, &operationStatus)
		if err != nil {
			return Environment{}, fmt.Errorf("read sandbox approval operation: %w", err)
		}
		if string(savedHash) != string(hash[:]) {
			return Environment{}, ErrIdempotencyConflict
		}
		if operationStatus != "succeeded" || !resultID.Valid {
			return Environment{}, ErrRegistryUnavailable
		}
		env, err := getEnvironment(ctx, tx, uuid.UUID(resultID.Bytes))
		if err != nil {
			return Environment{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Environment{}, fmt.Errorf("commit sandbox approval read: %w", err)
		}
		return env, nil
	}
	if status != "draft" {
		return Environment{}, ErrAdmissionConflict
	}
	envID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO environments (id, application_id, kind, status)
		VALUES ($1, $2, 'sandbox', 'active')`, envID, in.ApplicationID)
	if err != nil {
		return Environment{}, fmt.Errorf("create sandbox environment: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE applications SET status='sandbox', revision=revision+1, updated_at=now() WHERE id=$1`, in.ApplicationID)
	if err != nil {
		return Environment{}, fmt.Errorf("admit application sandbox: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE registry_operations SET result_id=$1, status='succeeded', updated_at=now()
		WHERE actor_kind='operator' AND actor_id=$2 AND route=$3 AND idempotency_key=$4`,
		envID, in.OperatorAccountID, route, in.IdempotencyKey)
	if err != nil {
		return Environment{}, fmt.Errorf("complete sandbox approval: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO registry_audit
		(id, actor_kind, actor_id, application_id, environment_id, action, previous_status, new_status, operation_key)
		VALUES ($1, 'operator', $2, $3, $4, 'approve_sandbox', 'draft', 'sandbox', $5)`,
		uuid.New(), in.OperatorAccountID, in.ApplicationID, envID, in.IdempotencyKey)
	if err != nil {
		return Environment{}, fmt.Errorf("audit sandbox approval: %w", err)
	}
	env, err := getEnvironment(ctx, tx, envID)
	if err != nil {
		return Environment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Environment{}, fmt.Errorf("commit sandbox approval: %w", err)
	}
	return env, nil
}

func getEnvironment(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Environment, error) {
	var env Environment
	err := tx.QueryRow(ctx, `SELECT id, application_id, kind, status, revision, created_at FROM environments WHERE id=$1`, id).
		Scan(&env.ID, &env.ApplicationID, &env.Kind, &env.Status, &env.Revision, &env.CreatedAt)
	if err != nil {
		return Environment{}, fmt.Errorf("read environment: %w", err)
	}
	return env, nil
}
