package registry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	gameDeviceAssertionLifetime = 4 * time.Second
	gameMessagePermitLifetime   = 3750 * time.Millisecond
	gamePermitCompletionMargin  = 250 * time.Millisecond
)

var ErrPlayerBindingPermitDenied = errors.New("player binding execution permit denied")

// GameDeviceAuthorityClaims are extracted from the exact Auth assertion after
// the request's versioned Auth workload proof has been verified. The GIS
// workload proof authenticates Auth as the producer; this record is never
// accepted from an unauthenticated caller.
type GameDeviceAuthorityClaims struct {
	Issuer            string
	Audience          string
	Version           int64
	ApplicationID     uuid.UUID
	EnvironmentID     uuid.UUID
	AccountID         uuid.UUID
	ActorID           uuid.UUID
	BindingID         uuid.UUID
	DeviceID          uuid.UUID
	KeyID             uuid.UUID
	DeviceGeneration  int64
	AuthorityRevision int64
	AssertionID       uuid.UUID
	AssertionSHA256   [32]byte
	Status            string
	NotAfter          time.Time
	IssuedAt          time.Time
	ExpiresAt         time.Time
}

type PlayerBindingExecutionPermit struct {
	PermitID        uuid.UUID `json:"permit_id"`
	BindingID       uuid.UUID `json:"binding_id"`
	ApplicationID   uuid.UUID `json:"application_id"`
	EnvironmentID   uuid.UUID `json:"environment_id"`
	BindingRevision int64     `json:"binding_revision"`
	AssertionID     uuid.UUID `json:"assertion_jti"`
	OperationID     uuid.UUID `json:"operation_id"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type PlayerBindingPermitCompletion struct {
	PermitID    uuid.UUID `json:"permit_id"`
	OperationID uuid.UUID `json:"operation_id"`
	Outcome     string    `json:"outcome"`
	Status      string    `json:"status"`
}

// IssuePlayerBindingExecutionPermit persists a short execution lease under
// the binding row lock. Exact operation/assertion retries return the original
// permit and never extend its deadline.
func (s *Store) IssuePlayerBindingExecutionPermit(ctx context.Context, bindingID, operationID uuid.UUID,
	claims GameDeviceAuthorityClaims) (PlayerBindingExecutionPermit, error) {
	if bindingID == uuid.Nil || operationID == uuid.Nil || claims.BindingID != bindingID ||
		claims.ApplicationID == uuid.Nil || claims.EnvironmentID == uuid.Nil || claims.AccountID == uuid.Nil ||
		claims.ActorID == uuid.Nil || claims.DeviceID == uuid.Nil || claims.KeyID == uuid.Nil || claims.AssertionID == uuid.Nil ||
		claims.AssertionSHA256 == [32]byte{} ||
		claims.AuthorityRevision <= 0 || claims.DeviceGeneration <= 0 || claims.Version != 1 ||
		claims.Issuer != "auth" || claims.Audience != "voice.game-message" || claims.Status != "active" ||
		claims.IssuedAt.IsZero() || claims.ExpiresAt.IsZero() || claims.NotAfter.IsZero() ||
		!claims.ExpiresAt.After(claims.IssuedAt) || claims.ExpiresAt.After(claims.NotAfter) ||
		claims.ExpiresAt.After(claims.IssuedAt.Add(gameDeviceAssertionLifetime)) {
		return PlayerBindingExecutionPermit{}, ErrPlayerBindingPermitDenied
	}
	if s == nil || s.Pool == nil {
		return PlayerBindingExecutionPermit{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PlayerBindingExecutionPermit{}, fmt.Errorf("begin execution permit: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row, err := loadPlayerBinding(ctx, tx, bindingID, true)
	if err != nil {
		return PlayerBindingExecutionPermit{}, err
	}
	var saved PlayerBindingExecutionPermit
	var savedAssertionHash []byte
	err = tx.QueryRow(ctx, `SELECT permit_id, binding_id, application_id, environment_id, binding_revision,
		assertion_jti, operation_id, expires_at, assertion_sha256 FROM player_binding_execution_permits
		WHERE binding_id=$1 AND operation_id=$2`, bindingID, operationID).Scan(
		&saved.PermitID, &saved.BindingID, &saved.ApplicationID, &saved.EnvironmentID, &saved.BindingRevision,
		&saved.AssertionID, &saved.OperationID, &saved.ExpiresAt, &savedAssertionHash)
	if err == nil {
		if saved.AssertionID != claims.AssertionID || saved.ApplicationID != claims.ApplicationID ||
			saved.EnvironmentID != claims.EnvironmentID || !bytes.Equal(savedAssertionHash, claims.AssertionSHA256[:]) {
			return PlayerBindingExecutionPermit{}, ErrPlayerBindingConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return PlayerBindingExecutionPermit{}, fmt.Errorf("commit execution permit retry: %w", err)
		}
		return saved, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return PlayerBindingExecutionPermit{}, fmt.Errorf("read execution permit retry: %w", err)
	}
	if row.Status != "active" || row.ApplicationID != claims.ApplicationID || row.EnvironmentID != claims.EnvironmentID ||
		row.AccountID != claims.AccountID || row.ActorID != claims.ActorID || row.ProfileID == uuid.Nil ||
		row.DeviceID != claims.DeviceID || row.BindingID != claims.BindingID {
		return PlayerBindingExecutionPermit{}, ErrPlayerBindingPermitDenied
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return PlayerBindingExecutionPermit{}, fmt.Errorf("read execution permit clock: %w", err)
	}
	if !claims.ExpiresAt.After(now) || claims.IssuedAt.After(now.Add(250*time.Millisecond)) {
		return PlayerBindingExecutionPermit{}, ErrPlayerBindingPermitDenied
	}
	deadline := now.Add(gameMessagePermitLifetime)
	if claims.ExpiresAt.Before(deadline) {
		deadline = claims.ExpiresAt
	}
	permit := PlayerBindingExecutionPermit{PermitID: uuid.New(), BindingID: bindingID,
		ApplicationID: row.ApplicationID, EnvironmentID: row.EnvironmentID,
		BindingRevision: row.BindingRevision, AssertionID: claims.AssertionID, OperationID: operationID,
		ExpiresAt: deadline}
	_, err = tx.Exec(ctx, `INSERT INTO player_binding_execution_permits
		(permit_id, binding_id, operation_id, application_id, environment_id, binding_revision,
		 assertion_jti, assertion_sha256, expires_at, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'issued')`, permit.PermitID, permit.BindingID, permit.OperationID,
		permit.ApplicationID, permit.EnvironmentID, permit.BindingRevision, permit.AssertionID, claims.AssertionSHA256[:], permit.ExpiresAt)
	if err != nil {
		return PlayerBindingExecutionPermit{}, fmt.Errorf("persist execution permit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PlayerBindingExecutionPermit{}, fmt.Errorf("commit execution permit: %w", err)
	}
	return permit, nil
}

// CompletePlayerBindingExecutionPermit acknowledges the Messaging transaction.
// A terminal receipt is immutable and identical retries return the saved result.
func (s *Store) CompletePlayerBindingExecutionPermit(ctx context.Context, permitID, operationID uuid.UUID,
	outcome string) (PlayerBindingPermitCompletion, error) {
	if permitID == uuid.Nil || operationID == uuid.Nil || (outcome != "committed" && outcome != "aborted") {
		return PlayerBindingPermitCompletion{}, ErrInvalidPlayerBinding
	}
	if s == nil || s.Pool == nil {
		return PlayerBindingPermitCompletion{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PlayerBindingPermitCompletion{}, fmt.Errorf("begin execution permit completion: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var result PlayerBindingPermitCompletion
	var current string
	err = tx.QueryRow(ctx, `SELECT status FROM player_binding_execution_permits
		WHERE permit_id=$1 AND operation_id=$2 FOR UPDATE`, permitID, operationID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayerBindingPermitCompletion{}, ErrPlayerBindingNotFound
	}
	if err != nil {
		return PlayerBindingPermitCompletion{}, fmt.Errorf("read execution permit completion: %w", err)
	}
	if current == "committed" || current == "aborted" {
		if current != outcome {
			return PlayerBindingPermitCompletion{}, ErrPlayerBindingConflict
		}
		result = PlayerBindingPermitCompletion{PermitID: permitID, OperationID: operationID, Outcome: outcome, Status: "completed"}
		if err := tx.Commit(ctx); err != nil {
			return PlayerBindingPermitCompletion{}, fmt.Errorf("commit execution completion replay: %w", err)
		}
		return result, nil
	}
	if current != "issued" {
		return PlayerBindingPermitCompletion{}, ErrPlayerBindingConflict
	}
	command, err := tx.Exec(ctx, `UPDATE player_binding_execution_permits SET status=$3, completion_at=clock_timestamp()
		WHERE permit_id=$1 AND operation_id=$2 AND status='issued'
		AND clock_timestamp() <= expires_at + interval '250 milliseconds'`, permitID, operationID, outcome)
	if err != nil {
		return PlayerBindingPermitCompletion{}, fmt.Errorf("complete execution permit: %w", err)
	}
	if command.RowsAffected() != 1 {
		return PlayerBindingPermitCompletion{}, ErrPlayerBindingConflict
	}
	result = PlayerBindingPermitCompletion{PermitID: permitID, OperationID: operationID, Outcome: outcome, Status: "completed"}
	if err := tx.Commit(ctx); err != nil {
		return PlayerBindingPermitCompletion{}, fmt.Errorf("commit execution completion: %w", err)
	}
	return result, nil
}

func AssertionFingerprint(assertion []byte) [32]byte { return sha256.Sum256(assertion) }
