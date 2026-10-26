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
	b := a
	b.SourceID = uuid.New()
	b.FileID = uuid.New()
	b.SelectionRevision = 2
	require.NoError(t, repo.SaveBinding(ctx, a, capture))
	require.NoError(t, repo.SaveBinding(ctx, b, capture))
	_, found, err := repo.FindBinding(ctx, actor, a.Selection, a.FileID, capture)
	require.NoError(t, err)
	require.False(t, found, "A cannot remain current after File admitted B for the same actor/provider identity")
}
