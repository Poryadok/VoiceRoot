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
	"voice/backend/gameintegration/internal/callbacksecurity"
)

var (
	ErrUnsafeCallbackURL        = callbacksecurity.ErrUnsafeURL
	ErrInstallationConflict     = errors.New("installation scope conflict")
	ErrRateLimited              = errors.New("application quota exceeded")
	ErrApplicationSuspended     = errors.New("application suspended")
	ErrApplicationStateConflict = errors.New("application state transition conflict")
)

const createInstallationRoute = "installations.create"

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string { return ErrRateLimited.Error() }

func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

type CreateInstallationInput struct {
	OwnerAccountID uuid.UUID
	ApplicationID  uuid.UUID
	EnvironmentID  uuid.UUID
	CallbackURL    string
	IdempotencyKey string
}

type Installation struct {
	ID            uuid.UUID `json:"installation_id"`
	ApplicationID uuid.UUID `json:"application_id"`
	EnvironmentID uuid.UUID `json:"environment_id"`
	CallbackURL   string    `json:"callback_url"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Store) CreateInstallation(ctx context.Context, input CreateInstallationInput) (Installation, error) {
	in := input
	in.IdempotencyKey = trimASCIIWhitespace(in.IdempotencyKey)
	if in.OwnerAccountID == uuid.Nil || in.ApplicationID == uuid.Nil || in.EnvironmentID == uuid.Nil ||
		in.IdempotencyKey == "" || len(in.IdempotencyKey) > 128 || len(in.CallbackURL) == 0 || len(in.CallbackURL) > 2048 {
		return Installation{}, ErrInvalidApplication
	}
	if s == nil || s.Pool == nil {
		return Installation{}, ErrRegistryUnavailable
	}
	request, err := json.Marshal(struct {
		ApplicationID uuid.UUID `json:"application_id"`
		EnvironmentID uuid.UUID `json:"environment_id"`
		CallbackURL   string    `json:"callback_url"`
	}{in.ApplicationID, in.EnvironmentID, in.CallbackURL})
	if err != nil {
		return Installation{}, fmt.Errorf("hash installation request: %w", err)
	}
	requestHash := sha256.Sum256(request)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Installation{}, fmt.Errorf("begin installation registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID uuid.UUID
	var appStatus string
	err = tx.QueryRow(ctx, `SELECT owner_account_id, status FROM applications WHERE id=$1 FOR UPDATE`, in.ApplicationID).
		Scan(&ownerID, &appStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Installation{}, ErrInstallationConflict
	}
	if err != nil {
		return Installation{}, fmt.Errorf("resolve installation application: %w", err)
	}
	if ownerID != in.OwnerAccountID {
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "owner_mismatch", nil, nil); err != nil {
			return Installation{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Installation{}, fmt.Errorf("commit owner denial audit: %w", err)
		}
		return Installation{}, ErrInstallationConflict
	}
	if appStatus == "suspended" {
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "app_suspended", nil, nil); err != nil {
			return Installation{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Installation{}, fmt.Errorf("commit suspended application denial: %w", err)
		}
		return Installation{}, ErrApplicationSuspended
	}
	if appStatus != "sandbox" && appStatus != "active" {
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "application_state_denied", nil, nil); err != nil {
			return Installation{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Installation{}, fmt.Errorf("commit application state denial: %w", err)
		}
		return Installation{}, ErrInstallationConflict
	}

	// An identical successful retry returns its persisted result without using
	// another quota slot or repeating DNS admission.
	var savedHash []byte
	var savedID pgtype.UUID
	var operationStatus string
	err = tx.QueryRow(ctx, `SELECT request_hash, result_id, status FROM registry_operations
		WHERE actor_kind='account' AND actor_id=$1 AND route=$2 AND idempotency_key=$3`, in.OwnerAccountID, createInstallationRoute, in.IdempotencyKey).
		Scan(&savedHash, &savedID, &operationStatus)
	if err == nil {
		if string(savedHash) != string(requestHash[:]) {
			return Installation{}, ErrIdempotencyConflict
		}
		if operationStatus != "succeeded" || !savedID.Valid {
			return Installation{}, ErrRegistryUnavailable
		}
		installation, err := getInstallation(ctx, tx, uuid.UUID(savedID.Bytes))
		if err != nil {
			return Installation{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Installation{}, fmt.Errorf("commit installation retry: %w", err)
		}
		return installation, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Installation{}, fmt.Errorf("read installation idempotency record: %w", err)
	}

	now := s.now()
	windowStart := now.UTC().Truncate(time.Minute)
	if err := admitInstallationQuota(ctx, tx, in.ApplicationID, windowStart); err != nil {
		if errors.Is(err, ErrRateLimited) {
			if auditErr := writeQuotaDenial(ctx, tx, in, windowStart); auditErr != nil {
				return Installation{}, auditErr
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return Installation{}, fmt.Errorf("commit quota denial audit: %w", commitErr)
			}
			return Installation{}, &RateLimitError{RetryAfter: windowStart.Add(time.Minute).Sub(now)}
		}
		return Installation{}, err
	}

	var environmentApp uuid.UUID
	var environmentStatus string
	err = tx.QueryRow(ctx, `SELECT application_id, status FROM environments WHERE id=$1`, in.EnvironmentID).
		Scan(&environmentApp, &environmentStatus)
	if err != nil || environmentApp != in.ApplicationID || environmentStatus != "active" {
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "environment_scope_denied", nil, nil); err != nil {
			return Installation{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Installation{}, fmt.Errorf("commit installation scope denial: %w", err)
		}
		return Installation{}, ErrInstallationConflict
	}

	if err := callbacksecurity.ValidateURL(ctx, in.CallbackURL, s.CallbackResolver); err != nil {
		if auditErr := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "developer_asserted", "callback_destination_rejected", nil, nil); auditErr != nil {
			return Installation{}, auditErr
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return Installation{}, fmt.Errorf("commit callback destination denial: %w", commitErr)
		}
		return Installation{}, ErrUnsafeCallbackURL
	}

	command, err := tx.Exec(ctx, `INSERT INTO registry_operations
		(actor_kind, actor_id, route, idempotency_key, request_hash, status)
		VALUES ('account', $1, $2, $3, $4, 'pending') ON CONFLICT DO NOTHING`,
		in.OwnerAccountID, createInstallationRoute, in.IdempotencyKey, requestHash[:])
	if err != nil {
		return Installation{}, fmt.Errorf("insert installation operation: %w", err)
	}
	if command.RowsAffected() == 0 {
		return Installation{}, ErrIdempotencyConflict
	}
	installationID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO installations (id, application_id, environment_id, callback_url)
		VALUES ($1,$2,$3,$4)`, installationID, in.ApplicationID, in.EnvironmentID, in.CallbackURL)
	if err != nil {
		return Installation{}, fmt.Errorf("persist installation callback: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE registry_operations SET result_id=$1, status='succeeded', updated_at=now()
		WHERE actor_kind='account' AND actor_id=$2 AND route=$3 AND idempotency_key=$4`,
		installationID, in.OwnerAccountID, createInstallationRoute, in.IdempotencyKey)
	if err != nil {
		return Installation{}, fmt.Errorf("complete installation operation: %w", err)
	}
	if err := writeInstallationAudit(ctx, tx, in, "register_installation", "success", "developer_asserted", "installation_registered", nil, &installationID); err != nil {
		return Installation{}, err
	}
	installation, err := getInstallation(ctx, tx, installationID)
	if err != nil {
		return Installation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Installation{}, fmt.Errorf("commit installation registration: %w", err)
	}
	return installation, nil
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func admitInstallationQuota(ctx context.Context, tx pgx.Tx, appID uuid.UUID, windowStart time.Time) error {
	command, err := tx.Exec(ctx, `INSERT INTO app_quota_windows (application_id, window_start, request_count)
		VALUES ($1,$2,1) ON CONFLICT (application_id) DO NOTHING`, appID, windowStart)
	if err != nil {
		return fmt.Errorf("initialize installation quota: %w", err)
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	var savedWindow time.Time
	var count int
	if err := tx.QueryRow(ctx, `SELECT window_start, request_count FROM app_quota_windows
		WHERE application_id=$1 FOR UPDATE`, appID).Scan(&savedWindow, &count); err != nil {
		return fmt.Errorf("lock installation quota: %w", err)
	}
	if !savedWindow.Equal(windowStart) {
		_, err := tx.Exec(ctx, `UPDATE app_quota_windows SET window_start=$2, request_count=1, updated_at=now()
			WHERE application_id=$1`, appID, windowStart)
		if err != nil {
			return fmt.Errorf("roll installation quota window: %w", err)
		}
		return nil
	}
	if count >= 120 {
		return ErrRateLimited
	}
	_, err = tx.Exec(ctx, `UPDATE app_quota_windows SET request_count=request_count+1, updated_at=now()
		WHERE application_id=$1`, appID)
	if err != nil {
		return fmt.Errorf("increment installation quota: %w", err)
	}
	return nil
}

func writeQuotaDenial(ctx context.Context, tx pgx.Tx, in CreateInstallationInput, windowStart time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO registry_audit
		(id, actor_kind, actor_id, application_id, environment_id, action, new_status, operation_key,
		 source, result, reason_code, quota_window_start, denial_count)
		VALUES ($1, 'account', $2, $3, $4, 'quota_denied', 'denied', 'installation_registration',
		 'authenticated_account', 'denied', 'quota_exceeded', $5, 1)
		ON CONFLICT (application_id, quota_window_start) WHERE action='quota_denied'
		DO UPDATE SET denial_count=registry_audit.denial_count+1`,
		uuid.New(), in.OwnerAccountID, in.ApplicationID, in.EnvironmentID, windowStart)
	if err != nil {
		return fmt.Errorf("audit installation quota denial: %w", err)
	}
	return nil
}

