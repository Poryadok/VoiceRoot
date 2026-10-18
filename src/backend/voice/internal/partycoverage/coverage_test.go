package partycoverage

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func coverageDB(t *testing.T) (context.Context, *pgx.Conn, string) {
	t.Helper()
	dsn := testPostgresDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close(context.Background()) })
	schema := "coverage_" + uuid.NewString()[:8]
	_, err = conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE") })
	_, err = conn.Exec(ctx, "SET search_path TO "+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "voice_db", "000015_party_coverage.up.sql"))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, string(sql))
	require.NoError(t, err)
	return ctx, conn, schema
}

func fixtureBinding() RuntimeBinding {
	return RuntimeBinding{
		EnvironmentID: uuid.New(), AuthorityIncarnation: uuid.New(), CoverageEpoch: 1,
		DatabaseIdentity: sha256.Sum256([]byte("fixture-storage")), SourceCatalogHash: sha256.Sum256([]byte("fixture-catalogue")), SchemaContractHash: sha256.Sum256([]byte("fixture-schema")), WriterManifestHash: sha256.Sum256([]byte("fixture-writers")),
	}
}

// This intentionally seeds an issuer fixture. It proves storage/read behavior,
// never external retirement or real activation of the Voice runtime.
func fixtureActive(t *testing.T, ctx context.Context, c *pgx.Conn, b RuntimeBinding) (uuid.UUID, uuid.UUID) {
	t.Helper()
	op, id := uuid.New(), uuid.New()
	h := sha256.Sum256([]byte("fixture-only"))
	_, err := c.Exec(ctx, `INSERT INTO voice_party_coverage_operations VALUES($1,$2,$3,$4,$5,$6)`, op, b.EnvironmentID, b.AuthorityIncarnation, int64(b.CoverageEpoch), []byte("fixture-only"), h[:])
	require.NoError(t, err)
	d := receiptDocument{ProtocolVersion: 1, CoverageID: id, OperationID: op, Binding: b, FenceProofHash: h, DrainProofHash: h, NativeProofHash: h}
	raw, err := canonicalReceipt(d)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	_, err = c.Exec(ctx, `INSERT INTO voice_party_coverage_receipts VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$10,$11,$12)`, id, op, b.EnvironmentID, b.AuthorityIncarnation, int64(b.CoverageEpoch), b.DatabaseIdentity[:], b.SourceCatalogHash[:], b.SchemaContractHash[:], b.WriterManifestHash[:], h[:], raw, sum[:])
	require.NoError(t, err)
	_, err = c.Exec(ctx, `INSERT INTO voice_party_coverage_head VALUES(true,$1,$2,$3,'ACTIVE',$4,$5,1)`, b.EnvironmentID, b.AuthorityIncarnation, int64(b.CoverageEpoch), id, op)
	require.NoError(t, err)
	return id, op
}

func TestReadCoverageRequiresBoundReadOnlySnapshot(t *testing.T) {
	ctx, c, _ := coverageDB(t)
	b := fixtureBinding()
	id, _ := fixtureActive(t, ctx, c, b)
	tx, err := c.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	cut, err := ReadCoverage(ctx, tx, b)
	require.NoError(t, err)
	require.Equal(t, id, cut.CoverageID)
	require.Equal(t, uint64(1), cut.HeadRevision)
	for _, bad := range []RuntimeBinding{
		{}, {EnvironmentID: uuid.New()},
		func() RuntimeBinding { x := b; x.AuthorityIncarnation = uuid.New(); return x }(),
		func() RuntimeBinding { x := b; x.CoverageEpoch++; return x }(),
		func() RuntimeBinding { x := b; x.DatabaseIdentity[0]++; return x }(),
		func() RuntimeBinding { x := b; x.SourceCatalogHash[0]++; return x }(),
		func() RuntimeBinding { x := b; x.SchemaContractHash[0]++; return x }(),
		func() RuntimeBinding { x := b; x.WriterManifestHash[0]++; return x }(),
	} {
		_, err = ReadCoverage(ctx, tx, bad)
		require.ErrorIs(t, err, ErrUnavailable)
	}
	require.NoError(t, tx.Rollback(ctx))
	for _, opts := range []pgx.TxOptions{{}, {IsoLevel: pgx.RepeatableRead}, {AccessMode: pgx.ReadOnly}} {
		tx, err = c.BeginTx(ctx, opts)
		require.NoError(t, err)
		_, err = ReadCoverage(ctx, tx, b)
		require.ErrorIs(t, err, ErrUnavailable)
		require.NoError(t, tx.Rollback(ctx))
	}
}

func TestCoverageProofRowsAreAppendOnlyAndRuntimeCannotMint(t *testing.T) {
	ctx, c, schema := coverageDB(t)
	b := fixtureBinding()
	fixtureActive(t, ctx, c, b)
	for _, table := range []string{"voice_party_coverage_operations", "voice_party_coverage_receipts"} {
		_, err := c.Exec(ctx, "DELETE FROM "+table)
		require.Error(t, err)
	}
	require.ErrorIs(t, CheckRuntimePrivileges(ctx, mustTx(t, ctx, c)), ErrUnavailable, "owner cannot be the runtime")
	// Finish the owner transaction before creating a real restricted runtime role.
	_, err := c.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	role := "coverage_reader_" + uuid.NewString()[:8]
	_, err = c.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		c.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		c.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	_, err = c.Exec(ctx, "GRANT USAGE ON SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = c.Exec(ctx, "GRANT SELECT ON ALL TABLES IN SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	runtime := loginRoleConnection(t, ctx, c, schema, role)
	tx, err := runtime.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	require.NoError(t, CheckRuntimePrivileges(ctx, tx))
	_, err = ReadCoverage(ctx, tx, b)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	tx, err = runtime.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "DELETE FROM voice_party_coverage_receipts")
	require.Error(t, err)
	require.NoError(t, tx.Rollback(ctx))
}

