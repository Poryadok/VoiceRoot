package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/voice/internal/gameprovision"
	voicestore "voice/backend/voice/internal/store"
)

func TestProvisionedManagedGameSessionCanBeAdmittedAfterCallStoreMiss(t *testing.T) {
	ctx := context.Background()
	pool := startManagedGameSessionAdmissionPostgres(t, ctx)
	rooms := gameprovision.NewPostgresStore(pool)
	require.NoError(t, rooms.CheckSchema(ctx))

	chatID := uuid.NewString()
	request := &callsv1.ProvisionGameSessionRoomRequest{
		OperationId:   uuid.NewString(),
		ApplicationId: uuid.NewString(),
		EnvironmentId: uuid.NewString(),
		SessionId:     uuid.NewString(),
		Resource: &callsv1.GameSessionResourceRef{
			Kind:                callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_MATCH,
			ExternalResourceKey: "realm:match/admission-test",
		},
		ChatId:                  chatID,
		ChatCreationOperationId: uuid.NewString(),
	}
	provisioned, err := rooms.Provision(ctx, request)
	require.NoError(t, err)

	now := time.Now().UTC()
	newService := func() *VoiceGRPC {
		service := newTestVoiceService(now, &recordingEvents{})
		service.Calls = voicestore.NewMemoryCallStore()
		service.ManagedGameSessionRooms = rooms
		service.setManagedGameSessionGrantChecker(&managedGameSessionGrantStub{})
		service.ChatMembers = &mapChatMembers{members: map[string]map[string]bool{
			chatID: {"profile-member": true},
		}}
		return service
	}

	t.Run("JoinCall recovers room into an empty CallStore", func(t *testing.T) {
		service := newService()
		joined, err := service.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: provisioned.RoomId})
		require.NoError(t, err)
		require.Equal(t, provisioned.RoomId, joined.GetCallSession().GetRoomId())
		require.Equal(t, chatID, joined.GetCallSession().GetLinkedChat().GetId())
		require.True(t, joined.GetCallSession().GetInitiatorProfileId() == "", "GIS provisioning must not invent a room owner")
	})

	t.Run("token uses current Chat membership", func(t *testing.T) {
		service := newService()
		memberToken, err := service.GetJoinToken(voiceTestCtx("profile-member"), &callsv1.GetJoinTokenRequest{RoomId: provisioned.RoomId})
		require.NoError(t, err)
		require.NotEmpty(t, memberToken.GetJwt())

		_, err = service.GetJoinToken(voiceTestCtx("profile-outsider"), &callsv1.GetJoinTokenRequest{RoomId: provisioned.RoomId})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("Role grant is required in addition to Chat membership", func(t *testing.T) {
		service := newService()
		denied := &managedGameSessionGrantStub{err: status.Error(codes.PermissionDenied, "managed session grant missing")}
		installManagedGameSessionGrantChecker(t, service, denied)

		_, err := service.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: provisioned.RoomId})
		require.Equal(t, codes.PermissionDenied, status.Code(err), "Chat membership alone must not admit a game-session member")

		denied.err = nil
		joined, err := service.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: provisioned.RoomId})
		require.NoError(t, err, "a current Role grant plus Chat membership should admit")
		require.Equal(t, provisioned.RoomId, joined.GetCallSession().GetRoomId())
	})

	t.Run("missing Role checker fails closed", func(t *testing.T) {
		service := newService()
		service.ManagedGameSessionGrants = nil
		_, err := service.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: provisioned.RoomId})
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("revoked grant fences warm CallStore admission and token issuance", func(t *testing.T) {
		service := newService()
		grant := &managedGameSessionGrantStub{}
		installManagedGameSessionGrantChecker(t, service, grant)
		_, err := service.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: provisioned.RoomId})
		require.NoError(t, err, "initial grant should warm the CallStore")

		grant.err = status.Error(codes.PermissionDenied, "managed session grant revoked")
		_, err = service.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: provisioned.RoomId})
		require.Equal(t, codes.PermissionDenied, status.Code(err), "warm CallStore must not bypass grant revocation")
		_, err = service.GetJoinToken(voiceTestCtx("profile-member"), &callsv1.GetJoinTokenRequest{RoomId: provisioned.RoomId})
		require.Equal(t, codes.PermissionDenied, status.Code(err), "revoked grant must not issue a token")
	})

	t.Run("closed owner receipt fences warm CallStore admission and token issuance", func(t *testing.T) {
		service := newService()
		grant := &managedGameSessionGrantStub{}
		installManagedGameSessionGrantChecker(t, service, grant)
		_, err := service.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: provisioned.RoomId})
		require.NoError(t, err, "initial grant should warm the CallStore")

		closed := &closedManagedGameSessionRoomLookup{delegate: rooms, closed: true}
		service.ManagedGameSessionRooms = closed // represents the durable close receipt
		_, err = service.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: provisioned.RoomId})
		require.Error(t, err, "warm CallStore must not bypass a durable Voice close")
		_, err = service.GetJoinToken(voiceTestCtx("profile-member"), &callsv1.GetJoinTokenRequest{RoomId: provisioned.RoomId})
		require.Error(t, err, "a durable Voice close must not issue a token from a warm CallStore entry")
	})

	t.Run("closed owner record denies cold admission", func(t *testing.T) {
		closed := &closedManagedGameSessionRoomLookup{delegate: rooms}
		service := newService()
		service.ManagedGameSessionRooms = closed
		closed.closed = true // represents a durable CloseGameSessionRoom receipt

		_, err := service.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: provisioned.RoomId})
		require.Error(t, err, "closed durable room must not project into an empty CallStore")
		_, err = service.GetJoinToken(voiceTestCtx("profile-member"), &callsv1.GetJoinTokenRequest{RoomId: provisioned.RoomId})
		require.Error(t, err, "closed durable room must not issue a token after a cold lookup")
	})

	t.Run("missing verified profile is rejected before provisioning lookup", func(t *testing.T) {
		lookup := &countingManagedGameSessionRoomLookup{delegate: rooms}
		service := newService()
		service.ManagedGameSessionRooms = lookup

		_, err := service.JoinCall(context.Background(), &callsv1.JoinCallRequest{RoomId: provisioned.RoomId})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Zero(t, lookup.calls)
	})
}

