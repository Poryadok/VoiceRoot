package nativeinstaller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/space/internal/store"
)

// Legacy25Config carries the existing-25 upgrade inputs. The controller owns
// backup, Job and maintenance custody. Operation and backup snapshot correlations
// are generated fresh per run because legacy25 retains no private sidecar evidence.
type Legacy25Config struct {
	Config
}

// Omitting pg_catalog gives built-ins implicit first precedence while public
// remains the creation namespace for unchanged historical unqualified DDL.
// Explicit pg_temp last also prevents temporary relations preceding catalogs.
const legacy25TargetSearchPath = legacy24TargetSearchPath

func emitLegacy25(out io.Writer, stage string, receipt *Legacy25Receipt) error {
	raw, err := json.Marshal(legacy25Event{Event: stage, Receipt: receipt})
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


type legacy25Session struct {
	admin       *pgx.Conn
	conn        *pgx.Conn
	pool        *pgxpool.Pool
	store       *store.SpaceStore
	receipt     *Legacy25Receipt
	checkWindow func(context.Context) error
	migrate     func(context.Context, int) error
}

func (s *legacy25Session) close(primary error) error {
	if s.conn != nil {
		if err := s.conn.Close(context.Background()); err != nil {
			primary = errors.Join(primary, errors.New("legacy25 target close failed"))
		}
	}
	if s.pool != nil {
		s.pool.Close()
	}
	return cleanupAdmin(primary, s.admin, true)
}

func openLegacy25Session(ctx context.Context, c Config) (_ *legacy25Session, err error) {
	if ctx == nil || ctx.Err() != nil || c.Output == nil || c.Prior != nil || c.Mode != "" {
		return nil, errors.New("explicit legacy25 configuration required")
	}
	if !legacy24Hex(SourceRevision, 20) || !legacy24Hex(BuildSourceSHA256, 32) || !legacy24Hex(MigrateSHA256, 32) ||
		(c.RequiredBuildSHA256 != "" && c.RequiredBuildSHA256 != BuildSourceSHA256) {
		return nil, errors.New("original trusted legacy25 build identities required")
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
	application := "native-legacy25-" + runID
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
	s := &legacy25Session{admin: admin, receipt: &Legacy25Receipt{Schema: "voice.space.native-legacy25-upgrade.v1", Origin: "legacy25", RunID: runID, SourceRevision: SourceRevision, SourceSHA256: BuildSourceSHA256, ExecutableSHA256: executableSHA, MigratorSHA256: migrator, InventorySHA256: inventory}}
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
	target.RuntimeParams["search_path"] = legacy25TargetSearchPath
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
	q.Set("search_path", legacy25TargetSearchPath)
	childURL.RawQuery = q.Encode()
	s.migrate = func(migrationCtx context.Context, version int) error {
		if version != 44 && version != 45 && version != 50 {
			return errors.New("unsupported legacy25 migration stage")
		}
		if err := s.checkWindow(migrationCtx); err != nil {
			return err
		}
		if actual, err := ValidateSources(c.MigrationsDir); err != nil || actual != inventory {
			return errors.New("legacy25 SQL inventory changed")
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
		cmd.Env = append(cmd.Env, "PGOPTIONS=-c search_path="+legacy25TargetSearchPath)
		if err := runMigration(migrationCtx, cmd, version); err != nil {
			return err
		}
		return s.checkWindow(migrationCtx)
	}
	return s, nil
}


// admitLegacy25Source observes and admits the existing 25 target in one read-only
// transaction. Detached private observation only; it asserts no controller authority.
func admitLegacy25Source(ctx context.Context, conn *pgx.Conn) (_ *legacy24OriginObservation, err error) {
	tx, err := beginLegacySourceRead(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer finishLegacy24Read(tx, &err)
	if err = qualifyLegacy24InstallerTx(ctx, tx); err != nil {
		return nil, err
	}
	if err = qualifyLegacy24LifecycleTx(ctx, tx); err != nil {
		return nil, err
	}
	catalog, err := inspectLegacyCatalogTx(ctx, tx, 25, legacy25CatalogPG16SHA256)
	if err != nil {
		return nil, err
	}
	var noNativeSchema bool
	if err = tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname IN ('native_membership_guard','space_account_ban_guard'))`).Scan(&noNativeSchema); err != nil || !noNativeSchema {
		return nil, errors.New("HOLD: legacy25 native schema already exists")
	}
	var name, role, current, system, owner string
	var oid uint32
	if err = tx.QueryRow(ctx, `SELECT d.datname,d.oid,session_user,current_user,(SELECT system_identifier::text FROM pg_catalog.pg_control_system()),pg_get_userbyid(d.datdba) FROM pg_catalog.pg_database d WHERE d.datname='space_db'`).Scan(&name, &oid, &role, &current, &system, &owner); err != nil ||
		name != "space_db" || oid == 0 || role == "" || owner == "" || role != current || !regexp.MustCompile(`^[1-9][0-9]{0,19}$`).MatchString(system) {
		return nil, errors.New("legacy25 target identity unavailable")
	}
	accessHash, err := captureLegacyAccessTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	rows, err := captureLegacy25ProjectionTx(ctx, tx, nil)
	if err != nil {
		return nil, err
	}
	stableAccess, err := captureLegacy24StableAccessTx(ctx, tx, rows)
	if err != nil {
		return nil, err
	}
	functions, err := captureLegacy24FunctionsTx(ctx, tx, nil)
	if err != nil {
		return nil, err
	}
	derived, err := captureLegacy24DerivedTx(ctx, tx)
	if err != nil {
		return nil, err
	}
	// legacy25 keeps no independent-object digest: the 25..51 Space migrations
	// rewrite only public.* and the native guard namespaces, so an independent
	// fixture (e.g. unrelated_fixture.records with an implicit PRIMARY KEY
	// constraint) is retained without being re-digested. The legacy24 extras
	// predicate rejects any pg_class owning a pg_constraint, which the pinned
	// legacy25 fixture's PRIMARY KEY table has — it must not gate a 25-origin.
	// The preimage covers the public custody set (rows, derived, functions,
	// access) and preservation re-validates that set at 44 and 51.
	var extras []legacy24Extra
	ids, err := tx.Query(ctx, `SELECT id::text FROM public.spaces ORDER BY id LIMIT 1000001`)
	if err != nil {
		return nil, errors.New("legacy25 spaces unavailable")
	}
	spaces := make([]string, 0)
	for ids.Next() {
		var id string
		if ids.Scan(&id) != nil || len(spaces) >= 1000000 {
			ids.Close()
			return nil, errors.New("legacy25 spaces unavailable")
		}
		parsed, parseErr := uuid.Parse(id)
		if parseErr != nil || parsed == uuid.Nil || parsed.String() != id {
			ids.Close()
			return nil, errors.New("legacy25 spaces unavailable")
		}
		spaces = append(spaces, id)
	}
	if ids.Err() != nil {
		ids.Close()
		return nil, errors.New("legacy25 spaces unavailable")
	}
	ids.Close()
	if uint64(len(spaces)) != rows["spaces"].Count {
		return nil, errors.New("legacy25 spaces unavailable")
	}
	encoded, err := json.Marshal(struct {
		Rows               map[string]legacy25TableDigest `json:"rows"`
		Derived            *legacy24Derived               `json:"derived"`
		Spaces             []string                       `json:"spaces"`
		Extras             []legacy24Extra                `json:"extras"`
		StableAccessSHA256 string                         `json:"stable_access_sha256"`
		Functions          []legacy24FunctionAccess       `json:"functions"`
	}{rows, derived, spaces, extras, stableAccess, functions})
	if err != nil || len(encoded) > 1<<20 {
		return nil, errors.New("legacy25 preimage unavailable")
	}
	preimageHash := sha256.Sum256(encoded)
	if err = tx.Commit(ctx); err != nil {
		return nil, errors.New("legacy25 source unavailable")
	}
	return &legacy24OriginObservation{DatabaseOwner: owner, Target: &store.NativeInstallerSourceFacts{Version: 25, DatabaseSystemID: system,
		DatabaseName: name, DatabaseOID: oid, InstallerRole: role}, CatalogSHA256: catalog,
		PreimageSHA256: hex.EncodeToString(preimageHash[:]), AccessSHA256: accessHash, StableAccessSHA256: stableAccess, Rows: rows, Derived: derived, Spaces: spaces, Extras: extras, Functions: functions}, nil
}


// RunLegacy25Upgrade runs the pinned 25..51 Space SQL ladder as one verified
// transaction sequence, then re-reads the completion without the lease.
func RunLegacy25Upgrade(ctx context.Context, c Legacy25Config) error {
	if c.Output == nil {
		return errors.New("legacy25 upgrade output required")
	}
	s, err := openLegacy25Session(ctx, c.Config)
	if err != nil {
		return err
	}
	defer func() { err = s.close(err) }()
	origin, err := admitLegacy25Source(ctx, s.conn)
	if err != nil {
		return errors.Join(errors.New("HOLD: original legacy25 source is not admitted"), err)
	}
	r := s.receipt
	r.OperationID, r.SnapshotID = uuid.NewString(), uuid.NewString()
	r.Source25, r.CatalogSHA256, r.PreimageSHA256, r.SpaceCount = origin.Target, origin.CatalogSHA256, origin.PreimageSHA256, uint64(len(origin.Spaces))
	if err = s.checkWindow(ctx); err != nil {
		return err
	}
	emit := func(stage string) error {
		if err := s.checkWindow(ctx); err != nil {
			return err
		}
		return emitLegacy25(c.Output, stage, r)
	}
	if err = emit("source25"); err != nil {
		return err
	}
	if err = s.migrate(ctx, 44); err != nil {
		return err
	}
	r.Trusted44, err = s.store.InspectNativeInstallerSource(ctx, 44)
	if err != nil || !sameDatabase(origin.Target, r.Trusted44) {
		return errors.New("HOLD: actual native44 source verification failed")
	}
	if _, err = preserveLegacy25PublicCustody(ctx, s.conn, origin); err != nil {
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
	r.PreservationSHA256, err = preserveLegacy25PublicCustody(ctx, s.conn, origin)
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
	if !validLegacy25Completion(r) {
		return errors.New("HOLD: closed legacy25 completion failed")
	}
	return emit("upgrade-complete")
}
