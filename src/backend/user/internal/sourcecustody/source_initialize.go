package sourcecustody

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"voice/backend/user/internal/identityassociation"
)

const userMigrationDigest = "1ed126abaf97abb6e681ab02e6d1bb9d4403c0a370cca609e55a435fbdcccb4e"

// migrationArchive snapshots exact reviewed bytes before any source mutation.
func migrationArchive(dir string) (*zip.Reader, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, ErrUnavailable
	}
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			if !entry.Type().IsRegular() {
				return nil, ErrUnavailable
			}
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) != 22 {
		return nil, ErrUnavailable
	}
	hash := sha256.New()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for i, name := range names {
		if !strings.HasPrefix(name, fmt.Sprintf("%06d_", i+1)) {
			return nil, ErrUnavailable
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || len(data) == 0 || len(data) > 2<<20 || buffer.Len()+len(data) > 16<<20 {
			return nil, ErrUnavailable
		}
		var number [8]byte
		binary.BigEndian.PutUint32(number[:4], uint32(len(name)))
		_, _ = hash.Write(number[:4])
		_, _ = hash.Write([]byte(name))
		binary.BigEndian.PutUint64(number[:], uint64(len(data)))
		_, _ = hash.Write(number[:])
		_, _ = hash.Write(data)
		writer, err := archive.Create(name)
		if err != nil {
			return nil, ErrUnavailable
		}
		if _, err = writer.Write(data); err != nil {
			return nil, ErrUnavailable
		}
	}
	if hex.EncodeToString(hash.Sum(nil)) != userMigrationDigest || archive.Close() != nil {
		return nil, ErrUnavailable
	}
	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		return nil, ErrUnavailable
	}
	return reader, nil
}

// InitializeClosedSource migrates an independently created fresh source and
// records its key lineage. It never activates writer guards or opens capture.
func InitializeClosedSource(ctx context.Context, control, admin *pgxpool.Pool, request CreationRequest, physical PhysicalSource, migrationDir string, keys *identityassociation.Keyset) error {
	return initializeClosedSource(ctx, control, admin, request, physical, migrationDir, keys, false)
}

// MigrateQuarantinedSource upgrades a fenced existing version 21 database to
// the reviewed identity schema. It preserves existing rows, leaves guards and
// admission closed, and does not establish continuity or writer qualification.
func MigrateQuarantinedSource(ctx context.Context, control, admin *pgxpool.Pool, request CreationRequest, physical PhysicalSource, migrationDir string, keys *identityassociation.Keyset) error {
	return initializeClosedSource(ctx, control, admin, request, physical, migrationDir, keys, true)
}

