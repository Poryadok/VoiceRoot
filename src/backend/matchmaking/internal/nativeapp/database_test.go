package nativeapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	w "voice/backend/pkg/mmadmissionwire"
	"voice/backend/pkg/mmnativecontract"
)

func nativeDBFixture(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("BE011_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("real PostgreSQL unavailable: set BE011_TEST_PG_DSN")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	name := "mm_native_" + uuid.NewString()[:8]
	_, err = admin.Exec(ctx, `CREATE DATABASE `+name)
	require.NoError(t, err)
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.Database = name
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(context.Background(), `DROP DATABASE `+name); admin.Close() })
	files, err := filepath.Glob("../../../migrations/matchmaking_db/*.up.sql")
	require.NoError(t, err)
	sort.Strings(files)
	require.GreaterOrEqual(t, len(files), 25)
	for _, file := range files {
		b, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(b))
		require.NoError(t, err, file)
	}
	return pool
}

// Canonical LOCAL_OWNED receipt fixture. Only the independent admission/physical
// driver can establish a real launch; this test issues no installed authority.
func installNativeHeadFixture(t *testing.T, pool *pgxpool.Pool, d w.DescriptorV1) (w.DescriptorV1, w.AdmissionStampV1) {
	t.Helper()
	ctx := context.Background()
	receipt := w.ArmedReceiptV1{Version: 1, AuthorityScope: d.AuthorityScope, ScopeID: d.ScopeID, ControllerID: d.ControllerID, TransitionID: d.Binding.TransitionID, DatabaseLineageID: d.DatabaseLineageID, Binding: d.Binding, LaunchSpecHash: d.Binding.LaunchManifestHash, ClosureEvidenceHash: w.Hash{1}, PreparationEvidenceHash: w.Hash{2}}
	receipt.Binding.AdmissionReceiptHash = w.Hash{}
	hash, err := w.ArmedHash(receipt)
	require.NoError(t, err)
	d.Binding.AdmissionReceiptHash = hash
	root, err := w.HistoryGenesisHash(hash, d.DatabaseLineageID)
	require.NoError(t, err)
	raw, err := json.Marshal(receipt)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO mm_native_admission_receipts VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, hash[:], raw, uuid.UUID(d.DatabaseLineageID), uuid.UUID(d.Binding.AuthorityIncarnation), uuid.UUID(d.Binding.TransitionID), int64(d.Binding.AdmissionGeneration), uuid.UUID(d.Binding.EnvironmentID), uuid.UUID(d.Binding.InstallationID))
	require.NoError(t, err)
	b := d.Binding
	_, err = pool.Exec(ctx, `INSERT INTO mm_native_admission_head(protocol_version,state,environment_id,installation_id,authority_incarnation,transition_id,admission_generation,credential_cohort_id,database_binding_sha256,schema_contract_sha256,source_factory_manifest_sha256,fresh_route_manifest_sha256,admission_receipt_sha256,launch_manifest_sha256,expected_login_role_oid,expected_login_role_name,database_lineage_id,continuity_history_root_sha256) VALUES(1,'ADMITTED',$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`, uuid.UUID(b.EnvironmentID), uuid.UUID(b.InstallationID), uuid.UUID(b.AuthorityIncarnation), uuid.UUID(b.TransitionID), int64(b.AdmissionGeneration), uuid.UUID(b.CredentialCohortID), b.DatabaseBindingHash[:], b.SchemaContractHash[:], b.SourceFactoryManifestHash[:], b.FreshRouteManifestHash[:], b.AdmissionReceiptHash[:], b.LaunchManifestHash[:], b.ExpectedLoginRoleOID, b.ExpectedLoginRoleName, uuid.UUID(d.DatabaseLineageID), root[:])
	require.NoError(t, err)
	bh, err := w.BindingHash(b)
	require.NoError(t, err)
	return d, w.AdmissionStampV1{Version: 1, BindingHash: bh, DatabaseLineageID: d.DatabaseLineageID, ContinuityHistoryRootHash: root}
}