func mustTx(t *testing.T, ctx context.Context, c *pgx.Conn) pgx.Tx {
	t.Helper()
	tx, err := c.Begin(ctx)
	require.NoError(t, err)
	return tx
}

func TestStartupCannotDowngradeActiveOrSuspendedWithoutBinding(t *testing.T) {
	ctx, c, _ := coverageDB(t)
	tx, err := c.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	require.NoError(t, CheckStartup(ctx, tx, nil), "pre-cut empty schema is not an installed capability")
	require.NoError(t, tx.Rollback(ctx))
	b := fixtureBinding()
	fixtureActive(t, ctx, c, b)
	for _, phase := range []string{"ACTIVE", "SUSPENDED", "FROZEN", "PREPARING", "DRAINED"} {
		if phase != "ACTIVE" {
			_, err = c.Exec(ctx, `UPDATE voice_party_coverage_head SET phase=$1,coverage_id=NULL,revision=revision+1`, phase)
			require.NoError(t, err)
		}
		tx, err = c.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		require.NoError(t, err)
		require.ErrorIs(t, CheckStartup(ctx, tx, nil), ErrUnavailable, "missing config must not reenable old writers after any begun cut")
		require.NoError(t, tx.Rollback(ctx))
	}
}

func TestStartupRejectsCopiedReceiptWithWrongActualDatabaseIdentity(t *testing.T) {
	ctx, c, schema := coverageDB(t)
	b := fixtureBinding()
	fixtureActive(t, ctx, c, b)
	role := restrictedRuntimeRole(t, ctx, c, schema)
	runtime := loginRoleConnection(t, ctx, c, schema, role)
	tx, err := runtime.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.ErrorIs(t, CheckStartup(ctx, tx, &b), ErrUnavailable, "matching copied head/receipt/config cannot prove the actual connected database identity")
}

func restrictedRuntimeRole(t *testing.T, ctx context.Context, c *pgx.Conn, schema string) string {
	t.Helper()
	role := "coverage_runtime_" + uuid.NewString()[:8]
	_, err := c.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		c.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		c.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	_, err = c.Exec(ctx, "GRANT USAGE ON SCHEMA "+pgx.Identifier{schema}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = c.Exec(ctx, "GRANT SELECT ON voice_party_coverage_head,voice_party_coverage_receipts TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	_, err = c.Exec(ctx, "GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	return role
}

func TestStartupAcceptsRestrictedRuntimeWithObservedDatabaseBinding(t *testing.T) {
	ctx, c, schema := coverageDB(t)
	tx, err := c.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	identity, err := ObserveDatabaseIdentity(ctx, tx)
	require.NoError(t, err)
	require.NotEqual(t, [32]byte{}, identity)
	require.NoError(t, tx.Rollback(ctx))
	b := fixtureBinding()
	b.DatabaseIdentity = identity
	fixtureActive(t, ctx, c, b)
	role := restrictedRuntimeRole(t, ctx, c, schema)
	runtime := loginRoleConnection(t, ctx, c, schema, role)
	tx, err = runtime.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, CheckStartup(ctx, tx, &b))
}

func loginRoleConnection(t *testing.T, ctx context.Context, admin *pgx.Conn, schema, role string) *pgx.Conn {
	t.Helper()
	_, err := admin.Exec(ctx, "ALTER ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD 'isolated-test-only'")
	require.NoError(t, err)
	cfg := admin.Config().Copy()
	cfg.User = role
	cfg.Password = "isolated-test-only"
	cfg.RuntimeParams["search_path"] = schema
	delete(cfg.RuntimeParams, "role")
	c, err := pgx.ConnectConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { c.Close(context.Background()) })
	var session, current string
	require.NoError(t, c.QueryRow(ctx, `SELECT session_user,current_user`).Scan(&session, &current))
	require.Equal(t, role, session)
	require.Equal(t, role, current)
	return c
}

func TestReadCoverageUsesSameMVCCCutAndFreshPassDetectsSuspension(t *testing.T) {
	ctx, c, schema := coverageDB(t)
	b := fixtureBinding()
	fixtureActive(t, ctx, c, b)
	cfg := c.Config().Copy()
	cfg.RuntimeParams["search_path"] = schema
	other, err := pgx.ConnectConfig(ctx, cfg)
	require.NoError(t, err)
	defer other.Close(ctx)
	tx, err := c.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	first, err := ReadCoverage(ctx, tx, b)
	require.NoError(t, err)
	_, err = other.Exec(ctx, `UPDATE voice_party_coverage_head SET phase='SUSPENDED',coverage_id=NULL,revision=revision+1`)
	require.NoError(t, err)
	same, err := ReadCoverage(ctx, tx, b)
	require.NoError(t, err)
	require.Equal(t, first, same)
	require.NoError(t, tx.Rollback(ctx))
	fresh, err := c.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	defer fresh.Rollback(ctx)
	_, err = ReadCoverage(ctx, fresh, b)
	require.ErrorIs(t, err, ErrUnavailable)
}
