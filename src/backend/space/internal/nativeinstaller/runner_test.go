package nativeinstaller

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/store"
)

// These children exercise only the process/reporting boundary, never migrate or a database.
func diagnosticCommand(ctx context.Context, stderr, stdout string, exit string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "sh", "-c", `printf '%s' "$OUT"; printf '%s' "$ERR" >&2; exit "$CODE"`)
	cmd.Env = []string{"OUT=" + stdout, "ERR=" + stderr, "CODE=" + exit}
	return cmd
}

func TestRunMigrationSafeDiagnosticsAndCleanup(t *testing.T) {
	const secret = "postgres://synthetic-user:credential-sentinel@invalid/secret-query"
	database := "error: migration failed: permission denied in line 0: SELECT '" + secret + "' (details: pq: permission denied)\n"
	for _, tc := range []struct {
		name, stderr, stdout, category string
	}{
		{"pinned database format", database, "", "database"},
		{"unknown credential output", secret, "", "driver_exit"},
		{"forged SQLSTATE", "error: " + secret + " SQLSTATE 42501", "", "driver_exit"},
		{"SQLSTATE inside query", database + "SELECT 'SQLSTATE 28P01';", "", "database"},
		{"mixed streams", database, secret, "driver_exit"},
		{"mixed terminal errors", database + "error: " + secret, "", "driver_exit"},
		{"oversized stderr", database + strings.Repeat("x", 16*1024), "", "driver_exit"},
		{"oversized stdout", database, strings.Repeat("x", 16*1024), "driver_exit"},
		{"unanchored prefix", secret + database, "", "driver_exit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runMigration(context.Background(), diagnosticCommand(context.Background(), tc.stderr, tc.stdout, "7"), 44)
			require.EqualError(t, err, "migration stage 44 failed (category="+tc.category+", exit_code=7)")
			var primary *exec.ExitError
			require.ErrorAs(t, err, &primary)
			require.Equal(t, 7, primary.ExitCode())
			require.Empty(t, primary.Stderr, "exec error must not retain raw diagnostics")
			unlock := errors.New(secret + "/unlock")
			closeErr := errors.New(secret + "/close")
			admin := &cleanupAdminStub{unlockErr: unlock, closeErr: closeErr}
			joined := cleanupAdmin(err, admin, true)
			require.ErrorIs(t, joined, primary)
			require.ErrorIs(t, joined, unlock)
			require.ErrorIs(t, joined, closeErr)
			require.Equal(t, []string{"unlock", "close"}, admin.operations)
			for _, value := range []string{secret, "credential-sentinel", "SQLSTATE", "42501", "28P01", "SELECT", "permission denied"} {
				require.NotContains(t, joined.Error(), value)
			}
		})
	}
}

func TestRunMigrationContextStartAndSuccess(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	deadline, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, tc := range []struct {
		name, category string
		ctx            context.Context
		cause          error
	}{
		{"canceled", "canceled", canceled, context.Canceled},
		{"deadline", "deadline", deadline, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runMigration(tc.ctx, diagnosticCommand(tc.ctx, "", "", "0"), 45)
			require.EqualError(t, err, "migration stage 45 failed (category="+tc.category+", exit_code=-1)")
			require.ErrorIs(t, err, tc.cause)
		})
	}
	err := runMigration(context.Background(), exec.Command("/missing/credential-sentinel"), 50)
	require.EqualError(t, err, "migration stage 50 failed (category=start, exit_code=-1)")
	var pathError *os.PathError
	require.ErrorAs(t, err, &pathError)
	require.NoError(t, runMigration(context.Background(), diagnosticCommand(context.Background(), "secret-success", "secret-success", "0"), 50))
}

