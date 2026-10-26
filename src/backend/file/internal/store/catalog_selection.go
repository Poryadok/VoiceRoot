package store

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/file/internal/catalogasset"
)

// admitCatalogSelectionTx runs after operation replay/fence checks while the
// stable account advisory lock and exact actor gates are held. Only a NEW
// admission advances the aggregate, atomically with its pending File/receipt.
func admitCatalogSelectionTx(ctx context.Context, tx pgx.Tx, imp CatalogImport, c CatalogActorCapture, keys *catalogasset.ReplayKeys) (uuid.UUID, int64, error) {
	digests, err := keys.Selection(imp.Actor, c.AccountGeneration, c.ProfileGeneration, imp.ChatAccountGeneration, imp.ChatProfileGeneration, imp.Provider, imp.ProviderID)
	if err != nil {
		return uuid.Nil, 0, err
	}
	var head uuid.UUID
	var revision int64
	for _, d := range digests {
		var id uuid.UUID
		var n int64
		err = tx.QueryRow(ctx, `SELECT id,revision FROM file_catalog_selection_heads WHERE key_version=$1 AND selector_hmac=$2 FOR UPDATE`, d.KeyVersion, d.HMAC).Scan(&id, &n)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return uuid.Nil, 0, err
		}
		if head != uuid.Nil || n <= 0 || n == math.MaxInt64 {
			return uuid.Nil, 0, catalogasset.ErrAdmission
		}
		head, revision = id, n
	}
	if head == uuid.Nil {
		head, revision = uuid.New(), 1
		_, err = tx.Exec(ctx, `INSERT INTO file_catalog_selection_heads(id,key_version,selector_hmac,revision,current_source_id,current_file_id) VALUES($1,$2,$3,$4,$5,$6)`, head, digests[0].KeyVersion, digests[0].HMAC, revision, imp.SourceID, imp.FileID)
	} else {
		revision++
		_, err = tx.Exec(ctx, `UPDATE file_catalog_selection_heads SET revision=$2,current_source_id=$3,current_file_id=$4,terminal_fence_id=NULL WHERE id=$1`, head, revision, imp.SourceID, imp.FileID)
	}
	return head, revision, err
}

// readCatalogSelectionTx validates the retained proof and the head's structural
// consistency. A retired current generation still reports its highwater; it
// never falls back to a previous source. Admission additionally requires current.
func readCatalogSelectionTx(ctx context.Context, tx pgx.Tx, head uuid.UUID, revision int64, source, file uuid.UUID) (int64, bool, error) {
	if head == uuid.Nil || revision <= 0 {
		return 0, false, ErrCatalogTerminal
	}
	var current int64
	var sourceID, fileID, fence *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT revision,current_source_id,current_file_id,terminal_fence_id FROM file_catalog_selection_heads WHERE id=$1 FOR SHARE`, head).Scan(&current, &sourceID, &fileID, &fence)
	if err != nil {
		return 0, false, err
	}
	if current < revision || (sourceID == nil) != (fileID == nil) || (sourceID == nil) == (fence == nil) {
		return 0, false, catalogasset.ErrAdmission
	}
	if sourceID != nil && (*sourceID == uuid.Nil || *fileID == uuid.Nil) || fence != nil && *fence == uuid.Nil {
		return 0, false, catalogasset.ErrAdmission
	}
	if current == revision && sourceID != nil && (*sourceID != source || *fileID != file) {
		return 0, false, catalogasset.ErrAdmission
	}
	return current, current == revision && sourceID != nil, nil
}
