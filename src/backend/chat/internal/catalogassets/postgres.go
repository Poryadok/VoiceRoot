package catalogassets

import (
	"context"
	"crypto/hmac"
	"encoding/binary"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrActorRetired = errors.New("catalog actor retired")
var ErrActorUnknown = errors.New("catalog actor authority unavailable")

type PostgresRepository struct {
	Pool             *pgxpool.Pool
	Keys             *ActorKeyring
	Observer         ActorObserver
	MaxSelectorSlots int
}

type actorGate struct {
	id         uuid.UUID
	kind       int16
	generation uuid.UUID
	state      string
	epoch      int64
}

func (s *PostgresRepository) Preflight(ctx context.Context) error {
	if s == nil || s.Pool == nil || s.Keys == nil || s.Observer == nil || s.MaxSelectorSlots < 0 {
		return ErrActorUnknown
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return errors.Join(ErrActorUnknown, err)
	}
	defer rollbackCatalogTx(tx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4846246169872713547)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT id,actor_kind,generation,state,authority_epoch FROM catalog_actor_gates LIMIT 0; SELECT key_id,actor_kind,actor_hmac,gate_id FROM catalog_actor_aliases LIMIT 0; SELECT key_id,event_hmac,account_gate_id FROM catalog_actor_event_fences LIMIT 0; SELECT file_id,source_id,account_id,profile_id,provider,provider_id,account_gate_id,profile_gate_id,account_generation,profile_generation FROM catalog_asset_bindings LIMIT 0`); err != nil {
		return errors.Join(ErrActorUnknown, err)
	}
	if err = s.checkKeys(ctx, tx, true); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT id,account_gate_id,profile_gate_id,account_generation,profile_generation,selection_revision FROM catalog_selector_slots LIMIT 0; SELECT key_id,selector_hmac,slot_id FROM catalog_selector_aliases LIMIT 0; SELECT selector_slot_id,selection_revision FROM catalog_asset_bindings LIMIT 0; SELECT file_id FROM catalog_legacy_asset_bindings LIMIT 0`); err != nil {
		return errors.Join(ErrActorUnknown, err)
	}
	return tx.Commit(ctx)
}

