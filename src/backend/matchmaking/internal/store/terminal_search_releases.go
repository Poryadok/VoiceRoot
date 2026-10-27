package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/terminalrouting"
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
	RoutingLifecycle     *terminalrouting.Lifecycle
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
// A short SQL claim commits before Redis I/O, allowing independent deadline
// erasure while a projection is paused. ACK deletes only the exact claim.
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
	var fenceID, cycleID *string
	var purgeAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT search_session_id,generation,profile_id,space_id,game_id,mode,region,
		       routing_fence_id,routing_cycle_id,routing_purge_at
		FROM matchmaking_terminal_search_releases
		WHERE projection_claim_until IS NULL OR projection_claim_until <= clock_timestamp()
		ORDER BY created_at,search_session_id,generation
		LIMIT 1 FOR UPDATE SKIP LOCKED
	`).Scan(&release.SessionID, &release.Generation, &release.ProfileID, &release.SpaceID, &release.GameID, &release.Mode, &release.Region,
		&fenceID, &cycleID, &purgeAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if fenceID != nil {
		if cycleID == nil || purgeAt == nil {
			return false, terminalrouting.ErrAuthorityInvalid
		}
		life, err := loadTerminalRoutingCycle(ctx, tx, *fenceID, *cycleID)
		if err != nil {
			return false, err
		}
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return false, err
		}
		if !purgeAt.Equal(life.Schedule.PurgeAt) {
			return false, terminalrouting.ErrAuthorityConflict
		}
		if !now.Before(*purgeAt) || life.RoutingErased {
			if _, err := tx.Exec(ctx, `DELETE FROM matchmaking_terminal_search_releases WHERE search_session_id=$1 AND generation=$2`, release.SessionID, release.Generation); err != nil {
				return false, err
			}
			return true, tx.Commit(ctx)
		}
		release.RoutingLifecycle = &life
	}
	claim := uuid.New()
	if _, err := tx.Exec(ctx, `UPDATE matchmaking_terminal_search_releases
		SET projection_claim=$3,projection_claim_until=clock_timestamp()+interval '3 seconds'
		WHERE search_session_id=$1 AND generation=$2`, release.SessionID, release.Generation, claim); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_, _ = s.Pool.Exec(cleanup, `UPDATE matchmaking_terminal_search_releases
			SET projection_claim=NULL,projection_claim_until=NULL
			WHERE search_session_id=$1 AND generation=$2 AND projection_claim=$3`, release.SessionID, release.Generation, claim)
	}()
	projectionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	err = apply(projectionCtx, release)
	cancel()
	if err != nil {
		return false, err
	}
	if _, err := s.Pool.Exec(ctx, `DELETE FROM matchmaking_terminal_search_releases
		WHERE search_session_id=$1 AND generation=$2 AND projection_claim=$3`, release.SessionID, release.Generation, claim); err != nil {
		return false, err
	}
	return true, nil
}
