package registry

import (
	"bytes"
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
	ErrBotAuthorityDenied       = errors.New("bot authority denied")
	ErrBotAuthorityUnavailable  = errors.New("bot authority unavailable")
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
	BotID          uuid.UUID
	CallbackURL    string
	IdempotencyKey string
}

type BotAuthorityVerifier interface {
	VerifyGameIntegrationBot(context.Context, uuid.UUID, uuid.UUID) error
}

type Installation struct {
	ID            uuid.UUID `json:"installation_id"`
	ApplicationID uuid.UUID `json:"application_id"`
	EnvironmentID uuid.UUID `json:"environment_id"`
	BotID         uuid.UUID `json:"bot_id"`
	CallbackURL   string    `json:"callback_url"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Store) CreateInstallation(ctx context.Context, input CreateInstallationInput) (Installation, error) {
	in := input
	in.IdempotencyKey = trimASCIIWhitespace(in.IdempotencyKey)
	if in.OwnerAccountID == uuid.Nil || in.ApplicationID == uuid.Nil || in.EnvironmentID == uuid.Nil || in.BotID == uuid.Nil ||
		in.IdempotencyKey == "" || len(in.IdempotencyKey) > 128 || len(in.CallbackURL) == 0 || len(in.CallbackURL) > 2048 {
		return Installation{}, ErrInvalidApplication
	}
	if s == nil || s.Pool == nil {
		return Installation{}, ErrRegistryUnavailable
	}
	request, err := json.Marshal(struct {
		ApplicationID uuid.UUID `json:"application_id"`
		EnvironmentID uuid.UUID `json:"environment_id"`
		BotID         uuid.UUID `json:"bot_id"`
		CallbackURL   string    `json:"callback_url"`
	}{in.ApplicationID, in.EnvironmentID, in.BotID, in.CallbackURL})
	if err != nil {
		return Installation{}, fmt.Errorf("hash installation request: %w", err)
	}
	requestHash := sha256.Sum256(request)
	ownerID, existing, found, err := s.preflightInstallation(ctx, in, requestHash[:])
	if err != nil {
		return Installation{}, err
	}
	if found {
		return existing, nil
	}
	claimedResult, claimedReplay, err := s.admitInstallationAttempt(ctx, in, requestHash[:])
	if err != nil {
		return Installation{}, err
	}
	if claimedReplay {
		return claimedResult, nil
	}
	if s.BotAuthority == nil {
		s.recordBotAuthorityFailure(ctx, in, "bot_authority_unavailable")
		return Installation{}, ErrBotAuthorityUnavailable
	}
	if err := s.BotAuthority.VerifyGameIntegrationBot(ctx, in.BotID, ownerID); err != nil {
		if errors.Is(err, ErrBotAuthorityDenied) {
			s.recordBotAuthorityFailure(ctx, in, "bot_authority_denied")
			return Installation{}, ErrBotAuthorityDenied
		}
		s.recordBotAuthorityFailure(ctx, in, "bot_authority_unavailable")
		return Installation{}, ErrBotAuthorityUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Installation{}, fmt.Errorf("begin installation registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

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
		if err := markInstallationOperationFailed(ctx, tx, in); err != nil {
			return Installation{}, err
		}
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "owner_mismatch", nil, nil); err != nil {
			return Installation{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Installation{}, fmt.Errorf("commit owner denial audit: %w", err)
		}
		return Installation{}, ErrInstallationConflict
	}
	if appStatus == "suspended" {
		if err := markInstallationOperationFailed(ctx, tx, in); err != nil {
			return Installation{}, err
		}
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "app_suspended", nil, nil); err != nil {
			return Installation{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Installation{}, fmt.Errorf("commit suspended application denial: %w", err)
		}
		return Installation{}, ErrApplicationSuspended
	}
	if appStatus != "sandbox" && appStatus != "active" {
		if err := markInstallationOperationFailed(ctx, tx, in); err != nil {
			return Installation{}, err
		}
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "application_state_denied", nil, nil); err != nil {
			return Installation{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Installation{}, fmt.Errorf("commit application state denial: %w", err)
		}
		return Installation{}, ErrInstallationConflict
	}

	var environmentApp uuid.UUID
	var environmentStatus string
	err = tx.QueryRow(ctx, `SELECT application_id, status FROM environments WHERE id=$1 FOR SHARE`, in.EnvironmentID).
		Scan(&environmentApp, &environmentStatus)
	if err != nil || environmentApp != in.ApplicationID || environmentStatus != "active" {
		if err := markInstallationOperationFailed(ctx, tx, in); err != nil {
			return Installation{}, err
		}
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "environment_scope_denied", nil, nil); err != nil {
			return Installation{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Installation{}, fmt.Errorf("commit installation scope denial: %w", err)
		}
		return Installation{}, ErrInstallationConflict
	}

	if err := callbacksecurity.ValidateURL(ctx, in.CallbackURL, s.CallbackResolver); err != nil {
		if failErr := markInstallationOperationFailed(ctx, tx, in); failErr != nil {
			return Installation{}, failErr
		}
		if auditErr := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "developer_asserted", "callback_destination_rejected", nil, nil); auditErr != nil {
			return Installation{}, auditErr
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return Installation{}, fmt.Errorf("commit callback destination denial: %w", commitErr)
		}
		return Installation{}, ErrUnsafeCallbackURL
	}

	installationID := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO installations (id, application_id, environment_id, bot_id, callback_url)
		VALUES ($1,$2,$3,$4,$5)`, installationID, in.ApplicationID, in.EnvironmentID, in.BotID, in.CallbackURL)
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

