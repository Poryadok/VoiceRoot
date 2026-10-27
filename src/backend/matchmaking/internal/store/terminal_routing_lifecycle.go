package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"voice/backend/matchmaking/internal/terminalrouting"
)

type routingReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func lockTerminalRouting(ctx context.Context, tx pgx.Tx, fence string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,172020))`, "terminal-routing/"+fence)
	return err
}

func loadTerminalRoutingCycle(ctx context.Context, db routingReader, fence, cycle string) (terminalrouting.Lifecycle, error) {
	var raw []byte
	var deadline time.Time
	var erased bool
	err := db.QueryRow(ctx, `SELECT lifecycle_json,purge_at,routing_erased FROM matchmaking_terminal_routing_cycles
		WHERE fence_id=$1 AND cycle_id=$2`, fence, cycle).Scan(&raw, &deadline, &erased)
	if err != nil {
		return terminalrouting.Lifecycle{}, err
	}
	var life terminalrouting.Lifecycle
	if json.Unmarshal(raw, &life) != nil || life.Schedule.FenceID != fence || life.Schedule.CycleID != cycle ||
		!life.Schedule.PurgeAt.Equal(deadline) || life.RoutingErased != erased {
		return life, terminalrouting.ErrAuthorityInvalid
	}
	if _, err := terminalrouting.New(life.Schedule); err != nil {
		return life, err
	}
	return life, nil
}

func saveTerminalRoutingCycle(ctx context.Context, tx pgx.Tx, life terminalrouting.Lifecycle) error {
	raw, err := json.Marshal(life)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO matchmaking_terminal_routing_cycles
		(fence_id,cycle_id,purge_at,lifecycle_json,routing_erased) VALUES($1,$2,$3,$4,$5)
		ON CONFLICT(fence_id,cycle_id) DO UPDATE SET lifecycle_json=EXCLUDED.lifecycle_json,
		routing_erased=EXCLUDED.routing_erased,
		coverage_sha256=CASE WHEN EXCLUDED.routing_erased OR clock_timestamp()>=matchmaking_terminal_routing_cycles.purge_at
		THEN NULL ELSE matchmaking_terminal_routing_cycles.coverage_sha256 END`,
		life.Schedule.FenceID, life.Schedule.CycleID, life.Schedule.PurgeAt, raw, life.RoutingErased)
	return err
}

