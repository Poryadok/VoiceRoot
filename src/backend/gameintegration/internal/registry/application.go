package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidApplication  = errors.New("invalid application request")
	ErrIdempotencyConflict = errors.New("idempotency key has a different request")
	ErrRegistryUnavailable = errors.New("registry unavailable")
)

const createApplicationRoute = "applications.create"

type Store struct {
	Pool *pgxpool.Pool
}

type CreateApplicationInput struct {
	OwnerAccountID uuid.UUID
	Name           string
	GameID         *uuid.UUID
	IdempotencyKey string
}

type Application struct {
	ID             uuid.UUID
	OwnerAccountID uuid.UUID
	Name           string
	GameID         *uuid.UUID
	Status         string
	Revision       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func normalizeCreateApplication(in CreateApplicationInput) (CreateApplicationInput, [32]byte, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.OwnerAccountID == uuid.Nil || len(in.Name) == 0 || len([]rune(in.Name)) > 128 ||
		len(in.IdempotencyKey) == 0 || len(in.IdempotencyKey) > 128 ||
		(in.GameID != nil && *in.GameID == uuid.Nil) {
		return CreateApplicationInput{}, [32]byte{}, ErrInvalidApplication
	}
	canonical := struct {
		Name   string  `json:"name"`
		GameID *string `json:"game_id"`
	}{Name: in.Name}
	if in.GameID != nil {
		id := in.GameID.String()
		canonical.GameID = &id
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return CreateApplicationInput{}, [32]byte{}, fmt.Errorf("canonical application request: %w", err)
	}
	return in, sha256.Sum256(encoded), nil
}

// CreateApplication durably binds one owner-scoped request to one application.
// The operation row and application commit together, so a lost HTTP response
// can be retried without creating a second application.
func (s *Store) CreateApplication(ctx context.Context, input CreateApplicationInput) (Application, error) {
	in, hash, err := normalizeCreateApplication(input)
	if err != nil {
		return Application{}, err
	}
	if s == nil || s.Pool == nil {
		return Application{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Application{}, fmt.Errorf("begin registry operation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	command, err := tx.Exec(ctx, `
		INSERT INTO registry_operations
		(actor_kind, actor_id, route, idempotency_key, request_hash, status)
		VALUES ('account', $1, $2, $3, $4, 'pending')
		ON CONFLICT DO NOTHING`,
		in.OwnerAccountID, createApplicationRoute, in.IdempotencyKey, hash[:])
	if err != nil {
		return Application{}, fmt.Errorf("insert registry operation: %w", err)
	}
	if command.RowsAffected() == 0 {
		var savedHash []byte
		var resultID pgtype.UUID
		var status string
		err = tx.QueryRow(ctx, `
			SELECT request_hash, result_id, status FROM registry_operations
			WHERE actor_kind='account' AND actor_id=$1 AND route=$2 AND idempotency_key=$3`,
			in.OwnerAccountID, createApplicationRoute, in.IdempotencyKey).Scan(&savedHash, &resultID, &status)
		if err != nil {
			return Application{}, fmt.Errorf("read registry operation: %w", err)
		}
		if string(savedHash) != string(hash[:]) {
			return Application{}, ErrIdempotencyConflict
		}
		if status != "succeeded" || !resultID.Valid {
			return Application{}, ErrRegistryUnavailable
		}
		application, err := getApplication(ctx, tx, uuid.UUID(resultID.Bytes))
		if err != nil {
			return Application{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Application{}, fmt.Errorf("commit registry read: %w", err)
		}
		return application, nil
	}

	id := uuid.New()
	var gameID any
	if in.GameID != nil {
		gameID = *in.GameID
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO applications (id, owner_account_id, name, game_id, status)
		VALUES ($1, $2, $3, $4, 'draft')`, id, in.OwnerAccountID, in.Name, gameID)
	if err != nil {
		return Application{}, fmt.Errorf("create application: %w", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE registry_operations SET result_id=$1, status='succeeded', updated_at=now()
		WHERE actor_kind='account' AND actor_id=$2 AND route=$3 AND idempotency_key=$4`,
		id, in.OwnerAccountID, createApplicationRoute, in.IdempotencyKey)
	if err != nil {
		return Application{}, fmt.Errorf("complete registry operation: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO registry_audit
		(id, actor_kind, actor_id, application_id, action, new_status, operation_key)
		VALUES ($1, 'account', $2, $3, 'create_application', 'draft', $4)`,
		uuid.New(), in.OwnerAccountID, id, in.IdempotencyKey)
	if err != nil {
		return Application{}, fmt.Errorf("audit registry operation: %w", err)
	}
	application, err := getApplication(ctx, tx, id)
	if err != nil {
		return Application{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Application{}, fmt.Errorf("commit registry operation: %w", err)
	}
	return application, nil
}

func getApplication(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Application, error) {
	var application Application
	var gameID pgtype.UUID
	err := tx.QueryRow(ctx, `
		SELECT id, owner_account_id, name, game_id, status, revision, created_at, updated_at
		FROM applications WHERE id=$1`, id).Scan(
		&application.ID, &application.OwnerAccountID, &application.Name, &gameID,
		&application.Status, &application.Revision, &application.CreatedAt, &application.UpdatedAt)
	if err != nil {
		return Application{}, fmt.Errorf("read application: %w", err)
	}
	if gameID.Valid {
		parsed := uuid.UUID(gameID.Bytes)
		application.GameID = &parsed
	}
	return application, nil
}
