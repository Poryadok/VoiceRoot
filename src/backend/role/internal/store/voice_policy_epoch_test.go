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

	"voice/backend/role/permissions"
)

const (
	r22RoleEpochMigration = "000011_voice_policy_epoch.up.sql"
	r22RoleEventKind      = "voice_room_policy_invalidated"
)

type r22RoleOutbox struct {
	EventID   uuid.UUID
	EventKind string
	RoomID    *uuid.UUID
	ProfileID *uuid.UUID
	Payload   string
}

func r22RoleMigrationSQL(t *testing.T, name string) string {
	t.Helper()
	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "role_db", name)
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err, "R22.1 Role policy-epoch migration must exist")
	return string(sqlBytes)
}

func applyR22RoleEpochMigration(t *testing.T, ctx context.Context, st *RoleStore) {
	t.Helper()
	_, err := st.Pool.Exec(ctx, r22RoleMigrationSQL(t, r22RoleEpochMigration))
	require.NoError(t, err)
}

func runR22RoleEpochDown(t *testing.T, ctx context.Context, st *RoleStore) error {
	t.Helper()
	_, err := st.Pool.Exec(ctx, r22RoleMigrationSQL(t, "000011_voice_policy_epoch.down.sql"))
	return err
}

func r22RoleEpoch(t *testing.T, ctx context.Context, st *RoleStore, spaceID uuid.UUID) int64 {
	t.Helper()
	var epoch int64
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT policy_epoch FROM role_voice_policy_epochs WHERE space_id=$1`, spaceID).Scan(&epoch))
	require.Positive(t, epoch)
	return epoch
}

func r22RoleOutboxRow(t *testing.T, ctx context.Context, st *RoleStore, spaceID uuid.UUID, epoch int64) r22RoleOutbox {
	t.Helper()
	var out r22RoleOutbox
	require.NoError(t, st.Pool.QueryRow(ctx, `
SELECT event_id,event_kind,voice_room_id,profile_id,payload::text
FROM role_voice_policy_outbox
WHERE space_id=$1 AND policy_epoch=$2
`, spaceID, epoch).Scan(&out.EventID, &out.EventKind, &out.RoomID, &out.ProfileID, &out.Payload))
	require.NotEqual(t, uuid.Nil, out.EventID)
	require.Equal(t, r22RoleEventKind, out.EventKind)
	return out
}

func requireR22RoleSnapshot(t *testing.T, out r22RoleOutbox, spaceID uuid.UUID, epoch int64) {
	t.Helper()
	want := map[string]any{"space_id": spaceID.String(), "policy_epoch": epoch}
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

func r22InvokeRoleGrants(st *RoleStore, ctx context.Context, spaceID, roomID, profileID uuid.UUID) (reflect.Value, error) {
	method := reflect.ValueOf(st).MethodByName("ResolveVoiceRoomGrants")
	if !method.IsValid() || method.Type().NumIn() != 4 {
		return reflect.Value{}, errors.New("ResolveVoiceRoomGrants must accept context plus space, room and profile UUIDs")
	}
	result := method.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(spaceID), reflect.ValueOf(roomID), reflect.ValueOf(profileID)})
	if len(result) != 2 {
		return reflect.Value{}, errors.New("ResolveVoiceRoomGrants must return decision and error")
	}
	if !result[1].IsNil() {
		err, ok := result[1].Interface().(error)
		if !ok {
			return reflect.Value{}, errors.New("ResolveVoiceRoomGrants second result is not error")
		}
		return reflect.Value{}, err
	}
	decision := result[0]
	if !decision.IsValid() || (decision.Kind() == reflect.Pointer && decision.IsNil()) {
		return reflect.Value{}, errors.New("ResolveVoiceRoomGrants returned nil decision")
	}
	if decision.Kind() == reflect.Pointer {
		decision = decision.Elem()
	}
	if decision.Kind() != reflect.Struct {
		return reflect.Value{}, errors.New("ResolveVoiceRoomGrants decision must be a struct")
	}
	return decision, nil
}

func r22DecisionBool(t *testing.T, decision reflect.Value, name string) bool {
	t.Helper()
	field := decision.FieldByName(name)
	require.True(t, field.IsValid(), "Voice grant decision must expose %s", name)
	require.Equal(t, reflect.Bool, field.Kind())
	return field.Bool()
}

func r22DecisionEpoch(t *testing.T, decision reflect.Value) uint64 {
	t.Helper()
	field := decision.FieldByName("PolicyEpoch")
	require.True(t, field.IsValid(), "Voice grant decision must expose PolicyEpoch")
	require.Equal(t, reflect.Uint64, field.Kind())
	require.Positive(t, field.Uint())
	return field.Uint()
}

var r22VoiceGrantFields = []string{
	"CanJoin", "CanPublishAudio", "CanPublishVideo", "CanPublishScreenShare",
	"CanSubscribe", "CanMuteOthers", "CanDeafenOthers", "CanMoveOthers",
	"CanUsePTT", "PrioritySpeaker",
}

func requireR22VoiceGrantVector(t *testing.T, decision reflect.Value, trueFields ...string) {
	t.Helper()
	expected := make(map[string]bool, len(trueFields))
	for _, field := range trueFields {
		expected[field] = true
	}
	for _, field := range r22VoiceGrantFields {
		require.Equal(t, expected[field], r22DecisionBool(t, decision, field), field)
	}
	require.Equal(t, r22DecisionBool(t, decision, "CanJoin"), r22DecisionBool(t, decision, "CanSubscribe"), "VOICE_JOIN is the sole source of both join and subscribe")
}

func r22SecondRolePool(t *testing.T, ctx context.Context, pool *pgxpool.Pool, applicationName string) *pgxpool.Pool {
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

func r22WaitForRoleLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, applicationName string) {
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

func TestRoleVoicePolicyEpochMigration_BackfillsAndConstrainsPositiveEpoch(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}
	existingSpace := uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, existingSpace, uuid.New()))

	applyR22RoleEpochMigration(t, ctx, st)
	require.EqualValues(t, 1, r22RoleEpoch(t, ctx, st, existingSpace))
	_, err := pool.Exec(ctx, `UPDATE role_voice_policy_epochs SET policy_epoch=0 WHERE space_id=$1`, existingSpace)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO role_voice_policy_outbox(event_id,space_id,policy_epoch,event_kind,payload)
VALUES($1,$2,0,$3,'{}')
`, uuid.New(), existingSpace, r22RoleEventKind)
	require.Error(t, err)

	newSpace := uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, newSpace, uuid.New()))
	require.Positive(t, r22RoleEpoch(t, ctx, st, newSpace))
}

