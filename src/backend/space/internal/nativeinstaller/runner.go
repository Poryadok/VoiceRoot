package nativeinstaller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	migrateengine "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/space/internal/store"
)

// These identities are embedded by the trusted image build, never read from a
// deployment environment variable or a caller-selected expected catalog hash.
var SourceRevision, BuildSourceSHA256, MigrateSHA256 string

type Config struct {
	AdminURL      string
	MigrationsDir string
	MigratePath   string
	Mode          string
	Prior         *Receipt
	Output        io.Writer
	// An owning controller may tighten selection to this exact build identity;
	// this cannot replace any compiled SQL/catalog qualification.
	RequiredBuildSHA256 string
}

func fileSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	return hashReaderAndClose(f)
}

// Error text stays safe for the CLI; Unwrap retains cleanup identity for callers.
type cleanupFailure struct {
	message string
	cause   error
}

func (e cleanupFailure) Error() string { return e.message }
func (e cleanupFailure) Unwrap() error { return e.cause }

func hashReaderAndClose(f io.ReadCloser) (sum string, err error) {
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, cleanupFailure{message: "input close failed", cause: closeErr})
			sum = ""
		}
	}()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type adminCleanupConn interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Close(context.Context) error
}

func cleanupAdmin(primary error, admin adminCleanupConn, locked bool) error {
	if locked {
		if _, err := admin.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended('voice-space-native-installer',0))`); err != nil {
			primary = errors.Join(primary, cleanupFailure{message: "installer advisory unlock failed", cause: err})
		}
	}
	if err := admin.Close(context.Background()); err != nil {
		primary = errors.Join(primary, cleanupFailure{message: "administrative connection close failed", cause: err})
	}
	return primary
}

const migrationDiagnosticLimit = 16 * 1024

// Child output may contain SQL and credentials. Retain at most this fixed buffer
// transiently; stdout, multiple terminal errors, or overflow veto classification.
// Writers always drain successfully so diagnostics cannot change the child exit.
type migrationDiagnostics struct {
	sync.Mutex
	stderr          [migrationDiagnosticLimit]byte
	size, seen      int
	mixed, overflow bool
}

type migrationDiagnosticStream struct {
	diagnostics *migrationDiagnostics
	stdout      bool
}

func (w migrationDiagnosticStream) Write(p []byte) (int, error) {
	d := w.diagnostics
	d.Lock()
	defer d.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	if w.stdout {
		d.mixed = true
	}
	if d.overflow || len(p) > migrationDiagnosticLimit-d.seen {
		d.overflow = true
		clear(d.stderr[:])
		d.size = 0
		return len(p), nil
	}
	d.seen += len(p)
	if !w.stdout {
		d.size += copy(d.stderr[d.size:], p)
	}
	return len(p), nil
}

func (d *migrationDiagnostics) category(exitCode int) string {
	// Pinned migrate4.18.1's postgres driver starts these failures with this
	// prefix. pq1.10.9 omits SQLSTATE in Error(), so never parse code-shaped SQL
	// or arbitrary message text as a database code or a more specific cause.
	data := d.stderr[:d.size]
	if !d.overflow && !d.mixed {
		if exitCode == 1 && bytes.HasPrefix(data, []byte("error: migration failed: ")) && !bytes.Contains(data, []byte("\nerror:")) {
			return "postgres_statement"
		}
		if exitCode == 2 && pinnedMigrationUsage(data) {
			return "cli_usage"
		}
	}
	return "driver_exit_unknown"
}

// This digest covers v4.18.1 internal/cli.Main's complete public usage block,
// expanding create/goto/up/down/drop/forceUsage through "Source drivers: ".
// Driver registries are unordered maps, so only their complete footer syntax
// is checked separately. A partial help marker is never sufficient.
const migrateUsageV4181SHA256 = "a90dd653fc52aea7be51aff59412e0d1f0dd1b764b599163a52ee3a3c592d5a3"

func pinnedMigrationUsage(data []byte) bool {
	// Inspect the bounded buffer in place, without copying raw child output.
	for _, marker := range []string{"panic", "fatal", "stack", "goroutine"} {
		for i := 0; i+len(marker) <= len(data); i++ {
			if bytes.EqualFold(data[i:i+len(marker)], []byte(marker)) {
				return false
			}
		}
	}
	start := bytes.Index(data, []byte("Usage: migrate OPTIONS COMMAND [arg...]\n"))
	if start < 0 {
		return false
	}
	usage := data[start:]
	driverMarker := []byte("\nSource drivers: ")
	end := bytes.Index(usage, driverMarker)
	if end < 0 {
		return false
	}
	end += len(driverMarker)
	sum := sha256.Sum256(usage[:end])
	if hex.EncodeToString(sum[:]) != migrateUsageV4181SHA256 {
		return false
	}
	sources, rest, ok := bytes.Cut(usage[end:], []byte("\nDatabase drivers: "))
	if !ok || !migrationDriverList(sources) {
		return false
	}
	databases, suffix, ok := bytes.Cut(rest, []byte("\n"))
	return ok && len(suffix) == 0 && migrationDriverList(databases)
}

func migrationDriverList(data []byte) bool {
	for len(data) != 0 {
		name, rest, more := bytes.Cut(data, []byte(", "))
		if len(name) == 0 {
			return false
		}
		for _, c := range name {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' && c != '+' && c != '.' {
				return false
			}
		}
		if !more {
			return true
		}
		data = rest
	}
	return false
}

func runMigration(ctx context.Context, cmd *exec.Cmd, version int) error {
	var diagnostics migrationDiagnostics
	defer func() { clear(diagnostics.stderr[:]) }()
	cmd.Stdout = migrationDiagnosticStream{diagnostics: &diagnostics, stdout: true}
	cmd.Stderr = migrationDiagnosticStream{diagnostics: &diagnostics}
	err := cmd.Run()
	if err == nil {
		return nil
	}
	category, exitCode := "driver_exit_unknown", -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
		category = diagnostics.category(exitCode)
	}
	if errors.Is(err, context.Canceled) || (exitCode == -1 && errors.Is(ctx.Err(), context.Canceled)) {
		category = "canceled"
		err = errors.Join(err, context.Canceled)
	} else if errors.Is(err, context.DeadlineExceeded) || (exitCode == -1 && errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		category = "deadline"
		err = errors.Join(err, context.DeadlineExceeded)
	}
	return cleanupFailure{message: fmt.Sprintf("migration stage %d failed (category=%s, exit_code=%d)", version, category, exitCode), cause: err}
}

func Run(ctx context.Context, c Config) (err error) {
	if c.Output == nil || (c.Mode != "provision" && c.Mode != "verify") {
		return errors.New("explicit provision or verify mode and evidence output required")
	}
	cfg, childURL, err := installerConnections(c.AdminURL)
	if err != nil {
		return err
	}
	if (SourceRevision != "" && len(SourceRevision) != 40) || len(BuildSourceSHA256) != 64 || len(MigrateSHA256) != 64 {
		return errors.New("trusted installer build identities required")
	}
	if c.RequiredBuildSHA256 != "" && c.RequiredBuildSHA256 != BuildSourceSHA256 {
		return errors.New("selected deployment source does not match installer build")
	}
	if err := store.RequireNativeCatalog51Admission(); err != nil {
		return errors.New("HOLD: current native51 source/session qualification is not admitted")
	}
	if !native51EngineSourceBound() {
		return errors.New("HOLD: pinned native51 migration engine source unavailable")
	}
	inventory, err := ValidateSources(c.MigrationsDir)
	if err != nil {
		return err
	}
	migrator, err := fileSHA(c.MigratePath)
	if err != nil || migrator != MigrateSHA256 {
		return errors.New("pinned migrate executable mismatch")
	}
	executable, err := os.Executable()
	if err != nil {
		return errors.New("installer executable unavailable")
	}
	executableSHA, err := fileSHA(executable)
	if err != nil {
		return errors.New("installer identity unavailable")
	}
	runID := uuid.NewString()
	cfg.RuntimeParams["application_name"] = "native-installer-" + runID
	admin, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return errors.New("administrative database unavailable")
	}
	lockAcquired := false
	defer func() { err = cleanupAdmin(err, admin, lockAcquired) }()
	var locked bool
	if err = admin.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended('voice-space-native-installer',0))`).Scan(&locked); err != nil || !locked {
		return errors.New("another installer owns the provisioning window")
	}
	lockAcquired = true
	var role string
	var pid int32
	if err = admin.QueryRow(ctx, `SELECT session_user,pg_backend_pid() WHERE current_user=session_user`).Scan(&role, &pid); err != nil {
		return errors.New("administrative session identity required")
	}
	var exists bool
	if err = admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname='space_db')`).Scan(&exists); err != nil {
		return errors.New("target inspection failed")
	}
	if c.Mode == "provision" && exists {
		return errors.New("HOLD: existing database has no fresh creation custody; use retained controller evidence for read-only verify")
	}
	if c.Mode == "verify" && (!exists || c.Prior == nil) {
		return errors.New("HOLD: actual retained completed installation evidence required")
	}
	if c.Mode == "verify" && !validCompletion(c.Prior) {
		return errors.New("HOLD: complete linked controller evidence required")
	}
	r := &Receipt{RunID: runID, SourceRevision: SourceRevision, SourceSHA256: BuildSourceSHA256, ExecutableSHA256: executableSHA, MigratorSHA256: migrator, InventorySHA256: inventory}
	emit := func(name string) error { return json.NewEncoder(c.Output).Encode(event{Event: name, Receipt: r}) }
	if c.Mode == "provision" {
		if _, err = admin.Exec(ctx, `CREATE DATABASE space_db`); err != nil {
			return errors.New("fresh target creation failed")
		}
		r.Created = true
		r.Creation = &store.NativeInstallerSourceFacts{}
		if err = admin.QueryRow(ctx, `SELECT datname,oid,session_user,CASE WHEN has_function_privilege(session_user,'pg_catalog.pg_control_system()','EXECUTE') THEN (SELECT system_identifier::text FROM pg_catalog.pg_control_system()) ELSE '' END FROM pg_database WHERE datname='space_db'`).Scan(&r.Creation.DatabaseName, &r.Creation.DatabaseOID, &r.Creation.InstallerRole, &r.Creation.DatabaseSystemID); err != nil {
			return errors.New("created target identity unavailable")
		}
		if err = emit("created"); err != nil {
			return err
		}
	}
	target := cfg.Copy()
	target.Database = "space_db"
	pc, err := pgxpool.ParseConfig(target.ConnString())
	if err != nil {
		return errors.New("target configuration failed")
	}
	pc.ConnConfig = target
	pc.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return errors.New("target unavailable")
	}
	defer pool.Close()
	s := &store.SpaceStore{Pool: pool}
	checkWindow := func(windowCtx context.Context) error {
		var ok bool
		if err := admin.QueryRow(windowCtx, `SELECT pg_backend_pid()=$1 AND session_user=$2 AND current_user=session_user AND EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory' AND granted) AND NOT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname='space_db' AND application_name<>$3)`, pid, role, cfg.RuntimeParams["application_name"]).Scan(&ok); err != nil || !ok {
			return errors.New("HOLD: exclusive owning maintenance window lost")
		}
		if r.Created {
			var oid uint32
			if err := admin.QueryRow(windowCtx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&oid); err != nil || oid != r.Creation.DatabaseOID {
				return errors.New("HOLD: created target identity changed")
			}
		}
		return nil
	}
	if err = checkWindow(ctx); err != nil {
		return err
	}
	if c.Mode == "verify" {
		prior := c.Prior
		if prior.SourceRevision != r.SourceRevision || prior.SourceSHA256 != r.SourceSHA256 || prior.ExecutableSHA256 != r.ExecutableSHA256 || prior.MigratorSHA256 != r.MigratorSHA256 || prior.InventorySHA256 != r.InventorySHA256 {
			return errors.New("HOLD: retained build/source identity changed")
		}
		facts, err := s.InspectNativeInstallerSource(ctx, 51)
		if err != nil {
			return errors.New("HOLD: current51 source verification failed")
		}
		if !sameTarget(prior.Final, facts) || prior.Final.SealSHA256 != facts.SealSHA256 {
			return errors.New("HOLD: retained target identity or seal changed")
		}
		// Preserve original receipts; a verify result cannot manufacture transitions.
		return json.NewEncoder(c.Output).Encode(event{Event: "verified", Receipt: prior})
	}
	q := childURL.Query()
	q.Set("application_name", cfg.RuntimeParams["application_name"])
	childURL.RawQuery = q.Encode()
	migrate := func(version int) error {
		if err := checkWindow(ctx); err != nil {
			return err
		}
		if _, err := ValidateSources(c.MigrationsDir); err != nil {
			return err
		}
		if actual, err := fileSHA(c.MigratePath); err != nil || actual != MigrateSHA256 {
			return errors.New("pinned migrate executable changed")
		}
		cmd := exec.CommandContext(ctx, c.MigratePath, "-path", filepath.Clean(c.MigrationsDir), "-database", childURL.String(), "goto", strconv.Itoa(version))
		cmd.Env = migrationEnvironment(cfg, childURL)
		// Child output stays transient and bounded; only fixed metadata is exposed.
		if err := runMigration(ctx, cmd, version); err != nil {
			return err
		}
		if err := checkWindow(ctx); err != nil {
			return err
		}
		return emit("migrated" + strconv.Itoa(version))
	}
	if err = migrate(44); err != nil {
		return err
	}
	r.Trusted44, err = s.InspectNativeInstallerSource(ctx, 44)
	if err != nil {
		return errors.New("full native44 verification failed")
	}
	if !sameDatabase(r.Creation, r.Trusted44) {
		return errors.New("native44 creation identity mismatch")
	}
	if err = emit("trusted44"); err != nil {
		return err
	}
	if err = migrate(45); err != nil {
		return err
	}
	r.Transition45, err = s.TransitionNativeMembershipCatalog45(ctx)
	if err != nil {
		return errors.New("native45 transition failed")
	}
	if !matchesTransition(r.Trusted44, (*store.NativeMembershipCatalog45Receipt)(r.Transition45)) {
		return errors.New("native45 receipt linkage failed")
	}
	trusted45, err := s.InspectNativeInstallerSource(ctx, 45)
	if err != nil || trusted45.SealSHA256 != r.Transition45.NewSealSHA256 {
		return errors.New("full native45 verification failed")
	}
	if err = emit("trusted45"); err != nil {
		return err
	}
	if err = migrate(50); err != nil {
		return err
	}
	r.Transition50, err = s.TransitionNativeMembershipCatalog50(ctx)
	if err != nil {
		return errors.New("native50 transition failed")
	}
	if !matchesTransition(trusted45, (*store.NativeMembershipCatalog45Receipt)(r.Transition50)) {
		return errors.New("native50 receipt linkage failed")
	}
	if err = emit("trusted50"); err != nil {
		return err
	}
	r.Trusted50, err = s.InspectNativeInstallerSource(ctx, 50)
	if err != nil || r.Trusted50.SealSHA256 != r.Transition50.NewSealSHA256 || !sameTarget(r.Trusted44, r.Trusted50) {
		return errors.New("full native50 prerequisite verification failed")
	}
	// No child goto51: the actual real engine/file-source adapter owns a single
	// transaction for exact51 SQL, full qualification, epoch CAS and history.
	r.Transition51, err = runNativeCatalog51(ctx, s, r.Trusted50, c.MigrationsDir, checkWindow)
	if err != nil {
		return errors.New("native51 atomic migration failed")
	}
	if !matchesTransition(r.Trusted50, &r.Transition51.NativeMembershipCatalog45Receipt) || r.Transition51.Profile != store.NativeCatalogRowJSONProfile {
		return errors.New("native51 receipt linkage failed")
	}
	if err = emit("trusted51"); err != nil {
		return err
	}
	r.Final, err = s.InspectNativeInstallerSource(ctx, 51)
	if err != nil || r.Final.SealSHA256 != r.Transition51.NewSealSHA256 || !sameTarget(r.Trusted44, r.Final) {
		return errors.New("final full native51 verification failed")
	}
	if err = checkWindow(ctx); err != nil {
		return err
	}
	return emit("complete")
}

