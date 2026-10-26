package catalogassets

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *PostgresRepository) SaveBinding(ctx context.Context, binding Binding, capture ActorCapture) error {
	if binding.Legacy || !validCapture(capture) || binding.ActorCapture != capture || binding.FileID == uuid.Nil || binding.SourceID == uuid.Nil || binding.SelectorSlotID == uuid.Nil || binding.SelectionRevision == 0 || binding.SelectionRevision > math.MaxInt64 || !validSelection(binding.Provider, binding.ProviderID) || s == nil || s.Observer == nil {
		return ErrActorUnknown
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCatalogTx(tx)
	account, profile, err := s.capturedGates(ctx, tx, binding.Actor, capture)
	if err != nil {
		return err
	}
	aliases, err := s.Keys.selectorAliases(binding.Actor, capture, binding.Selection)
	if err != nil {
		return ErrActorUnknown
	}
	slot, found, err := lookupSelector(ctx, tx, aliases)
	if err != nil {
		return err
	}
	if !found || slot.id != binding.SelectorSlotID || slot.accountGate != account.id || slot.profileGate != profile.id || slot.ActorCapture != capture {
		return ErrSelectorInvalidated
	}
	if binding.SelectionRevision < slot.revision {
		return ErrSelectionSuperseded
	}
	observation, err := s.Observer.Observe(ctx, binding.Actor)
	if err != nil {
		return errors.Join(ErrActorUnknown, err)
	}
	if observation.SessionEpochFloor <= 0 || observation.SessionEpochFloor < account.epoch || observation.Profile == ProfileUnknown {
		return ErrActorUnknown
	}
	if observation.Profile == ProfileSoftDeleted {
		if err = closeGate(ctx, tx, profile); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM catalog_selector_slots WHERE profile_gate_id=$1`, profile.id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM catalog_legacy_asset_bindings WHERE profile_id=$1`, binding.ProfileID); err != nil {
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		return ErrActorRetired
	}
	if observation.Profile != ProfileLive {
		return ErrActorUnknown
	}
	if binding.SelectionRevision == slot.revision {
		var matches bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog_asset_bindings WHERE selector_slot_id=$1 AND selection_revision=$2 AND source_id=$3 AND file_id=$4 AND account_id=$5 AND profile_id=$6 AND provider=$7 AND provider_id=$8 AND account_generation=$9 AND profile_generation=$10)`, slot.id, binding.SelectionRevision, binding.SourceID, binding.FileID, binding.AccountID, binding.ProfileID, binding.Provider, binding.ProviderID, capture.AccountGeneration, capture.ProfileGeneration).Scan(&matches)
		if err != nil {
			return err
		}
		if !matches {
			return ErrBindingConflict
		}
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_asset_bindings WHERE selector_slot_id=$1`, slot.id); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO catalog_asset_bindings(file_id,source_id,account_id,profile_id,provider,provider_id,account_gate_id,profile_gate_id,account_generation,profile_generation,selector_slot_id,selection_revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, binding.FileID, binding.SourceID, binding.AccountID, binding.ProfileID, binding.Provider, binding.ProviderID, account.id, profile.id, capture.AccountGeneration, capture.ProfileGeneration, slot.id, binding.SelectionRevision)
	if err != nil {
		var constraint *pgconn.PgError
		if errors.As(err, &constraint) && constraint.Code == "23505" {
			return ErrBindingConflict
		}
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE catalog_selector_slots SET selection_revision=$2,last_used_at=now() WHERE id=$1`, slot.id, binding.SelectionRevision); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresRepository) FindBinding(ctx context.Context, actor Actor, selection Selection, file uuid.UUID, capture ActorCapture) (Binding, bool, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return Binding{}, false, err
	}
	defer rollbackCatalogTx(tx)
	account, profile, err := s.capturedGates(ctx, tx, actor, capture)
	if err != nil {
		return Binding{}, false, err
	}
	aliases, err := s.Keys.selectorAliases(actor, capture, selection)
	if err != nil {
		return Binding{}, false, ErrActorUnknown
	}
	slot, found, err := lookupSelector(ctx, tx, aliases)
	if err != nil {
		return Binding{}, false, err
	}
	if !found || slot.revision == 0 {
		return Binding{}, false, nil
	}
	if slot.accountGate != account.id || slot.profileGate != profile.id || slot.ActorCapture != capture {
		return Binding{}, false, ErrActorUnknown
	}
	binding := Binding{Actor: actor, ActorCapture: capture, Selection: selection, FileID: file, SelectorSlotID: slot.id, SelectionRevision: slot.revision}
	err = tx.QueryRow(ctx, `SELECT source_id FROM catalog_asset_bindings WHERE file_id=$1 AND account_id=$2 AND profile_id=$3 AND provider=$4 AND provider_id=$5 AND selector_slot_id=$6 AND selection_revision=$7 AND account_generation=$8 AND profile_generation=$9`, file, actor.AccountID, actor.ProfileID, selection.Provider, selection.ProviderID, slot.id, slot.revision, capture.AccountGeneration, capture.ProfileGeneration).Scan(&binding.SourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Binding{}, false, nil
	}
	if err != nil {
		return Binding{}, false, err
	}
	if binding.SourceID == uuid.Nil {
		return Binding{}, false, ErrActorUnknown
	}
	if err = tx.Commit(ctx); err != nil {
		return Binding{}, false, err
	}
	return binding, true, nil
}