func TestRoleVoicePolicyEpochMigration_EmptyDownCanReapply(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}

	applyR22RoleEpochMigration(t, ctx, st)
	require.NoError(t, runR22RoleEpochDown(t, ctx, st))
	var epochTable, outboxTable *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('role_voice_policy_epochs')::text,to_regclass('role_voice_policy_outbox')::text`).Scan(&epochTable, &outboxTable))
	require.Nil(t, epochTable)
	require.Nil(t, outboxTable)
	var baseTable string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('roles')::text`).Scan(&baseTable))
	require.Equal(t, "roles", baseTable)

	applyR22RoleEpochMigration(t, ctx, st)
	spaceID := uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, spaceID, uuid.New()))
	require.Positive(t, r22RoleEpoch(t, ctx, st, spaceID))
}

func TestRoleVoicePolicyEpochMigration_DownWaitsForConcurrentEvidenceThenRefuses(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}
	applyR22RoleEpochMigration(t, ctx, st)

	spaceID, roleID, profileID := uuid.New(), uuid.New(), uuid.New()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	_, err = tx.Exec(ctx, `INSERT INTO roles(id,space_id,name) VALUES($1,$2,'concurrent DOWN evidence')`, roleID, spaceID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$2)`, spaceID, profileID, roleID)
	require.NoError(t, err)

	const app = "r22-role-down-barrier"
	second := r22SecondRolePool(t, ctx, pool, app)
	downSQL := r22RoleMigrationSQL(t, "000011_voice_policy_epoch.down.sql")
	downResult := make(chan error, 1)
	go func() {
		_, downErr := second.Exec(ctx, downSQL)
		downResult <- downErr
	}()

	r22WaitForRoleLock(t, ctx, pool, app)
	require.NoError(t, tx.Commit(ctx))
	require.Error(t, <-downResult, "DOWN must recheck evidence after waiting for its authority-table locks")

	var roleExists, assignmentExists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE id=$1)`, roleID).Scan(&roleExists))
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM member_roles WHERE space_id=$1 AND profile_id=$2 AND role_id=$3)`, spaceID, profileID, roleID).Scan(&assignmentExists))
	require.True(t, roleExists)
	require.True(t, assignmentExists)
	epoch := r22RoleEpoch(t, ctx, st, spaceID)
	var outboxRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM role_voice_policy_outbox WHERE space_id=$1 AND policy_epoch=$2`, spaceID, epoch).Scan(&outboxRows))
	require.Equal(t, 1, outboxRows, "refused DOWN must preserve the committed invalidation snapshot")
}

