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
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

// Exercise complete deployment catalogs, including driver-managed dirty state.
func TestAuthorityRevisionActualMigratorUpgradesBothOwnerCatalogs(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and pinned migration driver")
	}
	integrationtest.ConfigureDockerTesting()
	for _, owner := range []string{"space", "role", "role_sdk"} {
		t.Run(owner, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			pg, err := postgres.Run(ctx, integrationtest.PostgresImage, postgres.BasicWaitStrategies(), postgres.WithDatabase("authority_driver"), postgres.WithUsername("migration_fixture"), postgres.WithPassword("fixture_only"))
			if pg != nil {
				t.Cleanup(func() {
					cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					require.True(t, pg.Terminate(cleanup) == nil, "terminate owned authority PostgreSQL")
				})
			}
			require.True(t, err == nil, "start owned authority PostgreSQL")
			dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
			require.True(t, err == nil, "resolve private fixture connection")
			pool, err := pgxpool.New(ctx, strings.Replace(dsn, "localhost", "127.0.0.1", 1))
			require.True(t, err == nil, "connect private fixture")
			t.Cleanup(pool.Close)
			privateURL := &url.URL{Scheme: "postgres", User: url.UserPassword("migration_fixture", "fixture_only"), Host: "127.0.0.1:5432", Path: "/authority_driver", RawQuery: "sslmode=disable"}
			directoryOwner := owner
			if owner == "role_sdk" {
				directoryOwner = "role"
			}
			directory := filepath.Join(repoRoot(t), "src/backend/migrations", directoryOwner+"_db")
			files, err := os.ReadDir(directory)
			require.NoError(t, err)
			copied := []testcontainers.ContainerFile{}
			for _, file := range files {
				if strings.HasSuffix(file.Name(), ".sql") {
					copied = append(copied, testcontainers.ContainerFile{HostFilePath: filepath.Join(directory, file.Name()), ContainerFilePath: "/migrations/" + file.Name(), FileMode: 0644})
				}
			}
			migrate := func(wantSuccess bool, arguments ...string) {
				t.Helper()
				command := append([]string{"-path", "/migrations", "-database", privateURL.String()}, arguments...)
				process, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "migrate/migrate:v4.18.1", NetworkMode: container.NetworkMode("container:" + pg.GetContainerID()), Files: copied, Cmd: command, WaitingFor: wait.ForExit()}, Started: true})
				if process != nil {
					defer func() {
						cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						require.True(t, process.Terminate(cleanup) == nil, "terminate owned migration process")
					}()
				}
				require.True(t, err == nil, "start/wait for pinned driver")
				state, err := process.State(ctx)
				require.True(t, err == nil, "read migration process state")
				if wantSuccess {
					require.Equal(t, 0, state.ExitCode, "driver upgrade must complete")
				} else {
					require.NotEqual(t, 0, state.ExitCode, "driver must refuse unsafe downgrade")
				}
			}
			previous, current, table, epoch := "23", int64(24), "space_voice_access_epochs", "access_epoch"
			activationBump := int64(1)
			if owner == "role" {
				previous, current, table, epoch = "14", 15, "role_voice_policy_epochs", "policy_epoch"
			}
			if owner == "role_sdk" {
				previous, current, table, epoch = "15", 16, "role_voice_policy_epochs", "policy_epoch"
				activationBump = 0
			}
			migrate(true, "goto", previous)
			space := uuid.New()
			if owner == "space" {
				_, err = pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'driver scope',$2)`, space, uuid.New())
			} else {
				_, err = pool.Exec(ctx, `INSERT INTO roles(space_id,name) VALUES($1,'driver scope')`, space)
			}
			require.NoError(t, err)
			var before, after int64
			require.NoError(t, pool.QueryRow(ctx, "SELECT "+epoch+" FROM "+table+" WHERE space_id=$1", space).Scan(&before))
			migrate(true, "up", "1")
			var version int64
			var dirty bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
			require.Equal(t, current, version)
			require.False(t, dirty)
			require.NoError(t, pool.QueryRow(ctx, "SELECT "+epoch+" FROM "+table+" WHERE space_id=$1", space).Scan(&after))
			require.Equal(t, before+activationBump, after)
			if owner == "role_sdk" {
				var sdk int64
				require.NoError(t, pool.QueryRow(ctx, `SELECT revision FROM role_sdk_authority_revision WHERE singleton`).Scan(&sdk))
				require.Equal(t, int64(1), sdk)
				reader := &RoleStore{Pool: pool}
				revision, err := reader.ReadAuthorityRevision(ctx, &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String()})
				require.NoError(t, err)
				require.Equal(t, uint64(before+sdk), revision)
			}
			migrate(false, "down", "1")
			require.NoError(t, pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
			require.True(t, dirty, "refused downgrade must retain maintenance state")
			require.NoError(t, pool.QueryRow(ctx, "SELECT "+epoch+" FROM "+table+" WHERE space_id=$1", space).Scan(&after))
			require.Equal(t, before+activationBump, after, "refused rollback must preserve floors")
			if owner == "role_sdk" {
				var sdk int64
				require.NoError(t, pool.QueryRow(ctx, `SELECT revision FROM role_sdk_authority_revision WHERE singleton`).Scan(&sdk))
				require.Equal(t, int64(1), sdk)
			}
		})
	}
}
