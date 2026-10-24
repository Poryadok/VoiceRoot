package permissions

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestAdmissionPermissionMigrationPreservesCustomDelegations(t *testing.T) {
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "admissionroles", "")
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "src", "backend", "migrations", "role_db")
	read := func(name string) string {
		raw, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		return string(raw)
	}
	_, err := pool.Exec(ctx, read("000001_init.up.sql"))
	require.NoError(t, err)
	space := uuid.New()
	for _, name := range []string{RoleOwner, RoleAdmin, RoleModerator, RoleMember, RoleGuest, "Custom"} {
		_, err = pool.Exec(ctx, `INSERT INTO roles(space_id,name,is_system,permissions) VALUES($1,$2,$3,1)`, space, name, name != "Custom")
		require.NoError(t, err)
	}
	up := read("000019_approve_join_requests.up.sql")
	down := read("000019_approve_join_requests.down.sql")
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)
	bit, err := MaskFor(SpaceApproveJoinRequests)
	require.NoError(t, err)
	for _, name := range []string{RoleOwner, RoleAdmin, RoleModerator, RoleMember, RoleGuest, "Custom"} {
		var mask uint64
		require.NoError(t, pool.QueryRow(ctx, `SELECT permissions FROM roles WHERE space_id=$1 AND name=$2`, space, name).Scan(&mask))
		require.EqualValues(t, 1, mask&1, "existing permissions preserved")
		require.Equal(t, name == RoleOwner || name == RoleAdmin || name == RoleModerator, mask&bit != 0, name)
	}
	_, err = pool.Exec(ctx, `UPDATE roles SET permissions=permissions | $2 WHERE space_id=$1 AND name='Custom'`, space, int64(bit))
	require.NoError(t, err)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, down)
	require.ErrorContains(t, err, "configured delegation prevents downgrade")
	_, rollbackErr := conn.Exec(ctx, "ROLLBACK")
	require.NoError(t, rollbackErr)
	conn.Release()
	var custom uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT permissions FROM roles WHERE space_id=$1 AND name='Custom'`, space).Scan(&custom))
	require.NotZero(t, custom&bit, "failed downgrade must not discard delegation")
}