func TestRoleVoicePolicyEpochMigration_DownRefusesEachEvidenceClass(t *testing.T) {
	tests := []struct {
		name string
		seed func(*testing.T, context.Context, *RoleStore) uuid.UUID
	}{
		{"owner epoch", func(t *testing.T, ctx context.Context, st *RoleStore) uuid.UUID {
			spaceID := uuid.New()
			_, err := st.Pool.Exec(ctx, `INSERT INTO role_voice_policy_epochs(space_id,policy_epoch) VALUES($1,1)`, spaceID)
			require.NoError(t, err)
			return spaceID
		}},
		{"outbox snapshot", func(t *testing.T, ctx context.Context, st *RoleStore) uuid.UUID {
			spaceID := uuid.New()
			payload, err := json.Marshal(map[string]any{"space_id": spaceID.String(), "policy_epoch": 1})
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `INSERT INTO role_voice_policy_epochs(space_id,policy_epoch) VALUES($1,1)`, spaceID)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `
INSERT INTO role_voice_policy_outbox(event_id,space_id,policy_epoch,event_kind,payload)
VALUES($1,$2,1,$3,$4)
`, uuid.New(), spaceID, r22RoleEventKind, payload)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `SET session_replication_role=replica`)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `DELETE FROM role_voice_policy_epochs WHERE space_id=$1`, spaceID)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `SET session_replication_role=origin`)
			require.NoError(t, err)
			return spaceID
		}},
		{"owning Role scope", func(t *testing.T, ctx context.Context, st *RoleStore) uuid.UUID {
			spaceID := uuid.New()
			require.NoError(t, st.BootstrapSpaceRoles(ctx, spaceID, uuid.New()))
			_, err := st.Pool.Exec(ctx, `SET session_replication_role=replica`)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `DELETE FROM role_voice_policy_epochs WHERE space_id=$1`, spaceID)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `SET session_replication_role=origin`)
			require.NoError(t, err)
			return spaceID
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := StartRoleDBForStoreTest(t, ctx)
			ApplyRoleMigrationsForStoreTest(t, ctx, pool)
			st := &RoleStore{Pool: pool}
			applyR22RoleEpochMigration(t, ctx, st)
			spaceID := tc.seed(t, ctx, st)

			require.Error(t, runR22RoleEpochDown(t, ctx, st))
			var epochTable, outboxTable string
			require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('role_voice_policy_epochs')::text,to_regclass('role_voice_policy_outbox')::text`).Scan(&epochTable, &outboxTable))
			require.Equal(t, "role_voice_policy_epochs", epochTable)
			require.Equal(t, "role_voice_policy_outbox", outboxTable)
			var evidence int
			require.NoError(t, pool.QueryRow(ctx, `
SELECT
 (SELECT count(*) FROM role_voice_policy_epochs WHERE space_id=$1) +
 (SELECT count(*) FROM role_voice_policy_outbox WHERE space_id=$1) +
 (SELECT count(*) FROM roles WHERE space_id=$1)
