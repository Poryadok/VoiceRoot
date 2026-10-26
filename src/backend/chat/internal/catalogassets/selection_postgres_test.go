package catalogassets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestCatalogCurrentSelectionRejectsSupersededFile(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	_, path, _, _ := runtime.Caller(0)
	up, err := os.ReadFile(filepath.Join(filepath.Dir(path), "..", "..", "..", "migrations", "chat_db", "000028_catalog_assets.up.sql"))
	require.NoError(t, err)
	ordered, err := os.ReadFile(filepath.Join(filepath.Dir(path), "..", "..", "..", "migrations", "chat_db", "000029_catalog_selection_cache.up.sql"))
	require.NoError(t, err)
	up = append(append(up, '\n'), ordered...)
	pool := integrationtest.StartPostgres(t, ctx, "chat_catalog_selection", "")
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	keys, err := NewActorKeyring("one", map[string][]byte{"one": bytes.Repeat([]byte{9}, 32)})
	require.NoError(t, err)
	repo := &PostgresRepository{Pool: pool, Keys: keys, Observer: &postgresActorObserver{state: ProfileLive, floor: 1}}
	require.NoError(t, repo.Preflight(ctx))
	actor := Actor{AccountID: uuid.New(), ProfileID: uuid.New()}
	capture, err := repo.CaptureActor(ctx, actor, true)
	require.NoError(t, err)
	a := Binding{Actor: actor, ActorCapture: capture, Selection: Selection{Provider: "fixture", ProviderID: "one"}, SourceID: uuid.New(), FileID: uuid.New(), SelectionRevision: 1}
	a.SelectorSlotID, err = repo.CaptureSelection(ctx, actor, a.Selection, capture)
	require.NoError(t, err)
	b := a
	b.SourceID = uuid.New()
	b.FileID = uuid.New()
	b.SelectionRevision = 2
	require.NoError(t, repo.SaveBinding(ctx, a, capture))
	require.NoError(t, repo.SaveBinding(ctx, b, capture))
	_, found, err := repo.FindBinding(ctx, actor, a.Selection, a.FileID, capture)
	require.NoError(t, err)
	require.False(t, found, "A cannot remain current after File admitted B for the same actor/provider identity")
	require.ErrorIs(t, repo.SaveBinding(ctx, a, capture), ErrSelectionSuperseded, "delayed A must not replace B")
	current, found, err := repo.FindBinding(ctx, actor, b.Selection, b.FileID, capture)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, b, current)
	forged := b
	forged.FileID = uuid.New()
	forged.SourceID = uuid.New()
	require.ErrorIs(t, repo.SaveBinding(ctx, forged, capture), ErrBindingConflict, "equal revision requires identical tuple")
	require.NoError(t, repo.RetireExactBinding(ctx, a), "stale A retirement cannot remove B")
	_, found, err = repo.FindBinding(ctx, actor, b.Selection, b.FileID, capture)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, repo.RetireExactBinding(ctx, b))
	_, found, err = repo.FindBinding(ctx, actor, b.Selection, b.FileID, capture)
	require.NoError(t, err)
	require.False(t, found)
	newSlot, err := repo.CaptureSelection(ctx, actor, b.Selection, capture)
	require.NoError(t, err)
	require.NotEqual(t, b.SelectorSlotID, newSlot)
	require.ErrorIs(t, repo.SaveBinding(ctx, a, capture), ErrSelectorInvalidated, "a delayed response targets only its old incarnation")
	c := b
	c.SourceID = uuid.New()
	c.FileID = uuid.New()
	c.SelectionRevision = 3
	c.SelectorSlotID = newSlot
	require.NoError(t, repo.SaveBinding(ctx, c, capture), "fresh current proof may populate the new incarnation")
	require.NoError(t, repo.RetireExactBinding(ctx, b), "old B cleanup cannot remove the recreated slot")
	_, found, err = repo.FindBinding(ctx, actor, c.Selection, c.FileID, capture)
	require.NoError(t, err)
	require.True(t, found)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM catalog_asset_bindings`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_name IN ('catalog_selector_slots','catalog_selector_aliases') AND column_name IN ('account_id','profile_id','provider','provider_id','source_url','operation_id','request_json')`).Scan(&count))
	require.Zero(t, count)

	// Empty slots are bounded disposable cache, including an in-flight capture.
	// Eviction must not grant a delayed response permission to recreate its slot.
	repo.MaxSelectorSlots = 2
	empty := c
	empty.ProviderID = "empty"
	empty.SelectorSlotID, err = repo.CaptureSelection(ctx, actor, empty.Selection, capture)
	require.NoError(t, err)
	pressure := empty.Selection
	pressure.ProviderID = "pressure"
	_, err = repo.CaptureSelection(ctx, actor, pressure, capture)
	require.NoError(t, err)
	require.ErrorIs(t, repo.SaveBinding(ctx, empty, capture), ErrSelectorInvalidated)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM catalog_selector_slots`).Scan(&count))
	require.Equal(t, 2, count)
	fresh, err := repo.CaptureSelection(ctx, actor, empty.Selection, capture)
	require.NoError(t, err)
	require.NotEqual(t, empty.SelectorSlotID, fresh)
	empty.SelectorSlotID = fresh
	empty.FileID, empty.SourceID = uuid.New(), uuid.New()
	require.NoError(t, repo.SaveBinding(ctx, empty, capture), "fresh current File proof can hydrate a new incarnation")
	require.NoError(t, repo.EvictEmptySelections(ctx, 100))
	_, found, err = repo.FindBinding(ctx, actor, empty.Selection, empty.FileID, capture)
	require.NoError(t, err)
	require.True(t, found, "empty cleanup cannot erase a populated slot")
	rotated, err := NewActorKeyring("two", map[string][]byte{"one": bytes.Repeat([]byte{9}, 32), "two": bytes.Repeat([]byte{8}, 32)})
	require.NoError(t, err)
	repo.Keys = rotated
	require.NoError(t, repo.Preflight(ctx))
	rotatedSlot, err := repo.CaptureSelection(ctx, actor, empty.Selection, capture)
	require.NoError(t, err)
	require.Equal(t, fresh, rotatedSlot, "key rotation preserves the current incarnation")
	legacyFile := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO catalog_legacy_asset_bindings(file_id,source_id,account_id,profile_id,provider,provider_id,account_gate_id,profile_gate_id,account_generation,profile_generation) SELECT $1,$2,account_id,profile_id,provider,provider_id,account_gate_id,profile_gate_id,account_generation,profile_generation FROM catalog_asset_bindings WHERE file_id=$3`, legacyFile, uuid.New(), empty.FileID)
	require.NoError(t, err)
	_, found, err = repo.FindBinding(ctx, actor, empty.Selection, legacyFile, capture)
	require.NoError(t, err)
	require.False(t, found, "legacy unordered records are never current authority")
	require.NoError(t, repo.PurgeAccount(ctx, actor.AccountID, uuid.New()))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM catalog_selector_slots`).Scan(&count))
	require.Zero(t, count, "purge removes both empty and populated cache metadata")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM catalog_legacy_asset_bindings`).Scan(&count))
	require.Zero(t, count)
	require.Error(t, repo.SaveBinding(ctx, empty, capture), "purged generation cannot reinstall a captured response")
}
