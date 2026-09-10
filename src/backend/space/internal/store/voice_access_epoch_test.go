package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const (
	r22SpaceEpochMigration = "000011_voice_access_epoch.up.sql"
	r22SpaceEventKind      = "voice_room_access_invalidated"
)

type r22SpaceOutbox struct {
	EventID   uuid.UUID
	EventKind string
	RoomID    *uuid.UUID
	ProfileID *uuid.UUID
	Payload   string
}

func r22SpaceMigrationSQL(t *testing.T, name string) string {
	t.Helper()
	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name)
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err, "R22.1 Space access-epoch migration must exist")
	return string(sqlBytes)
}

func applyR22SpaceEpochMigration(t *testing.T, ctx context.Context, st *SpaceStore) {
	t.Helper()
	_, err := st.Pool.Exec(ctx, r22SpaceMigrationSQL(t, r22SpaceEpochMigration))
	require.NoError(t, err)
}

func runR22SpaceEpochDown(t *testing.T, ctx context.Context, st *SpaceStore) error {
	t.Helper()
	_, err := st.Pool.Exec(ctx, r22SpaceMigrationSQL(t, "000011_voice_access_epoch.down.sql"))
	return err
}

func r22SpaceEpoch(t *testing.T, ctx context.Context, st *SpaceStore, spaceID uuid.UUID) int64 {
	t.Helper()
	var epoch int64
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT access_epoch FROM space_voice_access_epochs WHERE space_id=$1`, spaceID).Scan(&epoch))
	require.Positive(t, epoch)
	return epoch
}

func r22SpaceOutboxRow(t *testing.T, ctx context.Context, st *SpaceStore, spaceID uuid.UUID, epoch int64) r22SpaceOutbox {
	t.Helper()
	var out r22SpaceOutbox
	require.NoError(t, st.Pool.QueryRow(ctx, `
SELECT event_id,event_kind,voice_room_id,profile_id,payload::text
FROM space_voice_access_outbox
WHERE space_id=$1 AND access_epoch=$2
`, spaceID, epoch).Scan(&out.EventID, &out.EventKind, &out.RoomID, &out.ProfileID, &out.Payload))
	require.NotEqual(t, uuid.Nil, out.EventID, "durable invalidation needs one stable event_id")
	require.Equal(t, r22SpaceEventKind, out.EventKind)
	return out
}

func requireR22SpaceSnapshot(t *testing.T, out r22SpaceOutbox, spaceID uuid.UUID, epoch int64) {
	t.Helper()
	want := map[string]any{"space_id": spaceID.String(), "access_epoch": epoch}
	if out.RoomID != nil {
		want["voice_room_id"] = out.RoomID.String()
	}
	if out.ProfileID != nil {
		want["profile_id"] = out.ProfileID.String()
	}
	encoded, err := json.Marshal(want)
	require.NoError(t, err)
	require.JSONEq(t, string(encoded), out.Payload, "payload must be the exact immutable scope+epoch snapshot")
}

func r22ReflectedField(t *testing.T, decision reflect.Value, field string) reflect.Value {
	t.Helper()
	f := decision.FieldByName(field)
	require.True(t, f.IsValid(), "Voice access decision must expose %s", field)
	return f
}

func r22InvokeSpaceAccess(st *SpaceStore, ctx context.Context, expectedSpaceID, roomID, profileID uuid.UUID) (reflect.Value, error) {
	method := reflect.ValueOf(st).MethodByName("ResolveVoiceRoomAccess")
	if !method.IsValid() || method.Type().NumIn() != 4 {
		return reflect.Value{}, errors.New("ResolveVoiceRoomAccess must accept context plus expected Space, room and profile UUIDs")
	}
	result := method.Call([]reflect.Value{
		reflect.ValueOf(ctx), reflect.ValueOf(expectedSpaceID), reflect.ValueOf(roomID), reflect.ValueOf(profileID),
	})
	if len(result) != 2 {
		return reflect.Value{}, errors.New("ResolveVoiceRoomAccess must return decision and error")
	}
	if !result[1].IsNil() {
		err, ok := result[1].Interface().(error)
		if !ok {
			return reflect.Value{}, errors.New("ResolveVoiceRoomAccess second result is not error")
		}
		return reflect.Value{}, err
	}
	decision := result[0]
	if !decision.IsValid() || (decision.Kind() == reflect.Pointer && decision.IsNil()) {
		return reflect.Value{}, errors.New("ResolveVoiceRoomAccess returned nil decision")
	}
	if decision.Kind() == reflect.Pointer {
		decision = decision.Elem()
	}
	if decision.Kind() != reflect.Struct {
		return reflect.Value{}, errors.New("ResolveVoiceRoomAccess decision must be a struct")
	}
	return decision, nil
}

func r22SecondSpacePool(t *testing.T, ctx context.Context, pool *pgxpool.Pool, applicationName string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(pool.Config().ConnString())
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["application_name"] = applicationName
	second, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	require.NoError(t, second.Ping(ctx))
	t.Cleanup(second.Close)
	return second
}

func r22WaitForSpaceLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, applicationName string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		require.NoError(t, pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM pg_stat_activity
  WHERE application_name=$1 AND state='active' AND wait_event_type='Lock'
)`, applicationName).Scan(&waiting))
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("decision %q never blocked on the authority transaction lock", applicationName)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSpaceVoiceAccessEpochMigration_BackfillsAndConstrainsPositiveEpoch(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	existing, err := st.CreateSpace(ctx, uuid.New(), "pre-epoch", "", "private")
	require.NoError(t, err)

	applyR22SpaceEpochMigration(t, ctx, st)
	require.EqualValues(t, 1, r22SpaceEpoch(t, ctx, st, existing.ID))
	_, err = pool.Exec(ctx, `UPDATE space_voice_access_epochs SET access_epoch=0 WHERE space_id=$1`, existing.ID)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO space_voice_access_outbox(event_id,space_id,access_epoch,event_kind,payload)
VALUES($1,$2,0,$3,'{}')
`, uuid.New(), existing.ID, r22SpaceEventKind)
	require.Error(t, err)

	createdAfter, err := st.CreateSpace(ctx, uuid.New(), "post-epoch", "", "private")
	require.NoError(t, err)
	require.Positive(t, r22SpaceEpoch(t, ctx, st, createdAfter.ID))
}

func TestSpaceVoiceAccessEpochMigration_EmptyDownCanReapply(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	applyR22SpaceEpochMigration(t, ctx, st)
	require.NoError(t, runR22SpaceEpochDown(t, ctx, st))
	var epochTable, outboxTable *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('space_voice_access_epochs')::text,to_regclass('space_voice_access_outbox')::text`).Scan(&epochTable, &outboxTable))
	require.Nil(t, epochTable)
	require.Nil(t, outboxTable)
	var baseTable string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('spaces')::text`).Scan(&baseTable))
	require.Equal(t, "spaces", baseTable)

	applyR22SpaceEpochMigration(t, ctx, st)
	created, err := st.CreateSpace(ctx, uuid.New(), "after-reapply", "", "private")
	require.NoError(t, err)
	require.Positive(t, r22SpaceEpoch(t, ctx, st, created.ID))
}

func TestSpaceVoiceAccessEpochMigration_DownWaitsForConcurrentEvidenceThenRefuses(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	applyR22SpaceEpochMigration(t, ctx, st)

	spaceID, ownerID := uuid.New(), uuid.New()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	_, err = tx.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'concurrent DOWN evidence',$2)`, spaceID, ownerID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, spaceID, ownerID)
	require.NoError(t, err)

	const app = "r22-space-down-barrier"
	second := r22SecondSpacePool(t, ctx, pool, app)
	downSQL := r22SpaceMigrationSQL(t, "000011_voice_access_epoch.down.sql")
	downResult := make(chan error, 1)
	go func() {
		_, downErr := second.Exec(ctx, downSQL)
		downResult <- downErr
	}()

	r22WaitForSpaceLock(t, ctx, pool, app)
	require.NoError(t, tx.Commit(ctx))
	require.Error(t, <-downResult, "DOWN must recheck evidence after waiting for its authority-table locks")

	var spaceExists, memberExists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM spaces WHERE id=$1)`, spaceID).Scan(&spaceExists))
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, spaceID, ownerID).Scan(&memberExists))
	require.True(t, spaceExists)
	require.True(t, memberExists)
	epoch := r22SpaceEpoch(t, ctx, st, spaceID)
	var outboxRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_voice_access_outbox WHERE space_id=$1 AND access_epoch=$2`, spaceID, epoch).Scan(&outboxRows))
	require.Equal(t, 1, outboxRows, "refused DOWN must preserve the committed invalidation snapshot")
}

