package gameprovision

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/integrationtest"
)

func TestPostgresGameSessionProvisioning_ExactReplayAndConflict(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	first := NewPostgresStore(pool)
	require.NoError(t, first.CheckSchema(ctx))
	request := validProvisionRequest()

	original, err := first.Provision(ctx, request)
	require.NoError(t, err)
	originalBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(original)
	require.NoError(t, err)

	restarted := NewPostgresStore(pool)
	replayed, err := restarted.Provision(ctx, proto.Clone(request).(*callsv1.ProvisionGameSessionRoomRequest))
	require.NoError(t, err)
	replayedBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(replayed)
	require.NoError(t, err)
	require.Equal(t, originalBytes, replayedBytes, "fresh store recovery returns the immutable response bytes")

	room, err := restarted.GetRoom(ctx, original.RoomId)
	require.NoError(t, err, "a fresh Voice store resolves the stable room mapping for user admission")
	require.Equal(t, original.RoomId, room.RoomID)
	require.Equal(t, request.ChatId, room.ChatID)
	require.Equal(t, request.SessionId, room.SessionID)
	require.Equal(t, original.LivekitRoomName, room.LiveKitRoomName)

	changed := proto.Clone(request).(*callsv1.ProvisionGameSessionRoomRequest)
	changed.ChatId = uuid.NewString()
	_, err = restarted.Provision(ctx, changed)
	require.ErrorIs(t, err, ErrConflict, "same operation with changed deterministic request conflicts")
	changedSession := proto.Clone(request).(*callsv1.ProvisionGameSessionRoomRequest)
	changedSession.SessionId = uuid.NewString()
	_, err = restarted.Provision(ctx, changedSession)
	require.ErrorIs(t, err, ErrConflict, "the same resource cannot be rebound to another GIS session")

	otherOperation := proto.Clone(request).(*callsv1.ProvisionGameSessionRoomRequest)
	otherOperation.OperationId = uuid.NewString()
	_, err = restarted.Provision(ctx, otherOperation)
	require.ErrorIs(t, err, ErrConflict, "a second operation cannot remap the same external resource")

	var rooms, receipts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_room_instances`).Scan(&rooms))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_game_session_operations`).Scan(&receipts))
	require.Equal(t, 1, rooms)
	require.Equal(t, 1, receipts)
}

func TestPostgresGameSessionProvisioning_RoomLookupRequiresManagedRoom(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	store := NewPostgresStore(pool)
	response, err := store.Provision(ctx, validProvisionRequest())
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE voice_room_instances SET state='closed',closed_at=now() WHERE room_id=$1`, uuid.MustParse(response.RoomId))
	require.NoError(t, err)
	_, err = store.GetRoom(ctx, response.RoomId)
	require.ErrorIs(t, err, ErrNotFound, "closed managed rooms cannot be admitted")
	_, err = store.GetRoom(ctx, uuid.NewString())
	require.ErrorIs(t, err, ErrNotFound, "unmapped rooms cannot be admitted")
}

func TestPostgresGameSessionProvisioning_RoomReceiptAndMappingRollbackTogether(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	request := validProvisionRequest()

	_, err := pool.Exec(ctx, `
CREATE FUNCTION fail_game_session_receipt_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'injected receipt failure'; END;
$$;
CREATE TRIGGER fail_game_session_receipt BEFORE INSERT ON voice_game_session_operations
FOR EACH ROW EXECUTE FUNCTION fail_game_session_receipt_insert();`)
	require.NoError(t, err)
	_, err = NewPostgresStore(pool).Provision(ctx, request)
	require.Error(t, err, "the injected receipt failure must abort provisioning")

	var rooms, receipts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_room_instances`).Scan(&rooms))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_game_session_operations`).Scan(&receipts))
	require.Zero(t, rooms, "room insert must roll back with receipt insert")
	require.Zero(t, receipts)

	_, err = pool.Exec(ctx, `DROP TRIGGER fail_game_session_receipt ON voice_game_session_operations; DROP FUNCTION fail_game_session_receipt_insert();`)
	require.NoError(t, err)
	committed, err := NewPostgresStore(pool).Provision(ctx, request)
	require.NoError(t, err)
	recovered, err := NewPostgresStore(pool).Provision(ctx, proto.Clone(request).(*callsv1.ProvisionGameSessionRoomRequest))
	require.NoError(t, err)
	require.True(t, proto.Equal(committed, recovered))
}

func TestPostgresGameSessionProvisioning_OwnerlessPurposeDoesNotWeakenMatchSquadOwner(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	response, err := NewPostgresStore(pool).Provision(ctx, validProvisionRequest())
	require.NoError(t, err)

	var purpose string
	var owner *uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT purpose,owner_id FROM voice_room_instances WHERE room_id=$1`, uuid.MustParse(response.RoomId)).Scan(&purpose, &owner))
	require.Equal(t, "GAME_SESSION", purpose)
	require.Nil(t, owner, "GIS principal and players do not own the room")

	roomID, chatID := uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO voice_room_instances
