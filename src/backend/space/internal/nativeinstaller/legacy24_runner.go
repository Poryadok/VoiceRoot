package nativeinstaller

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/space/internal/store"
)

// Legacy24Config carries the original-24 upgrade inputs. The controller owns
// backup, Job and maintenance custody. These correlation
// strings do not replace actual source admission or the owning database window.
type Legacy24Config struct {
	Config
	OperationID        string
	SnapshotID         string
	OriginEvidencePath string
}

// Omitting pg_catalog gives built-ins implicit first precedence while public
// remains the creation namespace for unchanged historical unqualified DDL.
// Explicit pg_temp last also prevents temporary relations preceding catalogs.
const legacy24TargetSearchPath = "public,pg_temp"

func emitLegacy24(out io.Writer, stage string, receipt *Legacy24Receipt) error {
	raw, err := json.Marshal(legacy24Event{Event: stage, Receipt: receipt})
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	n, err := out.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

func legacy24Hex(value string, size int) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == size && hex.EncodeToString(raw) == value
}

type legacy24Session struct {
	admin       *pgx.Conn
	conn        *pgx.Conn
	pool        *pgxpool.Pool
	store       *store.SpaceStore
	receipt     *Legacy24Receipt
	checkWindow func(context.Context) error
	migrate     func(context.Context, int) error
}

func (s *legacy24Session) close(primary error) error {
	if s.conn != nil {
		if err := s.conn.Close(context.Background()); err != nil {
			primary = errors.Join(primary, errors.New("legacy24 target close failed"))
		}
	}
	if s.pool != nil {
		s.pool.Close()
	}
	return cleanupAdmin(primary, s.admin, true)
}