func TestSpaceVoiceAccessEpochMigration_DownRefusesEachEvidenceClass(t *testing.T) {
	tests := []struct {
		name string
		seed func(*testing.T, context.Context, *SpaceStore) uuid.UUID
	}{
		{"owner epoch", func(t *testing.T, ctx context.Context, st *SpaceStore) uuid.UUID {
			spaceID := uuid.New()
			_, err := st.Pool.Exec(ctx, `INSERT INTO space_voice_access_epochs(space_id,access_epoch) VALUES($1,1)`, spaceID)
			require.NoError(t, err)
			return spaceID
		}},
		{"outbox snapshot", func(t *testing.T, ctx context.Context, st *SpaceStore) uuid.UUID {
			spaceID := uuid.New()
			payload, err := json.Marshal(map[string]any{"space_id": spaceID.String(), "access_epoch": 1})
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `INSERT INTO space_voice_access_epochs(space_id,access_epoch) VALUES($1,1)`, spaceID)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `
INSERT INTO space_voice_access_outbox(event_id,space_id,access_epoch,event_kind,payload)
VALUES($1,$2,1,$3,$4)
`, uuid.New(), spaceID, r22SpaceEventKind, payload)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `SET session_replication_role=replica`)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `DELETE FROM space_voice_access_epochs WHERE space_id=$1`, spaceID)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `SET session_replication_role=origin`)
			require.NoError(t, err)
			return spaceID
		}},
		{"owning Space scope", func(t *testing.T, ctx context.Context, st *SpaceStore) uuid.UUID {
			space, err := st.CreateSpace(ctx, uuid.New(), "owning-scope", "", "private")
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `SET session_replication_role=replica`)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `DELETE FROM space_voice_access_epochs WHERE space_id=$1`, space.ID)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `SET session_replication_role=origin`)
			require.NoError(t, err)
			return space.ID
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := startSpacePostgresForStoreTest(t, ctx)
			applySpaceMigrationForStoreTest(t, ctx, pool)
			st := &SpaceStore{Pool: pool}
			applyR22SpaceEpochMigration(t, ctx, st)
			spaceID := tc.seed(t, ctx, st)

			require.Error(t, runR22SpaceEpochDown(t, ctx, st))
			var epochTable, outboxTable string
			require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('space_voice_access_epochs')::text,to_regclass('space_voice_access_outbox')::text`).Scan(&epochTable, &outboxTable))
			require.Equal(t, "space_voice_access_epochs", epochTable)
			require.Equal(t, "space_voice_access_outbox", outboxTable)
			var evidence int
			require.NoError(t, pool.QueryRow(ctx, `