func runNativeCatalog51(ctx context.Context, s *store.SpaceStore, prior *store.NativeInstallerSourceFacts, migrationsDir string, window func(context.Context) error) (receipt *store.NativeMembershipCatalog51Receipt, err error) {
	driver, err := s.NewNativeCatalog51Migration(ctx, prior, window)
	if err != nil {
		return nil, err
	}
	// Cleanup is required even when the source driver fails before an engine
	// instance exists. Close rolls back unfinished work and never commits.
	defer func() {
		if closeErr := driver.Close(); closeErr != nil {
			receipt = nil
			err = errors.Join(err, cleanupFailure{message: "native51 migration cleanup failed", cause: closeErr})
		}
	}()
	abs, err := filepath.Abs(migrationsDir)
	if err != nil {
		return nil, errors.New("native51 source path unavailable")
	}
	sourceURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String()
	engine, err := migrateengine.NewWithDatabaseInstance(sourceURL, "native51-transaction", driver)
	if err != nil {
		return nil, errors.New("native51 migration source unavailable")
	}
	defer func() {
		sourceErr, databaseErr := engine.Close()
		if sourceErr != nil || databaseErr != nil {
			receipt = nil
			err = errors.Join(err, cleanupFailure{message: "native51 migration engine close failed", cause: errors.Join(sourceErr, databaseErr)})
		}
	}()
	// Unlock cannot prove engine success: source/control errors may originate
	// outside the adapter. Only this explicit outer call can confirm commit.
	engineErr := engine.Migrate(51)
	receipt, err = driver.CommitQualified(engineErr)
	if err != nil {
		return nil, errors.New("native51 migration transaction rejected")
	}
	return receipt, nil
}

