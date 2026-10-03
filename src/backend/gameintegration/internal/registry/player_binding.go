package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidPlayerBinding      = errors.New("invalid player binding")
	ErrPlayerBindingNotFound     = errors.New("player binding not found")
	ErrPlayerBindingConflict     = errors.New("player binding authority conflict")
	ErrPlayerBindingDrainPending = errors.New("player binding revoke drain pending")
)

const (
	playerBindingDrainTimeout = 4250 * time.Millisecond
	playerBindingPollInterval = 25 * time.Millisecond
)

var bindingProviderPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
var bindingSubjectDigestPattern = regexp.MustCompile(`^hmac-sha256-v1:[A-Za-z0-9_-]{1,32}:[0-9a-f]{64}$`)

// CreatePlayerBindingInput is a GIS service seam for the future T13/T17
// binding writer. The writer must prove the external identity and selected
// profile before calling this method; Auth consent is not such proof.
type CreatePlayerBindingInput struct {
	BindingID             uuid.UUID
	ApplicationID         uuid.UUID
	EnvironmentID         uuid.UUID
	Provider              string
	ProviderSubjectDigest string
	AccountID             uuid.UUID
	ActorID               uuid.UUID
	ProfileID             uuid.UUID
	DeviceID              uuid.UUID
}

// PlayerBindingAuthority contains GIS-owned facts. Identity claims stay in
// GIS storage and are deliberately absent from the authority read response.
type PlayerBindingAuthority struct {
	ApplicationID    uuid.UUID       `json:"application_id"`
	EnvironmentID    uuid.UUID       `json:"environment_id"`
	BindingID        uuid.UUID       `json:"binding_id"`
	Status           string          `json:"status"`
	BindingRevision  int64           `json:"binding_revision"`
	CharacterContext json.RawMessage `json:"character_context"`
}

type playerBindingRecord struct {
	PlayerBindingAuthority
	Provider              string
	ProviderSubjectDigest string
	AccountID             uuid.UUID
	ActorID               uuid.UUID
	ProfileID             uuid.UUID
	DeviceID              uuid.UUID
}

