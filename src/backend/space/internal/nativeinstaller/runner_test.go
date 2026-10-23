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
	"io"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/store"
)

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
	end.Version = 50
	end.SealSHA256 = q.NewSealSHA256
	created := *f
	created.Version = 0
	created.Epoch = ""
	created.SealSHA256 = ""
	return &Receipt{RunID: uuid.NewString(), SourceRevision: strings.Repeat("d", 40), SourceSHA256: strings.Repeat("e", 64), ExecutableSHA256: strings.Repeat("f", 64), MigratorSHA256: strings.Repeat("a", 64), InventorySHA256: InventorySHA256, Created: true, Creation: &created, Trusted44: f, Transition45: p, Transition50: &q, Final: &end}
}

func TestRetainedEvidenceMustBeActualCompleteLinkedChain(t *testing.T) {
	for _, kind := range []string{"valid", "different target", "wrong parent seal", "not50", "notcreated", "verified only", "incomplete", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			r := completedFixture()
			name := "complete"
			switch kind {
			case "different target":
				r.Final.DatabaseOID++
			case "wrong parent seal":
				r.Transition50.OldSealSHA256 = strings.Repeat("d", 64)
			case "not50":
				r.Final.Version = 45
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