func TestNativeDatabaseGuardRequiresRestrictedActualLoginAndHistory(t *testing.T) {
	admin := nativeDBFixture(t)
	ctx := context.Background()
	role := "mm_native_app_" + uuid.NewString()[:8]
	_, err := admin.Exec(ctx, `CREATE ROLE `+role+` LOGIN PASSWORD 'native-fixture-owned'`)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `GRANT USAGE ON SCHEMA public TO `+role)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `GRANT SELECT ON ALL TABLES IN SCHEMA public TO `+role)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, `GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO `+role)
	require.NoError(t, err)
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User = role
	cfg.ConnConfig.Password = "native-fixture-owned"
	app, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		app.Close()
		_, _ = admin.Exec(context.Background(), `DROP OWNED BY `+role)
		_, _ = admin.Exec(context.Background(), `DROP ROLE `+role)
	})
	facts := observeDBFixture(t, app)
	d := descriptorFixture(t)
	d.Binding.DatabaseBindingHash = w.Hash(facts.DatabaseBindingHash)
	d.Binding.SchemaContractHash = w.Hash(facts.SchemaContractHash)
	d.Binding.ExpectedLoginRoleOID = facts.SessionRoleOID
	d.Binding.ExpectedLoginRoleName = facts.SessionRoleName
	d, stamp := installNativeHeadFixture(t, admin, d)
	require.NoError(t, checkNativeDatabase(ctx, app, d, stamp), "actual restricted LOGIN and canonical local head must proceed")
	require.ErrorIs(t, checkNativeDatabase(ctx, admin, d, stamp), errUnavailable, "owner/admin DSN cannot substitute for the bound LOGIN")
	changed := stamp
	changed.ContinuityHistoryRootHash = w.Hash{99}
	require.ErrorIs(t, checkNativeDatabase(ctx, app, d, changed), errUnavailable)
	_, err = admin.Exec(ctx, `GRANT INSERT(protocol_version) ON mm_native_admission_head TO `+role)
	require.NoError(t, err)
	require.ErrorIs(t, checkNativeDatabase(ctx, app, d, stamp), errUnavailable, "column-only authority must not be masked by has_table_privilege")
	_, err = admin.Exec(ctx, `REVOKE INSERT(protocol_version) ON mm_native_admission_head FROM `+role)
	require.NoError(t, err)
	require.NoError(t, checkNativeDatabase(ctx, app, d, stamp))
	_, err = admin.Exec(ctx, `GRANT INSERT ON matchmaking_match_response_receipts TO `+role)
	require.NoError(t, err)
	require.NoError(t, checkNativeDatabase(ctx, app, d, stamp), "serving may append a verified semantic receipt")
	_, err = admin.Exec(ctx, `GRANT UPDATE(request_sha256) ON matchmaking_match_response_receipts TO `+role)
	require.NoError(t, err)
	require.ErrorIs(t, checkNativeDatabase(ctx, app, d, stamp), errUnavailable, "column-only receipt rewrite authority must be denied")
	_, err = admin.Exec(ctx, `REVOKE UPDATE(request_sha256) ON matchmaking_match_response_receipts FROM `+role)
	require.NoError(t, err)
	require.NoError(t, checkNativeDatabase(ctx, app, d, stamp))
	_, err = admin.Exec(ctx, `GRANT INSERT ON matchmaking_native_timeout_outbox TO `+role+`; GRANT UPDATE(delivered_at,lease_token,lease_until,next_attempt_at) ON matchmaking_native_timeout_outbox TO `+role)
	require.NoError(t, err)
	require.NoError(t, checkNativeDatabase(ctx, app, d, stamp), "serving may append facts and update only delivery bookkeeping")
	_, err = admin.Exec(ctx, `GRANT UPDATE(payload) ON matchmaking_native_timeout_outbox TO `+role)
	require.NoError(t, err)
	require.ErrorIs(t, checkNativeDatabase(ctx, app, d, stamp), errUnavailable, "column-only frozen timeout rewrite must be denied")
	_, err = admin.Exec(ctx, `REVOKE UPDATE(payload) ON matchmaking_native_timeout_outbox FROM `+role)
	require.NoError(t, err)
	require.NoError(t, checkNativeDatabase(ctx, app, d, stamp))
	_, err = admin.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	require.ErrorIs(t, checkNativeDatabase(ctx, app, d, stamp), errUnavailable, "CLOSED is not bootstrap permission")
}

func observeDBFixture(t *testing.T, pool *pgxpool.Pool) mmnativecontract.DatabaseFacts {
	t.Helper()
	tx, err := pool.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	facts, err := mmnativecontract.ObserveDatabase(context.Background(), tx)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(context.Background()))
	return facts
}

func TestNativeDatabaseObservationBindsActualTargetAndCompleteDDL(t *testing.T) {
	a, b := nativeDBFixture(t), nativeDBFixture(t)
	first := observeDBFixture(t, a)
	same := observeDBFixture(t, a)
	other := observeDBFixture(t, b)
	require.NotZero(t, first.DatabaseBindingHash)
	require.NotZero(t, first.SchemaContractHash)
	require.NotZero(t, first.SessionRoleOID)
	require.NotEmpty(t, first.SessionRoleName)
	require.True(t, first.SessionEqualsCurrent)
	require.Equal(t, first, same)
	require.NotEqual(t, first.DatabaseBindingHash, other.DatabaseBindingHash, "same physical server does not identify the database")
	require.Equal(t, first.SchemaContractHash, other.SchemaContractHash, "unrelated database OIDs must not enter schema bytes")
	_, err := a.Exec(context.Background(), `ALTER TABLE parties ADD COLUMN unexpected_native_fixture text`)
	require.NoError(t, err)
	changed := observeDBFixture(t, a)
	require.Equal(t, first.DatabaseBindingHash, changed.DatabaseBindingHash)
	require.NotEqual(t, first.SchemaContractHash, changed.SchemaContractHash)
	_, err = a.Exec(context.Background(), `ALTER TABLE parties DROP COLUMN unexpected_native_fixture`)
	require.NoError(t, err)
	require.Equal(t, first.SchemaContractHash, observeDBFixture(t, a).SchemaContractHash)
	_, err = a.Exec(context.Background(), `ALTER TABLE mm_native_party_origins DISABLE TRIGGER mm_native_origin_no_truncate`)
	require.NoError(t, err)
	require.NotEqual(t, first.SchemaContractHash, observeDBFixture(t, a).SchemaContractHash, "retention trigger enablement is part of actual schema")
}