SELECT
 (SELECT count(*) FROM space_voice_access_epochs WHERE space_id=$1) +
 (SELECT count(*) FROM space_voice_access_outbox WHERE space_id=$1) +
 (SELECT count(*) FROM spaces WHERE id=$1)
`, spaceID).Scan(&evidence))
			require.Positive(t, evidence, "refused DOWN must preserve the exact blocking evidence")
		})
	}
}

func TestSpaceVoiceAccessEpochMigration_NonEmptyDownRefusesAndPreservesEvidence(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "preserve", "", "private")
	require.NoError(t, err)
	applyR22SpaceEpochMigration(t, ctx, st)
	memberID := uuid.New()
	_, err = st.JoinSpace(ctx, space.ID, memberID, uuid.New())
	require.NoError(t, err)
	before := r22SpaceEpoch(t, ctx, st, space.ID)
	out := r22SpaceOutboxRow(t, ctx, st, space.ID, before)

	require.Error(t, runR22SpaceEpochDown(t, ctx, st), "DOWN must refuse any owning-scope/epoch/outbox evidence")
	require.Equal(t, before, r22SpaceEpoch(t, ctx, st, space.ID))
	preserved := r22SpaceOutboxRow(t, ctx, st, space.ID, before)
	require.Equal(t, out, preserved)
	var stillMember bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space.ID, memberID).Scan(&stillMember))
	require.True(t, stillMember)

	require.NoError(t, st.RemoveMember(ctx, space.ID, memberID), "refused DOWN must leave the authority contract operational")
	require.Greater(t, r22SpaceEpoch(t, ctx, st, space.ID), before)
}

func TestSpaceVoiceAccessEpoch_DecisionSanitizesUndiscoverableTuples(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	applyR22SpaceEpochMigration(t, ctx, st)

	ownerA, ownerB := uuid.New(), uuid.New()
	spaceA, err := st.CreateSpace(ctx, ownerA, "a", "", "private")
	require.NoError(t, err)
	spaceB, err := st.CreateSpace(ctx, ownerB, "b", "", "private")
	require.NoError(t, err)
	roomA, _, err := st.CreateVoiceRoom(ctx, spaceA.ID, "a", nil)
	require.NoError(t, err)
	roomB, _, err := st.CreateVoiceRoom(ctx, spaceB.ID, "b", nil)
	require.NoError(t, err)

	member, err := r22InvokeSpaceAccess(st, ctx, spaceA.ID, roomA.ID, ownerA)
	require.NoError(t, err)
	require.Equal(t, spaceA.ID, r22ReflectedField(t, member, "SpaceID").Interface().(uuid.UUID), "member decision returns the canonical Space")
	require.True(t, r22ReflectedField(t, member, "Member").Bool())
	require.True(t, r22ReflectedField(t, member, "Active").Bool())
	require.True(t, r22ReflectedField(t, member, "Discoverable").Bool())
	epoch := r22ReflectedField(t, member, "AccessEpoch")
	require.Equal(t, reflect.Uint64, epoch.Kind())
	require.Equal(t, uint64(r22SpaceEpoch(t, ctx, st, spaceA.ID)), epoch.Uint())

	deletedRoomID := roomA.ID
	require.NoError(t, st.DeleteVoiceRoom(ctx, deletedRoomID))
	cases := []struct {
		name, detail string
		expected     uuid.UUID
		room         uuid.UUID
		profile      uuid.UUID
	}{
		{"undiscoverable existing", "non-member cannot distinguish existence", spaceB.ID, roomB.ID, uuid.New()},
		{"absent", "unknown row", spaceB.ID, uuid.New(), ownerB},
		{"inactive", "current canonical model defines inactive as no extant row", spaceA.ID, deletedRoomID, ownerA},
		{"deleted", "hard-deleted room is indistinguishable from absent", spaceA.ID, deletedRoomID, ownerA},
		{"foreign", "room canonical Space differs from asserted path Space", spaceA.ID, roomB.ID, ownerB},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, gotErr := r22InvokeSpaceAccess(st, ctx, tc.expected, tc.room, tc.profile)
			require.False(t, decision.IsValid(), tc.detail)
			require.ErrorIs(t, gotErr, ErrVoiceRoomNotFound, tc.detail)
			require.Equal(t, ErrVoiceRoomNotFound.Error(), gotErr.Error(), "all undiscoverable tuples expose exactly one sanitized result")
		})
	}
}

func TestSpaceVoiceAccessEpoch_MembershipAndRoomChangesEnqueueExactInvalidation(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "mutations", "", "public")
	require.NoError(t, err)
	applyR22SpaceEpochMigration(t, ctx, st)

	beforeRoom := r22SpaceEpoch(t, ctx, st, space.ID)
	room, _, err := st.CreateVoiceRoom(ctx, space.ID, "room", nil)
	require.NoError(t, err)
	afterRoom := r22SpaceEpoch(t, ctx, st, space.ID)
	require.Equal(t, beforeRoom+1, afterRoom)
	roomEvent := r22SpaceOutboxRow(t, ctx, st, space.ID, afterRoom)
	require.Equal(t, room.ID, *roomEvent.RoomID)
	require.Nil(t, roomEvent.ProfileID)
	requireR22SpaceSnapshot(t, roomEvent, space.ID, afterRoom)

	profileID := uuid.New()
	_, err = st.JoinSpace(ctx, space.ID, profileID, uuid.New())
	require.NoError(t, err)
	afterJoin := r22SpaceEpoch(t, ctx, st, space.ID)
	require.Equal(t, afterRoom+1, afterJoin)
	joinEvent := r22SpaceOutboxRow(t, ctx, st, space.ID, afterJoin)
	require.Nil(t, joinEvent.RoomID)
	require.Equal(t, profileID, *joinEvent.ProfileID)
	requireR22SpaceSnapshot(t, joinEvent, space.ID, afterJoin)

	require.NoError(t, st.RemoveMember(ctx, space.ID, profileID))
	afterRemoval := r22SpaceEpoch(t, ctx, st, space.ID)
	require.Equal(t, afterJoin+1, afterRemoval)
	removeEvent := r22SpaceOutboxRow(t, ctx, st, space.ID, afterRemoval)
	require.Nil(t, removeEvent.RoomID)
	require.Equal(t, profileID, *removeEvent.ProfileID)
	requireR22SpaceSnapshot(t, removeEvent, space.ID, afterRemoval)

	require.NoError(t, st.DeleteVoiceRoom(ctx, room.ID))
	afterDelete := r22SpaceEpoch(t, ctx, st, space.ID)
	require.Equal(t, afterRemoval+1, afterDelete)
	deleteEvent := r22SpaceOutboxRow(t, ctx, st, space.ID, afterDelete)
	require.Equal(t, room.ID, *deleteEvent.RoomID)
	require.Nil(t, deleteEvent.ProfileID)
	requireR22SpaceSnapshot(t, deleteEvent, space.ID, afterDelete)
}

func TestSpaceVoiceAccessEpoch_OutboxIsUniqueImmutableSnapshot(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "outbox", "", "private")
	require.NoError(t, err)
	applyR22SpaceEpochMigration(t, ctx, st)
	profileID := uuid.New()
	_, err = st.JoinSpace(ctx, space.ID, profileID, uuid.New())
	require.NoError(t, err)
	epoch := r22SpaceEpoch(t, ctx, st, space.ID)
	out := r22SpaceOutboxRow(t, ctx, st, space.ID, epoch)
	requireR22SpaceSnapshot(t, out, space.ID, epoch)

	_, err = pool.Exec(ctx, `UPDATE space_voice_access_outbox SET payload='{}' WHERE event_id=$1`, out.EventID)
	require.Error(t, err, "published snapshot rows are immutable")
	_, err = pool.Exec(ctx, `DELETE FROM space_voice_access_outbox WHERE event_id=$1`, out.EventID)
	require.Error(t, err, "stable event identity cannot be deleted")
	_, err = pool.Exec(ctx, `
