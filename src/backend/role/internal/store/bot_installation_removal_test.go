package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/integrationtest"
)

func TestBotInstallationRemoval_SourceMigration(t *testing.T) {
	ctx := context.Background()
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	f.Run(true, "goto", "21")
	f.Run(true, "goto", "25")
	var table string
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT 'role_bot_removal_operations'::regclass::text").Scan(&table))
	require.Equal(t, "role_bot_removal_operations", table)
	f.Run(true, "down", "1")
	f.Run(true, "up", "1")
}

func removalRequest() *rolev1.ApplyBotInstallationRemovalRequest {
	return &rolev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), DecisionId: uuid.NewString(), DecisionSha256: bytes.Repeat([]byte{1}, 32), Purpose: "space_uninstall", BotId: uuid.NewString(), ActorProfileId: uuid.NewString(), SpaceId: uuid.NewString(), FrozenScopeSha256: bytes.Repeat([]byte{2}, 32), Evidence: &rolev1.ApplyBotInstallationRemovalRequest_Native{Native: &rolev1.BotRemovalNativeEvidence{InstallationId: uuid.NewString(), InstallOperationId: uuid.NewString(), Generation: 1}}}
}

func removalFixture(t *testing.T) integrationtest.SourceMigrationFixture {
	t.Helper()
	f := integrationtest.NewSourceMigrationFixture(t, context.Background(), filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	f.Run(true, "goto", "25")
	return f
}

func insertRemovalRole(t *testing.T, p *pgxpool.Pool, space, actor, name string, system bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := p.Exec(context.Background(), "INSERT INTO roles(id,space_id,name,is_system,created_by_profile_id) VALUES($1,$2,$3,$4,$5)", id, space, name, system, actor)
	require.NoError(t, err)
	return id
}

func TestBotInstallationRemoval_ScopedExactReplayAndRestart(t *testing.T) {
	ctx := context.Background()
	f := removalFixture(t)
	s := &RoleStore{Pool: f.Pool}
	req := removalRequest()
	actor := uuid.MustParse(req.ActorProfileId)
	space := uuid.MustParse(req.SpaceId)
	created := insertRemovalRole(t, f.Pool, req.SpaceId, req.ActorProfileId, "Bot created", false)
	other := insertRemovalRole(t, f.Pool, req.SpaceId, uuid.NewString(), "Other custom", false)
	system := insertRemovalRole(t, f.Pool, req.SpaceId, req.ActorProfileId, "Member", true)
	neighbor := insertRemovalRole(t, f.Pool, uuid.NewString(), req.ActorProfileId, "Neighbor", false)
	human := uuid.New()
	authority := uuid.New()
	bind := bytes.Repeat([]byte{7}, 32)
	_, err := f.Pool.Exec(ctx, `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$2),($1,$2,$4,$2),($1,$5,$6,$5);
 INSERT INTO role_member_contributions(id,authorization_id,space_id,profile_id,role_id,source_binding,source_kind,membership_generation,owner_generation) VALUES($7,$8,$1,$5,$6,$9,'ORDINARY',1,0)`, pgx.QueryExecModeSimpleProtocol, space, actor, other, system, human, created, uuid.New(), authority, []byte{1})
	require.NoError(t, err)
	_, err = f.Pool.Exec(ctx, `INSERT INTO role_member_admission_receipts(operation_id,operation_kind,protocol_version,space_id,profile_id,generation,binding_sha256,role_id,receipt_sha256) VALUES($1,'ENSURE',1,$2,$3,1,$4,$5,$4);
 INSERT INTO role_member_admission_preparations(space_id,profile_id,membership_generation,admission_operation_id,role_id,receipt_sha256) VALUES($2,$3,1,$1,$5,$4)`, pgx.QueryExecModeSimpleProtocol, authority, space, human, bind, created)
	require.NoError(t, err)
	actorPreparation := uuid.New()
	_, err = f.Pool.Exec(ctx, `INSERT INTO role_member_admission_receipts(operation_id,operation_kind,protocol_version,space_id,profile_id,generation,binding_sha256,role_id,receipt_sha256) VALUES($1,'ENSURE',1,$2,$3,1,$4,$5,$4);
 INSERT INTO role_member_admission_preparations(space_id,profile_id,membership_generation,admission_operation_id,role_id,receipt_sha256) VALUES($2,$3,1,$1,$5,$4)`, pgx.QueryExecModeSimpleProtocol, actorPreparation, space, actor, bind, other)
	require.NoError(t, err)
	result, err := s.ApplyBotInstallationRemoval(ctx, req)
	require.NoError(t, err)
	require.Equal(t, uint64(1), result.Receipt.DeletedRoleCount)
	require.Equal(t, uint64(3), result.Receipt.RemovedAssignmentCount)
	require.Equal(t, "COMPLETED", result.Receipt.Outcome)
	require.Equal(t, "role", result.Receipt.Participant)
	wire, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	want := sha256.Sum256(wire)
	require.Equal(t, want[:], result.Receipt.RequestSha256)
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE id=ANY($1)", []uuid.UUID{other, system, neighbor}).Scan(&count))
	require.Equal(t, 3, count)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_member_admission_preparations WHERE role_id=$1", created).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_member_admission_preparations WHERE space_id=$1 AND profile_id=$2", space, actor).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_member_admission_receipts WHERE operation_id=$1", authority).Scan(&count))
	require.Equal(t, 1, count)
	// Recreate the application/store and recover the original response, then
	// replay after a legitimate new installation produced new scoped effects.
	fresh := insertRemovalRole(t, f.Pool, req.SpaceId, req.ActorProfileId, "New install role", false)
	restart := &RoleStore{Pool: f.Pool}
	again, err := restart.ApplyBotInstallationRemoval(ctx, proto.Clone(req).(*rolev1.ApplyBotInstallationRemovalRequest))
	require.NoError(t, err)
	require.True(t, proto.Equal(result, again))
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE id=$1", fresh).Scan(&count))
	require.Equal(t, 1, count)
	changed := proto.Clone(req).(*rolev1.ApplyBotInstallationRemovalRequest)
	changed.FrozenScopeSha256[0] ^= 1
	_, err = s.ApplyBotInstallationRemoval(ctx, changed)
	require.ErrorIs(t, err, ErrBotRemovalConflict)
	foreignSpace := proto.Clone(req).(*rolev1.ApplyBotInstallationRemovalRequest)
	foreignSpace.OperationId = uuid.NewString()
	foreignSpace.SpaceId = uuid.NewString()
	_, err = s.ApplyBotInstallationRemoval(ctx, foreignSpace)
	require.ErrorIs(t, err, ErrBotRemovalDenied)
	require.ErrorIs(t, s.WithinSpaces(ctx, []uuid.UUID{space}, func(tx *RoleStore) error {
		return tx.CheckBotInstallationRemovalFence(ctx, uuid.MustParse(req.BotId), actor, space, uuid.MustParse(req.GetNative().InstallationId))
	}), ErrBotRemovalDenied)
	require.NoError(t, s.WithinSpaces(ctx, []uuid.UUID{space}, func(tx *RoleStore) error {
		return tx.CheckBotInstallationRemovalFence(ctx, uuid.MustParse(req.BotId), actor, space, uuid.New())
	}))
	global := proto.Clone(req).(*rolev1.ApplyBotInstallationRemovalRequest)
	global.OperationId = uuid.NewString()
	global.DecisionId = uuid.NewString()
	global.Purpose = "global_delete"
	adopted, err := s.ApplyBotInstallationRemoval(ctx, global)
	require.NoError(t, err)
	require.Zero(t, adopted.Receipt.DeletedRoleCount)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE id=$1", fresh).Scan(&count))
	require.Equal(t, 1, count)
	require.ErrorIs(t, s.WithinSpaces(ctx, []uuid.UUID{space}, func(tx *RoleStore) error {
		return tx.CheckBotInstallationRemovalFence(ctx, uuid.MustParse(req.BotId), actor, space, uuid.New())
	}), ErrBotRemovalDenied)
	// No DOWN may erase even a blocked/complete marker or known actor binding.
	f.Run(false, "down", "1")
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_removal_operations").Scan(&count))
	require.Equal(t, 2, count)
}

