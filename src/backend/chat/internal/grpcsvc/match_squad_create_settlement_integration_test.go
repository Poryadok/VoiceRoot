package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/store"
)

// This is the provider prerequisite of BE012c9, not an MM compensation receipt.
// A channel represents the original request delayed in transport; every actual
// arrival below executes the production handler/store against real PostgreSQL.
func TestChatDelayedCreateAndConcurrentReplayCannotResurrectAfterSettlement(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(matchSquadContext(), 45*time.Second)
	defer cancel()
	pool := startChatPostgresForTest(t, ctx)
	paths, err := filepath.Glob(filepath.Join(repoRoot(t), "src/backend/migrations/chat_db/*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, filepath.Base(path))
	}
	service := &MatchSquadChatGRPC{Store: &store.MatchSquadStore{Pool: pool}}
	matchID, operationID := uuid.New(), uuid.New()
	request := newMatchSquadCreateRequest(matchID, operationID, uuid.New())
	type result struct {
		response *chatv1.CreateMatchSquadChatResponse
		err      error
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
		response, err := service.CreateMatchSquadChat(ctx, proto.Clone(request).(*chatv1.CreateMatchSquadChatRequest))
		delayed <- result{response, err}
	}()
	<-entered
	created, err := service.CreateMatchSquadChat(ctx, request)
	require.NoError(t, err)
	creationBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(created.GetReceipt())
	require.NoError(t, err)
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	require.NoError(t, err)
	receipts := make(chan result, 6)
	start := make(chan struct{})
	for range 6 {
		go func() {
			<-start
			response, err := service.CreateMatchSquadChat(ctx, proto.Clone(request).(*chatv1.CreateMatchSquadChatRequest))
			receipts <- result{response, err}
		}()
	}
	close(start)
	teardown := &chatv1.TeardownMatchSquadChatRequest{ProtocolVersion: 1, TeardownOperationId: uuid.NewString(), MatchId: matchID.String(), ChatId: created.GetReceipt().GetChatId(), CreationReceiptId: created.GetReceipt().GetReceiptId(),
		ParticipantManifestSha256: created.GetReceipt().GetParticipantManifestSha256(), CreationRequestSha256: sha256BytesForTest(requestBytes)}
	closed, err := service.TeardownMatchSquadChat(ctx, teardown)
	require.NoError(t, err)
	for range 6 {
		var got result
		select {
		case got = <-receipts:
		case <-ctx.Done():
			t.Fatal("concurrent Create did not settle")
		}
		require.NoError(t, got.err)
		bytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(got.response.GetReceipt())
		require.NoError(t, err)
		require.Equal(t, creationBytes, bytes)
	}
	unblock()
	select {
	case got := <-delayed:
		require.NoError(t, got.err)
		bytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(got.response.GetReceipt())
		require.NoError(t, err)
		require.Equal(t, creationBytes, bytes)
	case <-ctx.Done():
		t.Fatal("delayed original did not return retained creation evidence")
	}
	var members, operations int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_members WHERE chat_id=$1`, created.GetReceipt().GetChatId()).Scan(&members))
	require.Zero(t, members)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_match_squad_operations WHERE match_id=$1`, matchID).Scan(&operations))
	require.Equal(t, 1, operations)
	teardownBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(closed.GetReceipt())
	require.NoError(t, err)
	command := newMatchSquadCompactionRequest(matchID, uuid.MustParse(created.GetReceipt().GetChatId()), uuid.MustParse(created.GetReceipt().GetReceiptId()), sha256BytesForTest(requestBytes), uuid.MustParse(teardown.TeardownOperationId), uuid.MustParse(closed.GetReceipt().GetReceiptId()), request.ParticipantManifestSha256, teardownBytes)
	_, err = service.CompactMatchSquadChat(ctx, command)
	require.NoError(t, err)
	for range 6 {
		go func() {
			response, err := service.CreateMatchSquadChat(ctx, proto.Clone(request).(*chatv1.CreateMatchSquadChatRequest))
			receipts <- result{response, err}
		}()
	}
	for range 6 {
		select {
		case got := <-receipts:
			require.Equal(t, codes.FailedPrecondition, status.Code(got.err), "compaction tombstone is not a creation/absence receipt")
			require.Nil(t, got.response)
		case <-ctx.Done():
			t.Fatal("compacted Create replay did not settle")
		}
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM chat_members WHERE chat_id=$1`, created.GetReceipt().GetChatId()).Scan(&members))
	require.Zero(t, members)
}
