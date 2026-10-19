package matchsquad

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/matchsquadprincipal"
	"voice/backend/voice/internal/store"
)

type settlementRoomEffects struct {
	RoomEffects
	ensures atomic.Int64
}

func (e *settlementRoomEffects) EnsureRoom(ctx context.Context, room string) error {
	e.ensures.Add(1)
	return e.RoomEffects.EnsureRoom(ctx, room)
}

// Real SQL ownership/receipt fences are exercised. Projection and media effects
// are faithful in-process adapters, not a proof of physical LiveKit absence.
func TestVoiceDelayedCreateAndConcurrentReplayCannotResurrectAfterSettlement(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := startMatchSquadPostgres(t, ctx) // Native existing helper applies1..11.
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	paths, err := filepath.Glob(filepath.Join(root, "src/backend/migrations/voice_db/*.up.sql"))
	require.NoError(t, err)
	for _, path := range paths {
		version, err := strconv.Atoi(filepath.Base(path)[:6])
		require.NoError(t, err)
		if version <= 11 {
			continue
		}
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, filepath.Base(path))
	}
	calls := store.NewMemoryCallStore()
	baseEffects := &testMatchSquadEffects{calls: calls}
	effects := &settlementRoomEffects{RoomEffects: baseEffects}
	service := &Service{Pool: pool, Calls: calls, Effects: effects}
	f := &matchSquadPostgresFixture{t: t, ctx: ctx, pool: pool, calls: calls, effects: baseEffects, service: service}
	service.Members = f.memberService(map[uuid.UUID]uuid.UUID{})
	request := validCreateRequest()
	deadline, _ := ctx.Deadline()
	createCtx, cancelCreate := context.WithDeadline(verifiedServiceContext(t, request, matchsquadprincipal.CreateMethod, request.OperationId), deadline)
	defer cancelCreate()
	type result struct {
		receipt []byte
		err     error
	}
	delayed := make(chan result, 1)
	release, entered := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	go func() {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			delayed <- result{err: ctx.Err()}
			return
		}
		receipt, err := service.Create(createCtx, proto.Clone(request).(*callsv1.CreateMatchSquadRoomRequest))
		delayed <- result{receipt, err}
	}()
	<-entered
	creationBytes, err := service.Create(createCtx, request)
	require.NoError(t, err)
	creation := new(callsv1.MatchSquadRoomReceipt)
	require.NoError(t, proto.Unmarshal(creationBytes, creation))
	results := make(chan result, 6)
	start := make(chan struct{})
	for range 6 {
		go func() {
			<-start
			receipt, err := service.Create(createCtx, proto.Clone(request).(*callsv1.CreateMatchSquadRoomRequest))
			results <- result{receipt, err}
		}()
	}
	teardown := &callsv1.TeardownMatchSquadRoomRequest{ProtocolVersion: 1, TeardownOperationId: uuid.NewString(), MatchId: request.MatchId, RoomId: creation.RoomId,
		CreationReceiptId: creation.ReceiptId, ParticipantManifestSha256: creation.ParticipantManifestSha256, CreationRequestSha256: creation.RequestSha256}
	teardownCtx, cancelTeardown := context.WithDeadline(verifiedServiceContext(t, teardown, matchsquadprincipal.TeardownMethod, teardown.TeardownOperationId), deadline)
	defer cancelTeardown()
	close(start)
	teardownBytes, err := service.Teardown(teardownCtx, teardown)
	require.NoError(t, err)
	for range 6 {
		select {
		case got := <-results:
			require.NoError(t, got.err)
			require.Equal(t, creationBytes, got.receipt)
		case <-ctx.Done():
			t.Fatal("concurrent Create did not settle")
		}
	}
	ensuresAfterClose := effects.ensures.Load()
	unblock()
	select {
	case got := <-delayed:
		require.NoError(t, got.err)
		require.Equal(t, creationBytes, got.receipt)
	case <-ctx.Done():
		t.Fatal("delayed original did not return retained creation evidence")
	}
	require.Equal(t, ensuresAfterClose, effects.ensures.Load(), "closed Create replay must not call media EnsureRoom")
	call, err := calls.GetCall(ctx, creation.RoomId)
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ENDED, call.Status)
	require.Empty(t, call.States)
	var roomState, operationState string
	require.NoError(t, pool.QueryRow(ctx, `SELECT r.state,o.state FROM voice_room_instances r JOIN voice_match_squad_operations o USING(room_id) WHERE r.room_id=$1`, uuid.MustParse(creation.RoomId)).Scan(&roomState, &operationState))
	require.Equal(t, "closed", roomState)
	require.Equal(t, "closed", operationState)
	closed := new(callsv1.MatchSquadRoomTeardownReceipt)
	require.NoError(t, proto.Unmarshal(teardownBytes, closed))
	teardownHash := sha256.Sum256(teardownBytes)
	completedAt := time.Date(2040, 1, 2, 3, 4, 5, 0, time.UTC) // Protected synthetic aggregate attestation, not wall-clock retention proof.
	compact := &callsv1.CompactMatchSquadRoomRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), TeardownAggregateId: uuid.NewString(), MatchId: request.MatchId, RoomId: creation.RoomId,
		CreationReceiptId: creation.ReceiptId, CreationRequestSha256: creation.RequestSha256, TeardownOperationId: teardown.TeardownOperationId, TeardownReceiptId: closed.ReceiptId,
		TeardownReceiptSha256: teardownHash[:], ParticipantManifestSha256: creation.ParticipantManifestSha256, AggregateCompletedAt: timestamppb.New(completedAt), CompactionAuthorizedAt: timestamppb.New(completedAt.Add(30 * 24 * time.Hour))}
	compactCtx, cancelCompact := context.WithDeadline(verifiedServiceContext(t, compact, matchsquadprincipal.CompactMethod, compact.OperationId), deadline)
	defer cancelCompact()
	_, err = service.Compact(compactCtx, compact)
	require.NoError(t, err)
	for range 6 {
		go func() {
			receipt, err := service.Create(createCtx, proto.Clone(request).(*callsv1.CreateMatchSquadRoomRequest))
			results <- result{receipt, err}
		}()
	}
	for range 6 {
		select {
		case got := <-results:
			require.Equal(t, codes.FailedPrecondition, status.Code(got.err), "tombstone must not fabricate a creation/absence receipt")
			require.Empty(t, got.receipt)
		case <-ctx.Done():
			t.Fatal("compacted Create did not settle")
		}
	}
	require.Equal(t, ensuresAfterClose, effects.ensures.Load())
	call, err = calls.GetCall(ctx, creation.RoomId)
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ENDED, call.Status)
	require.Empty(t, call.States)
}
