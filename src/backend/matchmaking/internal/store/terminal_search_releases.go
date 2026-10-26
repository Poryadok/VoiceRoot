package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"voice/backend/matchmaking/internal/criteria"
)

// TerminalSearchRelease is immutable routing authorized by final completion.
// It survives parent erasure only while pending; successful projection deletes
// the entire row. Canonical deletion-deadline integration remains an A4 gate.
type TerminalSearchRelease struct {
	SessionID, ProfileID uuid.UUID
	SpaceID              *uuid.UUID
	GameID               uuid.UUID
	Mode, Region         string
	Generation           int64
}

func insertTerminalSearchRelease(ctx context.Context, tx pgx.Tx, sess SearchSession) error {
	crit, err := criteria.Parse(sess.Criteria)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO matchmaking_terminal_search_releases
		(search_session_id,generation,profile_id,space_id,game_id,mode,region)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, sess.ID, sess.RecoveryGeneration, sess.ProfileID, sess.SpaceID, sess.GameID, sess.Mode, crit.Region)
	return err
}

// ApplyPendingTerminalSearchReleases owns a separate pending-only stream that
// legacy recovery consumers cannot discover. Each attempt locks only its outbox
// row: it never acquires parent locks after that lock or requires erased parents.
// Redis acknowledgement precedes SQL deletion; a lost SQL acknowledgement leaves
// the immutable obligation available for exact-owner, generation-fenced retry.
func (s *MatchStore) ApplyPendingTerminalSearchReleases(ctx context.Context, limit int, apply func(context.Context, TerminalSearchRelease) error) error {
	if s == nil || s.Pool == nil || apply == nil {
		return errors.New("terminal search release projector unavailable")
	}
	if limit <= 0 {
		limit = 100
	}
	for range limit {
		processed, err := s.applyNextTerminalSearchRelease(ctx, apply)
		if err != nil || !processed {
			return err
		}
	}
	return nil
}

func (s *MatchStore) applyNextTerminalSearchRelease(ctx context.Context, apply func(context.Context, TerminalSearchRelease) error) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var release TerminalSearchRelease
	err = tx.QueryRow(ctx, `
		SELECT search_session_id,generation,profile_id,space_id,game_id,mode,region
		FROM matchmaking_terminal_search_releases
		ORDER BY created_at,search_session_id,generation
		LIMIT 1 FOR UPDATE SKIP LOCKED
	`).Scan(&release.SessionID, &release.Generation, &release.ProfileID, &release.SpaceID, &release.GameID, &release.Mode, &release.Region)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	projectionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	err = apply(projectionCtx, release)
	cancel()
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM matchmaking_terminal_search_releases WHERE search_session_id=$1 AND generation=$2`, release.SessionID, release.Generation); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