func TestBotInstallationRemoval_OwnerBlockedRetainsOnlySafety(t *testing.T) {
	ctx := context.Background()
	f := removalFixture(t)
	s := &RoleStore{Pool: f.Pool}
	req := removalRequest()
	req.Purpose = "global_delete"
	owner := insertRemovalRole(t, f.Pool, req.SpaceId, uuid.NewString(), "Owner", true)
	custom := insertRemovalRole(t, f.Pool, req.SpaceId, req.ActorProfileId, "Still retained", false)
	_, err := f.Pool.Exec(ctx, "INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$2)", req.SpaceId, req.ActorProfileId, owner)
	require.NoError(t, err)
	response, err := s.ApplyBotInstallationRemoval(ctx, req)
	require.ErrorIs(t, err, ErrBotRemovalBlocked)
	require.Nil(t, response)
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE id=ANY($1)", []uuid.UUID{owner, custom}).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_global_removals").Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_removal_operations WHERE response_bytes IS NULL").Scan(&count))
	require.Equal(t, 1, count)
	foreign := proto.Clone(req).(*rolev1.ApplyBotInstallationRemovalRequest)
	foreign.OperationId = uuid.NewString()
	foreign.ActorProfileId = uuid.NewString()
	_, err = s.ApplyBotInstallationRemoval(ctx, foreign)
	require.ErrorIs(t, err, ErrBotRemovalDenied)
}