func TestProvisionedManagedGameSessionCloseResumesDurableClosingAfterStoreRestart(t *testing.T) {
	ctx := context.Background()
	pool := startManagedGameSessionAdmissionPostgres(t, ctx)
	request := &callsv1.ProvisionGameSessionRoomRequest{
		OperationId: uuid.NewString(), ApplicationId: uuid.NewString(), EnvironmentId: uuid.NewString(), SessionId: uuid.NewString(),
		Resource: &callsv1.GameSessionResourceRef{Kind: callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_MATCH, ExternalResourceKey: "realm:match/close-recovery"},
		ChatId:   uuid.NewString(), ChatCreationOperationId: uuid.NewString(),
	}
	firstStore := gameprovision.NewPostgresStore(pool)
	provisioned, err := firstStore.Provision(ctx, request)
	require.NoError(t, err)
	closeRequest := &callsv1.CloseGameSessionRoomRequest{
		OperationId: uuid.NewString(), ApplicationId: request.ApplicationId, EnvironmentId: request.EnvironmentId,
		SessionId: request.SessionId, Resource: request.Resource, ChatId: request.ChatId,
		ChatCreationOperationId: request.ChatCreationOperationId,
	}
	firstFence := &failingManagedGameSessionMediaFencer{err: status.Error(codes.Unavailable, "media controller restart")}
	_, err = firstStore.CloseGameSessionRoom(ctx, closeRequest, firstFence)
	require.Error(t, err)
	require.Equal(t, 1, firstFence.calls)
	var closingStatus string
	var closingAt time.Time
	err = pool.QueryRow(ctx, `SELECT status, closing_at FROM voice_game_session_closures WHERE operation_id = $1`, closeRequest.OperationId).Scan(&closingStatus, &closingAt)
	require.NoError(t, err)
	require.Equal(t, "CLOSING", closingStatus, "a failed media fence must leave the durable close retryable")
	_, err = firstStore.GetRoom(ctx, provisioned.RoomId)
	require.ErrorIs(t, err, gameprovision.ErrNotFound, "CLOSING must fence admission before media is stopped")

	retryAt := closingAt.Add(5*time.Second + time.Millisecond)
	if delay := time.Until(retryAt); delay > 0 {
		time.Sleep(delay)
	}
	secondFence := &recordingManagedGameSessionMediaFencer{active: true}
	replayed, err := gameprovision.NewPostgresStore(pool).CloseGameSessionRoom(ctx, closeRequest, secondFence)
	require.NoError(t, err, "exact retry must resume a durable CLOSING operation after the original fencer failed")
	require.Equal(t, "CLOSED", replayed.GetStatus())
	require.Equal(t, 1, secondFence.calls)
	require.False(t, secondFence.active)
	require.Equal(t, closeRequest.OperationId, replayed.GetOperationId())
}

