package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
)

func matchSquadContext() context.Context {
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "matchmaking", Subject: "service:matchmaking", Audience: "chat"})
}

func newMatchSquadCreateRequest(matchID, operationID uuid.UUID, participants ...uuid.UUID) *chatv1.CreateMatchSquadChatRequest {
	request := &chatv1.CreateMatchSquadChatRequest{ProtocolVersion: 1, OperationId: operationID.String(), MatchId: matchID.String()}
	for _, profileID := range participants {
		request.Participants = append(request.Participants, &chatv1.MatchSquadParticipant{ProfileId: profileID.String()})
	}
	request.ParticipantManifestSha256 = participantManifestHash(participants)
	return request
}

func TestMatchSquadChat_RequiresVerifiedPrincipalBeforeStoreAccess(t *testing.T) {
	service := &MatchSquadChatGRPC{}
	_, err := service.CreateMatchSquadChat(context.Background(), newMatchSquadCreateRequest(uuid.New(), uuid.New(), uuid.New()))
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = service.TeardownMatchSquadChat(context.Background(), &chatv1.TeardownMatchSquadChatRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestMatchSquadChat_IdempotentCreateAndOwnedTeardown(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	service := &MatchSquadChatGRPC{Store: &store.MatchSquadStore{Pool: pool}}
	participants := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	// Sort by UUID bytes to exercise the wire's canonical participant order.
	for i := 1; i < len(participants); i++ {
		for j := i; j > 0 && bytes.Compare(participants[j][:], participants[j-1][:]) < 0; j-- {
			participants[j], participants[j-1] = participants[j-1], participants[j]
		}
	}
	matchID, operationID := uuid.New(), uuid.New()
	request := newMatchSquadCreateRequest(matchID, operationID, participants...)
	first, err := service.CreateMatchSquadChat(matchSquadContext(), request)
	require.NoError(t, err)
	firstBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(first.GetReceipt())
	require.NoError(t, err)
	replayed, err := service.CreateMatchSquadChat(matchSquadContext(), proto.Clone(request).(*chatv1.CreateMatchSquadChatRequest))
	require.NoError(t, err)
	replayedBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(replayed.GetReceipt())
	require.NoError(t, err)
	require.Equal(t, firstBytes, replayedBytes)

	changed := proto.Clone(request).(*chatv1.CreateMatchSquadChatRequest)
	changed.Participants[0].ProfileId = uuid.NewString()
	_, err = service.CreateMatchSquadChat(matchSquadContext(), changed)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats WHERE id=$1", first.GetReceipt().GetChatId()).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_members WHERE chat_id=$1", first.GetReceipt().GetChatId()).Scan(&count))
	require.Equal(t, len(participants), count)

	teardownID := uuid.New()
	teardownRequest := &chatv1.TeardownMatchSquadChatRequest{ProtocolVersion: 1, TeardownOperationId: teardownID.String(), MatchId: matchID.String(), ChatId: first.GetReceipt().GetChatId(), CreationReceiptId: first.GetReceipt().GetReceiptId(), ParticipantManifestSha256: first.GetReceipt().GetParticipantManifestSha256(), CreationRequestSha256: first.GetReceipt().GetRequestSha256()}
	teardown, err := service.TeardownMatchSquadChat(matchSquadContext(), teardownRequest)
	require.NoError(t, err)
	require.Equal(t, chatv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, teardown.GetReceipt().GetStatus())
	teardownBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(teardown.GetReceipt())
	require.NoError(t, err)
	teardownReplay, err := service.TeardownMatchSquadChat(matchSquadContext(), proto.Clone(teardownRequest).(*chatv1.TeardownMatchSquadChatRequest))
	require.NoError(t, err)
	teardownReplayBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(teardownReplay.GetReceipt())
	require.NoError(t, err)
	require.Equal(t, teardownBytes, teardownReplayBytes)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats WHERE id=$1", first.GetReceipt().GetChatId()).Scan(&count))
	require.Equal(t, 1, count, "history chat remains durable after ownership teardown")
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_members WHERE chat_id=$1", first.GetReceipt().GetChatId()).Scan(&count))
	require.Zero(t, count, "teardown revokes every membership")
	_, err = service.CreateMatchSquadChat(matchSquadContext(), request)
	require.NoError(t, err, "a creation receipt replay returns historical evidence")
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_members WHERE chat_id=$1", first.GetReceipt().GetChatId()).Scan(&count))
	require.Zero(t, count, "creation replay cannot restore terminal membership")
	_, err = service.TeardownMatchSquadChat(matchSquadContext(), &chatv1.TeardownMatchSquadChatRequest{ProtocolVersion: 1, TeardownOperationId: uuid.NewString(), MatchId: matchID.String(), ChatId: first.GetReceipt().GetChatId(), CreationReceiptId: first.GetReceipt().GetReceiptId(), ParticipantManifestSha256: first.GetReceipt().GetParticipantManifestSha256(), CreationRequestSha256: first.GetReceipt().GetRequestSha256()})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "a second teardown operation cannot take ownership")
	_, err = pool.Exec(ctx, `DELETE FROM chats WHERE id=$1`, first.GetReceipt().GetChatId())
	require.Error(t, err, "permanent ownership fence protects the durable chat row")
	var storedCreateRequest, storedCreateReceipt, storedTeardownRequest, storedTeardownReceipt []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT creation_request_bytes, creation_receipt_bytes, teardown_request_bytes, teardown_receipt_bytes FROM chat_match_squad_operations WHERE match_id=$1`, matchID).Scan(&storedCreateRequest, &storedCreateReceipt, &storedTeardownRequest, &storedTeardownReceipt))
	require.NotEmpty(t, storedCreateRequest)
	require.Equal(t, firstBytes, storedCreateReceipt)
	require.NotEmpty(t, storedTeardownRequest)
	require.Equal(t, teardownBytes, storedTeardownReceipt)
	downMigration, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "chat_db", "000021_match_squad_operations.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(downMigration))
	require.Error(t, err, "DOWN must refuse to erase replay evidence or the terminal fence")
}

func TestMatchSquadChat_InvalidManifestHasNoSideEffects(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	service := &MatchSquadChatGRPC{Store: &store.MatchSquadStore{Pool: pool}}
	request := newMatchSquadCreateRequest(uuid.New(), uuid.New(), uuid.New())
	wrongManifest := sha256.Sum256([]byte("wrong"))
	request.ParticipantManifestSha256 = wrongManifest[:]
	_, err := service.CreateMatchSquadChat(matchSquadContext(), request)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats").Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_match_squad_operations").Scan(&count))
	require.Zero(t, count)
}

func TestMatchSquadChat_ConcurrentSameOperationReturnsOneReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	service := &MatchSquadChatGRPC{Store: &store.MatchSquadStore{Pool: pool}}
	request := newMatchSquadCreateRequest(uuid.New(), uuid.New(), uuid.New())
	const attempts = 8
	var wait sync.WaitGroup
	receipts := make(chan []byte, attempts)
	errs := make(chan error, attempts)
	for range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response, err := service.CreateMatchSquadChat(matchSquadContext(), proto.Clone(request).(*chatv1.CreateMatchSquadChatRequest))
			if err != nil {
				errs <- err
				return
			}
			encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(response.GetReceipt())
			if err != nil {
				errs <- err
				return
			}
			receipts <- encoded
		}()
	}
	wait.Wait()
	close(receipts)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var expected []byte
	for receipt := range receipts {
		if expected == nil {
			expected = receipt
		}
		require.Equal(t, expected, receipt)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_match_squad_operations").Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats").Scan(&count))
	require.Equal(t, 1, count)
}