(room_id,space_id,voice_room_id,livekit_room_name,state,roster_version,created_at,updated_at,closed_at,room_type,purpose,chat_id,owner_id,creation_operation_id,creation_manifest_hash,creation_receipt_id,chat_creation_receipt_id)
VALUES($1,NULL,NULL,$2,'active',0,now(),now(),NULL,'group_voice','MATCH_SQUAD',$3,NULL,$4,$5,$6,$7)`,
		roomID, "voice-test-"+roomID.String(), chatID, uuid.New(), make([]byte, 32), uuid.New(), uuid.New())
	require.Error(t, err, "MATCH_SQUAD still requires its owner ID")
}

func TestPostgresGameSessionRoster_RevisionReceiptExpiryRemovalAndRestart(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	store := NewPostgresStore(pool)
	provisioned, err := store.Provision(ctx, validProvisionRequest())
	require.NoError(t, err)
	firstProfile, secondProfile, thirdProfile := uuid.NewString(), uuid.NewString(), uuid.NewString()
	profiles := []string{firstProfile, secondProfile}
	sort.Strings(profiles)
	deadline := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Microsecond)
	request := rosterRequest(provisioned, 2, profiles, deadline)
	fencer := &recordingRosterMediaFencer{}
	accepted, err := store.ApplyGameSessionRoster(ctx, request, fencer)
	require.NoError(t, err)
	require.Equal(t, uint64(2), accepted.AcceptedRevision)
	require.Equal(t, deadline, accepted.LeaseExpiresAt.AsTime())
	require.NotEmpty(t, accepted.ReceiptId)
	require.Len(t, accepted.RequestHash, 32)
	require.Empty(t, fencer.removed)

	// Exact operation replay returns the immutable receipt and never renews.
	restarted := NewPostgresStore(pool)
	replayed, err := restarted.ApplyGameSessionRoster(ctx, proto.Clone(request).(*callsv1.ApplyGameSessionRosterRequest), fencer)
	require.NoError(t, err)
	require.True(t, proto.Equal(accepted, replayed))
	require.Equal(t, deadline, replayed.LeaseExpiresAt.AsTime())

	// Lower revisions are no-ops; equal revision with changed content conflicts.
	lower := rosterRequest(provisioned, 1, []string{thirdProfile}, deadline.Add(time.Minute))
	stale, err := restarted.ApplyGameSessionRoster(ctx, lower, fencer)
	require.NoError(t, err)
	require.Equal(t, uint64(2), stale.AcceptedRevision)
	require.Equal(t, deadline, stale.LeaseExpiresAt.AsTime())
	changedProfiles := []string{firstProfile, thirdProfile}
	sort.Strings(changedProfiles)
	changedEqual := rosterRequest(provisioned, 2, changedProfiles, deadline.Add(time.Minute))
	_, err = restarted.ApplyGameSessionRoster(ctx, changedEqual, fencer)
	require.ErrorIs(t, err, ErrConflict)
	changedDeadline := rosterRequest(provisioned, 2, profiles, deadline.Add(time.Minute))
	_, err = restarted.ApplyGameSessionRoster(ctx, changedDeadline, fencer)
	require.ErrorIs(t, err, ErrConflict, "same revision with changed request body cannot renew the stored deadline")

	// A higher complete revision commits the roster before fencing removed media.
	higher := rosterRequest(provisioned, 3, []string{secondProfile}, deadline.Add(time.Minute))
	updated, err := restarted.ApplyGameSessionRoster(ctx, higher, fencer)
	require.NoError(t, err)
	require.Equal(t, uint64(3), updated.AcceptedRevision)
	removedExpected := profiles[0]
	if removedExpected == secondProfile {
		removedExpected = profiles[1]
	}
	require.Equal(t, []string{removedExpected}, fencer.removed)
	room, err := NewPostgresStore(pool).GetRoom(ctx, provisioned.RoomId)
	require.NoError(t, err, "roster survives store restart")
	require.Equal(t, uint64(3), room.RosterRevision)
	require.Equal(t, []string{secondProfile}, room.ProfileIDs)
	require.Equal(t, deadline.Add(time.Minute), room.LeaseExpiresAt.UTC())

	// Admission is denied at equality with the DB lease deadline.
	_, err = NewPostgresStore(pool).GetRoomAt(ctx, provisioned.RoomId, deadline.Add(time.Minute))
	require.ErrorIs(t, err, ErrNotFound)
	expiryFencer := &recordingRosterMediaFencer{}
	require.NoError(t, NewPostgresStore(pool).FenceExpiredGameSessionLeasesAt(ctx, expiryFencer, deadline.Add(time.Minute)))
	require.Equal(t, []string{secondProfile}, expiryFencer.removed, "expired leases fence current participants")
	var fencedAt *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT lease_fenced_at FROM voice_game_session_operations WHERE room_id=$1`, uuid.MustParse(provisioned.RoomId)).Scan(&fencedAt))
	require.NotNil(t, fencedAt, "fence completion is durable across worker restart")
}

