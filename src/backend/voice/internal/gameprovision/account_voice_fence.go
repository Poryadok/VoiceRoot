package gameprovision

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAccountProfileMappingConflict = errors.New("voice profile account mapping conflicts with User")
var ErrActiveAccountVoiceSession = errors.New("account already has an active Voice session")
var ErrAccountVoiceFenceUnavailable = errors.New("account Voice fence unavailable")

// AccountVoiceFenceStore serializes Voice admission across profiles/devices.
// Account/profile identity must come from the verified Gateway principal and
// the User-owned profile lookup, never from a request body.
type AccountVoiceFenceStore interface {
	Reserve(context.Context, uuid.UUID, uuid.UUID, string) (newReservation bool, err error)
	Commit(context.Context, uuid.UUID, uuid.UUID, string) error
	Release(context.Context, uuid.UUID, uuid.UUID, string) error
	Transfer(context.Context, uuid.UUID, uuid.UUID, string, string) error
}

// UnavailableAccountVoiceFenceStore makes production admission fail closed
// when durable account fencing is not configured or its schema is unavailable.
type UnavailableAccountVoiceFenceStore struct{}

func (UnavailableAccountVoiceFenceStore) Reserve(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	return false, ErrAccountVoiceFenceUnavailable
}

func (UnavailableAccountVoiceFenceStore) Commit(context.Context, uuid.UUID, uuid.UUID, string) error {
	return ErrAccountVoiceFenceUnavailable
}

func (UnavailableAccountVoiceFenceStore) Release(context.Context, uuid.UUID, uuid.UUID, string) error {
	return ErrAccountVoiceFenceUnavailable
}

func (UnavailableAccountVoiceFenceStore) Transfer(context.Context, uuid.UUID, uuid.UUID, string, string) error {
	return ErrAccountVoiceFenceUnavailable
}

type PostgresAccountVoiceFenceStore struct{ pool *pgxpool.Pool }

func NewPostgresAccountVoiceFenceStore(pool *pgxpool.Pool) *PostgresAccountVoiceFenceStore {
	return &PostgresAccountVoiceFenceStore{pool: pool}
}

// Reserve records the trusted mapping and a short-lived reservation in one
// transaction. An exact active tuple is idempotent; a second profile/session
// for the same account conflicts until the owning session releases its fence.
func (s *PostgresAccountVoiceFenceStore) Reserve(ctx context.Context, accountID, profileID uuid.UUID, roomID string) (bool, error) {
	if s == nil || s.pool == nil || accountID == uuid.Nil || profileID == uuid.Nil || roomID == "" {
		return false, ErrAccountVoiceFenceUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin account Voice reservation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `INSERT INTO voice_profile_account_mappings(profile_id,account_id)
VALUES($1,$2) ON CONFLICT(profile_id) DO NOTHING`, profileID, accountID); err != nil {
		return false, fmt.Errorf("persist Voice profile mapping: %w", err)
	}
	var mappedAccount uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT account_id FROM voice_profile_account_mappings WHERE profile_id=$1 FOR UPDATE`, profileID).Scan(&mappedAccount); err != nil {
		return false, fmt.Errorf("read Voice profile mapping: %w", err)
	}
	if mappedAccount != accountID {
		return false, ErrAccountProfileMappingConflict
	}
	var priorProfile uuid.UUID
	var priorRoom, priorState string
	priorErr := tx.QueryRow(ctx, `SELECT profile_id,room_id,state FROM voice_account_voice_fences WHERE account_id=$1 FOR UPDATE`, accountID).Scan(&priorProfile, &priorRoom, &priorState)
	if priorErr != nil && !errors.Is(priorErr, pgx.ErrNoRows) {
		return false, fmt.Errorf("read existing account Voice fence: %w", priorErr)
	}
	tag, err := tx.Exec(ctx, `INSERT INTO voice_account_voice_fences(account_id,profile_id,room_id,state,reservation_expires_at)
VALUES($1,$2,$3,'reserving',clock_timestamp()+interval '30 seconds')
ON CONFLICT(account_id) DO UPDATE SET
 profile_id=EXCLUDED.profile_id,room_id=EXCLUDED.room_id,
 state=CASE WHEN voice_account_voice_fences.state='active' THEN 'active' ELSE 'reserving' END,
 reservation_expires_at=CASE WHEN voice_account_voice_fences.state='active' THEN NULL ELSE clock_timestamp()+interval '30 seconds' END,
 updated_at=clock_timestamp()
WHERE voice_account_voice_fences.admission_operation_id IS NULL AND (
      (voice_account_voice_fences.profile_id=EXCLUDED.profile_id AND voice_account_voice_fences.room_id=EXCLUDED.room_id)
   OR (voice_account_voice_fences.state='reserving' AND voice_account_voice_fences.reservation_expires_at <= clock_timestamp()))`, accountID, profileID, roomID)
	if err != nil {
		return false, fmt.Errorf("reserve account Voice fence: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return false, ErrActiveAccountVoiceSession
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	newReservation := errors.Is(priorErr, pgx.ErrNoRows) || priorState == "reserving" && (priorProfile != profileID || priorRoom != roomID)
	return newReservation, nil
}

func (s *PostgresAccountVoiceFenceStore) Commit(ctx context.Context, accountID, profileID uuid.UUID, roomID string) error {
	if s == nil || s.pool == nil {
		return ErrAccountVoiceFenceUnavailable
	}
	tag, err := s.pool.Exec(ctx, `UPDATE voice_account_voice_fences
SET state='active',reservation_expires_at=NULL,updated_at=clock_timestamp()
WHERE account_id=$1 AND profile_id=$2 AND room_id=$3
  AND admission_operation_id IS NULL AND (state='active' OR reservation_expires_at > clock_timestamp())`, accountID, profileID, roomID)
	if err != nil {
		return fmt.Errorf("commit account Voice fence: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrActiveAccountVoiceSession
	}
	return nil
}

func (s *PostgresAccountVoiceFenceStore) Release(ctx context.Context, accountID, profileID uuid.UUID, roomID string) error {
	if s == nil || s.pool == nil {
		return ErrAccountVoiceFenceUnavailable
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM voice_account_voice_fences WHERE account_id=$1 AND profile_id=$2 AND room_id=$3 AND admission_operation_id IS NULL`, accountID, profileID, roomID)
	if err != nil {
		return fmt.Errorf("release account Voice fence: %w", err)
	}
	return nil
}