func initializeClosedSource(ctx context.Context, control, admin *pgxpool.Pool, request CreationRequest, physical PhysicalSource, migrationDir string, keys *identityassociation.Keyset, existing bool) error {
	if ctx == nil || ctx.Err() != nil || control == nil || admin == nil || keys == nil || !sourceIDPattern.MatchString(request.SourceID) || request.Epoch <= 0 || !databaseName.MatchString(request.Database) || physical.Database != request.Database || physical.SystemIdentifier != request.SourceSystemID || physical.DatabaseOID == 0 || !systemID.MatchString(request.ControlSystemID) || !systemID.MatchString(request.SourceSystemID) || request.ControlSystemID == request.SourceSystemID {
		return ErrUnavailable
	}
	archive, err := migrationArchive(migrationDir)
	if err != nil {
		return ErrUnavailable
	}
	digest, err := keys.CustodyDigest()
	if err != nil || keys.Snapshot().Revision != 1 {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	source, err := admin.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer source.Release()
	checkSource := func(expectAllowed bool) error {
		var system, current string
		var oid uint32
		var allowed, owner, publicConnect bool
		if source.QueryRow(ctx, `SELECT system_identifier::text,current_database() FROM pg_control_system()`).Scan(&system, &current) != nil || system != physical.SystemIdentifier || current == physical.Database {
			return ErrUnavailable
		}
		if source.QueryRow(ctx, `SELECT oid,datallowconn,pg_get_userbyid(datdba)=current_user,EXISTS(SELECT 1 FROM aclexplode(COALESCE(datacl,acldefault('d',datdba))) a WHERE a.grantee=0 AND a.privilege_type='CONNECT') FROM pg_database WHERE datname=$1`, physical.Database).Scan(&oid, &allowed, &owner, &publicConnect) != nil || oid != physical.DatabaseOID || allowed != expectAllowed || !owner || publicConnect {
			return ErrUnavailable
		}
		var untrusted int
		if source.QueryRow(ctx, `SELECT count(*) FROM pg_roles r WHERE r.rolcanlogin AND NOT r.rolsuper AND r.rolname<>current_user AND has_database_privilege(r.oid,$1::oid,'CONNECT')`, physical.DatabaseOID).Scan(&untrusted) != nil || untrusted != 0 {
			return ErrUnavailable
		}
		return nil
	}
	if checkSource(false) != nil {
		return ErrUnavailable
	}
	var sessions, prepared int
	if source.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE datid=$1),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2)`, physical.DatabaseOID, physical.Database).Scan(&sessions, &prepared) != nil || sessions != 0 || prepared != 0 {
		return ErrUnavailable
	}
	owner, err := control.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	var locked bool
	if err = owner.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, leaderKey).Scan(&locked); err != nil || !locked {
		if err != nil {
			destroy(owner)
		} else {
			owner.Release()
		}
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		var unlocked bool
		if owner.QueryRow(cleanup, `SELECT pg_advisory_unlock($1::bigint)`, leaderKey).Scan(&unlocked) != nil || !unlocked {
			destroy(owner)
		} else {
			owner.Release()
		}
	}()
	var controlSystem, controlDatabase string
	var controlDatabaseOID uint32
	if owner.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&controlSystem, &controlDatabase, &controlDatabaseOID) != nil || controlSystem != request.ControlSystemID || controlSystem == physical.SystemIdentifier || controlDatabase == "" || controlDatabaseOID == 0 {
		return ErrUnavailable
	}
	// Keep the independently retained physical binding fixed through source
	// closure. The creation phase commits separately before each source action.
	pin, err := control.Begin(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = pin.Rollback(cleanup)
	}()
	var pinSystem, pinDatabase string
	var pinDatabaseOID uint32
	if pin.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&pinSystem, &pinDatabase, &pinDatabaseOID) != nil || pinSystem != controlSystem || pinDatabase != controlDatabase || pinDatabaseOID != controlDatabaseOID {
		return ErrUnavailable
	}
	var pinned PhysicalSource
	if pin.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM user_custody_physical_source WHERE source_id=$1 AND epoch=$2 FOR UPDATE`, request.SourceID, request.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != physical {
		return ErrUnavailable
	}
	previousPhase := "CREATED_CLOSED"
	if existing {
		previousPhase = "QUARANTINED_EXISTING"
	}
	tag, err := owner.Exec(ctx, `UPDATE user_custody_creation SET phase='MIGRATING',keyset_digest=$1 WHERE singleton AND phase=$8 AND source_id=$2 AND epoch=$3 AND control_system_id=$4 AND source_system_id=$5 AND database_name=$6 AND database_oid=$7 AND history_generation IS NULL AND user_source_generation IS NULL AND keyset_digest IS NULL AND to_regclass('public.user_custody_state') IS NULL`, digest[:], request.SourceID, request.Epoch, request.ControlSystemID, request.SourceSystemID, physical.Database, physical.DatabaseOID, previousPhase)
	if err != nil || tag.RowsAffected() != 1 {
		return ErrUnavailable
	}
	// On any later failure, attempt to close the source with independent cleanup
	// time. The durable MIGRATING/MIGRATED state never authorizes a retry/open.
	closed := false
	defer func() {
		if !closed {
			cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			_, _ = source.Exec(cleanup, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
			_, _ = source.Exec(cleanup, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datid=$1`, physical.DatabaseOID)
		}
	}()
	if checkSource(false) != nil {
		return ErrUnavailable
	}
	if _, err = source.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`); err != nil || checkSource(true) != nil {
		return ErrUnavailable
	}
	config := admin.Config().Copy()
	config.ConnConfig.Database = physical.Database
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = make(map[string]string)
	}
	config.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	config.ConnConfig.RuntimeParams["lock_timeout"] = "2000"
	db := stdlib.OpenDB(*config.ConnConfig)
	if existing {
		var version int
		var dirty, identityExists bool
		if db.QueryRowContext(ctx, `SELECT version,dirty,to_regclass('public.identity_meta') IS NOT NULL FROM schema_migrations`).Scan(&version, &dirty, &identityExists) != nil || version != 21 || dirty || identityExists {
			_ = db.Close()
			return ErrUnavailable
		}
	}
	src, err := iofs.New(archive, ".")
	if err != nil {
		_ = db.Close()
		return ErrUnavailable
	}
	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{DatabaseName: physical.Database})
	if err != nil {
		_ = src.Close()
		_ = db.Close()
		return ErrUnavailable
	}
	m, err := migrate.NewWithInstance("reviewed-user", src, physical.Database, driver)
	if err != nil {
		_ = src.Close()
		_ = driver.Close()
		return ErrUnavailable
	}
	err = m.Migrate(22)
	sourceErr, dbErr := m.Close()
	if err != nil || sourceErr != nil || dbErr != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return ErrUnavailable
	}
	defer pool.Close()
	var version int
	var dirty bool
	var history, generation uuid.UUID
	if pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty) != nil || version != 22 || dirty || pool.QueryRow(ctx, `SELECT history_generation,user_source_generation FROM identity_meta WHERE singleton`).Scan(&history, &generation) != nil || history == uuid.Nil || generation == uuid.Nil || history == generation {
		return ErrUnavailable
	}
	// Persist both generations outside the source before provisioning any keys.
	tag, err = owner.Exec(ctx, `UPDATE user_custody_creation SET phase='MIGRATED',history_generation=$1,user_source_generation=$2 WHERE singleton AND phase='MIGRATING' AND source_id=$3 AND epoch=$4 AND keyset_digest=$5`, history, generation, request.SourceID, request.Epoch, digest[:])
	if err != nil || tag.RowsAffected() != 1 {
		return ErrUnavailable
	}
	if existing {
		_, err = identityassociation.NewMigratedStore(ctx, pool, keys, identityassociation.MigrationSourcePin{HistoryGeneration: history, UserSourceGeneration: generation})
	} else {
		_, err = identityassociation.NewFreshStore(ctx, pool, keys, identityassociation.FreshSourcePin{HistoryGeneration: history, UserSourceGeneration: generation})
	}
	if err != nil {
		return ErrUnavailable
	}
	pool.Close()
	if _, err = source.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`); err != nil || checkSource(false) != nil {
		return ErrUnavailable
	}
	if source.QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_stat_activity WHERE datid=$1),(SELECT count(*) FROM pg_prepared_xacts WHERE database=$2)`, physical.DatabaseOID, physical.Database).Scan(&sessions, &prepared) != nil || sessions != 0 || prepared != 0 {
		return ErrUnavailable
	}
	if pin.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM user_custody_physical_source WHERE source_id=$1 AND epoch=$2`, request.SourceID, request.Epoch).Scan(&pinned.SystemIdentifier, &pinned.Database, &pinned.DatabaseOID) != nil || pinned != physical {
		return ErrUnavailable
	}
	closed = true
	// Publish CLOSED in the same transaction that holds the binding lock, so
	// loss of that session cannot commit a state for an unlocked binding.
	tx := pin
	_, err = tx.Exec(ctx, `CREATE TABLE public.user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL CHECK(state IN ('OPEN','CLOSED')),epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	if err != nil {
		return ErrUnavailable
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.user_custody_state VALUES(true,'CLOSED',$1,$2,$3,$4,$5)`, request.Epoch, request.SourceID, history, generation, digest[:])
	if err != nil {
		return ErrUnavailable
	}
	finalPhase := "INITIALIZED_CLOSED"
	if existing {
		finalPhase = "EXISTING_INITIALIZED_CLOSED"
	}
	tag, err = tx.Exec(ctx, `UPDATE public.user_custody_creation SET phase=$6 WHERE singleton AND phase='MIGRATED' AND source_id=$1 AND epoch=$2 AND history_generation=$3 AND user_source_generation=$4 AND keyset_digest=$5`, request.SourceID, request.Epoch, history, generation, digest[:], finalPhase)
	if err != nil || tag.RowsAffected() != 1 || tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}