INSERT INTO space_voice_access_outbox(event_id,space_id,access_epoch,event_kind,profile_id,payload)
VALUES($1,$2,$3,$4,$5,$6)
`, uuid.New(), space.ID, epoch, r22SpaceEventKind, profileID, out.Payload)
	require.Error(t, err, "(space_id, access_epoch) identifies exactly one invalidation snapshot")
	require.Equal(t, out, r22SpaceOutboxRow(t, ctx, st, space.ID, epoch))

	spaceColumn, payloadSpace := uuid.New(), uuid.New()
	epochSpace := uuid.New()
	roomSpace, roomColumn := uuid.New(), uuid.New()
	profileSpace, profileColumn, payloadProfile := uuid.New(), uuid.New(), uuid.New()
	corruptions := []struct {
		name      string
		spaceID   uuid.UUID
		epoch     int64
		roomID    *uuid.UUID
		profileID *uuid.UUID
		payload   map[string]any
	}{
		{"space id", spaceColumn, epoch + 10, nil, out.ProfileID, map[string]any{"space_id": payloadSpace.String(), "profile_id": out.ProfileID.String(), "access_epoch": epoch + 10}},
		{"epoch", epochSpace, epoch + 11, nil, out.ProfileID, map[string]any{"space_id": epochSpace.String(), "profile_id": out.ProfileID.String(), "access_epoch": epoch + 12}},
		{"room id", roomSpace, epoch + 13, &roomColumn, out.ProfileID, map[string]any{"space_id": roomSpace.String(), "profile_id": out.ProfileID.String(), "access_epoch": epoch + 13}},
		{"profile id", profileSpace, epoch + 14, nil, &profileColumn, map[string]any{"space_id": profileSpace.String(), "profile_id": payloadProfile.String(), "access_epoch": epoch + 14}},
	}
	for _, tc := range corruptions {
		t.Run("reject mismatched "+tc.name, func(t *testing.T) {
			eventID := uuid.New()
			payload, marshalErr := json.Marshal(tc.payload)
			require.NoError(t, marshalErr)
			_, insertErr := pool.Exec(ctx, `
