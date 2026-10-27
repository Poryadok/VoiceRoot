package catalogassets

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	defaultSelectorSlots       = 16384
	profileSelectorSlots       = 256
	selectorCapacityLock int64 = 4846246169872713548
)

var ErrSelectionSuperseded = errors.New("catalog selection superseded")
var ErrSelectorInvalidated = errors.New("catalog selector cache invalidated")

type selectorSlot struct {
	id, accountGate, profileGate uuid.UUID
	ActorCapture
	revision uint64
}

func lookupSelector(ctx context.Context, tx pgx.Tx, aliases []selectorAlias) (selectorSlot, bool, error) {
	var id uuid.UUID
	for _, alias := range aliases {
		var found uuid.UUID
		err := tx.QueryRow(ctx, `SELECT slot_id FROM catalog_selector_aliases WHERE key_id=$1 AND selector_hmac=$2`, alias.keyID, alias.digest[:]).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return selectorSlot{}, false, err
		}
		if id != uuid.Nil && id != found {
			return selectorSlot{}, false, ErrActorUnknown
		}
		id = found
	}
	if id == uuid.Nil {
		return selectorSlot{}, false, nil
	}
	var slot selectorSlot
	err := tx.QueryRow(ctx, `SELECT id,account_gate_id,profile_gate_id,account_generation,profile_generation,selection_revision FROM catalog_selector_slots WHERE id=$1 FOR UPDATE`, id).Scan(&slot.id, &slot.accountGate, &slot.profileGate, &slot.AccountGeneration, &slot.ProfileGeneration, &slot.revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return selectorSlot{}, false, nil
	}
	if err != nil {
		return selectorSlot{}, false, err
	}
	return slot, true, nil
}

func (s *PostgresRepository) attachSelectorAlias(ctx context.Context, tx pgx.Tx, id uuid.UUID, aliases []selectorAlias) error {
	for _, alias := range aliases {
		if alias.keyID != s.Keys.active {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO catalog_selector_aliases(key_id,selector_hmac,slot_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, alias.keyID, alias.digest[:], id); err != nil {
			return err
		}
		var actual uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT slot_id FROM catalog_selector_aliases WHERE key_id=$1 AND selector_hmac=$2`, alias.keyID, alias.digest[:]).Scan(&actual); err != nil {
			return err
		}
		if actual != id {
			return ErrActorUnknown
		}
		return nil
	}
	return ErrActorUnknown
}

// CaptureSelection creates only disposable HMAC cache metadata before remote
// File work. It never stores a source URL or a business operation reservation.
func (s *PostgresRepository) CaptureSelection(ctx context.Context, actor Actor, selection Selection, capture ActorCapture) (uuid.UUID, error) {
	if !validCapture(capture) {
		return uuid.Nil, ErrActorUnknown
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer rollbackCatalogTx(tx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, selectorCapacityLock); err != nil {
		return uuid.Nil, err
	}
	account, profile, err := s.capturedGates(ctx, tx, actor, capture)
	if err != nil {
		return uuid.Nil, err
	}
	aliases, err := s.Keys.selectorAliases(actor, capture, selection)
	if err != nil {
		return uuid.Nil, ErrActorUnknown
	}
	slot, found, err := lookupSelector(ctx, tx, aliases)
	if err != nil {
		return uuid.Nil, err
	}
	if found {
		if slot.accountGate != account.id || slot.profileGate != profile.id || slot.ActorCapture != capture {
			return uuid.Nil, ErrActorUnknown
		}
	} else {
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM catalog_selector_slots WHERE profile_gate_id=$1`, profile.id).Scan(&count); err != nil {
			return uuid.Nil, err
		}
		if count >= profileSelectorSlots {
			if _, err = s.evictSelector(ctx, tx, profile.id, false); err != nil {
				return uuid.Nil, err
			}
		}
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM catalog_selector_slots`).Scan(&count); err != nil {
			return uuid.Nil, err
		}
		capacity := s.MaxSelectorSlots
		if capacity == 0 {
			capacity = defaultSelectorSlots
		}
		if capacity < 1 {
			return uuid.Nil, ErrActorUnknown
		}
		if count >= capacity {
			if _, err = s.evictSelector(ctx, tx, uuid.Nil, false); err != nil {
				return uuid.Nil, err
			}
		}
		slot = selectorSlot{id: uuid.New(), accountGate: account.id, profileGate: profile.id, ActorCapture: capture}
		if _, err = tx.Exec(ctx, `INSERT INTO catalog_selector_slots(id,account_gate_id,profile_gate_id,account_generation,profile_generation) VALUES($1,$2,$3,$4,$5)`, slot.id, account.id, profile.id, capture.AccountGeneration, capture.ProfileGeneration); err != nil {
			return uuid.Nil, err
		}
	}
	if err = s.attachSelectorAlias(ctx, tx, slot.id, aliases); err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE catalog_selector_slots SET last_used_at=now() WHERE id=$1`, slot.id); err != nil {
		return uuid.Nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return slot.id, nil
}

// evictSelector chooses without a row lock, then takes owning account/profile
// gates before deleting the incarnation. Cache eviction has no File side effect.
func (s *PostgresRepository) evictSelector(ctx context.Context, tx pgx.Tx, profile uuid.UUID, emptyOnly bool) (bool, error) {
	var id, accountGate, profileGate uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id,account_gate_id,profile_gate_id FROM catalog_selector_slots WHERE ($1::uuid='00000000-0000-0000-0000-000000000000'::uuid OR profile_gate_id=$1) AND (NOT $2::boolean OR selection_revision=0) ORDER BY (selection_revision<>0),last_used_at,id LIMIT 1`, profile, emptyOnly).Scan(&id, &accountGate, &profileGate)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var locked uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM catalog_actor_gates WHERE id=$1 AND actor_kind=1 FOR UPDATE`, accountGate).Scan(&locked); err != nil {
		return false, err
	}
	if err = tx.QueryRow(ctx, `SELECT id FROM catalog_actor_gates WHERE id=$1 AND actor_kind=2 FOR UPDATE`, profileGate).Scan(&locked); err != nil {
		return false, err
	}
	result, err := tx.Exec(ctx, `DELETE FROM catalog_selector_slots WHERE id=$1 AND (NOT $2::boolean OR selection_revision=0)`, id, emptyOnly)
	return result.RowsAffected() > 0, err
}

func (s *PostgresRepository) DropEmptySelection(ctx context.Context, actor Actor, capture ActorCapture, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrActorUnknown
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCatalogTx(tx)
	account, profile, err := s.capturedGates(ctx, tx, actor, capture)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_selector_slots WHERE id=$1 AND account_gate_id=$2 AND profile_gate_id=$3 AND account_generation=$4 AND profile_generation=$5 AND selection_revision=0`, id, account.id, profile.id, capture.AccountGeneration, capture.ProfileGeneration); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// EvictEmptySelections bounds crash-orphan metadata without a retention TTL or
// a claim about process death. A late response fails its old incarnation check.
func (s *PostgresRepository) EvictEmptySelections(ctx context.Context, limit int) error {
	if limit < 1 || limit > 100 {
		return ErrActorUnknown
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCatalogTx(tx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, selectorCapacityLock); err != nil {
		return err
	}
	for range limit {
		evicted, err := s.evictSelector(ctx, tx, uuid.Nil, true)
		if err != nil {
			return err
		}
		if !evicted {
			break
		}
	}
	return tx.Commit(ctx)
}