func TestMigrationDiagnosticStreamsBoundAndDrain(t *testing.T) {
	var output migrationDiagnostics
	stderr := migrationDiagnosticStream{diagnostics: &output}
	for _, chunk := range []string{"error: migra", "tion failed: ", "synthetic credential"} {
		n, err := stderr.Write([]byte(chunk))
		require.NoError(t, err)
		require.Equal(t, len(chunk), n)
	}
	require.Equal(t, "database", output.category())
	remaining := migrationDiagnosticLimit - output.seen
	n, err := stderr.Write(bytes.Repeat([]byte{'x'}, remaining))
	require.NoError(t, err)
	require.Equal(t, remaining, n)
	require.Equal(t, migrationDiagnosticLimit, output.size)
	require.False(t, output.overflow)
	n, err = stderr.Write([]byte("overflow credential"))
	require.NoError(t, err)
	require.Equal(t, len("overflow credential"), n)
	require.True(t, output.overflow)
	require.Zero(t, output.size)
	require.Equal(t, [migrationDiagnosticLimit]byte{}, output.stderr)
	require.Equal(t, "driver_exit", output.category())
	n, err = stderr.Write(bytes.Repeat([]byte{'x'}, migrationDiagnosticLimit*2))
	require.NoError(t, err)
	require.Equal(t, migrationDiagnosticLimit*2, n)
	require.Equal(t, [migrationDiagnosticLimit]byte{}, output.stderr)
}

func TestRunMigrationStartedDeadlineRetainsExitError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := runMigration(ctx, exec.CommandContext(ctx, "sleep", "10"), 44)
	require.EqualError(t, err, "migration stage 44 failed (category=deadline, exit_code=-1)")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var primary *exec.ExitError
	require.ErrorAs(t, err, &primary)
	require.Equal(t, -1, primary.ExitCode())
}

type cleanupReadCloser struct {
	io.Reader
	closeErr error
	closes   int
}

func (r *cleanupReadCloser) Close() error { r.closes++; return r.closeErr }

type failingHashReader struct{ err error }

func (r failingHashReader) Read([]byte) (int, error) { return 0, r.err }

func TestHashReaderAndClosePreservesBothFailures(t *testing.T) {
	readErr := errors.New("input read failed")
	closeErr := errors.New("postgres://secret-cleanup-fixture@invalid/hash")
	for _, tc := range []struct {
		name              string
		readErr, closeErr error
	}{
		{"success", nil, nil}, {"close failure", nil, closeErr},
		{"read failure", readErr, nil}, {"read and close failure", readErr, closeErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reader io.Reader = strings.NewReader("abc")
			if tc.readErr != nil {
				reader = failingHashReader{tc.readErr}
			}
			input := &cleanupReadCloser{Reader: reader, closeErr: tc.closeErr}
			got, err := hashReaderAndClose(input)
			require.Equal(t, 1, input.closes)
			if tc.readErr == nil && tc.closeErr == nil {
				require.NoError(t, err)
				require.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", got)
				return
			}
			require.Error(t, err)
			require.Empty(t, got)
			if tc.readErr != nil {
				require.ErrorIs(t, err, tc.readErr)
			}
			if tc.closeErr != nil {
				require.ErrorIs(t, err, tc.closeErr)
				require.ErrorContains(t, err, "input close failed")
			}
			require.NotContains(t, err.Error(), "secret-cleanup-fixture")
		})
	}
}

type cleanupAdminStub struct {
	unlockErr, closeErr error
	operations          []string
}

func (a *cleanupAdminStub) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	a.operations = append(a.operations, "unlock")
	if ctx.Err() != nil || sql != `SELECT pg_advisory_unlock(hashtextextended('voice-space-native-installer',0))` || len(args) != 0 {
		return pgconn.CommandTag{}, errors.New("unexpected cleanup invocation")
	}
	return pgconn.CommandTag{}, a.unlockErr
}
func (a *cleanupAdminStub) Close(ctx context.Context) error {
	a.operations = append(a.operations, "close")
	if ctx.Err() != nil {
		return errors.New("canceled cleanup context")
	}
	return a.closeErr
}

