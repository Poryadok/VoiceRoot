package roomlifecycle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const r223Table = "voice_lifecycle_redis_divergences"

func r223MigrationPath(t *testing.T, direction string) string {
	t.Helper()
	return filepath.Join(r22VoiceRepoRoot(t), "src", "backend", "migrations", "voice_db", "000002_redis_divergence."+direction+".sql")
}

func r223ReadMigration(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile(r223MigrationPath(t, direction))
	require.NoError(t, err)
	return string(raw)
}

func r223Start(t *testing.T, database string) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, database)
	_, err := pool.Exec(ctx, r223ReadMigration(t, "up"))
	require.NoError(t, err)
	return ctx, pool
}

func TestVoiceDBRedisDivergenceMigration_D1ExactSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence migration contract requires testcontainers")
	}
	ctx, pool := r223Start(t, "r223schema")
	type column struct{ name, dataType, nullable string }
	rows, err := pool.Query(ctx, `SELECT column_name,data_type,is_nullable FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1 ORDER BY ordinal_position`, r223Table)
	require.NoError(t, err)
	defer rows.Close()
	var got []column
	for rows.Next() {
		var c column
		require.NoError(t, rows.Scan(&c.name, &c.dataType, &c.nullable))
		got = append(got, c)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []column{
		{"divergence_id", "uuid", "NO"}, {"actor_profile_id", "uuid", "NO"}, {"operation_id", "uuid", "NO"},
		{"classification", "text", "NO"}, {"redis_class", "text", "NO"}, {"redis_schema_version", "smallint", "YES"},
		{"redis_state", "text", "YES"}, {"observation_digest", "bytea", "NO"}, {"subject_profile_id", "uuid", "YES"},
		{"classified_pg_state", "text", "YES"}, {"classified_lease_fence", "bigint", "YES"},
		{"first_observed_at", "timestamp with time zone", "NO"}, {"last_observed_at", "timestamp with time zone", "NO"},
		{"resolution_id", "uuid", "YES"}, {"resolution_kind", "text", "YES"}, {"resolution_evidence_digest", "bytea", "YES"},
		{"resolved_by_account_id", "uuid", "YES"}, {"resolved_at", "timestamp with time zone", "YES"},
		{"created_at", "timestamp with time zone", "NO"}, {"updated_at", "timestamp with time zone", "NO"},
	}, got)
	var foreignKeys int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conrelid=$1::regclass AND contype='f'`, r223Table).Scan(&foreignKeys))
	require.Zero(t, foreignKeys)
	var constraintNames []string
	constraintRows, err := pool.Query(ctx, `SELECT conname FROM pg_constraint WHERE conrelid=$1::regclass ORDER BY conname`, r223Table)
	require.NoError(t, err)
	defer constraintRows.Close()
	for constraintRows.Next() {
		var name string
		require.NoError(t, constraintRows.Scan(&name))
		constraintNames = append(constraintNames, name)
	}
	require.Equal(t, []string{
		"voice_lifecycle_redis_divergences_actor_profile_id_not_nil",
		"voice_lifecycle_redis_divergences_classification_check",
		"voice_lifecycle_redis_divergences_classification_shape_check",
		"voice_lifecycle_redis_divergences_classified_fence_check",
		"voice_lifecycle_redis_divergences_divergence_id_not_nil",
		"voice_lifecycle_redis_divergences_observation_digest_check",
		"voice_lifecycle_redis_divergences_operation_id_not_nil",
		"voice_lifecycle_redis_divergences_pkey",
		"voice_lifecycle_redis_divergences_redis_class_check",
		"voice_lifecycle_redis_divergences_redis_schema_version_check",
		"voice_lifecycle_redis_divergences_redis_state_check",
		"voice_lifecycle_redis_divergences_resolution_digest_check",
		"voice_lifecycle_redis_divergences_resolution_id_not_nil",
		"voice_lifecycle_redis_divergences_resolution_kind_check",
		"voice_lifecycle_redis_divergences_resolution_shape_check",
		"voice_lifecycle_redis_divergences_resolved_by_not_nil",
		"voice_lifecycle_redis_divergences_subject_profile_id_not_nil",
		"voice_lifecycle_redis_divergences_time_order_check",
	}, constraintNames)
	var triggerName, triggerDefinition, functionName string
	require.NoError(t, pool.QueryRow(ctx, `SELECT t.tgname,pg_get_triggerdef(t.oid),p.proname FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid WHERE t.tgrelid=$1::regclass AND NOT t.tgisinternal`, r223Table).Scan(&triggerName, &triggerDefinition, &functionName))
	require.Equal(t, "voice_lifecycle_redis_divergence_update_guard", triggerName)
	require.Equal(t, "voice_lifecycle_redis_divergence_update_guard_fn", functionName)
	require.Contains(t, triggerDefinition, "BEFORE UPDATE")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_lifecycle_redis_divergences`).Scan(&count))
	require.Zero(t, count)
	var primaryColumns []string
	require.NoError(t, pool.QueryRow(ctx, `
SELECT array_agg(a.attname ORDER BY key.ordinality)
FROM pg_constraint c
CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS key(attnum, ordinality)
JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=key.attnum
WHERE c.conrelid=$1::regclass AND c.contype='p'
GROUP BY c.oid`, r223Table).Scan(&primaryColumns))
	require.Equal(t, []string{"divergence_id"}, primaryColumns)

	var indexes []string
	indexRows, err := pool.Query(ctx, `SELECT indexname||'|'||indexdef FROM pg_indexes WHERE schemaname=current_schema() AND tablename=$1 ORDER BY indexname`, r223Table)
	require.NoError(t, err)
	defer indexRows.Close()
	for indexRows.Next() {
		var d string
		require.NoError(t, indexRows.Scan(&d))
		indexes = append(indexes, strings.ToLower(strings.Join(strings.Fields(d), " ")))
	}
	require.Equal(t, []string{
		"voice_lifecycle_redis_divergences_one_open_operation|create unique index voice_lifecycle_redis_divergences_one_open_operation on public.voice_lifecycle_redis_divergences using btree (actor_profile_id, operation_id) where (resolved_at is null)",
		"voice_lifecycle_redis_divergences_open_queue|create index voice_lifecycle_redis_divergences_open_queue on public.voice_lifecycle_redis_divergences using btree (first_observed_at, divergence_id) where (resolved_at is null)",
		"voice_lifecycle_redis_divergences_open_subject|create index voice_lifecycle_redis_divergences_open_subject on public.voice_lifecycle_redis_divergences using btree (subject_profile_id, first_observed_at) where ((resolved_at is null) and (subject_profile_id is not null))",
		"voice_lifecycle_redis_divergences_pkey|create unique index voice_lifecycle_redis_divergences_pkey on public.voice_lifecycle_redis_divergences using btree (divergence_id)",
		"voice_lifecycle_redis_divergences_retained_history|create index voice_lifecycle_redis_divergences_retained_history on public.voice_lifecycle_redis_divergences using btree (resolved_at, divergence_id) where (resolved_at is not null)",
	}, indexes)
}