func TestBotInstallationRemoval_FaultRollbackAndConcurrentDuplicate(t *testing.T) {
	ctx := context.Background()
	f := removalFixture(t)
	s := &RoleStore{Pool: f.Pool}
	req := removalRequest()
	custom := insertRemovalRole(t, f.Pool, req.SpaceId, req.ActorProfileId, "Bot custom", false)
	_, err := f.Pool.Exec(ctx, `CREATE FUNCTION fail_bot_receipt() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'injected receipt failure'; END$$; CREATE TRIGGER fail_receipt BEFORE UPDATE ON role_bot_removal_operations FOR EACH ROW EXECUTE FUNCTION fail_bot_receipt()`)
	require.NoError(t, err)
	_, err = s.ApplyBotInstallationRemoval(ctx, req)
	require.Error(t, err)
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE id=$1", custom).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_installation_removals").Scan(&count))
	require.Zero(t, count)
	_, err = f.Pool.Exec(ctx, "DROP TRIGGER fail_receipt ON role_bot_removal_operations")
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make([]*rolev1.ApplyBotInstallationRemovalResponse, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = (&RoleStore{Pool: f.Pool}).ApplyBotInstallationRemoval(ctx, proto.Clone(req).(*rolev1.ApplyBotInstallationRemovalRequest))
		}()
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.True(t, proto.Equal(results[0], results[1]))
	require.Equal(t, uint64(1), results[0].Receipt.DeletedRoleCount)
}

func TestBotInstallationRemoval_Validation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*rolev1.ApplyBotInstallationRemovalRequest)
		want   error
	}{
		{"zero generation", func(r *rolev1.ApplyBotInstallationRemovalRequest) { r.GetNative().Generation = 0 }, ErrBotRemovalInvalid},
		{"high generation", func(r *rolev1.ApplyBotInstallationRemovalRequest) { r.GetNative().Generation = 1 << 63 }, ErrBotRemovalInvalid},
		{"noncanonical UUID", func(r *rolev1.ApplyBotInstallationRemovalRequest) { r.BotId = "00000000-0000-0000-0000-000000000000" }, ErrBotRemovalInvalid},
		{"unknown nested", func(r *rolev1.ApplyBotInstallationRemovalRequest) {
			r.GetNative().ProtoReflect().SetUnknown([]byte{0x20, 1})
		}, ErrBotRemovalInvalid},
		{"missing evidence", func(r *rolev1.ApplyBotInstallationRemovalRequest) { r.Evidence = nil }, ErrBotRemovalInvalid},
		{"purpose", func(r *rolev1.ApplyBotInstallationRemovalRequest) { r.Purpose = "interactive" }, ErrBotRemovalDenied},
		{"orphan uninstall", func(r *rolev1.ApplyBotInstallationRemovalRequest) {
			r.Evidence = &rolev1.ApplyBotInstallationRemovalRequest_OrphanWhitelist{OrphanWhitelist: &rolev1.BotRemovalOrphanWhitelistEvidence{SnapshotId: uuid.NewString(), SnapshotSha256: make([]byte, 32)}}
		}, ErrBotRemovalDenied},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := removalRequest()
			tt.change(r)
			require.ErrorIs(t, ValidateBotInstallationRemoval(r), tt.want)
		})
	}
}

func TestBotInstallationRemoval_LegacyOrphanAndGlobalBinding(t *testing.T) {
	ctx := context.Background()
	f := removalFixture(t)
	s := &RoleStore{Pool: f.Pool}
	for _, kind := range []string{"legacy_installation", "orphan_whitelist"} {
		t.Run(kind, func(t *testing.T) {
			req := removalRequest()
			req.Purpose = "global_delete"
			if kind == "legacy_installation" {
				req.Evidence = &rolev1.ApplyBotInstallationRemovalRequest_LegacyInstallation{LegacyInstallation: &rolev1.BotRemovalLegacyInstallationEvidence{InstallationId: uuid.NewString(), SnapshotId: uuid.NewString(), SnapshotSha256: make([]byte, 32)}}
			} else {
				req.Evidence = &rolev1.ApplyBotInstallationRemovalRequest_OrphanWhitelist{OrphanWhitelist: &rolev1.BotRemovalOrphanWhitelistEvidence{SnapshotId: uuid.NewString(), SnapshotSha256: make([]byte, 32)}}
			}
			insertRemovalRole(t, f.Pool, req.SpaceId, req.ActorProfileId, "Legacy bot custom", false)
			r, err := s.ApplyBotInstallationRemoval(ctx, req)
			require.NoError(t, err)
			require.Equal(t, kind, r.Receipt.EvidenceKind)
			require.Equal(t, uint64(1), r.Receipt.DeletedRoleCount)
			evidence := req.GetLegacyInstallation()
			var msg proto.Message = evidence
			if kind == "orphan_whitelist" {
				msg = req.GetOrphanWhitelist()
			}
			wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(msg)
			require.NoError(t, err)
			digest := sha256.Sum256(wire)
			require.Equal(t, digest[:], r.Receipt.EvidenceSha256)
			again, err := s.ApplyBotInstallationRemoval(ctx, req)
			require.NoError(t, err)
			require.True(t, proto.Equal(r, again))
			changed := proto.Clone(req).(*rolev1.ApplyBotInstallationRemovalRequest)
			changed.OperationId = uuid.NewString()
			changed.DecisionId = uuid.NewString()
			_, err = s.ApplyBotInstallationRemoval(ctx, changed)
			require.ErrorIs(t, err, ErrBotRemovalConflict)
		})
	}
}