func native51EngineSourceBound() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/golang-migrate/migrate/v4" {
			return dep.Replace == nil && dep.Version == "v4.18.1" && dep.Sum == "h1:JML/k+t4tpHCpQTCAD62Nu43NUFzHY4CV3uAuvHGC+Y="
		}
	}
	return false
}

// installerConnections resolves one finite connection contract before mutation.
// Both actual drivers receive the same explicit authority and TLS inputs; only
// the deliberate admin/Space database paths differ. Unsupported indirection or
// ambiguous query selectors cannot be interpreted differently by pgx and pq.
func installerConnections(raw string) (*pgx.ConnConfig, *url.URL, error) {
	if os.Getenv("PGSERVICE") != "" || os.Getenv("PGSERVICEFILE") != "" {
		return nil, nil, errors.New("administrative service indirection is unsupported")
	}
	var u *url.URL
	if raw != "" {
		var err error
		u, err = url.Parse(raw)
		if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Opaque != "" || u.Fragment != "" {
			return nil, nil, errors.New("administrative configuration must use a PostgreSQL URI or PG environment")
		}
	} else {
		base, err := pgx.ParseConfig("")
		if err != nil {
			return nil, nil, errors.New("invalid administrative PG environment")
		}
		u = &url.URL{Scheme: "postgres", Host: net.JoinHostPort(base.Host, strconv.Itoa(int(base.Port))), User: url.UserPassword(base.User, base.Password)}
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, nil, errors.New("invalid administrative URI query")
	}
	for key, values := range q {
		if len(values) != 1 {
			return nil, nil, errors.New("ambiguous administrative URI query")
		}
		switch key {
		case "sslmode", "sslrootcert", "sslcert", "sslkey", "connect_timeout", "application_name":
		default:
			return nil, nil, errors.New("administrative URI connection selectors/options are unsupported")
		}
	}
	for key, env := range map[string]string{"sslmode": "PGSSLMODE", "sslrootcert": "PGSSLROOTCERT", "sslcert": "PGSSLCERT", "sslkey": "PGSSLKEY"} {
		if _, present := q[key]; !present && os.Getenv(env) != "" {
			q.Set(key, os.Getenv(env))
		}
	}
	if q.Get("sslmode") == "" {
		q.Set("sslmode", "require")
	}
	switch q.Get("sslmode") {
	case "disable", "require":
	case "verify-ca", "verify-full":
		if q.Get("sslrootcert") == "" {
			return nil, nil, errors.New("verified administrative TLS requires an explicit CA file")
		}
	default:
		return nil, nil, errors.New("unsupported administrative TLS mode")
	}
	u.Path, u.RawPath, u.RawQuery = "/postgres", "", q.Encode()
	cfg, err := pgx.ParseConfig(u.String())
	if err != nil || strings.ContainsAny(cfg.Host, "/,") {
		return nil, nil, errors.New("invalid or unsupported administrative TCP configuration")
	}
	// Freeze environment-resolved credentials/authority into both parser inputs.
	u.Host = net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port)))
	u.User = url.UserPassword(cfg.User, cfg.Password)
	cfg, err = pgx.ParseConfig(u.String())
	if err != nil {
		return nil, nil, errors.New("invalid administrative database configuration")
	}
	u.Path = "/space_db"
	return cfg, u, nil
}