func TestCleanupAdminPreservesPrimaryAndBothCleanupFailures(t *testing.T) {
	primary := errors.New("migration stage 44 failed")
	unlockErr := errors.New("postgres://secret-cleanup-fixture@invalid/unlock")
	closeErr := errors.New("postgres://secret-cleanup-fixture@invalid/close")
	for _, tc := range []struct {
		name                         string
		held                         bool
		primary, unlockErr, closeErr error
	}{
		{"unlocked success", false, nil, nil, nil},
		{"unlocked close failure", false, primary, unlockErr, closeErr},
		{"locked success", true, nil, nil, nil},
		{"primary only", true, primary, nil, nil},
		{"unlock failure", true, nil, unlockErr, nil},
		{"close failure", true, nil, nil, closeErr},
		{"all failures", true, primary, unlockErr, closeErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			admin := &cleanupAdminStub{unlockErr: tc.unlockErr, closeErr: tc.closeErr}
			err := cleanupAdmin(tc.primary, admin, tc.held)
			want := []string{"close"}
			if tc.held {
				want = []string{"unlock", "close"}
			}
			require.Equal(t, want, admin.operations)
			if tc.primary == nil && (!tc.held || tc.unlockErr == nil) && tc.closeErr == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			if tc.primary != nil {
				require.ErrorIs(t, err, tc.primary)
			}
			if tc.held && tc.unlockErr != nil {
				require.ErrorIs(t, err, tc.unlockErr)
				require.ErrorContains(t, err, "installer advisory unlock failed")
			}
			if tc.closeErr != nil {
				require.ErrorIs(t, err, tc.closeErr)
				require.ErrorContains(t, err, "administrative connection close failed")
			}
			require.NotContains(t, err.Error(), "secret-cleanup-fixture")
		})
	}
}

func TestAdminConfigurationRejectsKeywordDSNBeforeAnySourceOrDBWork(t *testing.T) {
	t.Setenv("PGHOST", "127.0.0.1")
	t.Setenv("PGSSLMODE", "disable")
	oldRevision, oldSource, oldMigrate := SourceRevision, BuildSourceSHA256, MigrateSHA256
	t.Cleanup(func() { SourceRevision, BuildSourceSHA256, MigrateSHA256 = oldRevision, oldSource, oldMigrate })
	SourceRevision, BuildSourceSHA256, MigrateSHA256 = "", strings.Repeat("a", 64), strings.Repeat("b", 64)
	for _, admin := range []string{"host=localhost user=installer sslmode=verify-full sslrootcert=/controlled/ca.pem", "host=localhost sslmode=require", "service=production", "https://localhost/postgres"} {
		err := Run(context.Background(), Config{AdminURL: admin, Mode: "provision", Output: io.Discard, RequiredBuildSHA256: "different"})
		require.ErrorContains(t, err, "PostgreSQL URI or PG environment", "keyword TLS settings must never be discarded for the migration child")
	}
	for _, admin := range []string{"", "postgres://installer@localhost/postgres?sslmode=disable", "postgresql://installer@localhost/postgres?sslmode=require"} {
		err := Run(context.Background(), Config{AdminURL: admin, Mode: "provision", Output: io.Discard, RequiredBuildSHA256: "different"})
		require.ErrorContains(t, err, "selected deployment source", "PG environment and PostgreSQL URIs proceed to the next guard without opening a DB")
	}
}

