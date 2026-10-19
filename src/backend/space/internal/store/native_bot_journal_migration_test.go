package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestNativeBotJournalForward42CancellationTombstone(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL native Bot journal migration")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Bot journal upgrade", "", "public")
	require.NoError(t, err)
	db.Run(true, "up")
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_bot_installation_heads WHERE space_id=$1`, space.ID).Scan(&count))
	require.Zero(t, count, "migration never adopts a legacy source as a Bot installation")
	bot, profile, install, operation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err = db.Pool.Exec(ctx, `INSERT INTO space_bot_installation_attempts(install_operation_id,installation_id,space_id,bot_id,profile_id,generation,state)
 VALUES($1,$2,$3,$4,$5,1,'CANCELED')`, operation, install, space.ID, bot, profile)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `INSERT INTO space_bot_installation_heads(space_id,bot_id,profile_id,installation_id,generation,retired_through,install_operation_id,state)
 VALUES($1,$2,$3,$4,1,1,$5,'CANCELED')`, space.ID, bot, profile, install, operation)
	require.NoError(t, err, "cancel-before-add preserves a terminal installation tombstone without membership")
	_, err = db.Pool.Exec(ctx, `UPDATE space_bot_installation_heads SET state='INSTALLED',retired_through=0,membership_generation=1 WHERE space_id=$1 AND bot_id=$2`, space.ID, bot)
	require.Error(t, err, "same-generation cancellation cannot be revived")
	_, err = db.Pool.Exec(ctx, `DELETE FROM space_bot_installation_heads WHERE space_id=$1`, space.ID)
	require.Error(t, err, "installation floor survives ordinary cleanup")
	_, err = db.Pool.Exec(ctx, `TRUNCATE space_bot_installation_heads`)
	require.Error(t, err)
	var member bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space.ID, profile).Scan(&member))
	require.False(t, member)
}