INSERT INTO space_voice_access_outbox(event_id,space_id,access_epoch,event_kind,voice_room_id,profile_id,payload)
VALUES($1,$2,$3,$4,$5,$6,$7)
`, eventID, tc.spaceID, tc.epoch, r22SpaceEventKind, tc.roomID, tc.profileID, payload)
			require.Error(t, insertErr, "columns and immutable payload must describe the same exact invalidation snapshot")
			var corruptRows int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_voice_access_outbox WHERE event_id=$1`, eventID).Scan(&corruptRows))
			require.Zero(t, corruptRows)
			require.Equal(t, out, r22SpaceOutboxRow(t, ctx, st, space.ID, epoch))
		})
	}
}

func TestSpaceVoiceAccessEpoch_OutboxFailureRollsBackWithoutOrphan(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "rollback", "", "public")
	require.NoError(t, err)
	applyR22SpaceEpochMigration(t, ctx, st)
	memberID := uuid.New()
	_, err = st.JoinSpace(ctx, space.ID, memberID, uuid.New())
	require.NoError(t, err)
	before := r22SpaceEpoch(t, ctx, st, space.ID)
	var outboxBefore int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_voice_access_outbox WHERE space_id=$1`, space.ID).Scan(&outboxBefore))

	_, err = pool.Exec(ctx, `