// preflightInstallation resolves the owner from GIS-owned state and returns an
// already committed idempotent result before making the Bot service call. This
// keeps retries stable during Bot outages without holding a GIS transaction
// open across the network.
func (s *Store) preflightInstallation(ctx context.Context, in CreateInstallationInput, requestHash []byte) (uuid.UUID, Installation, bool, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, Installation{}, false, fmt.Errorf("begin installation authority preflight: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var ownerID uuid.UUID
	var appStatus string
	err = tx.QueryRow(ctx, `SELECT owner_account_id, status FROM applications WHERE id=$1 FOR SHARE`, in.ApplicationID).
		Scan(&ownerID, &appStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, Installation{}, false, ErrInstallationConflict
	}
	if err != nil {
		return uuid.Nil, Installation{}, false, fmt.Errorf("resolve installation owner: %w", err)
	}
	if ownerID != in.OwnerAccountID {
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "owner_mismatch", nil, nil); err != nil {
			return uuid.Nil, Installation{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return uuid.Nil, Installation{}, false, fmt.Errorf("commit owner denial audit: %w", err)
		}
		return uuid.Nil, Installation{}, false, ErrInstallationConflict
	}
	if appStatus == "suspended" {
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "app_suspended", nil, nil); err != nil {
			return uuid.Nil, Installation{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return uuid.Nil, Installation{}, false, fmt.Errorf("commit suspended application denial: %w", err)
		}
		return uuid.Nil, Installation{}, false, ErrApplicationSuspended
	}
	if appStatus != "sandbox" && appStatus != "active" {
		if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", "application_state_denied", nil, nil); err != nil {
			return uuid.Nil, Installation{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return uuid.Nil, Installation{}, false, fmt.Errorf("commit application state denial: %w", err)
		}
		return uuid.Nil, Installation{}, false, ErrInstallationConflict
	}
	var savedHash []byte
	var savedID pgtype.UUID
	var operationStatus string
	err = tx.QueryRow(ctx, `SELECT request_hash, result_id, status FROM registry_operations
		WHERE actor_kind='account' AND actor_id=$1 AND route=$2 AND idempotency_key=$3`, in.OwnerAccountID, createInstallationRoute, in.IdempotencyKey).
		Scan(&savedHash, &savedID, &operationStatus)
	if err == nil {
		if !bytes.Equal(savedHash, requestHash) {
			return uuid.Nil, Installation{}, false, ErrIdempotencyConflict
		}
		if operationStatus == "failed" {
			failure := installationOperationFailure(ctx, tx, in)
			if err := tx.Commit(ctx); err != nil {
				return uuid.Nil, Installation{}, false, fmt.Errorf("commit failed installation retry: %w", err)
			}
			return uuid.Nil, Installation{}, false, failure
		}
		if operationStatus != "succeeded" || !savedID.Valid {
			return uuid.Nil, Installation{}, false, ErrRegistryUnavailable
		}
		installation, err := getInstallation(ctx, tx, uuid.UUID(savedID.Bytes))
		if err != nil {
			return uuid.Nil, Installation{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return uuid.Nil, Installation{}, false, fmt.Errorf("commit installation retry: %w", err)
		}
		return ownerID, installation, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, Installation{}, false, fmt.Errorf("read installation idempotency record: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, Installation{}, false, fmt.Errorf("commit installation authority preflight: %w", err)
	}
	return ownerID, Installation{}, false, nil
}

func (s *Store) recordBotAuthorityFailure(ctx context.Context, in CreateInstallationInput, reason string) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := markInstallationOperationFailed(ctx, tx, in); err != nil {
		return
	}
	if err := writeInstallationAudit(ctx, tx, in, "register_installation", "denied", "authenticated_account", reason, nil, nil); err != nil {
		return
	}
	_ = tx.Commit(ctx)
}

// admitInstallationAttempt durably consumes an authenticated owner's quota
// before the external Bot proof call. Failed Bot proofs therefore cannot be
// used to bypass the per-application request bound or amplify proof traffic.
func (s *Store) admitInstallationAttempt(ctx context.Context, in CreateInstallationInput, requestHash []byte) (Installation, bool, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Installation{}, false, fmt.Errorf("begin installation quota admission: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	claimed, err := tx.Exec(ctx, `INSERT INTO registry_operations
		(actor_kind, actor_id, route, idempotency_key, request_hash, status)
		VALUES ('account', $1, $2, $3, $4, 'pending') ON CONFLICT DO NOTHING`,
		in.OwnerAccountID, createInstallationRoute, in.IdempotencyKey, requestHash)
	if err != nil {
		return Installation{}, false, fmt.Errorf("claim installation operation: %w", err)
	}
	if claimed.RowsAffected() == 0 {
		var savedHash []byte
		var savedID pgtype.UUID
		var status string
		if err := tx.QueryRow(ctx, `SELECT request_hash, result_id, status FROM registry_operations
			WHERE actor_kind='account' AND actor_id=$1 AND route=$2 AND idempotency_key=$3`,
			in.OwnerAccountID, createInstallationRoute, in.IdempotencyKey).Scan(&savedHash, &savedID, &status); err != nil {
			return Installation{}, false, fmt.Errorf("read claimed installation operation: %w", err)
		}
		if !bytes.Equal(savedHash, requestHash) {
			return Installation{}, false, ErrIdempotencyConflict
		}
		if status == "succeeded" && savedID.Valid {
			installation, err := getInstallation(ctx, tx, uuid.UUID(savedID.Bytes))
			if err != nil {
				return Installation{}, false, err
			}
			if err := tx.Commit(ctx); err != nil {
				return Installation{}, false, fmt.Errorf("commit concurrent installation retry: %w", err)
			}
			return installation, true, nil
		}
		if status == "failed" {
			failure := installationOperationFailure(ctx, tx, in)
			if err := tx.Commit(ctx); err != nil {
				return Installation{}, false, fmt.Errorf("commit failed installation retry: %w", err)
			}
			return Installation{}, false, failure
		}
		return Installation{}, false, ErrRegistryUnavailable
	}
	now := s.now()
	windowStart := now.UTC().Truncate(time.Minute)
	if err := admitInstallationQuota(ctx, tx, in.ApplicationID, windowStart); err != nil {
		if errors.Is(err, ErrRateLimited) {
			if _, deleteErr := tx.Exec(ctx, `DELETE FROM registry_operations WHERE actor_kind='account'
				AND actor_id=$1 AND route=$2 AND idempotency_key=$3 AND status='pending'`,
				in.OwnerAccountID, createInstallationRoute, in.IdempotencyKey); deleteErr != nil {
				return Installation{}, false, deleteErr
			}
			if auditErr := writeQuotaDenial(ctx, tx, in, windowStart); auditErr != nil {
				return Installation{}, false, auditErr
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return Installation{}, false, fmt.Errorf("commit quota denial audit: %w", commitErr)
			}
			return Installation{}, false, &RateLimitError{RetryAfter: windowStart.Add(time.Minute).Sub(now)}
		}
		return Installation{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Installation{}, false, fmt.Errorf("commit installation quota admission: %w", err)
	}
	return Installation{}, false, nil
}

func installationOperationFailure(ctx context.Context, tx pgx.Tx, in CreateInstallationInput) error {
	var reason string
	err := tx.QueryRow(ctx, `SELECT reason_code FROM registry_audit WHERE actor_kind='account' AND actor_id=$1
		AND application_id=$2 AND operation_key=$3 AND action='register_installation'
		ORDER BY created_at DESC LIMIT 1`, in.OwnerAccountID, in.ApplicationID, in.IdempotencyKey).Scan(&reason)
	if err != nil {
		return ErrRegistryUnavailable
	}
	if reason == "bot_authority_denied" {
		return ErrBotAuthorityDenied
	}
	if reason == "bot_authority_unavailable" {
		return ErrBotAuthorityUnavailable
	}
	if reason == "app_suspended" {
		return ErrApplicationSuspended
	}
	if reason == "callback_destination_rejected" {
		return ErrUnsafeCallbackURL
	}
	if reason == "owner_mismatch" || reason == "environment_scope_denied" || reason == "application_state_denied" {
		return ErrInstallationConflict
	}
	return ErrRegistryUnavailable
}

func markInstallationOperationFailed(ctx context.Context, tx pgx.Tx, in CreateInstallationInput) error {
	command, err := tx.Exec(ctx, `UPDATE registry_operations SET status='failed', updated_at=now()
		WHERE actor_kind='account' AND actor_id=$1 AND route=$2 AND idempotency_key=$3 AND status='pending'`,
		in.OwnerAccountID, createInstallationRoute, in.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("mark installation operation failed: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrRegistryUnavailable
	}
	return nil
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
	err := tx.QueryRow(ctx, `SELECT id, application_id, environment_id, bot_id, callback_url, status, created_at
		FROM installations WHERE id=$1`, id).Scan(
		&installation.ID, &installation.ApplicationID, &installation.EnvironmentID, &installation.BotID,
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