func TestCanonicalConnectionsMatchPinnedDriverParsers(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	require.NoError(t, err)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600))
	for _, env := range []string{"PGSERVICE", "PGSERVICEFILE", "PGSSLMODE", "PGSSLROOTCERT", "PGSSLCERT", "PGSSLKEY"} {
		t.Setenv(env, "")
	}
	t.Setenv("PGHOST", "127.0.0.1")
	t.Setenv("PGPORT", "6543")
	t.Setenv("PGUSER", "installer")
	t.Setenv("PGPASSWORD", "fixture:password@only")
	t.Setenv("PGDATABASE", "foreign_environment_db")
	for _, tc := range []struct{ name, raw, ambient, want string }{
		{"URI explicit disable", "postgres://installer:fixture@127.0.0.1:6543/foreign?sslmode=disable", "require", "disable"},
		{"URI explicit require", "postgresql://installer:fixture@127.0.0.1:6543/foreign?sslmode=require", "disable", "require"},
		{"URI verified CA", "postgres://installer@127.0.0.1:6543/foreign?sslmode=verify-ca&sslrootcert=" + url.QueryEscape(caFile), "disable", "verify-ca"},
		{"URI verified hostname", "postgres://installer@127.0.0.1:6543/foreign?sslmode=verify-full&sslrootcert=" + url.QueryEscape(caFile), "disable", "verify-full"},
		{"PG env disable", "", "disable", "disable"},
		{"PG env require", "", "require", "require"},
		{"PG env verified CA", "", "verify-ca", "verify-ca"},
		{"PG env verified hostname", "", "verify-full", "verify-full"},
		{"PG env default", "", "", "require"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PGSSLMODE", tc.ambient)
			if tc.raw == "" && strings.HasPrefix(tc.ambient, "verify-") {
				t.Setenv("PGSSLROOTCERT", caFile)
			}
			admin, childURI, err := installerConnections(tc.raw)
			require.NoError(t, err)
			child, err := pgx.ParseConfig(childURI.String())
			require.NoError(t, err)
			require.Equal(t, "postgres", admin.Database)
			require.Equal(t, "space_db", child.Database)
			require.Equal(t, admin.Host, child.Host)
			require.Equal(t, admin.Port, child.Port)
			require.Equal(t, admin.User, child.User)
			require.Equal(t, admin.Password, child.Password)
			require.Equal(t, tc.want, childURI.Query().Get("sslmode"))
			adminDSN, err := pq.ParseURL(admin.ConnString())
			require.NoError(t, err)
			childDSN, err := pq.ParseURL(childURI.String())
			require.NoError(t, err)
			require.Equal(t, 1, strings.Count(adminDSN, "dbname='"))
			require.Equal(t, 1, strings.Count(childDSN, "dbname='"))
			require.Equal(t, strings.Replace(adminDSN, "dbname='postgres'", "dbname='space_db'", 1), childDSN, "actual pq parser inputs differ only at the deliberate database target")
			require.Contains(t, childDSN, "sslmode='"+tc.want+"'")
			if tc.want == "disable" {
				require.Nil(t, admin.TLSConfig)
				require.Nil(t, child.TLSConfig)
			} else {
				require.NotNil(t, admin.TLSConfig)
				require.NotNil(t, child.TLSConfig)
				require.Equal(t, admin.TLSConfig.InsecureSkipVerify, child.TLSConfig.InsecureSkipVerify)
				require.Equal(t, admin.TLSConfig.ServerName, child.TLSConfig.ServerName)
				if admin.TLSConfig.RootCAs != nil {
					require.True(t, admin.TLSConfig.RootCAs.Equal(child.TLSConfig.RootCAs))
				}
			}
			env := map[string]string{}
			for _, value := range migrationEnvironment(admin, childURI) {
				key, val, _ := strings.Cut(value, "=")
				env[key] = val
			}
			require.Equal(t, admin.Password, env["PGPASSWORD"])
			require.Equal(t, "/dev/null", env["PGPASSFILE"])
			require.Equal(t, "space_db", env["PGDATABASE"])
			require.Equal(t, tc.want, env["PGSSLMODE"])
			require.Equal(t, childURI.Query().Get("sslrootcert"), env["PGSSLROOTCERT"])
		})
	}
	for _, raw := range []string{
		"postgres://localhost/postgres?dbname=zz_other", "postgres://localhost/postgres?%64bname=zz_other",
		"postgres://localhost/postgres?dbname=space_db&dbname=zz_other", "postgres://localhost/postgres?sslmode=disable&ssl%6dode=require",
		"postgres://localhost/postgres?sslmode=require&sslmode=verify-full", "postgres://localhost/postgres?sslmode=prefer",
		"postgres://localhost/postgres?sslmode=verify-full", "postgres://localhost/postgres?options=-c+role%3Dother",
		"host=localhost sslmode=verify-full sslrootcert=" + caFile,
	} {
		_, _, err := installerConnections(raw)
		require.Error(t, err, "unsupported/ambiguous input must fail before mutation")
		require.NotContains(t, err.Error(), "fixture:password", "connection errors must redact credentials")
	}
	// This is the actual pinned child's parser behavior that made a query target
	// unsafe: path replacement alone emits two competing database selectors.
	unsafe, err := pq.ParseURL("postgres://localhost/space_db?dbname=zz_other")
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(unsafe, "dbname='"))
}