func TestPostgresGameSessionRoster_RejectsNewRevisionAfterCloseWins(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	store := NewPostgresStore(pool)
	provisioned, err := store.Provision(ctx, validProvisionRequest())
	require.NoError(t, err)
	profiles := []string{uuid.NewString()}
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
	initial := rosterRequest(provisioned, 1, profiles, deadline)
	_, err = store.ApplyGameSessionRoster(ctx, initial, &recordingRosterMediaFencer{})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE voice_room_instances SET state='closing',updated_at=now() WHERE room_id=$1`, uuid.MustParse(provisioned.RoomId))
	require.NoError(t, err)

	late := rosterRequest(provisioned, 2, nil, deadline.Add(time.Minute))
	_, err = store.ApplyGameSessionRoster(ctx, late, &recordingRosterMediaFencer{})
	require.ErrorIs(t, err, ErrNotFound, "once close marks the room non-active, delayed updates cannot mutate roster state")
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT roster_revision FROM voice_game_session_operations WHERE room_id=$1`, uuid.MustParse(provisioned.RoomId)).Scan(&revision))
	require.EqualValues(t, 1, revision)
}

func TestPostgresSdkConversionFence_ExactReplayConflictAndReceiptRecovery(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	store := NewPostgresStore(pool)
	req := &callsv1.FenceSdkConversionRequest{
		OperationId: uuid.NewString(), BindingId: uuid.NewString(), SourceAccountId: uuid.NewString(),
		SourceActorId: uuid.NewString(), SourceProfileId: uuid.NewString(), TargetAccountId: uuid.NewString(),
		TargetProfileId: uuid.NewString(), FrozenAuthorityEpoch: 9, FreezeReceiptId: uuid.NewString(),
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	first, err := store.ReserveSdkConversionFence(ctx, req, false, "source-room", now)
	require.NoError(t, err)
	require.False(t, first.Complete)
	require.Equal(t, "source-room", first.Response.GetSourceRoomId())
	require.Len(t, first.Response.GetRequestHash(), 32)

	replayed, err := NewPostgresStore(pool).ReserveSdkConversionFence(ctx, proto.Clone(req).(*callsv1.FenceSdkConversionRequest), false, "", now.Add(time.Second))
	require.NoError(t, err, "pending receipt recovers after a Voice store restart")
	require.Equal(t, first.Response.GetReceiptId(), replayed.Response.GetReceiptId())
	require.Equal(t, "source-room", replayed.SourceRoom)

	changed := proto.Clone(req).(*callsv1.FenceSdkConversionRequest)
	changed.TargetProfileId = uuid.NewString()
	_, err = store.ReserveSdkConversionFence(ctx, changed, false, "source-room", now)
	require.ErrorIs(t, err, ErrConflict, "an operation cannot be replayed with a changed target")

	fencedAt := now.Add(2 * time.Second)
	completed, err := store.CompleteSdkConversionFence(ctx, uuid.MustParse(req.OperationId), fencedAt)
	require.NoError(t, err)
	require.Equal(t, first.Response.GetReceiptId(), completed.GetReceiptId())
	require.Equal(t, fencedAt, completed.GetObservedEjectionAt().AsTime())

	completedReplay, err := NewPostgresStore(pool).ReserveSdkConversionFence(ctx, proto.Clone(req).(*callsv1.FenceSdkConversionRequest), false, "", now)
	require.NoError(t, err)
	require.True(t, completedReplay.Complete)
	require.True(t, proto.Equal(completed, completedReplay.Response), "completed replay returns the immutable owner receipt")
}

func TestPostgresSdkConversionFence_TargetConflictIsDurable(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	store := NewPostgresStore(pool)
	req := &callsv1.FenceSdkConversionRequest{
		OperationId: uuid.NewString(), BindingId: uuid.NewString(), SourceAccountId: uuid.NewString(),
		SourceActorId: uuid.NewString(), SourceProfileId: uuid.NewString(), TargetAccountId: uuid.NewString(),
		TargetProfileId: uuid.NewString(), FrozenAuthorityEpoch: 9, FreezeReceiptId: uuid.NewString(),
	}
	reserved, err := store.ReserveSdkConversionFence(ctx, req, true, "", time.Now().UTC())
	require.NoError(t, err)
	require.False(t, reserved.Complete)
	require.True(t, reserved.Response.GetTargetSessionConflict())
	completed, err := store.CompleteSdkConversionFence(ctx, uuid.MustParse(req.OperationId), time.Now().UTC())
	require.NoError(t, err, "Voice still commits the conflict receipt after completing the source fence attempt")
	require.True(t, completed.GetTargetSessionConflict(), "completed receipt preserves the target conflict")
	replayed, err := NewPostgresStore(pool).ReserveSdkConversionFence(ctx, proto.Clone(req).(*callsv1.FenceSdkConversionRequest), false, "", time.Now().UTC())
	require.NoError(t, err)
	require.True(t, replayed.Complete)
	require.True(t, replayed.Response.GetTargetSessionConflict(), "a later empty target lookup cannot erase a committed conflict")
	require.Equal(t, reserved.Response.GetReceiptId(), replayed.Response.GetReceiptId())
}

func TestPostgresSdkConversionFence_BlocksBothProfilesUntilActivationReceipt(t *testing.T) {
	ctx := context.Background()
	pool := startGameSessionPostgres(t, ctx)
	store := NewPostgresStore(pool)
	operation, binding, sourceProfile, targetProfile := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	req := &callsv1.FenceSdkConversionRequest{OperationId: operation.String(), BindingId: binding.String(),
		SourceAccountId: uuid.NewString(), SourceActorId: uuid.NewString(), SourceProfileId: sourceProfile.String(),
		TargetAccountId: uuid.NewString(), TargetProfileId: targetProfile.String(), FrozenAuthorityEpoch: 11,
		FreezeReceiptId: uuid.NewString()}
	reserved, err := store.ReserveSdkConversionFence(ctx, req, false, "", time.Now().UTC())
	require.NoError(t, err)
	require.False(t, reserved.Complete)
	for _, profile := range []uuid.UUID{sourceProfile, targetProfile} {
		fenced, checkErr := store.IsSdkConversionProfileFenced(ctx, profile.String())
		require.NoError(t, checkErr)
		require.True(t, fenced, "both identities remain denied during pending and fenced states")
	}
	voiceReceipt, err := store.CompleteSdkConversionFence(ctx, operation, time.Now().UTC())
	require.NoError(t, err)
	activation := &callsv1.CompleteSdkConversionActivationRequest{OperationId: operation.String(), BindingId: binding.String(),
		FrozenAuthorityEpoch: 11, FreezeReceiptId: req.GetFreezeReceiptId(), VoiceReceiptId: voiceReceipt.GetReceiptId(),
		ActivationReceiptId: uuid.NewString()}
	activated, err := store.CompleteSdkConversionActivation(ctx, activation, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, "activated", activated.GetState())
	require.Equal(t, activation.GetActivationReceiptId(), activated.GetActivationReceiptId())
	replay, err := store.CompleteSdkConversionActivation(ctx, proto.Clone(activation).(*callsv1.CompleteSdkConversionActivationRequest), time.Now().UTC())
	require.NoError(t, err)
	require.True(t, proto.Equal(activated, replay), "activation retry recovers its immutable receipt")
	for _, profile := range []uuid.UUID{sourceProfile, targetProfile} {
		fenced, checkErr := store.IsSdkConversionProfileFenced(ctx, profile.String())
		require.NoError(t, checkErr)
		require.Equal(t, profile == sourceProfile, fenced, "activation permanently retires the source and releases the target")
	}
}

func rosterRequest(room *callsv1.ProvisionGameSessionRoomResponse, revision uint64, profiles []string, deadline time.Time) *callsv1.ApplyGameSessionRosterRequest {
	return &callsv1.ApplyGameSessionRosterRequest{OperationId: uuid.NewString(), ApplicationId: room.ApplicationId, EnvironmentId: room.EnvironmentId,
		SessionId: room.SessionId, VoiceRoomId: room.RoomId, RosterRevision: revision, ProfileIds: profiles, LeaseExpiresAt: timestamppb.New(deadline)}
}

type recordingRosterMediaFencer struct{ removed []string }

func (f *recordingRosterMediaFencer) FenceManagedGameSessionMembers(_ context.Context, _, _ string, ids []string) error {
	f.removed = append(f.removed, ids...)
	return nil
}

func validProvisionRequest() *callsv1.ProvisionGameSessionRoomRequest {
	return &callsv1.ProvisionGameSessionRoomRequest{
		OperationId: uuid.NewString(), ApplicationId: uuid.NewString(), EnvironmentId: uuid.NewString(), SessionId: uuid.NewString(),
		Resource: &callsv1.GameSessionResourceRef{Kind: callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_MATCH, ExternalResourceKey: "realm:match/opaque-key"},
		ChatId:   uuid.NewString(), ChatCreationOperationId: uuid.NewString(),
	}
}

func startGameSessionPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	migrations := filepath.Join(root, "src", "backend", "migrations", "voice_db")
	pool := integrationtest.StartPostgres(t, ctx, "voice_game_session", filepath.Join(migrations, "000001_room_lifecycle.up.sql"))
	for _, name := range []string{"000002_redis_divergence", "000003_matchmaking_membership", "000004_game_session_rooms", "000005_game_session_close", "000006_game_session_roster_lease", "000007_t17_sdk_conversion_fence", "000008_account_voice_fence"} {
		body, err := os.ReadFile(filepath.Join(migrations, name+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return pool
}
