package reportaudience

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestPostgresSnapshotCountsDistinctLiveProfileAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")))
	target, owner, other, archived := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name,is_primary,deleted_at,frozen_at)
	VALUES ($1,$2,'target','0001','Target',true,NULL,NULL),
	($3,$2,'sameowner','0002','Same owner',false,NULL,NULL),
	($4,$5,'other','0003','Other',true,NULL,NULL),
	($6,$5,'frozen','0004','Frozen',false,NULL,now()),
	($7,$8,'archived','0005','Archived',true,now(),NULL)`, target, owner, uuid.New(), uuid.New(), other, uuid.New(), uuid.New(), archived)
	require.NoError(t, err)
	source := PostgresSource{Pool: pool}
	snapshot, err := source.Snapshot(ctx, target)
	require.NoError(t, err)
	require.Equal(t, owner, snapshot.AccountID)
	require.Positive(t, snapshot.Revision)
	require.Equal(t, Live, snapshot.State)
	require.Equal(t, []uuid.UUID{other}, snapshot.Candidates)
	require.NoError(t, source.Recheck(ctx, snapshot.Binding))
	_, err = pool.Exec(ctx, `UPDATE profiles SET frozen_at=now() WHERE id=$1`, target)
	require.NoError(t, err)
	require.Error(t, source.Recheck(ctx, snapshot.Binding), "lifecycle revision changes invalidate the observation")
	current, err := source.Snapshot(ctx, target)
	require.NoError(t, err)
	require.Equal(t, Live, current.State, "frozen target profiles remain reportable and public-potential audience does not shrink")
	_, err = pool.Exec(ctx, `UPDATE profiles SET deleted_at=now() WHERE id=$1`, target)
	require.NoError(t, err)
	deleted, err := source.Snapshot(ctx, target)
	require.NoError(t, err)
	require.Equal(t, Deleted, deleted.State)
	require.Empty(t, deleted.Candidates)
	unknown := uuid.New()
	missing, err := source.Snapshot(ctx, unknown)
	require.NoError(t, err)
	require.Equal(t, Missing, missing.State)
	require.Equal(t, unknown, missing.ProfileID)
}
