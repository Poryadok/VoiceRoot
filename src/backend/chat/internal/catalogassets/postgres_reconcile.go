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
	rows, err := tx.Query(ctx, `SELECT file_id,source_id,account_id,profile_id,provider,provider_id,account_generation,profile_generation FROM catalog_asset_bindings WHERE file_id>$1 ORDER BY file_id LIMIT $2`, after, limit)
	if err != nil {
		return nil, err
	}
	var out []Binding
	for rows.Next() {
		var b Binding
		if err = rows.Scan(&b.FileID, &b.SourceID, &b.AccountID, &b.ProfileID, &b.Provider, &b.ProviderID, &b.AccountGeneration, &b.ProfileGeneration); err != nil {
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
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackCatalogTx(tx)
	if _, _, err = s.lockActor(ctx, tx, b.Actor); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM catalog_asset_bindings WHERE file_id=$1 AND source_id=$2 AND account_id=$3 AND profile_id=$4 AND provider=$5 AND provider_id=$6 AND account_generation=$7 AND profile_generation=$8`, b.FileID, b.SourceID, b.AccountID, b.ProfileID, b.Provider, b.ProviderID, b.AccountGeneration, b.ProfileGeneration)
	if err != nil {
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
	if _, err = tx.Exec(ctx, `DELETE FROM catalog_asset_bindings WHERE profile_gate_id=$1 AND profile_generation=$2`, profile.id, b.ProfileGeneration); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