func TestVoiceDBRedisDivergenceMigration_D1ClosedSetsAndShapes(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence migration contract requires testcontainers")
	}
	ctx, pool := r223Start(t, "r223sets")
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	insert := func(classification, redisClass string, subject any, pgState any, fence any) error {
		_, err := pool.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,observation_digest,subject_profile_id,classified_pg_state,classified_lease_fence,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$10,$10)`, uuid.New(), uuid.New(), uuid.New(), classification, redisClass, make([]byte, 32), subject, pgState, fence, base)
		return err
	}
	for _, redisClass := range []string{"legacy_schema", "malformed", "binding_mismatch", "owner_mismatch", "state_order_mismatch", "receipt_mismatch", "deadline_mismatch", "ttl_invalid"} {
		require.NoError(t, insert("orphan", redisClass, nil, nil, nil), redisClass)
	}
	_, err := pool.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,redis_schema_version,redis_state,observation_digest,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,'orphan','legacy_schema',2,'pending',$4,$5,$5,$5,$5)`, uuid.New(), uuid.New(), uuid.New(), make([]byte, 32), base)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,redis_schema_version,redis_state,observation_digest,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,'orphan','legacy_schema',2,'completed',$4,$5,$5,$5,$5)`, uuid.New(), uuid.New(), uuid.New(), make([]byte, 32), base)
	require.NoError(t, err)
	for _, classification := range []string{"decided", "completed", "quarantined"} {
		require.NoError(t, insert(classification, "malformed", uuid.New(), classification, int64(0)), classification)
		for _, bad := range []struct{ subject, pgState, fence any }{
			{nil, classification, int64(0)}, {uuid.New(), nil, int64(0)}, {uuid.New(), classification, nil}, {uuid.New(), "orphan", int64(0)}, {uuid.New(), classification, int64(-1)},
		} {
			require.Error(t, insert(classification, "malformed", bad.subject, bad.pgState, bad.fence))
		}
	}
	for _, bad := range []struct{ subject, pgState, fence any }{{uuid.New(), nil, nil}, {nil, "orphan", nil}, {nil, nil, int64(0)}} {
		require.Error(t, insert("orphan", "malformed", bad.subject, bad.pgState, bad.fence))
	}
}

func TestVoiceDBRedisDivergenceMigration_D1ResolutionAndTimestampConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence migration contract requires testcontainers")
	}
	ctx, pool := r223Start(t, "r223resolution")
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	seed := func() uuid.UUID {
		incident := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,observation_digest,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,'orphan','malformed',$4,$5,$5,$5,$5)`, incident, uuid.New(), uuid.New(), make([]byte, 32), base)
		require.NoError(t, err)
		return incident
	}
	type resolution struct {
		id     any
		kind   any
		digest any
		staff  any
		at     any
	}
	valid := resolution{uuid.New(), "orphan_removed", make([]byte, 32), uuid.New(), base.Add(time.Minute)}
	for _, kind := range []string{"orphan_removed", "mirror_restored_exact", "mirror_reset_for_rebuild", "expired_operation_retired"} {
		incident := seed()
		tag, err := pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET resolution_id=$1,resolution_kind=$2,resolution_evidence_digest=$3,resolved_by_account_id=$4,resolved_at=$5,updated_at=$5 WHERE divergence_id=$6`, uuid.New(), kind, make([]byte, 32), uuid.New(), base.Add(time.Minute), incident)
		require.NoError(t, err, kind)
		require.Equal(t, int64(1), tag.RowsAffected(), kind)
		var storedKind string
		require.NoError(t, pool.QueryRow(ctx, `SELECT resolution_kind FROM voice_lifecycle_redis_divergences WHERE divergence_id=$1`, incident).Scan(&storedKind))
		require.Equal(t, kind, storedKind)
	}
	invalid := []resolution{
		{nil, valid.kind, valid.digest, valid.staff, valid.at}, {valid.id, nil, valid.digest, valid.staff, valid.at},
		{valid.id, valid.kind, nil, valid.staff, valid.at}, {valid.id, valid.kind, valid.digest, nil, valid.at},
		{valid.id, valid.kind, valid.digest, valid.staff, nil}, {valid.id, nil, nil, nil, nil},
		{uuid.Nil, valid.kind, valid.digest, valid.staff, valid.at},
		{valid.id, "unknown", valid.digest, valid.staff, valid.at}, {valid.id, " orphan_removed", valid.digest, valid.staff, valid.at},
		{valid.id, valid.kind, make([]byte, 31), valid.staff, valid.at}, {valid.id, valid.kind, valid.digest, uuid.Nil, valid.at},
		{valid.id, valid.kind, valid.digest, valid.staff, base.Add(-time.Second)},
	}
	for _, bad := range invalid {
		incident := seed()
		_, err := pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET resolution_id=$1,resolution_kind=$2,resolution_evidence_digest=$3,resolved_by_account_id=$4,resolved_at=$5,updated_at=$6 WHERE divergence_id=$7`, bad.id, bad.kind, bad.digest, bad.staff, bad.at, base.Add(time.Minute), incident)
		require.Error(t, err)
	}
	incident := seed()
	tag, err := pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET resolution_id=$1,resolution_kind=$2,resolution_evidence_digest=$3,resolved_by_account_id=$4,resolved_at=$5,updated_at=$5 WHERE divergence_id=$6`, valid.id, valid.kind, valid.digest, valid.staff, valid.at, incident)
	require.NoError(t, err)
	require.Equal(t, int64(1), tag.RowsAffected())
	var gotID uuid.UUID
	var gotKind string
	var gotDigest []byte
	var gotStaff uuid.UUID
	var gotAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT resolution_id,resolution_kind,resolution_evidence_digest,resolved_by_account_id,resolved_at FROM voice_lifecycle_redis_divergences WHERE divergence_id=$1`, incident).Scan(&gotID, &gotKind, &gotDigest, &gotStaff, &gotAt))
	require.Equal(t, valid.id, gotID)
	require.Equal(t, valid.kind, gotKind)
	require.Equal(t, valid.digest, gotDigest)
	require.Equal(t, valid.staff, gotStaff)
	require.True(t, valid.at.(time.Time).Equal(gotAt))
	incident = seed()
	for _, statement := range []string{
		`UPDATE voice_lifecycle_redis_divergences SET last_observed_at=first_observed_at-interval '1 second' WHERE divergence_id=$1`,
		`UPDATE voice_lifecycle_redis_divergences SET updated_at=created_at-interval '1 second' WHERE divergence_id=$1`,
	} {
		_, err := pool.Exec(ctx, statement, incident)
		require.Error(t, err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,observation_digest,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,'orphan','malformed',$4,$5,$6,$5,$5)`, uuid.New(), uuid.New(), uuid.New(), make([]byte, 32), base, base.Add(-time.Second))
	require.Error(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,observation_digest,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,'orphan','malformed',$4,$5,$5,$5,$6)`, uuid.New(), uuid.New(), uuid.New(), make([]byte, 32), base, base.Add(-time.Second))
	require.Error(t, err)
}