`, spaceID).Scan(&evidence))
			require.Positive(t, evidence, "refused DOWN must preserve the exact blocking evidence")
		})
	}
}

func TestRoleVoicePolicyEpochMigration_NonEmptyDownRefusesAndPreservesEvidence(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}
	spaceID, ownerID, memberID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, spaceID, ownerID))
	applyR22RoleEpochMigration(t, ctx, st)
	memberRoleID, err := st.RoleIDByName(ctx, spaceID, permissions.RoleMember)
	require.NoError(t, err)
	require.NoError(t, st.AssignMemberRole(ctx, spaceID, memberID, memberRoleID, ownerID))
	before := r22RoleEpoch(t, ctx, st, spaceID)
	out := r22RoleOutboxRow(t, ctx, st, spaceID, before)

	require.Error(t, runR22RoleEpochDown(t, ctx, st), "DOWN must refuse any owning-scope/epoch/outbox evidence")
	require.Equal(t, before, r22RoleEpoch(t, ctx, st, spaceID))
	require.Equal(t, out, r22RoleOutboxRow(t, ctx, st, spaceID, before))
	var assignment bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM member_roles WHERE space_id=$1 AND profile_id=$2 AND role_id=$3)`, spaceID, memberID, memberRoleID).Scan(&assignment))
	require.True(t, assignment)

	require.NoError(t, st.RevokeMemberRole(ctx, spaceID, memberID, memberRoleID), "refused DOWN must leave the authority contract operational")
	require.Greater(t, r22RoleEpoch(t, ctx, st, spaceID), before)
}

func TestRoleVoicePolicyEpoch_MapsEveryCanonicalVoicePermissionAndOverride(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}
	applyR22RoleEpochMigration(t, ctx, st)

	tests := []struct {
		permission string
		fields     []string
	}{
		{permissions.VoiceJoin, []string{"CanJoin", "CanSubscribe"}},
		{permissions.VoiceSpeak, []string{"CanPublishAudio"}},
		{permissions.VoiceVideo, []string{"CanPublishVideo"}},
		{permissions.VoiceScreenShare, []string{"CanPublishScreenShare"}},
		{permissions.VoiceMuteOthers, []string{"CanMuteOthers"}},
		{permissions.VoiceDeafenOthers, []string{"CanDeafenOthers"}},
		{permissions.VoiceMoveOthers, []string{"CanMoveOthers"}},
		{permissions.VoiceUsePTT, []string{"CanUsePTT"}},
		{permissions.VoicePrioritySpeaker, []string{"PrioritySpeaker"}},
	}
	for _, tc := range tests {
		t.Run(tc.permission, func(t *testing.T) {
			spaceID, ownerID, memberID, roomID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			require.NoError(t, st.BootstrapSpaceRoles(ctx, spaceID, ownerID))
			zeroDefault, err := st.CreateCustomRole(ctx, spaceID, "zero default", 0, 20, &ownerID)
			require.NoError(t, err)
			require.NoError(t, st.SetDefaultJoinRole(ctx, spaceID, zeroDefault.ID))
			bit, err := permissions.MaskFor(tc.permission)
			require.NoError(t, err)
			grantRole, err := st.CreateCustomRole(ctx, spaceID, "one voice bit", bit, 21, &ownerID)
			require.NoError(t, err)
			require.NoError(t, st.AssignMemberRole(ctx, spaceID, memberID, grantRole.ID, ownerID))

			base, err := r22InvokeRoleGrants(st, ctx, spaceID, roomID, memberID)
			require.NoError(t, err)
			requireR22VoiceGrantVector(t, base, tc.fields...)
			baseEpoch := r22DecisionEpoch(t, base)
			require.Equal(t, uint64(r22RoleEpoch(t, ctx, st, spaceID)), baseEpoch)

			require.NoError(t, st.SetVoiceRoomOverride(ctx, roomID, grantRole.ID, 0, bit))
			denied, err := r22InvokeRoleGrants(st, ctx, spaceID, roomID, memberID)
			require.NoError(t, err)
			requireR22VoiceGrantVector(t, denied)
			require.Greater(t, r22DecisionEpoch(t, denied), baseEpoch)

			zero := uint64(0)
			_, err = st.UpdateRole(ctx, grantRole.ID, nil, &zero, nil)
			require.NoError(t, err)
			require.NoError(t, st.SetVoiceRoomOverride(ctx, roomID, grantRole.ID, bit, 0))
			overrideAllowed, err := r22InvokeRoleGrants(st, ctx, spaceID, roomID, memberID)
			require.NoError(t, err)
			requireR22VoiceGrantVector(t, overrideAllowed, tc.fields...)
			require.Greater(t, r22DecisionEpoch(t, overrideAllowed), r22DecisionEpoch(t, denied))
		})
	}

	missingSpace, ownerID, roomID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, missingSpace, ownerID))
	missing, err := r22InvokeRoleGrants(st, ctx, missingSpace, roomID, uuid.New())
	require.NoError(t, err)
	requireR22VoiceGrantVector(t, missing)
}

