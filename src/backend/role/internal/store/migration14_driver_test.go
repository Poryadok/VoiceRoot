package store

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
	"voice/backend/pkg/integrationtest"
)

func TestRoleMigration14ActualDriverLoadsCatalogAndUpgradesHistoricalSources(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and pinned golang-migrate containers")
	}
	integrationtest.ConfigureDockerTesting()
	for _, source := range []string{"canonical13", "legacy13"} {
		t.Run(source, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			pg, err := postgres.Run(ctx, integrationtest.PostgresImage, postgres.BasicWaitStrategies(), postgres.WithDatabase("role14_driver"), postgres.WithUsername("migration_fixture"), postgres.WithPassword("fixture_only"))
			if pg != nil {
				t.Cleanup(func() {
					cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					require.True(t, pg.Terminate(cleanup) == nil, "terminate owned migration PostgreSQL fixture")
				})
			}
			require.True(t, err == nil, "start owned migration PostgreSQL fixture")
			dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
			require.True(t, err == nil, "resolve private PostgreSQL connection")
			pool, err := pgxpool.New(ctx, strings.Replace(dsn, "localhost", "127.0.0.1", 1))
			require.True(t, err == nil, "connect private PostgreSQL fixture")
			t.Cleanup(pool.Close)
			privateURL := &url.URL{Scheme: "postgres", User: url.UserPassword("migration_fixture", "fixture_only"), Host: "127.0.0.1:5432", Path: "/role14_driver", RawQuery: "sslmode=disable"}
			directory := filepath.Join(repoRoot(t), "src/backend/migrations/role_db")
			files, err := os.ReadDir(directory)
			require.NoError(t, err)
			copied := []testcontainers.ContainerFile{}
			for _, file := range files {
				if strings.HasSuffix(file.Name(), ".sql") {
					copied = append(copied, testcontainers.ContainerFile{HostFilePath: filepath.Join(directory, file.Name()), ContainerFilePath: "/migrations/" + file.Name(), FileMode: 0644})
				}
			}
			migrate := func(arguments ...string) {
				t.Helper()
				command := append([]string{"-path", "/migrations", "-database", privateURL.String()}, arguments...)
				process, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{
					Image: "migrate/migrate:v4.18.1", NetworkMode: container.NetworkMode("container:" + pg.GetContainerID()), Files: copied, Cmd: command, WaitingFor: wait.ForExit(),
				}, Started: true})
				if process != nil {
					defer func() {
						cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						require.True(t, process.Terminate(cleanup) == nil, "terminate owned migration process")
					}()
				}
				// Driver errors/logs can contain the DSN; report only fixed diagnostics.
				require.True(t, err == nil, "start/wait for pinned migration driver")
				state, err := process.State(ctx)
				require.True(t, err == nil, "read owned migration process state")
				require.Equal(t, 0, state.ExitCode, "pinned migration driver must accept source catalog and upgrade")
			}
			migrate("goto", "13")
			if source == "legacy13" {
				_, err = pool.Exec(ctx, `DROP TABLE game_session_grants,game_session_grant_sessions,game_session_grant_operations`)
				require.NoError(t, err)
				role14LegacyFence(t, ctx, pool)
				role14FenceEvidence(t, ctx, pool)
			}
			var version int64
			var dirty bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
			require.EqualValues(t, 13, version)
			require.False(t, dirty)
			before := ""
			if source == "legacy13" {
				before = role14Rows(t, ctx, pool, "role_space_deletion_fence_receipts")
			}
			migrate("up", "1")
			require.NoError(t, pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
			require.EqualValues(t, 14, version)
			require.False(t, dirty, "only successful actual migration clears dirty state")
			if source == "legacy13" {
				require.Equal(t, before, role14Rows(t, ctx, pool, "role_space_deletion_fence_receipts"))
			}
		})
	}
}