func TestVoiceDBRedisDivergenceMigration_D1EveryEvidenceFieldIsImmutable(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence migration contract requires testcontainers")
	}
	ctx, pool := r223Start(t, "r223immutable")
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	for _, mutation := range []string{
		"divergence_id=gen_random_uuid()", "actor_profile_id=gen_random_uuid()", "operation_id=gen_random_uuid()",
		"classification='completed'", "redis_class='legacy_schema'", "redis_schema_version=1", "redis_state='pending'",
		"observation_digest=decode(repeat('01',32),'hex')", "subject_profile_id=gen_random_uuid()",
		"classified_pg_state='decided'", "classified_lease_fence=1", "first_observed_at=first_observed_at+interval '1 second'", "created_at=created_at+interval '1 second'",
	} {
		incident := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,observation_digest,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,'orphan','malformed',$4,$5,$5,$5,$5)`, incident, uuid.New(), uuid.New(), make([]byte, 32), base)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, "UPDATE voice_lifecycle_redis_divergences SET "+mutation+" WHERE divergence_id=$1", incident)
		require.Error(t, err, mutation)
	}
}

func TestVoiceDBRedisDivergenceMigration_D1DDLProtocol(t *testing.T) {
	up := strings.ToLower(strings.Join(strings.Fields(r223ReadMigration(t, "up")), " "))
	down := strings.ToLower(strings.Join(strings.Fields(r223ReadMigration(t, "down")), " "))
	require.Contains(t, down, "lock table voice_lifecycle_redis_divergences in access exclusive mode")
	require.NotContains(t, up, "insert into")
	for _, foreign := range []string{"voice_room_instances", "voice_room_memberships", "voice_lifecycle_operations", "voice_lifecycle_effects", "voice_media_epoch_denials", "voice_event_outbox"} {
		require.NotContains(t, up, "alter table "+foreign)
		require.NotContains(t, up, "drop table "+foreign)
	}
}

func TestVoiceDBRedisDivergenceMigration_D1UpIsIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence migration contract requires testcontainers")
	}
	ctx, pool := r223Start(t, "r223upagain")
	before := r22VoiceSchemaSnapshot(t, ctx, pool)
	_, err := pool.Exec(ctx, r223ReadMigration(t, "up"))
	require.NoError(t, err)
	require.Equal(t, before, r22VoiceSchemaSnapshot(t, ctx, pool))
}

func TestVoiceDBRedisDivergenceMigration_D1UpAddsOnlyApprovedObjects(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence migration contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "r223catalog")
	query := `SELECT kind||'|'||name FROM (
SELECT 'relation' kind,c.relname name FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND c.relkind IN ('r','i')
UNION ALL SELECT 'function',p.proname FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname=current_schema()
UNION ALL SELECT 'trigger',t.tgname FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND NOT t.tgisinternal
) x ORDER BY kind,name`
	read := func() []string {
		rows, err := pool.Query(ctx, query)
		require.NoError(t, err)
		defer rows.Close()
		var out []string
		for rows.Next() {
			var value string
			require.NoError(t, rows.Scan(&value))
			out = append(out, value)
		}
		return out
	}
	before := read()
	_, err := pool.Exec(ctx, r223ReadMigration(t, "up"))
	require.NoError(t, err)
	after := read()
	beforeSet := make(map[string]struct{}, len(before))
	for _, value := range before {
		beforeSet[value] = struct{}{}
	}
	var added []string
	for _, value := range after {
		if _, ok := beforeSet[value]; !ok {
			added = append(added, value)
		}
	}
	require.Equal(t, []string{
		"function|voice_lifecycle_redis_divergence_update_guard_fn",
		"relation|voice_lifecycle_redis_divergences", "relation|voice_lifecycle_redis_divergences_one_open_operation",
		"relation|voice_lifecycle_redis_divergences_open_queue", "relation|voice_lifecycle_redis_divergences_open_subject",
		"relation|voice_lifecycle_redis_divergences_pkey", "relation|voice_lifecycle_redis_divergences_retained_history",
		"trigger|voice_lifecycle_redis_divergence_update_guard",
	}, added)
}

func TestVoiceDBRedisDivergenceMigration_D1ClosedConstraintsAndGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence migration contract requires testcontainers")
	}
	ctx, pool := r223Start(t, "r223constraints")
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	validInsert := func(incident, actor, operation uuid.UUID, classification, redisClass string, schema any, redisState any, digest []byte, subject any, pgState any, fence any) error {
		_, err := pool.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,redis_schema_version,redis_state,observation_digest,subject_profile_id,classified_pg_state,classified_lease_fence,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12,$12,$12)`, incident, actor, operation, classification, redisClass, schema, redisState, digest, subject, pgState, fence, base)
		return err
	}
	for _, tc := range []struct {
		name string
		args []any
	}{
		{"nil incident", []any{uuid.Nil, uuid.New(), uuid.New(), "orphan", "malformed", nil, nil, make([]byte, 32), nil, nil, nil}},
		{"nil actor", []any{uuid.New(), uuid.Nil, uuid.New(), "orphan", "malformed", nil, nil, make([]byte, 32), nil, nil, nil}},
		{"nil operation", []any{uuid.New(), uuid.New(), uuid.Nil, "orphan", "malformed", nil, nil, make([]byte, 32), nil, nil, nil}},
		{"unknown classification", []any{uuid.New(), uuid.New(), uuid.New(), "other", "malformed", nil, nil, make([]byte, 32), nil, nil, nil}},
		{"unknown redis class", []any{uuid.New(), uuid.New(), uuid.New(), "orphan", "timeout", nil, nil, make([]byte, 32), nil, nil, nil}},
		{"nonpositive schema", []any{uuid.New(), uuid.New(), uuid.New(), "orphan", "legacy_schema", 0, nil, make([]byte, 32), nil, nil, nil}},
		{"unknown redis state", []any{uuid.New(), uuid.New(), uuid.New(), "orphan", "malformed", nil, "broken", make([]byte, 32), nil, nil, nil}},
		{"short digest", []any{uuid.New(), uuid.New(), uuid.New(), "orphan", "malformed", nil, nil, make([]byte, 31), nil, nil, nil}},
		{"long digest", []any{uuid.New(), uuid.New(), uuid.New(), "orphan", "malformed", nil, nil, make([]byte, 33), nil, nil, nil}},
		{"orphan with subject", []any{uuid.New(), uuid.New(), uuid.New(), "orphan", "malformed", nil, nil, make([]byte, 32), uuid.New(), nil, nil}},
		{"decided without snapshot", []any{uuid.New(), uuid.New(), uuid.New(), "decided", "malformed", nil, nil, make([]byte, 32), nil, nil, nil}},
		{"mismatched state", []any{uuid.New(), uuid.New(), uuid.New(), "decided", "malformed", nil, nil, make([]byte, 32), uuid.New(), "completed", int64(0)}},
		{"negative fence", []any{uuid.New(), uuid.New(), uuid.New(), "decided", "malformed", nil, nil, make([]byte, 32), uuid.New(), "decided", int64(-1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := tc.args
			require.Error(t, validInsert(a[0].(uuid.UUID), a[1].(uuid.UUID), a[2].(uuid.UUID), a[3].(string), a[4].(string), a[5], a[6], a[7].([]byte), a[8], a[9], a[10]))
		})
	}

	incident, actor, operation := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, validInsert(incident, actor, operation, "orphan", "malformed", nil, nil, make([]byte, 32), nil, nil, nil))
	require.Error(t, validInsert(uuid.New(), actor, operation, "orphan", "malformed", nil, nil, make([]byte, 32), nil, nil, nil))
	later := base.Add(time.Minute)
	tag, err := pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET last_observed_at=$1,updated_at=$1 WHERE divergence_id=$2`, later, incident)
	require.NoError(t, err)
	require.Equal(t, int64(1), tag.RowsAffected())
	var observedAt, updatedAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT last_observed_at,updated_at FROM voice_lifecycle_redis_divergences WHERE divergence_id=$1`, incident).Scan(&observedAt, &updatedAt))
	require.True(t, later.Equal(observedAt))
	require.True(t, later.Equal(updatedAt))
	_, err = pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET observation_digest=$1 WHERE divergence_id=$2`, append(make([]byte, 31), 1), incident)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET last_observed_at=$1,resolution_id=$2,resolution_kind='orphan_removed',resolution_evidence_digest=$3,resolved_by_account_id=$4,resolved_at=$1,updated_at=$1 WHERE divergence_id=$5`, later.Add(time.Minute), uuid.New(), make([]byte, 32), uuid.New(), incident)
	require.Error(t, err, "resolution may change only the complete resolution tuple and updated_at")
	_, err = pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET resolution_id=$1 WHERE divergence_id=$2`, uuid.New(), incident)
	require.Error(t, err, "partial resolution tuple is forbidden")
	_, err = pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET last_observed_at=$1 WHERE divergence_id=$2`, base.Add(-time.Second), incident)
	require.Error(t, err, "last observation cannot precede first observation")
	_, err = pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET resolution_id=$1,resolution_kind='orphan_removed',resolution_evidence_digest=$2,resolved_by_account_id=$3,resolved_at=$4,updated_at=$4 WHERE divergence_id=$5`, uuid.New(), make([]byte, 32), uuid.New(), later, incident)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET updated_at=$1 WHERE divergence_id=$2`, later.Add(time.Minute), incident)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET resolution_id=NULL,resolution_kind=NULL,resolution_evidence_digest=NULL,resolved_by_account_id=NULL,resolved_at=NULL WHERE divergence_id=$1`, incident)
	require.Error(t, err)
	require.NoError(t, validInsert(uuid.New(), actor, operation, "orphan", "malformed", nil, nil, make([]byte, 32), nil, nil, nil))
}

func TestVoiceDBRedisDivergenceMigration_D1StrictDownAndReapply(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence migration contract requires testcontainers")
	}
	for _, resolved := range []bool{false, true} {
		t.Run(map[bool]string{false: "unresolved", true: "resolved"}[resolved], func(t *testing.T) {
			ctx := context.Background()
			pool := r22StartVoicePostgres(t, ctx, "r223down"+map[bool]string{false: "open", true: "closed"}[resolved])
			baselineSchema := r22VoiceSchemaSnapshot(t, ctx, pool)
			_, err := pool.Exec(ctx, r223ReadMigration(t, "up"))
			require.NoError(t, err)
			base := time.Now().UTC()
			incident := uuid.New()
			_, err = pool.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,observation_digest,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,'orphan','malformed',$4,$5,$5,$5,$5)`, incident, uuid.New(), uuid.New(), make([]byte, 32), base)
			require.NoError(t, err)
			if resolved {
				_, err = pool.Exec(ctx, `UPDATE voice_lifecycle_redis_divergences SET resolution_id=$1,resolution_kind='orphan_removed',resolution_evidence_digest=$2,resolved_by_account_id=$3,resolved_at=$4,updated_at=$4 WHERE divergence_id=$5`, uuid.New(), make([]byte, 32), uuid.New(), base, incident)
				require.NoError(t, err)
			}
			beforeSchema := r22VoiceSchemaSnapshot(t, ctx, pool)
			beforeIncident := r22TableRowsSnapshot(t, ctx, pool, r223Table)
			conn, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			_, err = conn.Exec(ctx, r223ReadMigration(t, "down"))
			r22RequireSQLState(t, err, "55000")
			_, err = conn.Exec(ctx, "ROLLBACK")
			require.NoError(t, err)
			var count int
			require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM voice_lifecycle_redis_divergences`).Scan(&count))
			require.Equal(t, 1, count)
			require.Equal(t, beforeSchema, r22VoiceSchemaSnapshot(t, ctx, pool))
			require.NotEqual(t, baselineSchema, beforeSchema, "R22.3 guard is visible while UP is applied")
			require.Equal(t, beforeIncident, r22TableRowsSnapshot(t, ctx, pool, r223Table))
		})
	}
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, "r223emptydown")
	baselineSchema := r22VoiceSchemaSnapshot(t, ctx, pool)
	seedTx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = r22TrySeedMigrationRows(ctx, seedTx, nil)
	require.NoError(t, err)
	require.NoError(t, seedTx.Commit(ctx))
	beforeRows := map[string][]string{}
	for _, table := range r22VoiceTables {
		beforeRows[table] = r22TableRowsSnapshot(t, ctx, pool, table)
	}
	_, err = pool.Exec(ctx, r223ReadMigration(t, "up"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, r223ReadMigration(t, "down"))
	require.NoError(t, err)
	var operationCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_lifecycle_operations`).Scan(&operationCount))
	require.Equal(t, 1, operationCount)
	require.Equal(t, baselineSchema, r22VoiceSchemaSnapshot(t, ctx, pool))
	for _, table := range r22VoiceTables {
		require.Equal(t, beforeRows[table], r22TableRowsSnapshot(t, ctx, pool, table), table)
	}
	var missing any
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('voice_lifecycle_redis_divergences')`).Scan(&missing))
	require.Nil(t, missing)
	_, err = pool.Exec(ctx, r223ReadMigration(t, "up"))
	require.NoError(t, err)
}

func TestVoiceDBRedisDivergenceMigration_D1DownRechecksAfterWaiting(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL divergence migration contract requires testcontainers")
	}
	ctx, pool := r223Start(t, "r223downrace")
	writer, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer writer.Release()
	tx, err := writer.Begin(ctx)
	require.NoError(t, err)
	base := time.Now().UTC()
	incident := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO voice_lifecycle_redis_divergences(divergence_id,actor_profile_id,operation_id,classification,redis_class,observation_digest,first_observed_at,last_observed_at,created_at,updated_at) VALUES($1,$2,$3,'orphan','malformed',$4,$5,$5,$5,$5)`, incident, uuid.New(), uuid.New(), make([]byte, 32), base)
	require.NoError(t, err)
	var exactIncident string
	require.NoError(t, tx.QueryRow(ctx, `SELECT to_jsonb(row_data)::text FROM voice_lifecycle_redis_divergences row_data WHERE divergence_id=$1`, incident).Scan(&exactIncident))
	beforeSchema := r22VoiceSchemaSnapshot(t, ctx, pool)
	down, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer down.Release()
	var pid int
	require.NoError(t, down.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid))
	result := make(chan error, 1)
	go func() { _, e := down.Exec(ctx, r223ReadMigration(t, "down")); result <- e }()
	r22WaitForBackendLock(t, ctx, pool, pid)
	require.NoError(t, tx.Commit(ctx))
	r22RequireSQLState(t, <-result, "55000")
	_, err = down.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	var count int
	require.NoError(t, down.QueryRow(ctx, `SELECT count(*) FROM voice_lifecycle_redis_divergences`).Scan(&count))
	require.Equal(t, 1, count)
	require.Equal(t, beforeSchema, r22VoiceSchemaSnapshot(t, ctx, pool))
	rows := r22TableRowsSnapshot(t, ctx, pool, r223Table)
	require.Equal(t, []string{exactIncident}, rows)
}