func openLegacy24Session(ctx context.Context, c Config) (_ *legacy24Session, err error) {
	if ctx == nil || ctx.Err() != nil || c.Output == nil || c.Prior != nil || c.Mode != "" {
		return nil, errors.New("explicit legacy24 configuration required")
	}
	if !legacy24Hex(SourceRevision, 20) || !legacy24Hex(BuildSourceSHA256, 32) || !legacy24Hex(MigrateSHA256, 32) ||
		(c.RequiredBuildSHA256 != "" && c.RequiredBuildSHA256 != BuildSourceSHA256) {
		return nil, errors.New("original trusted legacy24 build identities required")
	}
	if store.RequireNativeCatalog51Admission() != nil || !native51EngineSourceBound() {
		return nil, errors.New("HOLD: current native51 source/session qualification is not admitted")
	}
	inventory, err := ValidateSources(c.MigrationsDir)
	if err != nil {
		return nil, err
	}
	migrator, err := fileSHA(c.MigratePath)
	if err != nil || migrator != MigrateSHA256 {
		return nil, errors.New("pinned migrate executable mismatch")
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, errors.New("installer executable unavailable")
	}
	executableSHA, err := fileSHA(executable)
	if err != nil {
		return nil, errors.New("installer identity unavailable")
	}
	cfg, childURL, err := installerConnections(c.AdminURL)
	if err != nil {
		return nil, err
	}
	runID := uuid.NewString()
	application := "native-legacy24-" + runID
	cfg.RuntimeParams["application_name"] = application
	cfg.RuntimeParams["search_path"] = "pg_catalog"
	admin, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("administrative database unavailable")
	}
	var locked bool
	if err = admin.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended('voice-space-native-installer',0))`).Scan(&locked); err != nil || !locked {
		_ = admin.Close(context.Background())
		return nil, errors.New("another installer owns the maintenance window")
	}
	s := &legacy24Session{admin: admin, receipt: &Legacy24Receipt{Schema: "voice.space.native-legacy24-upgrade.v1", Origin: "legacy24", RunID: runID, SourceRevision: SourceRevision, SourceSHA256: BuildSourceSHA256, ExecutableSHA256: executableSHA, MigratorSHA256: migrator, InventorySHA256: inventory}}
	defer func() {
		if err != nil {
			err = s.close(err)
		}
	}()
	var role, system string
	var pid int32
	var oid uint32
	if err = admin.QueryRow(ctx, `SELECT session_user,pg_backend_pid(),(SELECT oid FROM pg_database WHERE datname='space_db'),(SELECT system_identifier::text FROM pg_control_system()) WHERE current_user=session_user`).Scan(&role, &pid, &oid, &system); err != nil || oid == 0 || system == "" {
		return nil, errors.New("HOLD: existing target identity unavailable")
	}
	s.checkWindow = func(checkCtx context.Context) error {
		var ok bool
		err := admin.QueryRow(checkCtx, `SELECT pg_backend_pid()=$1 AND session_user=$2 AND current_user=session_user
   AND (SELECT oid FROM pg_database WHERE datname='space_db')=$3
   AND (SELECT system_identifier::text FROM pg_control_system())=$4
   AND EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory' AND granted AND mode='ExclusiveLock'
     AND classid=((hashtextextended('voice-space-native-installer',0)>>32)&4294967295)::oid
     AND objid=(hashtextextended('voice-space-native-installer',0)&4294967295)::oid AND objsubid=1)
   AND NOT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname='space_db' AND application_name IS DISTINCT FROM $5)`, pid, role, oid, system, application).Scan(&ok)
		if err != nil || !ok {
			return errors.New("HOLD: exclusive original maintenance window lost")
		}
		return nil
	}
	if err = s.checkWindow(ctx); err != nil {
		return nil, err
	}
	target := cfg.Copy()
	target.Database = "space_db"
	target.RuntimeParams["search_path"] = legacy24TargetSearchPath
	s.conn, err = pgx.ConnectConfig(ctx, target)
	if err != nil {
		return nil, errors.New("existing target unavailable")
	}
	pc, err := pgxpool.ParseConfig(target.ConnString())
	if err != nil {
		return nil, errors.New("target configuration failed")
	}
	pc.ConnConfig = target
	pc.MaxConns = 2
	s.pool, err = pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, errors.New("target pool unavailable")
	}
	s.store = &store.SpaceStore{Pool: s.pool}
	q := childURL.Query()
	q.Set("application_name", application)
	q.Set("search_path", legacy24TargetSearchPath)
	childURL.RawQuery = q.Encode()
	s.migrate = func(migrationCtx context.Context, version int) error {
		if version != 25 && version != 44 && version != 45 && version != 50 {
			return errors.New("unsupported legacy24 migration stage")
		}
		if err := s.checkWindow(migrationCtx); err != nil {
			return err
		}
		if actual, err := ValidateSources(c.MigrationsDir); err != nil || actual != inventory {
			return errors.New("legacy24 SQL inventory changed")
		}
		if actual, err := fileSHA(c.MigratePath); err != nil || actual != migrator {
			return errors.New("pinned migrate executable changed")
		}
		cmd := exec.CommandContext(migrationCtx, c.MigratePath, "-path", filepath.Clean(c.MigrationsDir), "-database", childURL.String(), "goto", strconv.Itoa(version))
		// Historical SQL is unqualified. Neither ambient PGOPTIONS nor caller URI
		// selectors may redirect its fixed public search path.
		for _, entry := range migrationEnvironment(cfg, childURL) {
			if !strings.HasPrefix(entry, "PGOPTIONS=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "PGOPTIONS=-c search_path="+legacy24TargetSearchPath)
		if err := runMigration(migrationCtx, cmd, version); err != nil {
			return err
		}
		return s.checkWindow(migrationCtx)
	}
	return s, nil
}

func retainLegacy24Origin(path string, origin *legacy24OriginObservation, r *Legacy24Receipt) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("HOLD: original legacy24 evidence must be new and private")
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, errors.New("original legacy24 evidence close failed"))
		}
	}()
	if err = writeLegacy24Origin(f, origin, r); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return errors.New("original legacy24 evidence durability failed")
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return errors.New("original legacy24 evidence directory unavailable")
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return errors.New("original legacy24 evidence directory durability failed")
	}
	return nil
}

// RunLegacy24Upgrade never resumes. An interrupted run leaves its private
// original evidence and actual emitted prefix for the controller to reconcile.
func RunLegacy24Upgrade(ctx context.Context, c Legacy24Config) (err error) {
	if c.Output == nil || c.OriginEvidencePath == "" || len(c.OperationID) == 0 || len(c.OperationID) > 256 || len(c.SnapshotID) == 0 || len(c.SnapshotID) > 256 {
		return errors.New("legacy24 operation, backup snapshot, private original evidence and output required")
	}
	s, err := openLegacy24Session(ctx, c.Config)
	if err != nil {
		return err
	}
	defer func() { err = s.close(err) }()
	origin, err := admitLegacy24Source(ctx, s.conn)
	if err != nil {
		return errors.New("HOLD: original legacy24 source is not admitted")
	}
	r := s.receipt
	r.OperationID, r.SnapshotID = c.OperationID, c.SnapshotID
	r.Source24, r.Clean24, r.Catalog24SHA256 = origin.Target, true, origin.CatalogSHA256
	r.PreimageSHA256, r.AccessSHA256, r.SpaceCount = origin.PreimageSHA256, origin.AccessSHA256, uint64(len(origin.Spaces))
	if err = s.checkWindow(ctx); err != nil {
		return err
	}
	if err = retainLegacy24Origin(c.OriginEvidencePath, origin, r); err != nil {
		return err
	}
	emit := func(stage string) error {
		if err := s.checkWindow(ctx); err != nil {
			return err
		}
		return emitLegacy24(c.Output, stage, r)
	}
	if err = emit("source24"); err != nil {
		return err
	}
	if err = s.migrate(ctx, 25); err != nil {
		return err
	}
	r.Trusted25, err = verifyLegacy24Intermediate(ctx, s.conn, origin)
	if err != nil {
		return errors.New("HOLD: genuine25 original preservation failed")
	}
	if err = emit("trusted25"); err != nil {
		return err
	}
	if err = s.migrate(ctx, 44); err != nil {
		return err
	}
	r.Trusted44, err = s.store.InspectNativeInstallerSource(ctx, 44)
	if err != nil || !sameDatabase(origin.Target, r.Trusted44) {
		return errors.New("HOLD: actual native44 source verification failed")
	}
	if _, err = verifyLegacy24Preserved(ctx, s.conn, origin); err != nil {
		return errors.New("HOLD: native44 original preservation failed")
	}
	if err = emit("trusted44"); err != nil {
		return err
	}
	if err = s.migrate(ctx, 45); err != nil {
		return err
	}
	r.Transition45, err = s.store.TransitionNativeMembershipCatalog45(ctx)
	if err != nil || !matchesTransition(r.Trusted44, r.Transition45) {
		return errors.New("HOLD: native45 transition failed")
	}
	r.Trusted45, err = s.store.InspectNativeInstallerSource(ctx, 45)
	if err != nil || !sameTarget(r.Trusted44, r.Trusted45) || r.Trusted45.SealSHA256 != r.Transition45.NewSealSHA256 {
		return errors.New("HOLD: full native45 verification failed")
	}
	if err = emit("trusted45"); err != nil {
		return err
	}
	if err = s.migrate(ctx, 50); err != nil {
		return err
	}
	r.Transition50, err = s.store.TransitionNativeMembershipCatalog50(ctx)
	if err != nil || !matchesTransition(r.Trusted45, (*store.NativeMembershipCatalog45Receipt)(r.Transition50)) {
		return errors.New("HOLD: native50 transition failed")
	}
	r.Trusted50, err = s.store.InspectNativeInstallerSource(ctx, 50)
	if err != nil || !sameTarget(r.Trusted44, r.Trusted50) || r.Trusted50.SealSHA256 != r.Transition50.NewSealSHA256 {
		return errors.New("HOLD: full native50 verification failed")
	}
	if err = emit("trusted50"); err != nil {
		return err
	}
	r.Transition51, err = runNativeCatalog51(ctx, s.store, r.Trusted50, c.MigrationsDir, s.checkWindow)
	if err != nil || r.Transition51 == nil || !matchesTransition(r.Trusted50, &r.Transition51.NativeMembershipCatalog45Receipt) || r.Transition51.Profile != store.NativeCatalogRowJSONProfile {
		return errors.New("HOLD: native51 atomic migration failed")
	}
	r.Final, err = s.store.InspectNativeInstallerSource(ctx, 51)
	if err != nil || !sameTarget(r.Trusted44, r.Final) || r.Final.SealSHA256 != r.Transition51.NewSealSHA256 {
		return errors.New("HOLD: full native51 verification failed")
	}
	if err = emit("trusted51"); err != nil {
		return err
	}
	for _, id := range origin.Spaces {
		if err = s.checkWindow(ctx); err != nil {
			return err
		}
		space, parseErr := uuid.Parse(id)
		if parseErr != nil || space == uuid.Nil || space.String() != id {
			return errors.New("HOLD: admitted Space identity changed")
		}
		if err = s.store.InitializeNativeMembership(ctx, space); err != nil {
			return errors.New("HOLD: original Space initialization incomplete; do not replay")
		}
	}
	r.PreservationSHA256, err = verifyLegacy24Preserved(ctx, s.conn, origin)
	if err != nil {
		return errors.New("HOLD: final original preservation failed")
	}
	r.InitializationSHA256, err = legacy24Initialization(ctx, s.conn, origin, r.Final)
	if err != nil {
		return err
	}
	r.InitializationCount = uint64(len(origin.Spaces))
	if err = emit("initialized"); err != nil {
		return err
	}
	r.Scope = "upgrade-complete"
	if !validLegacy24Completion(r) {
		return errors.New("HOLD: closed legacy24 completion failed")
	}
	return emit("upgrade-complete")
}