func TestAdminURIRejectsConnectionSelectorQueryBeforeAnySourceOrDBWork(t *testing.T) {
	oldRevision, oldSource, oldMigrate := SourceRevision, BuildSourceSHA256, MigrateSHA256
	t.Cleanup(func() { SourceRevision, BuildSourceSHA256, MigrateSHA256 = oldRevision, oldSource, oldMigrate })
	SourceRevision, BuildSourceSHA256, MigrateSHA256 = "", strings.Repeat("a", 64), strings.Repeat("b", 64)
	for _, query := range []string{"dbname=zz_other", "database=zz_other", "%64bname=zz_other", "DBNAME=zz_other", "host=other", "hostaddr=192.0.2.1", "port=6543", "user=other", "password=other", "service=other", "servicefile=/other"} {
		err := Run(context.Background(), Config{AdminURL: "postgres://installer@localhost/postgres?" + query, Mode: "provision", Output: io.Discard, RequiredBuildSHA256: "different"})
		require.ErrorContains(t, err, "connection selectors", "URI path replacement cannot leave an overriding migration target/identity query")
	}
}

func completedFixture() *Receipt {
	f := &store.NativeInstallerSourceFacts{Version: 44, DatabaseSystemID: "cluster", DatabaseName: "space_db", DatabaseOID: 42, InstallerRole: "installer", Epoch: "94db7e08-e958-44fa-94b2-988410ab0044", SealSHA256: strings.Repeat("a", 64)}
	p := &store.NativeMembershipCatalog45Receipt{DatabaseSystemID: f.DatabaseSystemID, DatabaseName: f.DatabaseName, DatabaseOID: f.DatabaseOID, InstallerRole: f.InstallerRole, Epoch: f.Epoch, OldSealSHA256: f.SealSHA256, NewSealSHA256: strings.Repeat("b", 64), Changed: true}
	q := store.NativeMembershipCatalog50Receipt(*p)
	q.OldSealSHA256 = p.NewSealSHA256
	q.NewSealSHA256 = strings.Repeat("c", 64)
	end := *f
	end.Version = 51
	end.SealSHA256 = strings.Repeat("9", 64)
	trusted50 := *f
	trusted50.Version, trusted50.SealSHA256 = 50, q.NewSealSHA256
	t51 := store.NativeMembershipCatalog45Receipt(q)
	t51.OldSealSHA256, t51.NewSealSHA256, t51.MigrationSHA256 = q.NewSealSHA256, end.SealSHA256, store.NativeCatalog51MigrationSHA256
	transition51 := &store.NativeMembershipCatalog51Receipt{NativeMembershipCatalog45Receipt: t51, Profile: store.NativeCatalogRowJSONProfile, Engine: "github.com/golang-migrate/migrate/v4@v4.18.1"}
	created := *f
	created.Version = 0
	created.Epoch = ""
	created.SealSHA256 = ""
	return &Receipt{RunID: uuid.NewString(), SourceRevision: strings.Repeat("d", 40), SourceSHA256: strings.Repeat("e", 64), ExecutableSHA256: strings.Repeat("f", 64), MigratorSHA256: strings.Repeat("a", 64), InventorySHA256: InventorySHA256, Created: true, Creation: &created, Trusted44: f, Transition45: p, Transition50: &q, Trusted50: &trusted50, Transition51: transition51, Final: &end}
}

