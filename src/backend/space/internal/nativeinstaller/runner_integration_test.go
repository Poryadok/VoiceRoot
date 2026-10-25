//go:build integration

package nativeinstaller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
	"voice/backend/space/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// The caller supplies one genuinely fresh owned private server. This test never
// discovers a shared environment or a Docker socket and never resets a target.
func TestProductionFreshChainAndRetainedVerify(t *testing.T) {
	admin := os.Getenv("NATIVE_INSTALLER_TEST_ADMIN_URL")
	driver := os.Getenv("NATIVE_INSTALLER_TEST_MIGRATE")
	require.NotEmpty(t, admin, "owned private fresh server required")
	require.NotEmpty(t, driver)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var out bytes.Buffer
	c := Config{AdminURL: admin, MigratePath: driver, MigrationsDir: "../../../migrations/space_db", Mode: "provision", Output: &out}
	require.NoError(t, Run(ctx, c))
	prior, err := ReadReceipt(bytes.NewReader(out.Bytes()))
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		owner, err := pgx.Connect(cleanupCtx, admin)
		require.NoError(t, err)
		defer owner.Close(context.Background())
		var actualOID uint32
		require.NoError(t, owner.QueryRow(cleanupCtx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&actualOID))
		require.Equal(t, prior.Creation.DatabaseOID, actualOID)
		_, err = owner.Exec(cleanupCtx, `DROP DATABASE space_db`)
		require.NoError(t, err)
	})
	require.EqualValues(t, 44, prior.Trusted44.Version)
	require.EqualValues(t, 51, prior.Final.Version)
	require.True(t, prior.Transition45.Changed)
	require.True(t, prior.Transition50.Changed)
	require.EqualValues(t, 50, prior.Trusted50.Version)
	require.True(t, prior.Transition51.Changed)
	require.Equal(t, store.NativeCatalogRowJSONProfile, prior.Transition51.Profile)
	out.Reset()
	c.Mode = "verify"
	c.Prior = prior
	require.NoError(t, Run(ctx, c))
	require.Contains(t, out.String(), `"event":"verified"`)
	require.NotContains(t, out.String(), `"event":"complete"`)
	out.Reset()
	c.Mode = "provision"
	c.Prior = nil
	require.ErrorContains(t, Run(ctx, c), "HOLD")
	require.Empty(t, out.String(), "existing target cannot be relabeled fresh")
}

type failedEvidenceWriter struct {
	at    string
	saved bytes.Buffer
}

func (w *failedEvidenceWriter) Write(p []byte) (int, error) {
	var e event
	if err := json.Unmarshal(p, &e); err != nil {
		return 0, err
	}
	if e.Event == w.at {
		return 0, errors.New("controlled deployment evidence persistence failure")
	}
	return w.saved.Write(p)
}

// The natural evidence-writer boundary injects loss of the owning deployment
// window before/after every real committed migration and transition. There is
// no production fault flag, fake migrate engine, marker force or epoch update.
func TestCommittedPhaseFailureNeverAutoResumes(t *testing.T) {
	adminURL := os.Getenv("NATIVE_INSTALLER_TEST_ADMIN_URL")
	driver := os.Getenv("NATIVE_INSTALLER_TEST_MIGRATE")
	require.NotEmpty(t, adminURL)
	require.NotEmpty(t, driver)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	require.NoError(t, err)
	defer admin.Close(context.Background())
	var existed bool
	require.NoError(t, admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname='space_db')`).Scan(&existed))
	require.False(t, existed, "fixture must start absent; never reset an unknown database")
	cases := []struct {
		event   string
		version int64
	}{{"created", 0}, {"migrated44", 44}, {"trusted44", 44}, {"migrated45", 45}, {"trusted45", 45}, {"migrated50", 50}, {"trusted50", 50}, {"trusted51", 51}, {"complete", 51}}
	for _, test := range cases {
		t.Run(test.event, func(t *testing.T) {
			w := &failedEvidenceWriter{at: test.event}
			c := Config{AdminURL: adminURL, MigratePath: driver, MigrationsDir: "../../../migrations/space_db", Mode: "provision", Output: w}
			require.ErrorContains(t, Run(ctx, c), "evidence persistence failure")
			var ownedOID uint32
			require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&ownedOID))
			require.NotZero(t, ownedOID)
			targetCfg, err := pgx.ParseConfig(adminURL)
			require.NoError(t, err)
			targetCfg.Database = "space_db"
			target, err := pgx.ConnectConfig(ctx, targetCfg)
			require.NoError(t, err)
			var marker bool
			require.NoError(t, target.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&marker))
			if test.version == 0 {
				require.False(t, marker)
			} else {
				var version int64
				var dirty bool
				require.NoError(t, target.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
				require.Equal(t, test.version, version)
				require.False(t, dirty)
			}
			require.NoError(t, target.Close(ctx))
			c.Output = &bytes.Buffer{}
			require.ErrorContains(t, Run(ctx, c), "HOLD: existing database")
			_, err = ReadReceipt(bytes.NewReader(w.saved.Bytes()))
			require.Error(t, err, "partial events never become completed provenance")
			var sameOID uint32
			require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&sameOID))
			require.Equal(t, ownedOID, sameOID)
			// This fixture alone just created this exact database; the external private
			// server is discarded at the end. No cleanup targets an unknown/shared DB.
			_, err = admin.Exec(ctx, `DROP DATABASE space_db`)
			require.NoError(t, err)
		})
	}
}
