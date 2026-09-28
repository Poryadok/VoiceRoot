package gameprovision

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

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
	for _, name := range []string{"000002_redis_divergence", "000003_matchmaking_membership", "000004_game_session_rooms", "000005_game_session_close"} {
		body, err := os.ReadFile(filepath.Join(migrations, name+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return pool
}