func TestProvisionedManagedGameSessionClosePersistsReceiptAndFencesActiveMedia(t *testing.T) {
	ctx := context.Background()
	pool := startManagedGameSessionAdmissionPostgres(t, ctx)
	rooms := gameprovision.NewPostgresStore(pool)
	require.NoError(t, rooms.CheckSchema(ctx))
	request := &callsv1.ProvisionGameSessionRoomRequest{
		OperationId: uuid.NewString(), ApplicationId: uuid.NewString(), EnvironmentId: uuid.NewString(),
		SessionId: uuid.NewString(),
		Resource:  &callsv1.GameSessionResourceRef{Kind: callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_MATCH, ExternalResourceKey: "realm:match/close-test"},
		ChatId:    uuid.NewString(), ChatCreationOperationId: uuid.NewString(),
	}
	provisioned, err := rooms.Provision(ctx, request)
	require.NoError(t, err)
	fencer := &recordingManagedGameSessionMediaFencer{active: true}
	closeRequest := &callsv1.CloseGameSessionRoomRequest{
		OperationId: uuid.NewString(), ApplicationId: request.ApplicationId, EnvironmentId: request.EnvironmentId,
		SessionId: request.SessionId,
		Resource:  request.Resource, ChatId: request.ChatId, ChatCreationOperationId: request.ChatCreationOperationId,
	}

	receipt, err := rooms.CloseGameSessionRoom(ctx, closeRequest, fencer)
	require.NoError(t, err)
	require.NotEmpty(t, receipt.GetCloseReceiptId())
	require.NotEmpty(t, receipt.GetRequestHash())
	require.Equal(t, provisioned.GetRoomId(), receipt.GetRoomId())
	require.Equal(t, "CLOSED", receipt.GetStatus())
	require.False(t, fencer.active, "close receipt cannot precede fencing active media")
	require.Equal(t, 1, fencer.calls)
	require.False(t, receipt.GetClosedAt().AsTime().Before(receipt.GetMediaFencedAt().AsTime()))
	require.False(t, fencer.startedAt.IsZero(), "recording fencer must capture the start of its successful attempt")
	require.False(t, receipt.GetMediaFencedAt().AsTime().Before(fencer.startedAt))
	require.LessOrEqual(t, receipt.GetMediaFencedAt().AsTime().Sub(fencer.startedAt), 5*time.Second,
		"the five second bound applies to one media-fence attempt")
	_, err = rooms.GetRoom(ctx, provisioned.GetRoomId())
	require.ErrorIs(t, err, gameprovision.ErrNotFound, "closed room must no longer be projected as joinable")

	replayed, err := rooms.CloseGameSessionRoom(ctx, closeRequest, fencer)
	require.NoError(t, err)
	require.Equal(t, receipt.GetCloseReceiptId(), replayed.GetCloseReceiptId())
	require.Equal(t, 1, fencer.calls, "exact retry must reuse the durable close receipt")
}

type managedGameSessionGrantChecker interface {
	CheckGameSessionGrant(context.Context, string, string, string, string, string) error
}

type managedGameSessionGrantStub struct{ err error }

func (stub *managedGameSessionGrantStub) CheckGameSessionGrant(context.Context, string, string, string, string, string) error {
	return stub.err
}

func installManagedGameSessionGrantChecker(t *testing.T, service *VoiceGRPC, checker managedGameSessionGrantChecker) {
	t.Helper()
	service.setManagedGameSessionGrantChecker(checker)
}

type recordingManagedGameSessionMediaFencer struct {
	active    bool
	calls     int
	startedAt time.Time
}

type failingManagedGameSessionMediaFencer struct {
	err   error
	calls int
}

func (fencer *failingManagedGameSessionMediaFencer) FenceManagedGameSession(context.Context, string, string) error {
	fencer.calls++
	return fencer.err
}

func (fencer *recordingManagedGameSessionMediaFencer) FenceManagedGameSession(context.Context, string, string) error {
	fencer.calls++
	fencer.startedAt = time.Now().UTC()
	fencer.active = false
	return nil
}

type closedManagedGameSessionRoomLookup struct {
	delegate grpcManagedGameSessionRoomLookup
	closed   bool
}

func (lookup *closedManagedGameSessionRoomLookup) GetRoom(ctx context.Context, roomID string) (gameprovision.Room, error) {
	if lookup.closed {
		return gameprovision.Room{}, gameprovision.ErrNotFound
	}
	return lookup.delegate.GetRoom(ctx, roomID)
}

type countingManagedGameSessionRoomLookup struct {
	delegate grpcManagedGameSessionRoomLookup
	calls    int
}

func (lookup *countingManagedGameSessionRoomLookup) GetRoom(ctx context.Context, roomID string) (gameprovision.Room, error) {
	lookup.calls++
	return lookup.delegate.GetRoom(ctx, roomID)
}

type grpcManagedGameSessionRoomLookup interface {
	GetRoom(context.Context, string) (gameprovision.Room, error)
}

func startManagedGameSessionAdmissionPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	migrations := filepath.Join(root, "src", "backend", "migrations", "voice_db")
	pool := integrationtest.StartPostgres(t, ctx, "voice_managed_admission", filepath.Join(migrations, "000001_room_lifecycle.up.sql"))
	for _, name := range []string{"000002_redis_divergence", "000003_matchmaking_membership", "000004_game_session_rooms", "000005_game_session_close"} {
		body, err := os.ReadFile(filepath.Join(migrations, name+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return pool
}
