package sourcecustody

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestAdoptPristineExisting22PreservesProfilesOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	_, control := recoveryTLSDatabase(t, ctx, "adopt-control")
	_, admin := recoveryTLSDatabase(t, ctx, "adopt-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_adoption_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	physical, id, _ := prepareExistingMigration(t, ctx, control, admin, request, 22)
	_, keys := migrationFixture(t)
	pool := adoptionProbe(t, ctx, admin, physical, true)
	var profilesBefore string
	require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM profiles p`).Scan(&profilesBefore))
	binding := Binding{SourceID: request.SourceID, Epoch: request.Epoch}
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_generation,user_source_generation FROM identity_meta WHERE singleton`).Scan(&binding.HistoryGeneration, &binding.UserSourceGeneration))
	var err error
	binding.KeysetDigest, err = keys.CustodyDigest()
	require.NoError(t, err)
	pool.Close()
	_, err = admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
	require.NoError(t, err)
	receiptID := uuid.New()
	require.NoError(t, AdoptPristineExistingSource(ctx, control, admin, request, physical, binding, keys, receiptID, [32]byte{1}))
	var allowed bool
	var sessions int
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn,(SELECT count(*) FROM pg_stat_activity WHERE datid=$1) FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed, &sessions))
	require.False(t, allowed)
	require.Zero(t, sessions)
	var phase, state, receiptPhase string
	var storedReceipt uuid.UUID
	var evidence []byte
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "EXISTING_INITIALIZED_CLOSED", phase)
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase,receipt_id,evidence_digest FROM user_custody_adoption WHERE singleton`).Scan(&receiptPhase, &storedReceipt, &evidence))
	require.Equal(t, "ADOPTED_CLOSED", receiptPhase)
	require.Equal(t, receiptID, storedReceipt)
	require.Equal(t, byte(1), evidence[0])
	require.Error(t, AdoptPristineExistingSource(ctx, control, admin, request, physical, binding, keys, uuid.New(), [32]byte{2}), "adoption cannot relabel a completed source")
	controller, err := Start(ctx, control, binding)
	require.NoError(t, err)
	_, _, err = controller.AcquireAttested(ctx, binding)
	require.Error(t, err, "CLOSED adoption has no opening authority")
	require.NoError(t, controller.Shutdown(ctx))
	pool = adoptionProbe(t, ctx, admin, physical, true)
	defer pool.Close()
	var profilesAfter string
	require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(p) ORDER BY id)::text FROM profiles p`).Scan(&profilesAfter))
	require.Equal(t, profilesBefore, profilesAfter)
	var guards bool
	var associations int
	require.NoError(t, pool.QueryRow(ctx, `SELECT guards_active,(SELECT count(*) FROM identity_associations) FROM identity_meta WHERE singleton`).Scan(&guards, &associations))
	require.False(t, guards)
	require.Zero(t, associations)
	var name string
	require.NoError(t, pool.QueryRow(ctx, `SELECT display_name FROM profiles WHERE id=$1`, id).Scan(&name))
	require.Equal(t, "Retained profile", name)
}

func TestAdoptionRejectsUnqualifiedInputsOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	_, controlAdmin := recoveryTLSDatabase(t, ctx, "adoption-negative-control")
	_, admin := recoveryTLSDatabase(t, ctx, "adoption-negative-source")
	for _, tc := range []struct {
		name, mutation string
		changePin      func(*Binding, *PhysicalSource)
		beforeAttempt  bool
	}{
		{name: "wrong-physical", changePin: func(_ *Binding, p *PhysicalSource) { p.DatabaseOID++ }, beforeAttempt: true},
		{name: "wrong-history", changePin: func(b *Binding, _ *PhysicalSource) { b.HistoryGeneration = uuid.New() }},
		{name: "wrong-source", changePin: func(b *Binding, _ *PhysicalSource) { b.UserSourceGeneration = uuid.New() }},
		{name: "wrong-keyset", changePin: func(b *Binding, _ *PhysicalSource) { b.KeysetDigest[0] ^= 1 }, beforeAttempt: true},
		{name: "dirty-schema", mutation: `UPDATE schema_migrations SET dirty=true`},
		{name: "wrong-version", mutation: `UPDATE schema_migrations SET version=21`},
		{name: "schema-drift", mutation: `DROP INDEX public.association_unique_profile_alias`},
		{name: "partial-keyset", mutation: `INSERT INTO identity_key_versions(version,fingerprint) VALUES(17,decode(repeat('ab',32),'hex'))`},
		{name: "partial-history", mutation: `SELECT identity_append_event(NULL,NULL,'KEYSET',NULL,NULL,NULL)`},
		{name: "nonempty-head-digest", mutation: `ALTER TABLE identity_meta DISABLE TRIGGER USER; UPDATE identity_meta SET head_digest=decode(repeat('ab',32),'hex'); ALTER TABLE identity_meta ENABLE TRIGGER USER`},
		{name: "profile-baseline", mutation: `ALTER TABLE profiles DISABLE TRIGGER USER; UPDATE profiles SET identity_baseline_cut=1; ALTER TABLE profiles ENABLE TRIGGER USER`},
		{name: "parent-fence", mutation: `INSERT INTO identity_parent_fences VALUES('retained-parent',1,'ACTIVE','{}','{}')`},
		{name: "reservation", mutation: `INSERT INTO identity_key_versions(version,fingerprint) VALUES(17,decode(repeat('ab',32),'hex')); INSERT INTO identity_profile_reservations(key_version,profile_selector) VALUES(17,decode(repeat('bc',32),'hex'))`},
		{name: "active-guards", mutation: `UPDATE identity_meta SET guards_active=true`},
		{name: "public-connect", mutation: "", beforeAttempt: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			control, request := adoptionIsolatedControl(t, ctx, controlAdmin, admin)
			physical, _, _ := prepareExistingMigration(t, ctx, control, admin, request, 22)
			_, keys := migrationFixture(t)
			pool := adoptionProbe(t, ctx, admin, physical, true)
			binding := Binding{SourceID: request.SourceID, Epoch: request.Epoch}
			require.NoError(t, pool.QueryRow(ctx, `SELECT history_generation,user_source_generation FROM identity_meta WHERE singleton`).Scan(&binding.HistoryGeneration, &binding.UserSourceGeneration))
			binding.KeysetDigest, _ = keys.CustodyDigest()
			if tc.mutation != "" {
				_, err := pool.Exec(ctx, tc.mutation)
				require.NoError(t, err)
			}
			pool.Close()
			_, err := admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
			require.NoError(t, err)
			if tc.name == "public-connect" {
				_, err = admin.Exec(ctx, `GRANT CONNECT ON DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` TO PUBLIC`)
				require.NoError(t, err)
			}
			actualPhysical := physical
			if tc.changePin != nil {
				tc.changePin(&binding, &physical)
			}
			require.Error(t, AdoptPristineExistingSource(ctx, control, admin, request, physical, binding, keys, uuid.New(), [32]byte{1}))
			var allowed, stateExists, attemptExists bool
			require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, actualPhysical.DatabaseOID).Scan(&allowed))
			require.False(t, allowed)
			require.NoError(t, control.QueryRow(ctx, `SELECT to_regclass('public.user_custody_state') IS NOT NULL,to_regclass('public.user_custody_adoption') IS NOT NULL`).Scan(&stateExists, &attemptExists))
			require.False(t, stateExists)
			require.Equal(t, !tc.beforeAttempt, attemptExists)
			if attemptExists {
				var phase string
				require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_adoption WHERE singleton`).Scan(&phase))
				require.Equal(t, "ADOPTING", phase)
				require.NoError(t, RecoverInterruptedInitialization(ctx, control, admin, request, actualPhysical))
				require.Error(t, AdoptPristineExistingSource(ctx, control, admin, request, actualPhysical, binding, keys, uuid.New(), [32]byte{1}), "recovery fences but never grants adoption retry")
			}
		})
	}
}