CREATE FUNCTION r22_reject_space_voice_invalidation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'reject voice invalidation' USING ERRCODE='23514'; END $$;
CREATE TRIGGER r22_reject_space_voice_invalidation
BEFORE INSERT ON space_voice_access_outbox
FOR EACH ROW EXECUTE FUNCTION r22_reject_space_voice_invalidation();`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS r22_reject_space_voice_invalidation ON space_voice_access_outbox; DROP FUNCTION IF EXISTS r22_reject_space_voice_invalidation()`)
		require.NoError(t, cleanupErr)
	})

	require.Error(t, st.RemoveMember(ctx, space.ID, memberID))
	var stillMember bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space.ID, memberID).Scan(&stillMember))
	require.True(t, stillMember)
	require.Equal(t, before, r22SpaceEpoch(t, ctx, st, space.ID))
	var outboxAfter, orphan int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_voice_access_outbox WHERE space_id=$1`, space.ID).Scan(&outboxAfter))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_voice_access_outbox WHERE space_id=$1 AND access_epoch>$2`, space.ID, before).Scan(&orphan))
	require.Equal(t, outboxBefore, outboxAfter)
	require.Zero(t, orphan, "rolled-back owner epoch must leave no outbox orphan")
}

func TestSpaceVoiceAccessEpoch_DecisionLinearizesWithMutationAcrossPools(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "linearized", "", "private")
	require.NoError(t, err)
	applyR22SpaceEpochMigration(t, ctx, st)
	room, _, err := st.CreateVoiceRoom(ctx, space.ID, "room", nil)
	require.NoError(t, err)
	baseline := r22SpaceEpoch(t, ctx, st, space.ID)
	memberID := uuid.New()

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	_, err = tx.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, memberID)
	require.NoError(t, err, "uncommitted mutation must already own the epoch serialization lock")

	const app = "r22-space-decision-barrier"
	second := r22SecondSpacePool(t, ctx, pool, app)
	result := make(chan struct {
		decision reflect.Value
		err      error
	}, 1)
	go func() {
		decision, callErr := r22InvokeSpaceAccess(&SpaceStore{Pool: second}, ctx, space.ID, room.ID, memberID)
		result <- struct {
			decision reflect.Value
			err      error
		}{decision, callErr}
	}()

	r22WaitForSpaceLock(t, ctx, pool, app)
	require.NoError(t, tx.Commit(ctx))
	got := <-result
	require.NoError(t, got.err)
	require.True(t, r22ReflectedField(t, got.decision, "Member").Bool())
	require.True(t, r22ReflectedField(t, got.decision, "Discoverable").Bool())
	require.Equal(t, uint64(baseline+1), r22ReflectedField(t, got.decision, "AccessEpoch").Uint())
}

