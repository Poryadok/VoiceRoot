package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const applicationSuspensionRoute = "applications.suspension"

type SetApplicationSuspensionInput struct {
	ApplicationID     uuid.UUID
	OperatorAccountID uuid.UUID
	Suspended         bool
	IdempotencyKey    string
}

func (s *Store) SetApplicationSuspension(ctx context.Context, in SetApplicationSuspensionInput) (Application, error) {
	in.IdempotencyKey = trimASCIIWhitespace(in.IdempotencyKey)
	if in.ApplicationID == uuid.Nil || in.OperatorAccountID == uuid.Nil ||
		in.IdempotencyKey == "" || len(in.IdempotencyKey) > 128 {
		return Application{}, ErrInvalidApplication
	}
	if s == nil || s.Pool == nil {
		return Application{}, ErrRegistryUnavailable
	}
	request, err := json.Marshal(struct {
		ApplicationID uuid.UUID `json:"application_id"`
		Suspended     bool      `json:"suspended"`
	}{in.ApplicationID, in.Suspended})
	if err != nil {
		return Application{}, fmt.Errorf("hash application suspension: %w", err)
	}
	hash := sha256.Sum256(request)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Application{}, fmt.Errorf("begin application suspension: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID uuid.UUID
	var currentStatus string
	var suspendedFrom pgtype.Text
	err = tx.QueryRow(ctx, `SELECT owner_account_id, status, suspended_from_status FROM applications
		WHERE id=$1 FOR UPDATE`, in.ApplicationID).Scan(&ownerID, &currentStatus, &suspendedFrom)
	if errors.Is(err, pgx.ErrNoRows) {
		return Application{}, ErrAdmissionConflict
	}
	if err != nil {
		return Application{}, fmt.Errorf("lock application for suspension: %w", err)
	}
	if ownerID == in.OperatorAccountID {
		return Application{}, ErrSelfApproval
	}

	var savedHash []byte
	var savedID pgtype.UUID
	var savedRevision pgtype.Int8
	var savedStatus pgtype.Text
	var savedUpdatedAt pgtype.Timestamptz
	var operationStatus string
	err = tx.QueryRow(ctx, `SELECT request_hash, result_id, result_revision, result_status, result_updated_at, status FROM registry_operations
		WHERE actor_kind='operator' AND actor_id=$1 AND route=$2 AND idempotency_key=$3`,
		in.OperatorAccountID, applicationSuspensionRoute, in.IdempotencyKey).
		Scan(&savedHash, &savedID, &savedRevision, &savedStatus, &savedUpdatedAt, &operationStatus)
	if err == nil {
		if string(savedHash) != string(hash[:]) {
			return Application{}, ErrIdempotencyConflict
		}
		if operationStatus != "succeeded" || !savedID.Valid || !savedRevision.Valid || !savedStatus.Valid || !savedUpdatedAt.Valid {
			return Application{}, ErrRegistryUnavailable
		}
		application, getErr := getApplication(ctx, tx, uuid.UUID(savedID.Bytes))
		if getErr != nil {
			return Application{}, getErr
		}
		application.Status = savedStatus.String
		application.Revision = savedRevision.Int64
		application.UpdatedAt = savedUpdatedAt.Time
		if err := tx.Commit(ctx); err != nil {
			return Application{}, fmt.Errorf("commit application suspension retry: %w", err)
		}
		return application, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Application{}, fmt.Errorf("read application suspension operation: %w", err)
	}

	newStatus := ""
	action := ""
	if in.Suspended && (currentStatus == "sandbox" || currentStatus == "active") {
		newStatus = "suspended"
		action = "suspend_application"
	} else if !in.Suspended && currentStatus == "suspended" && suspendedFrom.Valid &&
		(suspendedFrom.String == "sandbox" || suspendedFrom.String == "active") {
		newStatus = suspendedFrom.String
		action = "restore_application"
	}
	if newStatus == "" {
		if err := writeSuspensionAudit(ctx, tx, in, currentStatus, currentStatus, map[bool]string{true: "suspend_application", false: "restore_application"}[in.Suspended], "denied"); err != nil {
			return Application{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Application{}, fmt.Errorf("commit blocked suspension audit: %w", err)
		}
		return Application{}, ErrApplicationStateConflict
	}

	command, err := tx.Exec(ctx, `INSERT INTO registry_operations
		(actor_kind, actor_id, route, idempotency_key, request_hash, status)
		VALUES ('operator', $1, $2, $3, $4, 'pending') ON CONFLICT DO NOTHING`,
		in.OperatorAccountID, applicationSuspensionRoute, in.IdempotencyKey, hash[:])
	if err != nil {
		return Application{}, fmt.Errorf("insert suspension operation: %w", err)
	}
	if command.RowsAffected() == 0 {
		return Application{}, ErrIdempotencyConflict
	}
	var revision int64
	var updatedAt time.Time
	err = tx.QueryRow(ctx, `UPDATE applications SET status=$2, suspended_from_status=$3,
		revision=revision+1, updated_at=now() WHERE id=$1 RETURNING revision,updated_at`,
		in.ApplicationID, newStatus, func() any {
			if in.Suspended {
				return currentStatus
			}
			return nil
		}()).Scan(&revision, &updatedAt)
	if err != nil {
		return Application{}, fmt.Errorf("update application suspension: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE registry_operations SET result_id=$1,result_revision=$2,result_status=$3,
		result_updated_at=$4,status='succeeded',updated_at=now()
		WHERE actor_kind='operator' AND actor_id=$5 AND route=$6 AND idempotency_key=$7`,
		in.ApplicationID, revision, newStatus, updatedAt, in.OperatorAccountID, applicationSuspensionRoute, in.IdempotencyKey)
	if err != nil {
		return Application{}, fmt.Errorf("complete suspension operation: %w", err)
	}
	if err := writeSuspensionAudit(ctx, tx, in, currentStatus, newStatus, action, "success"); err != nil {
		return Application{}, err
	}
	application, err := getApplication(ctx, tx, in.ApplicationID)
	if err != nil {
		return Application{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Application{}, fmt.Errorf("commit application suspension: %w", err)
	}
	return application, nil
}

func writeSuspensionAudit(ctx context.Context, tx pgx.Tx, in SetApplicationSuspensionInput, previous, next, action, result string) error {
	reasonCode := "application_suspended"
	if result == "denied" {
		reasonCode = "application_state_conflict"
	} else if action == "restore_application" {
		reasonCode = "application_restored"
	}
	_, err := tx.Exec(ctx, `INSERT INTO registry_audit
		(id, actor_kind, actor_id, application_id, action, previous_status, new_status,
		 operation_key, source, result, reason_code)
		VALUES ($1, 'operator', $2, $3, $4, $5, $6, $7, 'operator_allowlist', $8, $9)`,
		uuid.New(), in.OperatorAccountID, in.ApplicationID, action, previous, next, in.IdempotencyKey, result, reasonCode)
	if err != nil {
		return fmt.Errorf("audit application suspension: %w", err)
	}
	return nil
}