// CreatePlayerBinding persists an already-proven binding. Its ID is supplied
// by the GIS lifecycle writer and remains stable across exact retries.
func (s *Store) CreatePlayerBinding(ctx context.Context, in CreatePlayerBindingInput) (PlayerBindingAuthority, error) {
	if in.BindingID == uuid.Nil || in.ApplicationID == uuid.Nil || in.EnvironmentID == uuid.Nil ||
		!bindingProviderPattern.MatchString(in.Provider) || !bindingSubjectDigestPattern.MatchString(in.ProviderSubjectDigest) ||
		in.AccountID == uuid.Nil || in.ActorID == uuid.Nil || in.ProfileID == uuid.Nil || in.DeviceID == uuid.Nil {
		return PlayerBindingAuthority{}, ErrInvalidPlayerBinding
	}
	if s == nil || s.Pool == nil {
		return PlayerBindingAuthority{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PlayerBindingAuthority{}, fmt.Errorf("begin player binding: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var appStatus, environmentStatus string
	err = tx.QueryRow(ctx, `SELECT a.status, e.status FROM applications a
		JOIN environments e ON e.application_id=a.id WHERE a.id=$1 AND e.id=$2 FOR SHARE OF a,e`,
		in.ApplicationID, in.EnvironmentID).Scan(&appStatus, &environmentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlayerBindingAuthority{}, ErrInvalidPlayerBinding
		}
		return PlayerBindingAuthority{}, fmt.Errorf("read player binding app scope: %w", err)
	}
	if (appStatus != "sandbox" && appStatus != "active") || environmentStatus != "active" {
		return PlayerBindingAuthority{}, ErrInvalidPlayerBinding
	}

	_, err = tx.Exec(ctx, `INSERT INTO player_bindings
		(binding_id, application_id, environment_id, provider, provider_subject_digest, account_id, actor_id, profile_id, device_id,
		 status, authority_revision, permitted_character_context)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'active',1,'[]'::jsonb) ON CONFLICT (binding_id) DO NOTHING`,
		in.BindingID, in.ApplicationID, in.EnvironmentID, in.Provider, in.ProviderSubjectDigest,
		in.AccountID, in.ActorID, in.ProfileID, in.DeviceID)
	if err != nil {
		return PlayerBindingAuthority{}, fmt.Errorf("insert player binding: %w", err)
	}
	row, err := loadPlayerBinding(ctx, tx, in.BindingID, true)
	if err != nil {
		return PlayerBindingAuthority{}, err
	}
	if row.ApplicationID != in.ApplicationID || row.EnvironmentID != in.EnvironmentID || row.AccountID != in.AccountID ||
		row.Provider != in.Provider || row.ProviderSubjectDigest != in.ProviderSubjectDigest ||
		row.ActorID != in.ActorID || row.ProfileID != in.ProfileID || row.DeviceID != in.DeviceID {
		return PlayerBindingAuthority{}, ErrPlayerBindingConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return PlayerBindingAuthority{}, fmt.Errorf("commit player binding: %w", err)
	}
	return row.PlayerBindingAuthority, nil
}

// LoadPlayerBindingAuthority performs an execution-time GIS-owned read.
func (s *Store) LoadPlayerBindingAuthority(ctx context.Context, id uuid.UUID) (PlayerBindingAuthority, error) {
	if id == uuid.Nil {
		return PlayerBindingAuthority{}, ErrPlayerBindingNotFound
	}
	if s == nil || s.Pool == nil {
		return PlayerBindingAuthority{}, ErrRegistryUnavailable
	}
	row, err := loadPlayerBinding(ctx, s.Pool, id, false)
	if err != nil {
		return PlayerBindingAuthority{}, err
	}
	return row.PlayerBindingAuthority, nil
}

// RevokePlayerBindingForOwner checks the Auth-selected account before starting
// the GIS permit drain. Ownership is immutable after creation.
func (s *Store) RevokePlayerBindingForOwner(ctx context.Context, id, accountID uuid.UUID,
	expectedRevision int64, operationID uuid.UUID) (PlayerBindingAuthority, error) {
	if id == uuid.Nil || accountID == uuid.Nil {
		return PlayerBindingAuthority{}, ErrInvalidPlayerBinding
	}
	if s == nil || s.Pool == nil {
		return PlayerBindingAuthority{}, ErrRegistryUnavailable
	}
	row, err := loadPlayerBinding(ctx, s.Pool, id, false)
	if err != nil {
		return PlayerBindingAuthority{}, err
	}
	if row.AccountID != accountID {
		return PlayerBindingAuthority{}, ErrPlayerBindingNotFound
	}
	return s.RevokePlayerBinding(ctx, id, expectedRevision, operationID)
}

func (s *Store) LoadPlayerBindingOperationID(ctx context.Context, bindingID uuid.UUID) (uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return uuid.Nil, ErrRegistryUnavailable
	}
	var operationID uuid.UUID
	err := s.Pool.QueryRow(ctx, `SELECT operation_id FROM player_binding_exchange_operations WHERE binding_id=$1`,
		bindingID).Scan(&operationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrPlayerBindingNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("read player binding operation: %w", err)
	}
	if operationID == uuid.Nil {
		return uuid.Nil, ErrRegistryUnavailable
	}
	return operationID, nil
}

// RevokePlayerBinding serializes revocation with every GIS binding read/write.
// The operation ID makes a committed revoke safe to retry after response loss.
func (s *Store) RevokePlayerBinding(ctx context.Context, id uuid.UUID, expectedRevision int64, operationID uuid.UUID) (PlayerBindingAuthority, error) {
	if id == uuid.Nil || expectedRevision <= 0 || operationID == uuid.Nil {
		return PlayerBindingAuthority{}, ErrInvalidPlayerBinding
	}
	if s == nil || s.Pool == nil {
		return PlayerBindingAuthority{}, ErrRegistryUnavailable
	}
	boundedContext, cancel := context.WithTimeout(ctx, playerBindingDrainTimeout)
	defer cancel()
	ctx = boundedContext
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PlayerBindingAuthority{}, fmt.Errorf("begin player binding revoke: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row, err := loadPlayerBinding(ctx, tx, id, true)
	if err != nil {
		return PlayerBindingAuthority{}, err
	}
	var lastOperation *uuid.UUID
	var lastExpected *int64
	err = tx.QueryRow(ctx, `SELECT last_revocation_id, last_revocation_expected_revision FROM player_bindings
		WHERE binding_id=$1`, id).Scan(&lastOperation, &lastExpected)
	if err != nil {
		return PlayerBindingAuthority{}, fmt.Errorf("read player binding revoke replay: %w", err)
	}
	if row.Status == "revoked" {
		if lastOperation != nil && *lastOperation == operationID && lastExpected != nil && *lastExpected == expectedRevision {
			if err := tx.Commit(ctx); err != nil {
				return PlayerBindingAuthority{}, fmt.Errorf("commit player binding revoke replay: %w", err)
			}
			return row.PlayerBindingAuthority, nil
		}
		return PlayerBindingAuthority{}, ErrPlayerBindingConflict
	}
	if row.Status == "active" {
		if row.BindingRevision != expectedRevision || row.BindingRevision == int64(^uint64(0)>>1) {
			return PlayerBindingAuthority{}, ErrPlayerBindingConflict
		}
		row.Status = "revoking"
		row.BindingRevision++
		command, updateErr := tx.Exec(ctx, `UPDATE player_bindings SET status='revoking', authority_revision=$2,
			last_revocation_id=$3, last_revocation_expected_revision=$4, updated_at=now()
			WHERE binding_id=$1 AND status='active' AND authority_revision=$4`,
			id, row.BindingRevision, operationID, expectedRevision)
		if updateErr != nil {
			return PlayerBindingAuthority{}, fmt.Errorf("begin player binding revoke: %w", updateErr)
		}
		if command.RowsAffected() != 1 {
			return PlayerBindingAuthority{}, ErrPlayerBindingConflict
		}
	} else if row.Status != "revoking" || lastOperation == nil || *lastOperation != operationID ||
		lastExpected == nil || *lastExpected != expectedRevision || row.BindingRevision != expectedRevision+1 {
		return PlayerBindingAuthority{}, ErrPlayerBindingConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return PlayerBindingAuthority{}, fmt.Errorf("commit player binding revoke intent: %w", err)
	}
	return s.drainPlayerBindingRevocation(ctx, id, expectedRevision, operationID)
}

func (s *Store) drainPlayerBindingRevocation(ctx context.Context, id uuid.UUID, expectedRevision int64, operationID uuid.UUID) (PlayerBindingAuthority, error) {
	deadline := time.Now().Add(playerBindingDrainTimeout)
	for {
		if err := ctx.Err(); err != nil {
			return PlayerBindingAuthority{}, fmt.Errorf("player binding revoke drain: %w", ErrPlayerBindingDrainPending)
		}
		tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return PlayerBindingAuthority{}, fmt.Errorf("begin player binding drain check: %w", err)
		}
		row, err := loadPlayerBinding(ctx, tx, id, true)
		if err != nil {
			_ = tx.Rollback(ctx)
			return PlayerBindingAuthority{}, err
		}
		if row.Status == "revoked" {
			if err := tx.Commit(ctx); err != nil {
				return PlayerBindingAuthority{}, fmt.Errorf("commit player binding revoke read: %w", err)
			}
			return row.PlayerBindingAuthority, nil
		}
		if row.Status != "revoking" {
			_ = tx.Rollback(ctx)
			return PlayerBindingAuthority{}, ErrPlayerBindingConflict
		}
		_, err = tx.Exec(ctx, `UPDATE player_binding_execution_permits SET status='expired', completion_at=clock_timestamp()
			WHERE binding_id=$1 AND status='issued' AND expires_at + interval '500 milliseconds' <= clock_timestamp()`, id)
		if err != nil {
			_ = tx.Rollback(ctx)
			return PlayerBindingAuthority{}, fmt.Errorf("expire drained execution permits: %w", err)
		}
		var outstanding bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM player_binding_execution_permits
			WHERE binding_id=$1 AND status='issued')`, id).Scan(&outstanding)
		if err != nil {
			_ = tx.Rollback(ctx)
			return PlayerBindingAuthority{}, fmt.Errorf("check outstanding execution permits: %w", err)
		}
		if !outstanding {
			command, updateErr := tx.Exec(ctx, `UPDATE player_bindings SET status='revoked', updated_at=clock_timestamp()
				WHERE binding_id=$1 AND status='revoking' AND last_revocation_id=$2
				AND last_revocation_expected_revision=$3`, id, operationID, expectedRevision)
			if updateErr != nil {
				_ = tx.Rollback(ctx)
				return PlayerBindingAuthority{}, fmt.Errorf("commit drained player binding revoke: %w", updateErr)
			}
			if command.RowsAffected() != 1 {
				_ = tx.Rollback(ctx)
				return PlayerBindingAuthority{}, ErrPlayerBindingConflict
			}
			row.Status = "revoked"
			if err := tx.Commit(ctx); err != nil {
				return PlayerBindingAuthority{}, fmt.Errorf("commit drained player binding revoke: %w", err)
			}
			return row.PlayerBindingAuthority, nil
		}
		if err := tx.Commit(ctx); err != nil {
			return PlayerBindingAuthority{}, fmt.Errorf("commit player binding drain observation: %w", err)
		}
		if time.Now().After(deadline) {
			return PlayerBindingAuthority{}, ErrPlayerBindingDrainPending
		}
		timer := time.NewTimer(playerBindingPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return PlayerBindingAuthority{}, ErrPlayerBindingDrainPending
		case <-timer.C:
		}
	}
}

type playerBindingQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadPlayerBinding(ctx context.Context, q playerBindingQuerier, id uuid.UUID, forUpdate bool) (playerBindingRecord, error) {
	query := `SELECT binding_id, application_id, environment_id, provider, provider_subject_digest,
		account_id, actor_id, profile_id, device_id,
		status, authority_revision, permitted_character_context FROM player_bindings WHERE binding_id=$1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var row playerBindingRecord
	var contextBytes []byte
	err := q.QueryRow(ctx, query, id).Scan(&row.BindingID, &row.ApplicationID, &row.EnvironmentID,
		&row.Provider, &row.ProviderSubjectDigest, &row.AccountID,
		&row.ActorID, &row.ProfileID, &row.DeviceID, &row.Status, &row.BindingRevision, &contextBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return playerBindingRecord{}, ErrPlayerBindingNotFound
	}
	if err != nil {
		return playerBindingRecord{}, fmt.Errorf("read player binding authority: %w", err)
	}
	if row.BindingID != id || row.BindingRevision <= 0 ||
		(row.Status != "pending" && row.Status != "active" && row.Status != "revoking" && row.Status != "revoked") || !json.Valid(contextBytes) {
		return playerBindingRecord{}, ErrRegistryUnavailable
	}
	row.CharacterContext = append(json.RawMessage(nil), contextBytes...)
	return row, nil
}

var _ playerBindingQuerier = (*pgxpool.Pool)(nil)
var _ playerBindingQuerier = (pgx.Tx)(nil)