// ObserveTerminalRouting is a local already-verified-fact seam. It installs the
// original deadline even before User coverage. It allocates no listener, RPC or
// producer trust; the protected adapter/capability remains an activation gate.
func (s *MatchStore) ObserveTerminalRouting(ctx context.Context, observation terminalrouting.Observation) (terminalrouting.Lifecycle, error) {
	life, err := terminalrouting.New(observation.Schedule)
	if err != nil {
		return life, err
	}
	if s == nil || s.Pool == nil {
		return life, errors.New("terminal routing store unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return life, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockTerminalRouting(ctx, tx, observation.Schedule.FenceID); err != nil {
		return life, err
	}
	stored, err := loadTerminalRoutingCycle(ctx, tx, observation.Schedule.FenceID, observation.Schedule.CycleID)
	if err == nil {
		life = stored
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return life, err
	}
	var currentRaw []byte
	err = tx.QueryRow(ctx, `SELECT current_json FROM matchmaking_terminal_routing_heads WHERE fence_id=$1`, observation.Schedule.FenceID).Scan(&currentRaw)
	if err == nil {
		var current terminalrouting.Current
		if json.Unmarshal(currentRaw, &current) != nil {
			return life, terminalrouting.ErrAuthorityInvalid
		}
		if current.Revision >= life.Current.Revision {
			life.Current = current
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return life, err
	}
	// Fresh database time is read after the owning serialization wait.
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return life, err
	}
	life, err = life.Reconcile(observation, now)
	if err != nil {
		return life, err
	}
	currentRaw, err = json.Marshal(life.Current)
	if err != nil {
		return life, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO matchmaking_terminal_routing_heads(fence_id,current_json) VALUES($1,$2)
		ON CONFLICT(fence_id) DO UPDATE SET current_json=EXCLUDED.current_json`, observation.Schedule.FenceID, currentRaw); err != nil {
		return life, err
	}
	if err := saveTerminalRoutingCycle(ctx, tx, life); err != nil {
		return life, err
	}
	if life.RoutingErased {
		if _, err := tx.Exec(ctx, `DELETE FROM matchmaking_terminal_search_releases WHERE routing_fence_id=$1 AND routing_cycle_id=$2`,
			life.Schedule.FenceID, life.Schedule.CycleID); err != nil {
			return life, err
		}
	}
	return life, tx.Commit(ctx)
}

// BindTerminalRoutingCoverage consumes the protected User attestation's exact
// scope for terminal obligations already bound by an exact lifecycle-aware
// writer. The roster is never persisted here. Profile membership and cross-DB
// timestamps cannot establish an operation's cycle: unbound legacy routing is
// explicitly unqualified, including previously dispatched callbacks.
func (s *MatchStore) BindTerminalRoutingCoverage(ctx context.Context, coverage terminalrouting.Coverage) error {
	if err := coverage.Validate(); err != nil {
		return err
	}
	if s == nil || s.Pool == nil {
		return errors.New("terminal routing store unavailable")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockTerminalRouting(ctx, tx, coverage.Schedule.FenceID); err != nil {
		return err
	}
	life, err := loadTerminalRoutingCycle(ctx, tx, coverage.Schedule.FenceID, coverage.Schedule.CycleID)
	if err != nil {
		return err
	}
	var headRaw []byte
	if err := tx.QueryRow(ctx, `SELECT current_json FROM matchmaking_terminal_routing_heads WHERE fence_id=$1`, coverage.Schedule.FenceID).Scan(&headRaw); err != nil {
		return err
	}
	var head terminalrouting.Current
	if json.Unmarshal(headRaw, &head) != nil {
		return terminalrouting.ErrAuthorityInvalid
	}
	// Every cycle under the same fence shares this serialization lock. Consult
	// the highest durable authority rather than the older cycle's snapshot.
	life, err = life.Reconcile(terminalrouting.Observation{Schedule: life.Schedule, Current: head, Decision: life.Decision}, life.Schedule.ScheduledAt)
	if err != nil {
		return err
	}
	// A newer/current active account implies an intervening cycle transition.
	// Until its immutable decision is reconciled, purge_at alone cannot separate
	// old payload from fresh post-restore work owned by the same profile.
	if life.Decision == nil && (life.Current.State != terminalrouting.DeleteScheduled || life.Current.CycleID != life.Schedule.CycleID) {
		return terminalrouting.ErrReconciliationRequired
	}
	var unbound int
	err = tx.QueryRow(ctx, `SELECT 1 FROM matchmaking_terminal_search_releases
		WHERE profile_id=ANY($1::uuid[]) AND routing_fence_id IS NULL LIMIT 1 FOR UPDATE`, coverage.Profiles).Scan(&unbound)
	if err == nil {
		return terminalrouting.ErrReconciliationRequired
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	// New verifies schedule shape; Reconcile enforces the immutable exact tuple.
	if _, err := life.Reconcile(terminalrouting.Observation{Schedule: coverage.Schedule, Current: life.Current, Decision: life.Decision}, life.Schedule.ScheduledAt); err != nil {
		return err
	}
	var savedRevision *string
	var savedHash []byte
	if err := tx.QueryRow(ctx, `SELECT coverage_revision::text,coverage_sha256 FROM matchmaking_terminal_routing_cycles
		WHERE fence_id=$1 AND cycle_id=$2`, coverage.Schedule.FenceID, coverage.Schedule.CycleID).Scan(&savedRevision, &savedHash); err != nil {
		return err
	}
	if savedRevision != nil && (*savedRevision != strconv.FormatUint(coverage.Revision, 10) ||
		(len(savedHash) != 0 && !bytes.Equal(savedHash, coverage.SHA256[:]))) {
		return terminalrouting.ErrAuthorityConflict
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return err
	}
	life = life.Advance(now)
	if life.RoutingErased || !now.Before(life.Schedule.PurgeAt) {
		// Erase only exact previously authorized cycle bindings. Never infer
		// old operation ownership from a profile roster or another DB's clock.
		_, err = tx.Exec(ctx, `DELETE FROM matchmaking_terminal_search_releases
			WHERE routing_fence_id=$1 AND routing_cycle_id=$2`, coverage.Schedule.FenceID, coverage.Schedule.CycleID)
		if err != nil {
			return err
		}
		if err := saveTerminalRoutingCycle(ctx, tx, life); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return terminalrouting.ErrRoutingRetired
	}
	_, err = tx.Exec(ctx, `UPDATE matchmaking_terminal_routing_cycles SET coverage_revision=$3::numeric,coverage_sha256=$4
		WHERE fence_id=$1 AND cycle_id=$2`, coverage.Schedule.FenceID, coverage.Schedule.CycleID,
		strconv.FormatUint(coverage.Revision, 10), coverage.SHA256[:])
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ExpireTerminalRouting runs without Redis/RPC and ignores projection leases.
// Only short SQL transactions can contend with erasure, never a paused network
// command. The account's Auth terminal decision is not inferred by local expiry.
func (s *MatchStore) ExpireTerminalRouting(ctx context.Context, limit int) error {
	if s == nil || s.Pool == nil {
		return errors.New("terminal routing store unavailable")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `SELECT fence_id,cycle_id FROM matchmaking_terminal_routing_cycles c
		WHERE purge_at<=clock_timestamp() AND ((NOT routing_erased AND COALESCE(lifecycle_json->'Decision'->>'Kind','')<>'RESTORED') OR coverage_sha256 IS NOT NULL OR
		EXISTS(SELECT 1 FROM matchmaking_terminal_search_releases r WHERE r.routing_fence_id=c.fence_id AND r.routing_cycle_id=c.cycle_id))
		ORDER BY purge_at,fence_id,cycle_id LIMIT $1`, limit)
	if err != nil {
		return err
	}
	type key struct{ fence, cycle string }
	var keys []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.fence, &k.cycle); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, k)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, k := range keys {
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		err = func() error {
			defer func() { _ = tx.Rollback(ctx) }()
			if err := lockTerminalRouting(ctx, tx, k.fence); err != nil {
				return err
			}
			life, err := loadTerminalRoutingCycle(ctx, tx, k.fence, k.cycle)
			if err != nil {
				return err
			}
			var now time.Time
			if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
				return err
			}
			if now.Before(life.Schedule.PurgeAt) {
				return tx.Commit(ctx)
			}
			life = life.Advance(now)
			if err := saveTerminalRoutingCycle(ctx, tx, life); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM matchmaking_terminal_search_releases
				WHERE routing_fence_id=$1 AND routing_cycle_id=$2 AND routing_purge_at<=clock_timestamp()`, k.fence, k.cycle); err != nil {
				return err
			}
			return tx.Commit(ctx)
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