func TestBotInstallationRemoval_WriterBeforeAndAfterCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	f := removalFixture(t)
	s := &RoleStore{Pool: f.Pool}
	req := removalRequest()
	bot := uuid.MustParse(req.BotId)
	actor := uuid.MustParse(req.ActorProfileId)
	space := uuid.MustParse(req.SpaceId)
	installation := uuid.MustParse(req.GetNative().InstallationId)
	entered := make(chan struct{})
	release := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- s.WithinSpaces(ctx, []uuid.UUID{space}, func(tx *RoleStore) error {
			if err := tx.CheckBotInstallationRemovalFence(ctx, bot, actor, space, installation); err != nil {
				return err
			}
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			_, err := tx.CreateCustomRole(ctx, space, "Committed before cleanup", 0, 1, &actor)
			return err
		})
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	removalDone := make(chan error, 1)
	go func() { _, err := s.ApplyBotInstallationRemoval(ctx, req); removalDone <- err }()
	close(release)
	require.NoError(t, <-writerDone)
	require.NoError(t, <-removalDone)
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE space_id=$1 AND created_by_profile_id=$2", space, actor).Scan(&count))
	require.Zero(t, count)
	err := s.WithinSpaces(ctx, []uuid.UUID{space}, func(tx *RoleStore) error {
		if err := tx.CheckBotInstallationRemovalFence(ctx, bot, actor, space, installation); err != nil {
			return err
		}
		_, err := tx.CreateCustomRole(ctx, space, "Forbidden late write", 0, 1, &actor)
		return err
	})
	require.ErrorIs(t, err, ErrBotRemovalDenied)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE space_id=$1", space).Scan(&count))
	require.Zero(t, count)
	require.ErrorIs(t, s.CheckBotInstallationRemovalFence(ctx, bot, actor, space, installation), ErrScopeUnavailable)
}

func TestBotInstallationRemoval_FirstWriterPreventsSourceDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	f := removalFixture(t)
	tx, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, "INSERT INTO role_bot_actor_bindings(bot_id,actor_profile_id) VALUES($1,$2)", uuid.New(), uuid.New())
	require.NoError(t, err)
	done := make(chan struct{})
	go func() { defer close(done); f.Run(false, "down", "1") }()
	// The real migration driver's AccessExclusive lock must actually wait on
	// this first writer; elapsed sleep alone would not establish the ordering.
	require.Eventually(t, func() bool {
		var waiting bool
		err := f.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%LOCK TABLE role_bot_actor_bindings%')`).Scan(&waiting)
		return err == nil && waiting
	}, 15*time.Second, 20*time.Millisecond)
	require.NoError(t, tx.Commit(ctx))
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("source DOWN did not settle")
	}
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_actor_bindings").Scan(&count))
	require.Equal(t, 1, count)
}

func TestBotInstallationRemoval_StaleRepeatableReadCannotDropEvidence(t *testing.T) {
	ctx := context.Background()
	f := removalFixture(t)
	stale, err := f.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer func() { _ = stale.Rollback(context.Background()) }()
	var count int
	require.NoError(t, stale.QueryRow(ctx, "SELECT count(*) FROM role_bot_actor_bindings").Scan(&count))
	require.Zero(t, count)
	_, err = f.Pool.Exec(ctx, "INSERT INTO role_bot_actor_bindings(bot_id,actor_profile_id) VALUES($1,$2)", uuid.New(), uuid.New())
	require.NoError(t, err)
	// Execute the actual DOWN SQL on the old snapshot. Table locks alone do
	// not refresh this snapshot and must never authorize erasing the new row.
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/role_db/000025_bot_installation_removal.down.sql"))
	require.NoError(t, err)
	_, err = stale.Exec(ctx, string(down))
	require.ErrorContains(t, err, "READ COMMITTED")
	_ = stale.Rollback(ctx)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_actor_bindings").Scan(&count))
	require.Equal(t, 1, count)
}