func TestRoleVoicePolicyEpoch_DecisionFailsClosedForOwnershipFreezeAndRetirement(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}
	applyR22RoleEpochMigration(t, ctx, st)

	frozenSpace, frozenOwner, frozenMember, roomID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, frozenSpace, frozenOwner))
	memberRoleID, err := st.RoleIDByName(ctx, frozenSpace, permissions.RoleMember)
	require.NoError(t, err)
	require.NoError(t, st.AssignMemberRole(ctx, frozenSpace, frozenMember, memberRoleID, frozenOwner))
	_, err = pool.Exec(ctx, `
INSERT INTO ownership_transfer_v2(
 operation_id,space_id,protocol_version,old_owner_profile_id,new_owner_profile_id,
 intent_bytes,intent_hash,state,prepare_request_hash
) VALUES($1,$2,2,$3,$4,$5,$6,'prepared',$6)
`, uuid.New(), frozenSpace, frozenOwner, uuid.New(), []byte{1}, make([]byte, 32))
	require.NoError(t, err)
	frozenDecision, err := r22InvokeRoleGrants(st, ctx, frozenSpace, roomID, frozenMember)
	require.False(t, frozenDecision.IsValid())
	require.ErrorIs(t, err, ErrSpaceFrozen)

	retiredSpace, retiredOwner, retiredMember := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, retiredSpace, retiredOwner))
	retiredRoleID, err := st.RoleIDByName(ctx, retiredSpace, permissions.RoleMember)
	require.NoError(t, err)
	require.NoError(t, st.AssignMemberRole(ctx, retiredSpace, retiredMember, retiredRoleID, retiredOwner))
	_, err = pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,now())`, retiredSpace)
	require.NoError(t, err)
	retiredDecision, err := r22InvokeRoleGrants(st, ctx, retiredSpace, uuid.New(), retiredMember)
	require.False(t, retiredDecision.IsValid())
	require.ErrorIs(t, err, ErrSpaceRetired)
}

func TestRoleVoicePolicyEpoch_EffectivePermissionMutationsAdvanceOwnerEpoch(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}
	applyR22RoleEpochMigration(t, ctx, st)

	tests := []struct {
		name   string
		mutate func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error
		scope  string
	}{
		{"member assignment", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			return st.AssignMemberRole(ctx, spaceID, profileID, roleID, ownerID)
		}, "profile"},
		{"member revocation", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			if err := st.AssignMemberRole(ctx, spaceID, profileID, roleID, ownerID); err != nil {
				return err
			}
			return st.RevokeMemberRole(ctx, spaceID, profileID, roleID)
		}, "profile"},
		{"role permission mask", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			mask := uint64(1 << 17)
			_, err := st.UpdateRole(ctx, roleID, nil, &mask, nil)
			return err
		}, "space"},
		{"role hierarchy", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			position := int32(21)
			_, err := st.UpdateRole(ctx, roleID, nil, nil, &position)
			return err
		}, "space"},
		{"role deletion", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			return st.DeleteRole(ctx, roleID)
		}, "space"},
		{"default role", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			return st.SetDefaultJoinRole(ctx, spaceID, roleID)
		}, "space"},
		{"voice room override", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			return st.SetVoiceRoomOverride(ctx, roomID, roleID, 0, 1<<17)
		}, "room"},
		{"voice room override removal", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			if err := st.SetVoiceRoomOverride(ctx, roomID, roleID, 0, 1<<17); err != nil {
				return err
			}
			return st.RemoveVoiceRoomOverride(ctx, roomID, roleID)
		}, "room"},
		{"ownership freeze", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			_, err := pool.Exec(ctx, `INSERT INTO ownership_transfer_v2(operation_id,space_id,protocol_version,old_owner_profile_id,new_owner_profile_id,intent_bytes,intent_hash,state,prepare_request_hash) VALUES($1,$2,2,$3,$4,$5,$6,'prepared',$6)`, uuid.New(), spaceID, ownerID, profileID, []byte{1}, make([]byte, 32))
			return err
		}, "space"},
		{"ownership unfreeze", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			operationID := uuid.New()
			if _, err := pool.Exec(ctx, `INSERT INTO ownership_transfer_v2(operation_id,space_id,protocol_version,old_owner_profile_id,new_owner_profile_id,intent_bytes,intent_hash,state,prepare_request_hash) VALUES($1,$2,2,$3,$4,$5,$6,'prepared',$6)`, operationID, spaceID, ownerID, profileID, []byte{1}, make([]byte, 32)); err != nil {
				return err
			}
			_, err := pool.Exec(ctx, `UPDATE ownership_transfer_v2 SET state='aborted',abort_request_hash=$2,updated_at=now() WHERE operation_id=$1`, operationID, make([]byte, 32))
			return err
		}, "space"},
		{"retirement fence", func(spaceID, ownerID, roleID, profileID, roomID uuid.UUID) error {
			_, err := pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,now())`, spaceID)
			return err
		}, "space"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spaceID, ownerID, profileID, roomID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			require.NoError(t, st.BootstrapSpaceRoles(ctx, spaceID, ownerID))
			role, err := st.CreateCustomRole(ctx, spaceID, "epoch role", 0, 20, &ownerID)
			require.NoError(t, err)
			before := r22RoleEpoch(t, ctx, st, spaceID)
			require.NoError(t, tc.mutate(spaceID, ownerID, role.ID, profileID, roomID))
			after := r22RoleEpoch(t, ctx, st, spaceID)
			require.Greater(t, after, before)
			out := r22RoleOutboxRow(t, ctx, st, spaceID, after)
			requireR22RoleSnapshot(t, out, spaceID, after)
			switch tc.scope {
			case "profile":
				require.Nil(t, out.RoomID)
				require.Equal(t, profileID, *out.ProfileID)
			case "room":
				require.Equal(t, roomID, *out.RoomID)
				require.Nil(t, out.ProfileID)
			case "space":
				require.Nil(t, out.RoomID)
				require.Nil(t, out.ProfileID)
			}
		})
	}
}