func TestRetainedEvidenceMustBeActualCompleteLinkedChain(t *testing.T) {
	for _, kind := range []string{"valid", "different target", "wrong parent seal", "not51", "notcreated", "verified only", "incomplete", "duplicate", "old50 only", "foreign51 parent", "wrong51 profile"} {
		t.Run(kind, func(t *testing.T) {
			r := completedFixture()
			name := "complete"
			switch kind {
			case "different target":
				r.Final.DatabaseOID++
			case "wrong parent seal":
				r.Transition50.OldSealSHA256 = strings.Repeat("d", 64)
			case "not51":
				r.Final.Version = 45
			case "old50 only":
				r.Final.Version = 50
				r.Transition51 = nil
			case "foreign51 parent":
				r.Transition51.OldSealSHA256 = strings.Repeat("0", 64)
			case "wrong51 profile":
				r.Transition51.Profile = "native_catalog_50"
			case "notcreated":
				r.Created = false
			case "verified only":
				name = "verified"
			case "incomplete":
				r.Transition45 = nil
			}
			var out bytes.Buffer
			require.NoError(t, json.NewEncoder(&out).Encode(event{Event: name, Receipt: r}))
			if kind == "duplicate" {
				require.NoError(t, json.NewEncoder(&out).Encode(event{Event: name, Receipt: r}))
			}
			got, err := ReadReceipt(&out)
			if kind == "valid" {
				require.NoError(t, err)
				require.Equal(t, r, got)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestRetainedEvidenceRejectsOverflowBeyondValidPrefix(t *testing.T) {
	var prefix bytes.Buffer
	require.NoError(t, json.NewEncoder(&prefix).Encode(event{Event: "complete", Receipt: completedFixture()}))
	const limit = 1 << 20
	within := append(append([]byte(nil), prefix.Bytes()...), bytes.Repeat([]byte(" "), limit-prefix.Len())...)
	_, err := ReadReceipt(bytes.NewReader(within))
	require.NoError(t, err, "the exact bound still permits a complete original record")
	for _, suffix := range [][]byte{[]byte(" "), prefix.Bytes()} {
		oversized := append(append([]byte(nil), within...), suffix...)
		got, err := ReadReceipt(bytes.NewReader(oversized))
		require.Error(t, err, "overflow cannot hide a second complete record beyond the parsing bound")
		require.Nil(t, got)
	}
}

func TestNative51UnqualifiedSourceCannotCreateOrComplete(t *testing.T) {
	oldRevision, oldSource, oldMigrate := SourceRevision, BuildSourceSHA256, MigrateSHA256
	t.Cleanup(func() { SourceRevision, BuildSourceSHA256, MigrateSHA256 = oldRevision, oldSource, oldMigrate })
	SourceRevision, BuildSourceSHA256, MigrateSHA256 = "", strings.Repeat("a", 64), strings.Repeat("b", 64)
	var out bytes.Buffer
	err := Run(context.Background(), Config{AdminURL: "postgres://installer@127.0.0.1:1/postgres?sslmode=disable", Mode: "provision", Output: &out, MigrationsDir: "/unavailable"})
	require.ErrorContains(t, err, "HOLD: current native51 source/session qualification", "current stability HOLD precedes SQL inventory, connections and CREATE")
	require.Empty(t, out.String())
}

func TestSelectedSQLRejectsMissingExtraModifiedAndSymlink(t *testing.T) {
	original := filepath.Join("..", "..", "..", "migrations", "space_db")
	require.DirExists(t, original)
	for _, kind := range []string{"valid", "missing", "extra", "modified", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			for name := range selectedSQL {
				data, err := os.ReadFile(filepath.Join(original, name))
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0600))
			}
			const first = "000001_initial.up.sql"
			// Select a real name without relying on the naming of the initial schema.
			name := ""
			for n := range selectedSQL {
				name = n
				break
			}
			_ = first
			switch kind {
			case "missing":
				require.NoError(t, os.Remove(filepath.Join(dir, name)))
			case "extra":
				require.NoError(t, os.WriteFile(filepath.Join(dir, "000099_unqualified.up.sql"), []byte("SELECT 1;"), 0600))
			case "modified":
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0600))
			case "symlink":
				require.NoError(t, os.Remove(filepath.Join(dir, name)))
				if err := os.Symlink(filepath.Join(original, name), filepath.Join(dir, name)); err != nil {
					t.Skip("symlink unavailable on this test host")
				}
			}
			got, err := ValidateSources(dir)
			if kind == "valid" {
				require.NoError(t, err)
				require.Equal(t, InventorySHA256, got)
			} else {
				require.Error(t, err)
			}
		})
	}
}
