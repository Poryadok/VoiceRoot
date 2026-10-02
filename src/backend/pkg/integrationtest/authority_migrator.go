package integrationtest

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type SourceMigrationFixture struct {
	Pool *pgxpool.Pool
	Run  func(wantSuccess bool, arguments ...string)
}

// NewSourceMigrationFixture applies real driver transitions on a private owned
// PostgreSQL instance. Failure reporting excludes its connection credentials.
func NewSourceMigrationFixture(t *testing.T, ctx context.Context, directory string) SourceMigrationFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL and pinned migration driver")
	}
	ConfigureDockerTesting()
	pg, err := postgres.Run(ctx, PostgresImage, postgres.BasicWaitStrategies(), postgres.WithDatabase("authority_source"), postgres.WithUsername("source_fixture"), postgres.WithPassword("fixture_only"))
	if pg != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			require.True(t, pg.Terminate(cleanup) == nil, "terminate owned source database")
		})
	}
	require.True(t, err == nil, "start owned source PostgreSQL")
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.True(t, err == nil, "resolve private connection")
	pool, err := pgxpool.New(ctx, strings.Replace(dsn, "localhost", "127.0.0.1", 1))
	require.True(t, err == nil, "connect private owning database")
	t.Cleanup(pool.Close)
	files, err := os.ReadDir(directory)
	require.NoError(t, err)
	copied := []testcontainers.ContainerFile{}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") {
			copied = append(copied, testcontainers.ContainerFile{HostFilePath: filepath.Join(directory, file.Name()), ContainerFilePath: "/migrations/" + file.Name(), FileMode: 0644})
		}
	}
	privateURL := &url.URL{Scheme: "postgres", User: url.UserPassword("source_fixture", "fixture_only"), Host: "127.0.0.1:5432", Path: "/authority_source", RawQuery: "sslmode=disable"}
	run := func(wantSuccess bool, args ...string) {
		t.Helper()
		command := append([]string{"-path", "/migrations", "-database", privateURL.String()}, args...)
		process, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "migrate/migrate:v4.18.1", NetworkMode: container.NetworkMode("container:" + pg.GetContainerID()), Files: copied, Cmd: command, WaitingFor: wait.ForExit()}, Started: true})
		if process != nil {
			defer func() {
				cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
				defer stop()
				require.True(t, process.Terminate(cleanup) == nil, "terminate owned source migration process")
			}()
		}
		require.True(t, err == nil, "start and wait for pinned source migrator")
		state, err := process.State(ctx)
		require.True(t, err == nil, "read pinned migrator result")
		if wantSuccess {
			require.Equal(t, 0, state.ExitCode)
		} else {
			require.NotEqual(t, 0, state.ExitCode)
		}
	}
	return SourceMigrationFixture{Pool: pool, Run: run}
}
