package identityassociation

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNativeCapturedAccountInventoryIncludesArchivedAndRemovedFactsOnly(t *testing.T) {
	pool, first, account := nativeDatabase(t)
	store, keys := nativeStore(t, pool)
	ctx := context.Background()
	removed, uncaptured, foreign, foreignAccount := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name,is_primary) VALUES($1,$2,'removed','4321','Removed',false),($3,$2,'uncaptured','4322','Uncaptured',false),($4,$5,'foreignaccount','4323','Foreign',true)`, removed, account, uncaptured, foreign, foreignAccount)
	require.NoError(t, err)
	require.NoError(t, store.ActivateGuards(ctx))
	captured, err := store.CaptureProfiles(ctx, []uuid.UUID{first, removed, foreign})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE profiles SET deleted_at=clock_timestamp() WHERE id=$1`, first)
	require.NoError(t, err)
	prepared, err := keys.PrepareSelectors(removed, account)
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	terminal, err := store.AttachParentTx(ctx, tx, prepared, captured.Observations[removed].Fact.Revision, dueParentForTest(account))
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, store.PrepareTerminalDeleteTx(ctx, tx, prepared, terminal.Revision))
	_, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, removed)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	lookup, err := keys.PrepareAccountLookup(account)
	require.NoError(t, err)
	require.NotContains(t, fmt.Sprintf("%+v %#v", lookup, lookup), account.String())
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	inventory, err := store.ReadCapturedAccountInventoryTx(ctx, tx, lookup)
	require.NoError(t, err)
	require.Len(t, inventory.Facts, 2, "uncaptured surviving row and unrelated account are not invented as captured history")
	seen := map[uuid.UUID]Fact{}
	for _, fact := range inventory.Facts {
		seen[fact.AssociationID] = fact
		require.LessOrEqual(t, fact.Revision, inventory.Cut.ThroughRevision)
	}
	require.True(t, seen[captured.Observations[first].Fact.AssociationID].Deleted)
	require.NotNil(t, seen[terminal.AssociationID].Terminal)
	require.NotContains(t, seen, captured.Observations[foreign].Fact.AssociationID)
	require.Zero(t, inventory.UnresolvedErasureIssued)
	require.Zero(t, inventory.VerifiedErasureCovered)
	require.NoError(t, tx.Commit(ctx))
	// A profile UUID is never reinterpreted as its owner's account UUID.
	wrongPurposeInput, err := keys.PrepareAccountLookup(first)
	require.NoError(t, err)
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	empty, err := store.ReadCapturedAccountInventoryTx(ctx, tx, wrongPurposeInput)
	require.NoError(t, err)
	require.Empty(t, empty.Facts)
	require.NoError(t, tx.Commit(ctx))
}
