package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Onboarding step_id tokens (docs/features/onboarding.md — stable client↔server IDs).
const (
	OnboardingStepSaveAccount  = "save_account"
	OnboardingStepChatsNav     = "chats_nav"
	OnboardingStepSpaces       = "spaces"
	OnboardingStepMatchmaking  = "matchmaking"
	OnboardingStepWrapUp       = "wrap_up"
	OnboardingStepDismiss      = "dismiss" // "Пропустить" / skip entire tutorial
	onboardingMaxStepIDRunes   = 64
)

var canonicalOnboardingSteps = []string{
	OnboardingStepSaveAccount,
	OnboardingStepChatsNav,
	OnboardingStepSpaces,
	OnboardingStepMatchmaking,
	OnboardingStepWrapUp,
}

// OnboardingStateRow mirrors user_db.onboarding_state (docs/microservices/user-service.md).
type OnboardingStateRow struct {
	ProfileID      uuid.UUID
	CompletedSteps []string
	Completed      bool
	CompletedAt    *time.Time
}

// ErrInvalidOnboardingStep is returned when step_id is empty or too long.
var ErrInvalidOnboardingStep = errors.New("invalid onboarding step_id")

// resolveProfileID picks the active profile: explicit profileID if owned, otherwise primary for account.
func resolveProfileID(ctx context.Context, pool *pgxpool.Pool, accountID uuid.UUID, headerProfileID *uuid.UUID) (uuid.UUID, error) {
	if headerProfileID != nil && *headerProfileID != uuid.Nil {
		var ok bool
		err := pool.QueryRow(ctx, `SELECT true FROM profiles WHERE id = $1 AND account_id = $2`, *headerProfileID, accountID).Scan(&ok)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, pgx.ErrNoRows
		}
		if err != nil {
			return uuid.Nil, err
		}
		return *headerProfileID, nil
	}
	var id uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM profiles WHERE account_id = $1 AND is_primary = true LIMIT 1`, accountID).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// GetOrCreateOnboardingState ensures a row exists for profile_id and returns it.
func (s *ProfileStore) GetOrCreateOnboardingState(ctx context.Context, accountID uuid.UUID, headerProfileID *uuid.UUID) (*OnboardingStateRow, error) {
	profileID, err := resolveProfileID(ctx, s.pool, accountID, headerProfileID)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO onboarding_state (profile_id) VALUES ($1)
		ON CONFLICT (profile_id) DO NOTHING`,
		profileID,
	)
	if err != nil {
		return nil, err
	}

	row, err := scanOnboardingRow(ctx, tx, profileID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return row, nil
}

func scanOnboardingRow(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, profileID uuid.UUID) (*OnboardingStateRow, error) {
	var raw []byte
	var r OnboardingStateRow
	err := q.QueryRow(ctx, `
		SELECT profile_id, completed_steps, completed, completed_at
		FROM onboarding_state WHERE profile_id = $1`,
		profileID,
	).Scan(&r.ProfileID, &raw, &r.Completed, &r.CompletedAt)
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &r.CompletedSteps); err != nil {
			return nil, err
		}
	}
	if r.CompletedSteps == nil {
		r.CompletedSteps = []string{}
	}
	return &r, nil
}

func stepSetContains(steps []string, want string) bool {
	for _, s := range steps {
		if s == want {
			return true
		}
	}
	return false
}

func allCanonicalPresent(steps []string) bool {
next:
	for _, c := range canonicalOnboardingSteps {
		for _, s := range steps {
			if s == c {
				continue next
			}
		}
		return false
	}
	return true
}

func shouldMarkCompleted(steps []string, newStep string) bool {
	if newStep == OnboardingStepDismiss || newStep == OnboardingStepWrapUp {
		return true
	}
	tmp := append(append([]string(nil), steps...), newStep)
	return allCanonicalPresent(tmp)
}

// CompleteOnboardingStep appends step_id (idempotent), updates completed flags per product rules.
func (s *ProfileStore) CompleteOnboardingStep(ctx context.Context, accountID uuid.UUID, headerProfileID *uuid.UUID, stepID string) (*OnboardingStateRow, error) {
	stepID = strings.TrimSpace(stepID)
	if stepID == "" || len([]rune(stepID)) > onboardingMaxStepIDRunes {
		return nil, ErrInvalidOnboardingStep
	}

	profileID, err := resolveProfileID(ctx, s.pool, accountID, headerProfileID)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO onboarding_state (profile_id) VALUES ($1)
		ON CONFLICT (profile_id) DO NOTHING`,
		profileID,
	)
	if err != nil {
		return nil, err
	}

	row, err := scanOnboardingRow(ctx, tx, profileID)
	if err != nil {
		return nil, err
	}
	if row.Completed {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return row, nil
	}
	if stepSetContains(row.CompletedSteps, stepID) {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return row, nil
	}

	nextSteps := append(append([]string(nil), row.CompletedSteps...), stepID)
	completed := shouldMarkCompleted(row.CompletedSteps, stepID)
	var completedAt *time.Time
	if completed {
		now := time.Now().UTC()
		completedAt = &now
	}

	raw, err := json.Marshal(nextSteps)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE onboarding_state SET
			completed_steps = $1::jsonb,
			completed = $2,
			completed_at = COALESCE($3, completed_at),
			updated_at = now()
		WHERE profile_id = $4`,
		raw, completed, completedAt, profileID,
	)
	if err != nil {
		return nil, err
	}
	out, err := scanOnboardingRow(ctx, tx, profileID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