func migrationEnvironment(cfg *pgx.ConnConfig, child *url.URL) []string {
	// pq.ParseURL omits empty values. Freeze these environment fallbacks too,
	// so an explicit empty password/TLS option cannot revive a weaker ambient
	// setting or a different target database's password-file entry.
	q := child.Query()
	bound := map[string]string{
		"PGHOST": cfg.Host, "PGPORT": strconv.Itoa(int(cfg.Port)), "PGUSER": cfg.User,
		"PGPASSWORD": cfg.Password, "PGDATABASE": "space_db", "PGPASSFILE": "/dev/null",
		"PGSSLMODE":     q.Get("sslmode"),
		"PGSSLROOTCERT": q.Get("sslrootcert"), "PGSSLCERT": q.Get("sslcert"), "PGSSLKEY": q.Get("sslkey"),
	}
	env := make([]string, 0, len(os.Environ())+len(bound))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		// pq rejects service keys by presence, even when their values are empty.
		// Omit them entirely without allowing inherited service indirection.
		if key == "PGSERVICE" || key == "PGSERVICEFILE" {
			continue
		}
		if _, replace := bound[key]; !replace {
			env = append(env, entry)
		}
	}
	for key, value := range bound {
		env = append(env, key+"="+value)
	}
	return env
}

func sameTarget(a, b *store.NativeInstallerSourceFacts) bool {
	return a != nil && b != nil && a.DatabaseName == b.DatabaseName && a.DatabaseOID == b.DatabaseOID && a.DatabaseSystemID == b.DatabaseSystemID && a.InstallerRole == b.InstallerRole && a.Epoch == b.Epoch
}

func sameDatabase(a, b *store.NativeInstallerSourceFacts) bool {
	return a != nil && b != nil && a.DatabaseName == b.DatabaseName && a.DatabaseOID == b.DatabaseOID && a.DatabaseSystemID == b.DatabaseSystemID && a.InstallerRole == b.InstallerRole
}
func matchesTransition(a *store.NativeInstallerSourceFacts, b *store.NativeMembershipCatalog45Receipt) bool {
	return a != nil && b != nil && a.DatabaseName == b.DatabaseName && a.DatabaseOID == b.DatabaseOID && a.DatabaseSystemID == b.DatabaseSystemID && a.InstallerRole == b.InstallerRole && a.Epoch == b.Epoch && a.SealSHA256 == b.OldSealSHA256 && b.Changed
}