func TestRoleVoicePolicyEpoch_OutboxIsUniqueImmutableSnapshot(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}
	applyR22RoleEpochMigration(t, ctx, st)
	spaceID, ownerID, memberID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, spaceID, ownerID))
	memberRoleID, err := st.RoleIDByName(ctx, spaceID, permissions.RoleMember)
	require.NoError(t, err)
	require.NoError(t, st.AssignMemberRole(ctx, spaceID, memberID, memberRoleID, ownerID))
	epoch := r22RoleEpoch(t, ctx, st, spaceID)
	out := r22RoleOutboxRow(t, ctx, st, spaceID, epoch)
	requireR22RoleSnapshot(t, out, spaceID, epoch)

	_, err = pool.Exec(ctx, `UPDATE role_voice_policy_outbox SET payload='{}' WHERE event_id=$1`, out.EventID)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM role_voice_policy_outbox WHERE event_id=$1`, out.EventID)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO role_voice_policy_outbox(event_id,space_id,policy_epoch,event_kind,profile_id,payload)
VALUES($1,$2,$3,$4,$5,$6)
`, uuid.New(), spaceID, epoch, r22RoleEventKind, memberID, out.Payload)
	require.Error(t, err, "(space_id, policy_epoch) identifies exactly one invalidation snapshot")
	require.Equal(t, out, r22RoleOutboxRow(t, ctx, st, spaceID, epoch))
}

func TestRoleVoicePolicyEpoch_OutboxFailureRollsBackWithoutOrphan(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}
	applyR22RoleEpochMigration(t, ctx, st)
	spaceID, ownerID, roomID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, spaceID, ownerID))
	memberRoleID, err := st.RoleIDByName(ctx, spaceID, permissions.RoleMember)
	require.NoError(t, err)
	before := r22RoleEpoch(t, ctx, st, spaceID)
	var outboxBefore int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM role_voice_policy_outbox WHERE space_id=$1`, spaceID).Scan(&outboxBefore))

	_, err = pool.Exec(ctx, `
