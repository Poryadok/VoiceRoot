package registry

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const productionAdmissionRoute = "applications.stage_production_admission"

type PrepareProductionAdmissionInput struct {
	ApplicationID     uuid.UUID
	OperatorAccountID uuid.UUID
	IdempotencyKey    string
}

// PrepareProductionAdmission creates an isolated, pending production
// environment. It deliberately does not activate the application or issue
// credentials; activation requires gates beyond this registry operation.
func (s *Store) PrepareProductionAdmission(ctx context.Context, input PrepareProductionAdmissionInput) (Environment, error) {
	in := input
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.ApplicationID == uuid.Nil || in.OperatorAccountID == uuid.Nil || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 128 {
		return Environment{}, ErrInvalidApplication
	}
	if s == nil || s.Pool == nil {
		return Environment{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Environment{}, fmt.Errorf("begin production admission staging: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID uuid.UUID
	var appStatus string
	err = tx.QueryRow(ctx, `SELECT owner_account_id,status FROM applications WHERE id=$1 FOR UPDATE`, in.ApplicationID).
		Scan(&ownerID, &appStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Environment{}, ErrAdmissionConflict
	}
	if err != nil {
		return Environment{}, fmt.Errorf("lock application for production admission: %w", err)
	}
	if ownerID == in.OperatorAccountID {
		return Environment{}, ErrSelfApproval
	}

	hash := sha256.Sum256([]byte(in.ApplicationID.String() + ":production:pending"))
	var savedHash []byte
	var resultID pgtype.UUID
	var operationStatus string
	err = tx.QueryRow(ctx, `SELECT request_hash,result_id,status FROM registry_operations
		WHERE actor_kind='operator' AND actor_id=$1 AND route=$2 AND idempotency_key=$3`,
		in.OperatorAccountID, productionAdmissionRoute, in.IdempotencyKey).
		Scan(&savedHash, &resultID, &operationStatus)
	if err == nil {
		if !equalBytes(savedHash, hash[:]) {
			return Environment{}, ErrIdempotencyConflict
		}
		if operationStatus != "succeeded" || !resultID.Valid {
			return Environment{}, ErrRegistryUnavailable
		}
		env, readErr := getEnvironment(ctx, tx, uuid.UUID(resultID.Bytes))
		if readErr != nil {
			return Environment{}, readErr
		}
		if err := tx.Commit(ctx); err != nil {
			return Environment{}, fmt.Errorf("commit production admission retry: %w", err)
		}
		return env, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Environment{}, fmt.Errorf("read production admission idempotency: %w", err)
	}
	if appStatus != "sandbox" {
		return Environment{}, ErrAdmissionConflict
	}
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM environments WHERE application_id=$1 AND kind='production')`, in.ApplicationID).Scan(&exists)
	if err != nil {
		return Environment{}, fmt.Errorf("check existing production environment: %w", err)
	}
	if exists {
		return Environment{}, ErrAdmissionConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO registry_operations
		(actor_kind,actor_id,route,idempotency_key,request_hash,status)
		VALUES ('operator',$1,$2,$3,$4,'pending')`, in.OperatorAccountID, productionAdmissionRoute, in.IdempotencyKey, hash[:])
	if err != nil {
		return Environment{}, fmt.Errorf("claim production admission idempotency: %w", err)
	}
	envID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO environments (id,application_id,kind,status)
		VALUES ($1,$2,'production','pending')`, envID, in.ApplicationID)
	if err != nil {
		return Environment{}, fmt.Errorf("create pending production environment: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE registry_operations SET result_id=$1,status='succeeded',updated_at=now()
		WHERE actor_kind='operator' AND actor_id=$2 AND route=$3 AND idempotency_key=$4`,
		envID, in.OperatorAccountID, productionAdmissionRoute, in.IdempotencyKey)
	if err != nil {
		return Environment{}, fmt.Errorf("complete production admission operation: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO registry_audit
		(id,actor_kind,actor_id,application_id,environment_id,action,previous_status,new_status,
		 operation_key,source,result,reason_code)
		VALUES ($1,'operator',$2,$3,$4,'stage_production_admission','sandbox','pending',$5,
		 'operator_approved','success','production_admission_staged')`,
		uuid.New(), in.OperatorAccountID, in.ApplicationID, envID, in.IdempotencyKey)
	if err != nil {
		return Environment{}, fmt.Errorf("audit production admission staging: %w", err)
	}
	env, err := getEnvironment(ctx, tx, envID)
	if err != nil {
		return Environment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Environment{}, fmt.Errorf("commit production admission staging: %w", err)
	}
	return env, nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
