package partycoverage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRetireRedisUserKillsQueuedWriterAndRestart(t *testing.T) {
	addr := testRedisAddress(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin := redis.NewClient(&redis.Options{Addr: addr})
	defer admin.Close()
	user := "coverage_old_" + uuid.NewString()[:8]
	key := "coverage-test:" + user
	require.NoError(t, admin.Do(ctx, "ACL", "SETUSER", user, "on", ">isolated-test-only", "~coverage-test:*", "+get", "+set", "+ping", "+multi", "+exec").Err())
	defer admin.Do(context.Background(), "ACL", "DELUSER", user)
	defer admin.Del(context.Background(), key)
	old := redis.NewClient(&redis.Options{Addr: addr, Username: user, Password: "isolated-test-only", MaxRetries: -1})
	defer old.Close()
	conn := old.Conn()
	defer conn.Close()
	require.NoError(t, conn.Ping(ctx).Err())
	require.NoError(t, connCommand(ctx, conn, "MULTI").Err())
	require.NoError(t, connCommand(ctx, conn, "SET", key, "paused-old-writer").Err())
	require.NoError(t, RetireRedisUser(ctx, admin, user))
	require.Error(t, connCommand(ctx, conn, "EXEC").Err(), "queued pre-cut write must not cross fence")
	require.Equal(t, int64(0), admin.Exists(ctx, key).Val())
	restarted := redis.NewClient(&redis.Options{Addr: addr, Username: user, Password: "isolated-test-only", MaxRetries: -1})
	defer restarted.Close()
	require.Error(t, restarted.Set(ctx, key, "restart", 0).Err())
	require.NoError(t, RetireRedisUser(ctx, admin, user))
}

func connCommand(ctx context.Context, c *redis.Conn, args ...interface{}) *redis.Cmd {
	cmd := redis.NewCmd(ctx, args...)
	_ = c.Process(ctx, cmd)
	return cmd
}

func TestRetireRedisUserRejectsDefaultAndAdministration(t *testing.T) {
	addr := testRedisAddress(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin := redis.NewClient(&redis.Options{Addr: addr})
	defer admin.Close()
	require.Error(t, RetireRedisUser(ctx, admin, "default"))
	user := "coverage_admin_" + uuid.NewString()[:8]
	require.NoError(t, admin.Do(ctx, "ACL", "SETUSER", user, "on", ">isolated-test-only", "~*", "+@all").Err())
	defer admin.Do(context.Background(), "ACL", "DELUSER", user)
	require.Error(t, RetireRedisUser(ctx, admin, user), "old credentials must not be able to restore their own ACL")
}
