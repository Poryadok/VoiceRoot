package sourcecustody

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
	"voice/backend/user/internal/identityassociation"
)

func existingMigrationFixture(t *testing.T, version uint) (context.Context, *pgxpool.Pool, *pgxpool.Pool, CreationRequest, PhysicalSource, uuid.UUID, string) {
	t.Helper()
	ctx, control, admin, request := creationFixture(t)
	physical, id, dir := prepareExistingMigration(t, ctx, control, admin, request, version)
	return ctx, control, admin, request, physical, id, dir
}

func prepareExistingMigration(t *testing.T, ctx context.Context, control, admin *pgxpool.Pool, request CreationRequest, version uint) (PhysicalSource, uuid.UUID, string) {
	t.Helper()
	_, err := admin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{request.Database}.Sanitize()+` TEMPLATE template0`)
	require.NoError(t, err)
	config := admin.Config().Copy()
	config.ConnConfig.Database = request.Database
	dir, _ := migrationFixture(t)
	archive, err := migrationArchive(dir)
	require.NoError(t, err)
	src, err := iofs.New(archive, ".")
	require.NoError(t, err)
	db := stdlib.OpenDB(*config.ConnConfig)
	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{DatabaseName: request.Database})
	require.NoError(t, err)
	m, err := migrate.NewWithInstance("reviewed-user", src, request.Database, driver)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(version))
	sourceErr, databaseErr := m.Close()
	require.NoError(t, sourceErr)
	require.NoError(t, databaseErr)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	id := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'retained','1234','Retained profile')`, id, uuid.New())
	require.NoError(t, err)
	pool.Close()
	physical := PhysicalSource{SystemIdentifier: request.SourceSystemID, Database: request.Database}
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname=$1`, request.Database).Scan(&physical.DatabaseOID))
	require.NoError(t, QuarantineExistingSource(ctx, control, admin, request, physical))
	_, err = admin.Exec(ctx, `REVOKE CONNECT ON DATABASE `+pgx.Identifier{request.Database}.Sanitize()+` FROM PUBLIC`)
	require.NoError(t, err)
	return physical, id, dir
}

func TestMigrateQuarantinedSourcePreservesExistingProfiles(t *testing.T) {
	ctx, control, admin, request, physical, id, dir := existingMigrationFixture(t, 21)
	_, keys := migrationFixture(t)
	require.NoError(t, MigrateQuarantinedSource(ctx, control, admin, request, physical, dir, keys))
	var phase, state string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "EXISTING_INITIALIZED_CLOSED", phase)
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	var allowed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	_, err := admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{request.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	config := admin.Config().Copy()
	config.ConnConfig.Database = request.Database
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer pool.Close()
	var name string
	var account uuid.UUID
	var guards bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT display_name,account_id FROM profiles WHERE id=$1`, id).Scan(&name, &account))
	require.Equal(t, "Retained profile", name)
	require.NoError(t, pool.QueryRow(ctx, `SELECT guards_active FROM identity_meta WHERE singleton`).Scan(&guards))
	require.False(t, guards)
	var history, generation uuid.UUID
	require.NoError(t, control.QueryRow(ctx, `SELECT history_generation,user_source_generation FROM user_custody_state WHERE singleton`).Scan(&history, &generation))
	store, err := identityassociation.NewStore(pool, keys, history)
	require.NoError(t, err)
	// Fixture-only activation demonstrates retained-row behavior; it is not a
	// writer qualification or authority to activate a deployment.
	require.NoError(t, store.ActivateGuards(ctx))
	digest, err := keys.CustodyDigest()
	require.NoError(t, err)
	pin := identityassociation.PhysicalSourceIdentity{SystemIdentifier: physical.SystemIdentifier, Database: physical.Database, DatabaseOID: physical.DatabaseOID}
	wrong := pin
	wrong.DatabaseOID++
	var headBefore, headAfter uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&headBefore))
	_, err = store.CaptureProfilesForPhysicalSource(ctx, []uuid.UUID{id}, history, generation, digest, wrong)
	require.Error(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&headAfter))
	require.Equal(t, headBefore, headAfter)
	result, err := store.CaptureProfilesForPhysicalSource(ctx, []uuid.UUID{id}, history, generation, digest, pin)
	require.NoError(t, err)
	require.Equal(t, identityassociation.CaptureAssociated, result.Observations[id].Status)
	require.Equal(t, account, result.Observations[id].AccountID)
}

func TestMigrateQuarantinedSourceRejectsOtherVersion(t *testing.T) {
	ctx, control, admin, request, physical, _, dir := existingMigrationFixture(t, 20)
	_, keys := migrationFixture(t)
	require.Error(t, MigrateQuarantinedSource(ctx, control, admin, request, physical, dir, keys))
	var allowed bool
	var phase string
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "MIGRATING", phase)
}

func TestMigrateQuarantinedSourceRejectsDirtySchema(t *testing.T) {
	ctx, control, admin, request, physical, _, dir := existingMigrationFixture(t, 21)
	_, err := admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{request.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	config := admin.Config().Copy()
	config.ConnConfig.Database = request.Database
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET dirty=true`)
	require.NoError(t, err)
	pool.Close()
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{request.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	_, keys := migrationFixture(t)
	require.Error(t, MigrateQuarantinedSource(ctx, control, admin, request, physical, dir, keys))
	var allowed, stateExists bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
	require.NoError(t, control.QueryRow(ctx, `SELECT to_regclass('public.user_custody_state') IS NOT NULL`).Scan(&stateExists))
	require.False(t, stateExists)
}

func TestMigrateExistingCommandOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	controlURL, control := recoveryTLSDatabase(t, ctx, "existing-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "existing-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, _, dir := prepareExistingMigration(t, ctx, control, admin, request, 21)
	keyDir := t.TempDir()
	key := bytes.Repeat([]byte{0x71}, 32)
	fingerprint := sha256.Sum256(key)
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "key.bin"), key, 0600))
	manifest := filepath.Join(keyDir, "manifest.json")
	require.NoError(t, os.WriteFile(manifest, []byte(fmt.Sprintf(`{"purpose":"voice.user.identity-association.v1","revision":1,"active_version":1,"versions":[{"version":1,"sha256":"%x","file":"key.bin"}]}`, fingerprint)), 0600))
	binary := filepath.Join(t.TempDir(), "sourcecustody.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../../cmd/sourcecustody")
	output, err := build.CombinedOutput()
	require.NoError(t, err, string(output))
	env := append(os.Environ(), "CUSTODY_CONTROL_DATABASE_URL="+controlURL, "CUSTODY_SOURCE_ADMIN_DATABASE_URL="+adminURL,
		"CUSTODY_CONTROL_SYSTEM_ID="+request.ControlSystemID, "CUSTODY_SOURCE_SYSTEM_ID="+request.SourceSystemID,
		"CUSTODY_SOURCE_DATABASE="+request.Database,
		"CUSTODY_SOURCE_ID="+request.SourceID, "CUSTODY_EPOCH="+strconv.FormatInt(request.Epoch, 10),
		"CUSTODY_USER_MIGRATIONS_DIR="+dir, "CUSTODY_USER_KEY_MANIFEST="+manifest, "CUSTODY_USER_KEYS_DIR="+keyDir)
	wrong := exec.CommandContext(ctx, binary, "migrate-existing")
	wrong.Env = append(append([]string(nil), env...), "CUSTODY_SOURCE_DATABASE_OID="+strconv.FormatUint(uint64(physical.DatabaseOID)+1, 10))
	_, err = wrong.CombinedOutput()
	require.Error(t, err)
	var phase string
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "QUARANTINED_EXISTING", phase)
	command := exec.CommandContext(ctx, binary, "migrate-existing")
	command.Env = append(env, "CUSTODY_SOURCE_DATABASE_OID="+strconv.FormatUint(uint64(physical.DatabaseOID), 10))
	output, err = command.CombinedOutput()
	require.NoError(t, err, string(output))
	var state string
	var allowed bool
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "EXISTING_INITIALIZED_CLOSED", phase)
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
	require.False(t, allowed)
}