func writeInstallationAudit(ctx context.Context, tx pgx.Tx, in CreateInstallationInput, action, result, source, reasonCode string, windowStart *time.Time, installationID *uuid.UUID) error {
	var window any
	if windowStart != nil {
		window = *windowStart
	}
	var installation any
	if installationID != nil {
		installation = *installationID
	}
	_, err := tx.Exec(ctx, `INSERT INTO registry_audit
		(id, actor_kind, actor_id, application_id, environment_id, installation_id, action, new_status,
		 operation_key, source, result, reason_code, quota_window_start)
		VALUES ($1, 'account', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		uuid.New(), in.OwnerAccountID, in.ApplicationID, in.EnvironmentID, installation,
		action, result, in.IdempotencyKey, source, result, reasonCode, window)
	if err != nil {
		return fmt.Errorf("audit installation registration: %w", err)
	}
	return nil
}

func getInstallation(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Installation, error) {
	var installation Installation
	err := tx.QueryRow(ctx, `SELECT id, application_id, environment_id, callback_url, status, created_at
		FROM installations WHERE id=$1`, id).Scan(
		&installation.ID, &installation.ApplicationID, &installation.EnvironmentID,
		&installation.CallbackURL, &installation.Status, &installation.CreatedAt)
	if err != nil {
		return Installation{}, fmt.Errorf("read installation: %w", err)
	}
	return installation, nil
}

func trimASCIIWhitespace(value string) string {
	start, end := 0, len(value)
	for start < end && (value[start] == ' ' || value[start] == '\t' || value[start] == '\r' || value[start] == '\n') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t' || value[end-1] == '\r' || value[end-1] == '\n') {
		end--
	}
	return value[start:end]
}
