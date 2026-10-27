package catalogassets

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

func (s *PostgresRepository) ScanBindings(ctx context.Context, after uuid.UUID, limit int) ([]Binding, error) {
	if limit < 1 || limit > 100 {
		return nil, ErrActorUnknown
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackCatalogTx(tx)
	rows, err := tx.Query(ctx, `SELECT file_id,source_id,account_id,profile_id,provider,provider_id,account_generation,profile_generation,selector_slot_id,selection_revision,legacy FROM (
SELECT file_id,source_id,account_id,profile_id,provider,provider_id,account_generation,profile_generation,selector_slot_id,selection_revision,false AS legacy FROM catalog_asset_bindings
UNION ALL SELECT file_id,source_id,account_id,profile_id,provider,provider_id,account_generation,profile_generation,'00000000-0000-0000-0000-000000000000'::uuid,0::bigint,true FROM catalog_legacy_asset_bindings
) projections WHERE file_id>$1 ORDER BY file_id LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	var out []Binding
	for rows.Next() {
		var b Binding
		if err = rows.Scan(&b.FileID, &b.SourceID, &b.AccountID, &b.ProfileID, &b.Provider, &b.ProviderID, &b.AccountGeneration, &b.ProfileGeneration, &b.SelectorSlotID, &b.SelectionRevision, &b.Legacy); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PostgresRepository) RetireExactBinding(ctx context.Context, b Binding) error {
	if b.FileID == uuid.Nil || b.SourceID == uuid.Nil || !validCapture(b.ActorCapture) {
		return ErrActorUnknown
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCatalogTx(tx)
	if b.Legacy {
		if b.SelectorSlotID != uuid.Nil || b.SelectionRevision != 0 {
			return ErrActorUnknown
		}
		if _, _, err = s.lockActor(ctx, tx, b.Actor); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM catalog_legacy_asset_bindings WHERE file_id=$1 AND source_id=$2 AND account_id=$3 AND profile_id=$4 AND provider=$5 AND provider_id=$6 AND account_generation=$7 AND profile_generation=$8`, b.FileID, b.SourceID, b.AccountID, b.ProfileID, b.Provider, b.ProviderID, b.AccountGeneration, b.ProfileGeneration)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if b.SelectorSlotID == uuid.Nil || b.SelectionRevision == 0 {
		return ErrActorUnknown
	}
	account, profile, err := s.capturedGates(ctx, tx, b.Actor, b.ActorCapture)
	if errors.Is(err, ErrActorRetired) {
		return nil
	}
	if err != nil {
		return err
	}
	aliases, err := s.Keys.selectorAliases(b.Actor, b.ActorCapture, b.Selection)
	if err != nil {
		return ErrActorUnknown
	}
	slot, found, err := lookupSelector(ctx, tx, aliases)
	if err != nil {
		return err
	}
	if !found || slot.id != b.SelectorSlotID || slot.accountGate != account.id || slot.profileGate != profile.id || slot.ActorCapture != b.ActorCapture || slot.revision > b.SelectionRevision {
		return nil
	}
	if slot.revision > 0 {
		var matches bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog_asset_bindings WHERE selector_slot_id=$1 AND selection_revision=$2 AND file_id=$3 AND source_id=$4 AND account_id=$5 AND profile_id=$6 AND provider=$7 AND provider_id=$8)`, slot.id, b.SelectionRevision, b.FileID, b.SourceID, b.AccountID, b.ProfileID, b.Provider, b.ProviderID).Scan(&matches); err != nil {
			return err
		}
		if !matches {
			return nil
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_selector_slots WHERE id=$1`, slot.id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresRepository) PurgeCapturedProfile(ctx context.Context, b Binding) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCatalogTx(tx)
	_, profile, err := s.capturedGates(ctx, tx, b.Actor, b.ActorCapture)
	if errors.Is(err, ErrActorRetired) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = closeGate(ctx, tx, profile); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_selector_slots WHERE profile_gate_id=$1`, profile.id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_legacy_asset_bindings WHERE profile_id=$1`, b.ProfileID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_asset_bindings WHERE profile_gate_id=$1 AND profile_generation=$2`, profile.id, b.ProfileGeneration); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