func TestSpaceVoiceAccessEpoch_ConcurrentMembershipChangesAreStrictlyMonotonic(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "concurrent", "", "public")
	require.NoError(t, err)
	applyR22SpaceEpochMigration(t, ctx, st)
	baseline := r22SpaceEpoch(t, ctx, st, space.ID)

	const writers = 8
	errs := make(chan error, writers)
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		profileID := uuid.New()
		go func() {
			<-start
			_, writeErr := pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, profileID)
			errs <- writeErr
		}()
	}
	close(start)
	for i := 0; i < writers; i++ {
		require.NoError(t, <-errs)
	}

	rows, err := pool.Query(ctx, `SELECT access_epoch FROM space_voice_access_outbox WHERE space_id=$1 AND access_epoch>$2 ORDER BY access_epoch`, space.ID, baseline)
	require.NoError(t, err)
	defer rows.Close()
	var epochs []int64
	for rows.Next() {
		var epoch int64
		require.NoError(t, rows.Scan(&epoch))
		epochs = append(epochs, epoch)
	}
	require.NoError(t, rows.Err())
	require.Len(t, epochs, writers)
	require.True(t, sort.SliceIsSorted(epochs, func(i, j int) bool { return epochs[i] < epochs[j] }))
	for i, epoch := range epochs {
		require.Equal(t, baseline+int64(i)+1, epoch)
	}
	require.Equal(t, baseline+writers, r22SpaceEpoch(t, ctx, st, space.ID))
}

func TestSpaceVoiceAccessEpoch_SpaceDeletionRetainsWildcardInvalidation(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "delete", "", "private")
	require.NoError(t, err)
	applyR22SpaceEpochMigration(t, ctx, st)
	baseline := r22SpaceEpoch(t, ctx, st, space.ID)

	require.NoError(t, st.DeleteSpace(ctx, space.ID))
	after := r22SpaceEpoch(t, ctx, st, space.ID)
	require.Greater(t, after, baseline)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT count(*) FROM space_voice_access_outbox
WHERE space_id=$1 AND access_epoch>$2 AND event_kind=$3
  AND voice_room_id IS NULL AND profile_id IS NULL
`, space.ID, baseline, r22SpaceEventKind).Scan(&count))
	require.Equal(t, 1, count, "hard delete must retain one all-rooms/all-profiles invalidation")
}

// The current canonical model has no room inactive/restore state: active means
// that the voice_rooms row exists. A future scheduled-delete/restore migration
// must add its own epoch tests when that distinct lifecycle lands.