func adoptionIsolatedControl(t *testing.T, ctx context.Context, controlAdmin, admin *pgxpool.Pool) (*pgxpool.Pool, CreationRequest) {
	t.Helper()
	name := "adopt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err := controlAdmin.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()+` TEMPLATE template0`)
	require.NoError(t, err)
	cfg := controlAdmin.Config().Copy()
	cfg.ConnConfig.Database = name
	control, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(control.Close)
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: name}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	return control, request
}

type adoptionCommitCancel struct {
	commits atomic.Int32
	at      int32
	cancel  context.CancelFunc
}

func (a *adoptionCommitCancel) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, adoptionCommitKey{}, strings.EqualFold(strings.TrimSpace(d.SQL), "commit"))
}

type adoptionCommitKey struct{}

func (a *adoptionCommitCancel) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	if commit, _ := ctx.Value(adoptionCommitKey{}).(bool); commit && d.Err == nil && a.commits.Add(1) == a.at {
		a.cancel()
	}
}

func TestAdoptionLostCommitAcknowledgementNeverRetriesOverTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two private TLS PostgreSQL servers")
	}
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	_, controlAdmin := recoveryTLSDatabase(t, ctx, "adoption-ack-control")
	_, admin := recoveryTLSDatabase(t, ctx, "adoption-ack-source")
	for _, at := range []int32{1, 2} {
		t.Run(string(rune('0'+at)), func(t *testing.T) {
			control, request := adoptionIsolatedControl(t, ctx, controlAdmin, admin)
			physical, _, _ := prepareExistingMigration(t, ctx, control, admin, request, 22)
			_, keys := migrationFixture(t)
			pool := adoptionProbe(t, ctx, admin, physical, true)
			binding := Binding{SourceID: request.SourceID, Epoch: request.Epoch}
			require.NoError(t, pool.QueryRow(ctx, `SELECT history_generation,user_source_generation FROM identity_meta WHERE singleton`).Scan(&binding.HistoryGeneration, &binding.UserSourceGeneration))
			binding.KeysetDigest, _ = keys.CustodyDigest()
			pool.Close()
			_, err := admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS false`)
			require.NoError(t, err)
			call, cancel := context.WithCancel(ctx)
			defer cancel()
			trace := &adoptionCommitCancel{at: at, cancel: cancel}
			cfg := control.Config().Copy()
			cfg.ConnConfig.Tracer = trace
			traced, err := pgxpool.NewWithConfig(ctx, cfg)
			require.NoError(t, err)
			defer traced.Close()
			require.Error(t, AdoptPristineExistingSource(call, traced, admin, request, physical, binding, keys, uuid.New(), [32]byte{1}))
			require.Equal(t, at, trace.commits.Load())
			var allowed bool
			require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, physical.DatabaseOID).Scan(&allowed))
			require.False(t, allowed)
			require.Error(t, AdoptPristineExistingSource(ctx, control, admin, request, physical, binding, keys, uuid.New(), [32]byte{1}))
			var phase string
			require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_adoption WHERE singleton`).Scan(&phase))
			if at == 1 {
				require.Equal(t, "ADOPTING", phase)
			} else {
				require.Equal(t, "ADOPTED_CLOSED", phase)
			}
		})
	}
}

func adoptionProbe(t *testing.T, ctx context.Context, admin *pgxpool.Pool, physical PhysicalSource, allow bool) *pgxpool.Pool {
	t.Helper()
	if allow {
		_, err := admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
		require.NoError(t, err)
	}
	cfg := admin.Config().Copy()
	cfg.ConnConfig.Database = physical.Database
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	return pool
}
