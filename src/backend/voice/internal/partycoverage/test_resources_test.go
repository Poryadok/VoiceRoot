package partycoverage

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"voice/backend/pkg/integrationtest"
)

func testPostgresDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("real PostgreSQL integration in non-short suite")
	}
	if dsn := os.Getenv("PARTY_COVERAGE_TEST_POSTGRES"); dsn != "" {
		return dsn
	}
	pool := integrationtest.StartPostgres(t, context.Background(), "partycoverage", "")
	return pool.Config().ConnString()
}

func testRedisAddress(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("real Redis integration in non-short suite")
	}
	if addr := os.Getenv("PARTY_COVERAGE_TEST_REDIS"); addr != "" {
		return addr
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{Image: "redis:7.4-bookworm", ExposedPorts: []string{"6379/tcp"}, Cmd: []string{"sh", "-c", "touch /data/users.acl && exec redis-server --aclfile /data/users.acl --save '' --appendonly no"}, WaitingFor: wait.ForLog("Ready to accept connections")}, Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		require.NoError(t, c.Terminate(cleanup))
	})
	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	return net.JoinHostPort(host, port.Port())
}
