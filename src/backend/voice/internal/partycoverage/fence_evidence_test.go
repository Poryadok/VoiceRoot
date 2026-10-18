package partycoverage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestPostgresEvidenceRequiresActualRoleAndDatabaseFence(t *testing.T) {
	ctx, c, _ := coverageDB(t)
	name := "evidence_old_" + uuid.NewString()[:8]
	_, err := c.Exec(ctx, "CREATE ROLE "+pgx.Identifier{name}.Sanitize()+" LOGIN")
	require.NoError(t, err)
	defer c.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{name}.Sanitize())
	tx, err := c.Begin(ctx)
	require.NoError(t, err)
	identity, err := ObserveDatabaseIdentity(ctx, tx)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	var oid uint32
	require.NoError(t, c.QueryRow(ctx, `SELECT oid FROM pg_roles WHERE rolname=$1`, name).Scan(&oid))
	binding := PostgresRoleBinding{identity, oid, name}
	_, err = ObservePostgresFence(ctx, c, binding)
	require.Error(t, err, "LOGIN role is not retired")
	require.NoError(t, RetirePostgresRole(ctx, c, name))
	evidence, err := ObservePostgresFence(ctx, c, binding)
	require.NoError(t, err)
	require.Equal(t, binding, evidence.Binding)
	require.NotZero(t, evidence.AuthorityHash)
	require.NotEmpty(t, evidence.AuthorityBytes)
	again, err := ObservePostgresFence(ctx, c, binding)
	require.NoError(t, err)
	require.Equal(t, evidence, again)
	wrong := binding
	wrong.RoleOID++
	_, err = ObservePostgresFence(ctx, c, wrong)
	require.Error(t, err)
	wrong = binding
	wrong.DatabaseIdentity[0]++
	_, err = ObservePostgresFence(ctx, c, wrong)
	require.Error(t, err)
	_, err = c.Exec(ctx, "ALTER ROLE "+pgx.Identifier{name}.Sanitize()+" LOGIN")
	require.NoError(t, err)
	_, err = ObservePostgresFence(ctx, c, binding)
	require.Error(t, err, "recovery must inspect current postcondition")
}

func TestRedisEvidenceRequiresSameActualProcessAndPersistedFence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin := redis.NewClient(&redis.Options{Addr: testRedisAddress(t)})
	defer admin.Close()
	name := "evidence_old_" + uuid.NewString()[:8]
	require.NoError(t, admin.Do(ctx, "ACL", "SETUSER", name, "on", ">isolated-test-only", "~evidence:*", "+get", "+set").Err())
	defer admin.Do(context.Background(), "ACL", "DELUSER", name)
	binding, err := ObserveRedisProcess(ctx, admin)
	require.NoError(t, err)
	_, err = ObserveRedisFence(ctx, admin, binding, name)
	require.Error(t, err, "enabled old client is not retired")
	require.NoError(t, RetireRedisUser(ctx, admin, name))
	evidence, err := ObserveRedisFence(ctx, admin, binding, name)
	require.NoError(t, err)
	require.Equal(t, binding, evidence.Binding)
	require.NotEmpty(t, evidence.ACLFilePath)
	require.NotZero(t, evidence.PersistedUserRulesHash)
	require.NotEmpty(t, evidence.PersistedUserRulesBytes)
	require.NotContains(t, string(evidence.PersistedUserRulesBytes), "passwords")
	require.Equal(t, "OK", evidence.ACLSaveResult)
	again, err := ObserveRedisFence(ctx, admin, binding, name)
	require.NoError(t, err)
	require.Equal(t, evidence, again)
	wrong := binding
	wrong.RunID = "different-resource-process"
	_, err = ObserveRedisFence(ctx, admin, wrong, name)
	require.Error(t, err)
	require.NoError(t, admin.Do(ctx, "ACL", "SETUSER", name, "on").Err())
	_, err = ObserveRedisFence(ctx, admin, binding, name)
	require.Error(t, err, "stored evidence cannot replace resource observation")
	privileged := "evidence_admin_" + uuid.NewString()[:8]
	require.NoError(t, admin.Do(ctx, "ACL", "SETUSER", privileged, "off", ">isolated-test-only", "~*", "+@all").Err())
	defer admin.Do(context.Background(), "ACL", "DELUSER", privileged)
	_, err = ObserveRedisFence(ctx, admin, binding, privileged)
	require.Error(t, err, "off administrator is not an inventoried ordinary writer fence")
}
