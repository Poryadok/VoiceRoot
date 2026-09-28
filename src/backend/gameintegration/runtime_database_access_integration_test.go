package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

const gisRuntimeTestPassword = "test-only-gis-runtime-password"

func TestGISRuntimeDatabaseRoleCanWriteOnlyGISOwnedTables(t *testing.T) {
	ctx := context.Background()
	migrationDir := filepath.Join("..", "migrations", "game_integration_db")
	adminPool := integrationtest.StartPostgres(t, ctx, "game_integration_db", filepath.Join(migrationDir, "000001_init.up.sql"))

	t12Migration, err := os.ReadFile(filepath.Join(migrationDir, "000002_t12_registry_security.up.sql"))
	require.NoError(t, err)
	_, err = adminPool.Exec(ctx, string(t12Migration))
	require.NoError(t, err)

	_, err = adminPool.Exec(ctx, `CREATE ROLE gameintegration_runtime WITH LOGIN PASSWORD '`+gisRuntimeTestPassword+`' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT`)
	require.NoError(t, err)
	t10Migration, err := os.ReadFile(filepath.Join(migrationDir, "000003_t10_runtime_principal.up.sql"))
	require.NoError(t, err)
	_, err = adminPool.Exec(ctx, string(t10Migration))
	require.NoError(t, err)
	t11Migration, err := os.ReadFile(filepath.Join(migrationDir, "000004_t11_installation_bot_binding.up.sql"))
	require.NoError(t, err)
	_, err = adminPool.Exec(ctx, string(t11Migration))
	require.NoError(t, err)
	for _, name := range []string{"000005_t16_player_binding_authority.up.sql", "000006_t16_execution_permits.up.sql"} {
		migration, readErr := os.ReadFile(filepath.Join(migrationDir, name))
		require.NoError(t, readErr)
		_, err = adminPool.Exec(ctx, string(migration))
		require.NoError(t, err)
	}
	_, err = adminPool.Exec(ctx, `CREATE DATABASE auth_db`)
	require.NoError(t, err)

	adminAuthConfig := adminPool.Config().ConnConfig.Copy()
	adminAuthConfig.Database = "auth_db"
	authAdmin, err := pgx.ConnectConfig(ctx, adminAuthConfig)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authAdmin.Close(context.Background()) })
	_, err = authAdmin.Exec(ctx, `CREATE TABLE protected_records (id UUID PRIMARY KEY)`)
	require.NoError(t, err)

	adminConfig := adminPool.Config().ConnConfig
	gisURL := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword("gameintegration_runtime", gisRuntimeTestPassword),
		Host:     net.JoinHostPort(adminConfig.Host, strconv.Itoa(int(adminConfig.Port))),
		Path:     "/game_integration_db",
		RawQuery: "sslmode=disable",
	}
	values := map[string]string{
		"DATABASE_URL":                  gisURL.String(),
		"GAME_INTEGRATION_REDIS_ADDR":   "127.0.0.1:6379",
		"GAME_INTEGRATION_JWKS_URL":     "https://auth.example/jwks",
		"GAME_INTEGRATION_JWT_ISSUER":   "https://auth.example",
		"GAME_INTEGRATION_JWT_AUDIENCE": "voice",
	}
	cfg, err := loadConfig(func(name string) string { return values[name] })
	require.NoError(t, err)
	runtimePool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	require.NoError(t, err)
	t.Cleanup(runtimePool.Close)
	require.NoError(t, runtimePool.Ping(ctx))

	var runtimeRole, runtimeDatabase string
	require.NoError(t, runtimePool.QueryRow(ctx, `SELECT current_user, current_database()`).Scan(&runtimeRole, &runtimeDatabase))
	require.Equal(t, "gameintegration_runtime", runtimeRole)
	require.Equal(t, "game_integration_db", runtimeDatabase)
	var isSuperuser, canCreateDB, canCreateRole, canInherit, canBypassRLS, canReplicate bool
	require.NoError(t, runtimePool.QueryRow(ctx, `
		SELECT rolsuper, rolcreatedb, rolcreaterole, rolinherit, rolbypassrls, rolreplication
		FROM pg_roles WHERE rolname = current_user
	`).Scan(&isSuperuser, &canCreateDB, &canCreateRole, &canInherit, &canBypassRLS, &canReplicate))
	require.False(t, isSuperuser)
	require.False(t, canCreateDB)
	require.False(t, canCreateRole)
	require.False(t, canInherit)
	require.False(t, canBypassRLS)
	require.False(t, canReplicate)
	applicationID, environmentID := uuid.New(), uuid.New()
	_, err = runtimePool.Exec(ctx,
		`INSERT INTO applications (id, owner_account_id, name, status) VALUES ($1, $2, 'isolation-test', 'draft')`,
		applicationID, uuid.New())
	require.NoError(t, err, "GIS runtime credentials must write GIS-owned registry rows")
	_, err = runtimePool.Exec(ctx, `INSERT INTO environments (id, application_id, kind, status)
		VALUES ($1,$2,'sandbox','active')`, environmentID, applicationID)
	require.NoError(t, err)
	bindingID := uuid.New()
	_, err = runtimePool.Exec(ctx, `INSERT INTO player_bindings
		(binding_id, application_id, environment_id, provider, provider_subject_digest, account_id, actor_id, profile_id, device_id, status, authority_revision)
		VALUES ($1,$2,$3,'test-provider',$4,$5,$6,$7,$8,'active',1)`, bindingID, applicationID, environmentID,
		"hmac-sha256-v1:test:"+strings.Repeat("a", 64), uuid.New(), uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err, "GIS runtime credentials must write GIS-owned player binding rows")
	_, err = runtimePool.Exec(ctx, `INSERT INTO player_binding_execution_permits
		(permit_id, binding_id, operation_id, application_id, environment_id, binding_revision,
		 assertion_jti, assertion_sha256, expires_at, status)
		VALUES ($1,$2,$3,$4,$5,1,$6,$7,now()+interval '1 second','issued')`, uuid.New(), bindingID,
		uuid.New(), applicationID, environmentID, uuid.New(), make([]byte, 32))
	require.NoError(t, err, "GIS runtime credentials must write GIS-owned permit rows")
	_, err = adminPool.Exec(ctx, `CREATE TABLE future_registry_rows (id BIGSERIAL PRIMARY KEY)`)
	require.NoError(t, err)
	var nextID int64
	require.NoError(t, runtimePool.QueryRow(ctx, `SELECT nextval('future_registry_rows_id_seq')`).Scan(&nextID))
	require.EqualValues(t, 1, nextID, "default sequence privileges must cover sequences created by later GIS migrations")
	_, err = runtimePool.Exec(ctx, `INSERT INTO future_registry_rows DEFAULT VALUES`)
	require.NoError(t, err, "default table and sequence privileges must cover later GIS migrations")

	authURL := gisURL
	authURL.Path = "/auth_db"
	authPool, err := pgxpool.New(ctx, authURL.String())
	require.NoError(t, err)
	t.Cleanup(authPool.Close)
	require.NoError(t, authPool.Ping(ctx), "PostgreSQL may allow connection while the role has no service-table privileges")
	_, err = authPool.Exec(ctx, `INSERT INTO protected_records (id) VALUES ($1)`, uuid.New())
	var postgresErr *pgconn.PgError
	require.ErrorAs(t, err, &postgresErr)
	require.Equal(t, "42501", postgresErr.Code, fmt.Sprintf("cross-service writes must fail by privilege, got %v", err))
}