CREATE FUNCTION r22_reject_role_voice_invalidation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'reject voice invalidation' USING ERRCODE='23514'; END $$;
CREATE TRIGGER r22_reject_role_voice_invalidation
BEFORE INSERT ON role_voice_policy_outbox
FOR EACH ROW EXECUTE FUNCTION r22_reject_role_voice_invalidation();`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS r22_reject_role_voice_invalidation ON role_voice_policy_outbox; DROP FUNCTION IF EXISTS r22_reject_role_voice_invalidation()`)
		require.NoError(t, cleanupErr)
	})

	require.Error(t, st.SetVoiceRoomOverride(ctx, roomID, memberRoleID, 0, 1<<17))
	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_room_overrides WHERE voice_room_id=$1 AND role_id=$2)`, roomID, memberRoleID).Scan(&exists))
	require.False(t, exists)
	require.Equal(t, before, r22RoleEpoch(t, ctx, st, spaceID))
	var outboxAfter, orphan int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM role_voice_policy_outbox WHERE space_id=$1`, spaceID).Scan(&outboxAfter))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM role_voice_policy_outbox WHERE space_id=$1 AND policy_epoch>$2`, spaceID, before).Scan(&orphan))
	require.Equal(t, outboxBefore, outboxAfter)
	require.Zero(t, orphan)
}

func TestRoleVoicePolicyEpoch_DecisionLinearizesWithMutationAcrossPools(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}
	applyR22RoleEpochMigration(t, ctx, st)
	spaceID, ownerID, memberID, roomID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, spaceID, ownerID))
	zeroDefault, err := st.CreateCustomRole(ctx, spaceID, "zero default", 0, 20, &ownerID)
	require.NoError(t, err)
	require.NoError(t, st.SetDefaultJoinRole(ctx, spaceID, zeroDefault.ID))
	joinBit, err := permissions.MaskFor(permissions.VoiceJoin)
	require.NoError(t, err)
	joinRole, err := st.CreateCustomRole(ctx, spaceID, "join", joinBit, 21, &ownerID)
	require.NoError(t, err)
	baseline := r22RoleEpoch(t, ctx, st, spaceID)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	_, err = tx.Exec(ctx, `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$4)`, spaceID, memberID, joinRole.ID, ownerID)
	require.NoError(t, err)

	const app = "r22-role-decision-barrier"
	second := r22SecondRolePool(t, ctx, pool, app)
	result := make(chan struct {
		decision reflect.Value
		err      error
	}, 1)
	go func() {
		decision, callErr := r22InvokeRoleGrants(&RoleStore{Pool: second}, ctx, spaceID, roomID, memberID)
		result <- struct {
			decision reflect.Value
			err      error
		}{decision, callErr}
	}()

	r22WaitForRoleLock(t, ctx, pool, app)
	require.NoError(t, tx.Commit(ctx))
	got := <-result
	require.NoError(t, got.err)
	requireR22VoiceGrantVector(t, got.decision, "CanJoin", "CanSubscribe")
	require.Equal(t, uint64(baseline+1), r22DecisionEpoch(t, got.decision))
}

func TestRoleVoicePolicyEpoch_ConcurrentAssignmentsAreStrictlyMonotonic(t *testing.T) {
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	st := &RoleStore{Pool: pool}
	applyR22RoleEpochMigration(t, ctx, st)
	spaceID, ownerID := uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, spaceID, ownerID))
	memberRoleID, err := st.RoleIDByName(ctx, spaceID, permissions.RoleMember)
	require.NoError(t, err)
	baseline := r22RoleEpoch(t, ctx, st, spaceID)

	const writers = 8
	errs := make(chan error, writers)
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		profileID := uuid.New()
		go func() {
			<-start
			_, writeErr := pool.Exec(ctx, `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$4)`, spaceID, profileID, memberRoleID, ownerID)
			errs <- writeErr
		}()
	}
	close(start)
	for i := 0; i < writers; i++ {
		require.NoError(t, <-errs)
	}

	rows, err := pool.Query(ctx, `SELECT policy_epoch FROM role_voice_policy_outbox WHERE space_id=$1 AND policy_epoch>$2 ORDER BY policy_epoch`, spaceID, baseline)
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
	require.Equal(t, baseline+writers, r22RoleEpoch(t, ctx, st, spaceID))
}