func (s *PostgresRepository) checkKeys(ctx context.Context, tx pgx.Tx, register bool) error {
	expected := s.Keys.fingerprints()
	seen := make(map[string]bool, len(expected))
	rows, err := tx.Query(ctx, `SELECT key_id,fingerprint FROM catalog_actor_keys ORDER BY key_id`)
	if err != nil {
		return errors.Join(ErrActorUnknown, err)
	}
	for rows.Next() {
		var id string
		var actual []byte
		if err = rows.Scan(&id, &actual); err != nil {
			rows.Close()
			return err
		}
		want, ok := expected[id]
		if !ok || !hmac.Equal(actual, want[:]) {
			rows.Close()
			return ErrActorUnknown
		}
		seen[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range s.Keys.ids {
		if seen[id] {
			continue
		}
		if !register {
			return ErrActorUnknown
		}
		fingerprint := expected[id]
		if _, err = tx.Exec(ctx, `INSERT INTO catalog_actor_keys(key_id,fingerprint) VALUES($1,$2)`, id, fingerprint[:]); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresRepository) begin(ctx context.Context) (pgx.Tx, error) {
	if s == nil || s.Pool == nil || s.Keys == nil {
		return nil, ErrActorUnknown
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, errors.Join(ErrActorUnknown, err)
	}
	if _, err = tx.Exec(ctx, `SET LOCAL lock_timeout='3s'; SET LOCAL statement_timeout='6s'`); err != nil {
		rollbackCatalogTx(tx)
		return nil, err
	}
	if err = s.checkKeys(ctx, tx, false); err != nil {
		rollbackCatalogTx(tx)
		return nil, err
	}
	return tx, nil
}

func rollbackCatalogTx(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

func (s *PostgresRepository) lockActor(ctx context.Context, tx pgx.Tx, actor Actor) ([]actorAlias, []actorAlias, error) {
	account, err := s.Keys.aliases(1, actor.AccountID)
	if err != nil {
		return nil, nil, ErrActorUnknown
	}
	profile, err := s.Keys.aliases(2, actor.ProfileID)
	if err != nil {
		return nil, nil, ErrActorUnknown
	}
	// Every operation acquires all account aliases before all profile aliases.
	for _, aliases := range [][]actorAlias{account, profile} {
		if err = lockAliases(ctx, tx, aliases); err != nil {
			return nil, nil, err
		}
	}
	return account, profile, nil
}
func lockAliases(ctx context.Context, tx pgx.Tx, aliases []actorAlias) error {
	for _, alias := range aliases {
		key := int64(binary.BigEndian.Uint64(alias.digest[:8]))
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, key); err != nil {
			return err
		}
	}
	return nil
}

func lookupGate(ctx context.Context, tx pgx.Tx, aliases []actorAlias) (actorGate, bool, error) {
	var id uuid.UUID
	for _, alias := range aliases {
		var found uuid.UUID
		err := tx.QueryRow(ctx, `SELECT gate_id FROM catalog_actor_aliases WHERE key_id=$1 AND actor_kind=$2 AND actor_hmac=$3`, alias.keyID, alias.kind, alias.digest[:]).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return actorGate{}, false, err
		}
		if id != uuid.Nil && id != found {
			return actorGate{}, false, ErrActorUnknown
		}
		id = found
	}
	if id == uuid.Nil {
		return actorGate{}, false, nil
	}
	var gate actorGate
	err := tx.QueryRow(ctx, `SELECT id,actor_kind,generation,state,authority_epoch FROM catalog_actor_gates WHERE id=$1 FOR UPDATE`, id).Scan(&gate.id, &gate.kind, &gate.generation, &gate.state, &gate.epoch)
	if err != nil {
		return actorGate{}, false, err
	}
	if gate.id == uuid.Nil || gate.generation == uuid.Nil || gate.kind != aliases[0].kind || (gate.state != "LIVE" && gate.state != "CLOSED") {
		return actorGate{}, false, ErrActorUnknown
	}
	return gate, true, nil
}

func (s *PostgresRepository) attachActiveAlias(ctx context.Context, tx pgx.Tx, gate actorGate, aliases []actorAlias) error {
	for _, alias := range aliases {
		if alias.keyID != s.Keys.active {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO catalog_actor_aliases(key_id,actor_kind,actor_hmac,gate_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, alias.keyID, alias.kind, alias.digest[:], gate.id); err != nil {
			return err
		}
		var found uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT gate_id FROM catalog_actor_aliases WHERE key_id=$1 AND actor_kind=$2 AND actor_hmac=$3`, alias.keyID, alias.kind, alias.digest[:]).Scan(&found); err != nil {
			return err
		}
		if found != gate.id {
			return ErrActorUnknown
		}
		return nil
	}
	return ErrActorUnknown
}
func (s *PostgresRepository) ensureGate(ctx context.Context, tx pgx.Tx, aliases []actorAlias, state string, epoch int64) (actorGate, error) {
	gate, found, err := lookupGate(ctx, tx, aliases)
	if err != nil {
		return actorGate{}, err
	}
	if !found {
		gate = actorGate{id: uuid.New(), kind: aliases[0].kind, generation: uuid.New(), state: state, epoch: epoch}
		_, err = tx.Exec(ctx, `INSERT INTO catalog_actor_gates(id,actor_kind,generation,state,authority_epoch) VALUES($1,$2,$3,$4,$5)`, gate.id, gate.kind, gate.generation, gate.state, gate.epoch)
		if err != nil {
			return actorGate{}, err
		}
	}
	if err = s.attachActiveAlias(ctx, tx, gate, aliases); err != nil {
		return actorGate{}, err
	}
	return gate, nil
}
func closeGate(ctx context.Context, tx pgx.Tx, gate actorGate) error {
	if gate.state == "CLOSED" {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE catalog_actor_gates SET generation=$2,state='CLOSED',updated_at=now() WHERE id=$1`, gate.id, uuid.New())
	return err
}
func admitGate(ctx context.Context, tx pgx.Tx, gate actorGate, epoch int64, initialize bool) (actorGate, error) {
	if epoch < gate.epoch {
		return actorGate{}, ErrActorUnknown
	}
	if gate.state != "LIVE" {
		if !initialize {
			return actorGate{}, ErrActorRetired
		}
		gate.generation = uuid.New()
		gate.state = "LIVE"
	}
	if epoch != gate.epoch || gate.state == "LIVE" {
		_, err := tx.Exec(ctx, `UPDATE catalog_actor_gates SET generation=$2,state='LIVE',authority_epoch=$3,updated_at=now() WHERE id=$1`, gate.id, gate.generation, epoch)
		if err != nil {
			return actorGate{}, err
		}
		gate.epoch = epoch
	}
	return gate, nil
}

func (s *PostgresRepository) CaptureActor(ctx context.Context, actor Actor, initialize bool) (ActorCapture, error) {
	if s == nil || s.Observer == nil {
		return ActorCapture{}, ErrActorUnknown
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return ActorCapture{}, err
	}
	defer rollbackCatalogTx(tx)
	accountAliases, profileAliases, err := s.lockActor(ctx, tx, actor)
	if err != nil {
		return ActorCapture{}, err
	}
	observation, err := s.Observer.Observe(ctx, actor)
	if err != nil {
		return ActorCapture{}, errors.Join(ErrActorUnknown, err)
	}
	if observation.SessionEpochFloor <= 0 || observation.Profile == ProfileUnknown {
		return ActorCapture{}, ErrActorUnknown
	}
	if observation.Profile == ProfileSoftDeleted {
		if !initialize {
			return ActorCapture{}, ErrActorRetired
		}
		gate, err := s.ensureGate(ctx, tx, profileAliases, "CLOSED", 0)
		if err != nil {
			return ActorCapture{}, err
		}
		if err = closeGate(ctx, tx, gate); err != nil {
			return ActorCapture{}, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM catalog_selector_slots WHERE profile_gate_id=$1`, gate.id); err != nil {
			return ActorCapture{}, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM catalog_legacy_asset_bindings WHERE profile_id=$1`, actor.ProfileID); err != nil {
			return ActorCapture{}, err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM catalog_asset_bindings WHERE profile_id=$1`, actor.ProfileID); err != nil {
			return ActorCapture{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return ActorCapture{}, err
		}
		return ActorCapture{}, ErrActorRetired
	}
	if observation.Profile != ProfileLive {
		return ActorCapture{}, ErrActorUnknown
	}
	account, found, err := lookupGate(ctx, tx, accountAliases)
	if err != nil {
		return ActorCapture{}, err
	}
	if !found {
		if !initialize {
			return ActorCapture{}, ErrActorRetired
		}
		account, err = s.ensureGate(ctx, tx, accountAliases, "LIVE", observation.SessionEpochFloor)
		if err != nil {
			return ActorCapture{}, err
		}
	}
	profile, found, err := lookupGate(ctx, tx, profileAliases)
	if err != nil {
		return ActorCapture{}, err
	}
	if !found {
		if !initialize {
			return ActorCapture{}, ErrActorRetired
		}
		profile, err = s.ensureGate(ctx, tx, profileAliases, "LIVE", 0)
		if err != nil {
			return ActorCapture{}, err
		}
	}
	if initialize {
		account, err = admitGate(ctx, tx, account, observation.SessionEpochFloor, true)
		if err != nil {
			return ActorCapture{}, err
		}
		profile, err = admitGate(ctx, tx, profile, 0, true)
		if err != nil {
			return ActorCapture{}, err
		}
		if err = s.attachActiveAlias(ctx, tx, account, accountAliases); err != nil {
			return ActorCapture{}, err
		}
		if err = s.attachActiveAlias(ctx, tx, profile, profileAliases); err != nil {
			return ActorCapture{}, err
		}
	} else if account.state != "LIVE" || profile.state != "LIVE" || observation.SessionEpochFloor < account.epoch {
		return ActorCapture{}, ErrActorRetired
	}
	capture := ActorCapture{AccountGeneration: account.generation, ProfileGeneration: profile.generation}
	if err = tx.Commit(ctx); err != nil {
		return ActorCapture{}, err
	}
	return capture, nil
}

func (s *PostgresRepository) capturedGates(ctx context.Context, tx pgx.Tx, actor Actor, capture ActorCapture) (actorGate, actorGate, error) {
	a, p, err := s.lockActor(ctx, tx, actor)
	if err != nil {
		return actorGate{}, actorGate{}, err
	}
	account, found, err := lookupGate(ctx, tx, a)
	if err != nil {
		return actorGate{}, actorGate{}, err
	}
	if !found || account.state != "LIVE" || account.generation != capture.AccountGeneration {
		return actorGate{}, actorGate{}, ErrActorRetired
	}
	profile, found, err := lookupGate(ctx, tx, p)
	if err != nil {
		return actorGate{}, actorGate{}, err
	}
	if !found || profile.state != "LIVE" || profile.generation != capture.ProfileGeneration {
		return actorGate{}, actorGate{}, ErrActorRetired
	}
	return account, profile, nil
}

func (s *PostgresRepository) PurgeAccount(ctx context.Context, accountID, eventID uuid.UUID) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCatalogTx(tx)
	aliases, err := s.Keys.aliases(1, accountID)
	if err != nil {
		return ErrActorUnknown
	}
	events, err := s.Keys.eventAliases(eventID)
	if err != nil {
		return ErrActorUnknown
	}
	if err = lockAliases(ctx, tx, aliases); err != nil {
		return err
	}
	gate, err := s.ensureGate(ctx, tx, aliases, "CLOSED", 0)
	if err != nil {
		return err
	}
	// Retain only purpose-HMAC event keys and the opaque logical gate. A repeated
	// deletion event must not close a generation admitted after an actual restore.
	for _, alias := range events {
		var prior uuid.UUID
		err = tx.QueryRow(ctx, `SELECT account_gate_id FROM catalog_actor_event_fences WHERE key_id=$1 AND event_hmac=$2`, alias.keyID, alias.digest[:]).Scan(&prior)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if prior != gate.id {
			return ErrBindingConflict
		}
		return tx.Commit(ctx)
	}
	if err = closeGate(ctx, tx, gate); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_selector_slots WHERE account_gate_id=$1`, gate.id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_legacy_asset_bindings WHERE account_id=$1`, accountID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_asset_bindings WHERE account_id=$1`, accountID); err != nil {
		return err
	}
	for _, alias := range events {
		if alias.keyID != s.Keys.active {
			continue
		}
		if _, err = tx.Exec(ctx, `INSERT INTO catalog_actor_event_fences(key_id,event_hmac,account_gate_id) VALUES($1,$2,$3)`, alias.keyID, alias.digest[:], gate.id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *PostgresRepository) PurgeProfile(ctx context.Context, actor Actor) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCatalogTx(tx)
	_, aliases, err := s.lockActor(ctx, tx, actor)
	if err != nil {
		return err
	}
	gate, err := s.ensureGate(ctx, tx, aliases, "CLOSED", 0)
	if err != nil {
		return err
	}
	if err = closeGate(ctx, tx, gate); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_selector_slots WHERE profile_gate_id=$1`, gate.id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_legacy_asset_bindings WHERE profile_id=$1`, actor.ProfileID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_asset_bindings WHERE profile_id=$1`, actor.ProfileID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