// Transfer changes the room bound to an already active account fence. The
// operation is idempotent so a retry after the call-store move can complete it.
func (s *PostgresAccountVoiceFenceStore) Transfer(ctx context.Context, accountID, profileID uuid.UUID, fromRoom, toRoom string) error {
	if s == nil || s.pool == nil || accountID == uuid.Nil || profileID == uuid.Nil || fromRoom == "" || toRoom == "" || fromRoom == toRoom {
		return ErrAccountVoiceFenceUnavailable
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin account Voice transfer: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO voice_profile_account_mappings(profile_id,account_id)
VALUES($1,$2) ON CONFLICT(profile_id) DO NOTHING`, profileID, accountID); err != nil {
		return fmt.Errorf("persist transferred Voice profile mapping: %w", err)
	}
	var mapped uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT account_id FROM voice_profile_account_mappings WHERE profile_id=$1 FOR UPDATE`, profileID).Scan(&mapped); err != nil {
		return fmt.Errorf("read transferred Voice profile mapping: %w", err)
	}
	if mapped != accountID {
		return ErrAccountProfileMappingConflict
	}
	tag, err := tx.Exec(ctx, `UPDATE voice_account_voice_fences SET room_id=$4,updated_at=clock_timestamp()
WHERE account_id=$1 AND profile_id=$2 AND room_id=$3 AND state='active' AND admission_operation_id IS NULL`, accountID, profileID, fromRoom, toRoom)
	if err != nil {
		return fmt.Errorf("transfer account Voice fence: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var currentProfile uuid.UUID
		var currentRoom, state string
		var admissionOperation *uuid.UUID
		readErr := tx.QueryRow(ctx, `SELECT profile_id,room_id,state,admission_operation_id FROM voice_account_voice_fences WHERE account_id=$1 FOR UPDATE`, accountID).Scan(&currentProfile, &currentRoom, &state, &admissionOperation)
		if errors.Is(readErr, pgx.ErrNoRows) {
			if _, err := tx.Exec(ctx, `INSERT INTO voice_account_voice_fences(account_id,profile_id,room_id,state,reservation_expires_at)
VALUES($1,$2,$3,'active',NULL)`, accountID, profileID, toRoom); err != nil {
				return ErrActiveAccountVoiceSession
			}
		} else if readErr != nil {
			return fmt.Errorf("read transferred account Voice fence: %w", readErr)
		} else if currentProfile != profileID || currentRoom != toRoom || state != "active" || admissionOperation != nil {
			return ErrActiveAccountVoiceSession
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit account Voice transfer: %w", err)
	}
	return nil
}

func ApplyAccountVoiceFenceSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return ErrAccountVoiceFenceUnavailable
	}
	var ready bool
	err := pool.QueryRow(ctx, `SELECT to_regclass('voice_profile_account_mappings') IS NOT NULL AND to_regclass('voice_account_voice_fences') IS NOT NULL`).Scan(&ready)
	if err != nil {
		return err
	}
	if !ready {
		return pgx.ErrNoRows
	}
	return nil
}
