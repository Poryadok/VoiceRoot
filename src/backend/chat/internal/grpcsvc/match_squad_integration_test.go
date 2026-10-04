package grpcsvc

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
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func newMatchSquadCompactionRequest(matchID, chatID, creationReceiptID, creationRequestHash, teardownOperationID, teardownReceiptID uuid.UUID, manifest, teardownReceiptBytes []byte) *chatv1.CompactMatchSquadChatRequest {
	aggregateCompletedAt := time.Date(2040, 1, 2, 3, 4, 5, 123456000, time.UTC)
	return &chatv1.CompactMatchSquadChatRequest{
		ProtocolVersion: 1, OperationId: uuid.NewString(), TeardownAggregateId: uuid.NewString(),
		MatchId: matchID.String(), ChatId: chatID.String(), CreationReceiptId: creationReceiptID.String(),
		CreationRequestSha256: creationRequestHash, TeardownOperationId: teardownOperationID.String(),
		TeardownReceiptId: teardownReceiptID.String(), TeardownReceiptSha256: sha256BytesForTest(teardownReceiptBytes),
		ParticipantManifestSha256: manifest, AggregateCompletedAt: timestamppb.New(aggregateCompletedAt),
		CompactionAuthorizedAt: timestamppb.New(aggregateCompletedAt.Add(30 * 24 * time.Hour)),
	}
}

func sha256BytesForTest(raw []byte) []byte {
	digest := sha256.Sum256(raw)
	return digest[:]
}

func TestMatchSquadChat_RequiresVerifiedPrincipalBeforeStoreAccess(t *testing.T) {
	service := &MatchSquadChatGRPC{}
	_, err := service.CreateMatchSquadChat(context.Background(), newMatchSquadCreateRequest(uuid.New(), uuid.New(), uuid.New()))
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = service.TeardownMatchSquadChat(context.Background(), &chatv1.TeardownMatchSquadChatRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = service.CompactMatchSquadChat(context.Background(), &chatv1.CompactMatchSquadChatRequest{})
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

func TestMatchSquadChat_AggregateAuthorizedCompactionIsAtomicAndReplayable(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	service := &MatchSquadChatGRPC{Store: &store.MatchSquadStore{Pool: pool}}
	profileID := uuid.New()
	matchID, createOperationID := uuid.New(), uuid.New()
	createRequest := newMatchSquadCreateRequest(matchID, createOperationID, profileID)
	created, err := service.CreateMatchSquadChat(matchSquadContext(), createRequest)
	require.NoError(t, err)
	createRequestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(createRequest)
	require.NoError(t, err)
	createReceiptBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(created.GetReceipt())
	require.NoError(t, err)
	createRequestHash := sha256.Sum256(createRequestBytes)
	chatID, err := uuid.Parse(created.GetReceipt().GetChatId())
	require.NoError(t, err)
	creationReceiptID, err := uuid.Parse(created.GetReceipt().GetReceiptId())
	require.NoError(t, err)
	pendingCommand := newMatchSquadCompactionRequest(matchID, chatID, creationReceiptID, createRequestHash[:], uuid.New(), uuid.New(), created.GetReceipt().GetParticipantManifestSha256(), []byte("no persisted teardown receipt"))
	_, err = service.CompactMatchSquadChat(matchSquadContext(), pendingCommand)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "pending operation evidence cannot be compacted")
	var pendingCreateBytes []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT creation_request_bytes FROM chat_match_squad_operations WHERE match_id=$1`, matchID).Scan(&pendingCreateBytes))
	require.Equal(t, createRequestBytes, pendingCreateBytes)

	teardownOperationID := uuid.New()
	teardownRequest := &chatv1.TeardownMatchSquadChatRequest{ProtocolVersion: 1, TeardownOperationId: teardownOperationID.String(), MatchId: matchID.String(), ChatId: created.GetReceipt().GetChatId(), CreationReceiptId: created.GetReceipt().GetReceiptId(), ParticipantManifestSha256: created.GetReceipt().GetParticipantManifestSha256(), CreationRequestSha256: createRequestHash[:]}
	teardown, err := service.TeardownMatchSquadChat(matchSquadContext(), teardownRequest)
	require.NoError(t, err)
	teardownReceiptBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(teardown.GetReceipt())
	require.NoError(t, err)
	teardownReceiptID, err := uuid.Parse(teardown.GetReceipt().GetReceiptId())
	require.NoError(t, err)
	command := newMatchSquadCompactionRequest(matchID, chatID, creationReceiptID, createRequestHash[:], teardownOperationID, teardownReceiptID, created.GetReceipt().GetParticipantManifestSha256(), teardownReceiptBytes)
	commandBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(command)
	require.NoError(t, err)
	for _, invalidate := range []func(*chatv1.CompactMatchSquadChatRequest){
		func(request *chatv1.CompactMatchSquadChatRequest) { request.ProtocolVersion = 0 },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.OperationId = "" },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.TeardownAggregateId = "" },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.MatchId = "" },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.ChatId = "" },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.CreationReceiptId = "" },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.TeardownOperationId = "" },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.TeardownReceiptId = "" },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.CreationRequestSha256 = nil },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.TeardownReceiptSha256 = nil },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.ParticipantManifestSha256 = nil },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.AggregateCompletedAt = nil },
		func(request *chatv1.CompactMatchSquadChatRequest) { request.CompactionAuthorizedAt = nil },
	} {
		invalid := proto.Clone(command).(*chatv1.CompactMatchSquadChatRequest)
		invalidate(invalid)
		_, invalidErr := service.CompactMatchSquadChat(matchSquadContext(), invalid)
		require.Equal(t, codes.InvalidArgument, status.Code(invalidErr))
	}

	tooEarly := proto.Clone(command).(*chatv1.CompactMatchSquadChatRequest)
	tooEarly.CompactionAuthorizedAt = timestamppb.New(tooEarly.GetAggregateCompletedAt().AsTime().Add(30*24*time.Hour - time.Nanosecond))
	_, err = service.CompactMatchSquadChat(matchSquadContext(), tooEarly)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "database interval must attest complete aggregate retention")

	wrongOwner := proto.Clone(command).(*chatv1.CompactMatchSquadChatRequest)
	wrongOwner.CreationReceiptId = uuid.NewString()
	_, err = service.CompactMatchSquadChat(matchSquadContext(), wrongOwner)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	wrongTeardownDigest := proto.Clone(command).(*chatv1.CompactMatchSquadChatRequest)
	wrongTeardownDigest.TeardownReceiptSha256 = sha256BytesForTest([]byte("wrong receipt"))
	_, err = service.CompactMatchSquadChat(matchSquadContext(), wrongTeardownDigest)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	wrongManifest := proto.Clone(command).(*chatv1.CompactMatchSquadChatRequest)
	wrongManifest.ParticipantManifestSha256 = sha256BytesForTest([]byte("wrong manifest"))
	_, err = service.CompactMatchSquadChat(matchSquadContext(), wrongManifest)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	var storedCreateRequest, storedCreateReceipt, storedTeardownRequest, storedTeardownReceipt []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT creation_request_bytes, creation_receipt_bytes, teardown_request_bytes, teardown_receipt_bytes FROM chat_match_squad_operations WHERE match_id=$1`, matchID).Scan(&storedCreateRequest, &storedCreateReceipt, &storedTeardownRequest, &storedTeardownReceipt))
	require.Equal(t, createRequestBytes, storedCreateRequest, "rejected commands cannot clear evidence")
	require.Equal(t, createReceiptBytes, storedCreateReceipt)
	require.Equal(t, teardownReceiptBytes, storedTeardownReceipt)

	const attempts = 8
	var wait sync.WaitGroup
	receipts := make(chan []byte, attempts)
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response, callErr := service.CompactMatchSquadChat(matchSquadContext(), proto.Clone(command).(*chatv1.CompactMatchSquadChatRequest))
			if callErr != nil {
				errs <- callErr
				return
			}
			encoded, marshalErr := proto.MarshalOptions{Deterministic: true}.Marshal(response.GetReceipt())
			if marshalErr != nil {
				errs <- marshalErr
				return
			}
			receipts <- encoded
		}()
	}
	wait.Wait()
	close(receipts)
	close(errs)
	for callErr := range errs {
		require.NoError(t, callErr)
	}
	var compactReceiptBytes []byte
	for receiptBytes := range receipts {
		if compactReceiptBytes == nil {
			compactReceiptBytes = receiptBytes
		}
		require.Equal(t, compactReceiptBytes, receiptBytes, "racing retry and lost-response recovery preserve exact compact receipt bytes")
	}
	compactReceipt := new(chatv1.MatchSquadChatCompactionReceipt)
	require.NoError(t, proto.Unmarshal(compactReceiptBytes, compactReceipt))
	require.Equal(t, chatv1.MatchSquadChatCompactionStatus_MATCH_SQUAD_CHAT_COMPACTION_STATUS_COMPACTED, compactReceipt.GetStatus())
	require.Equal(t, command.GetOperationId(), compactReceipt.GetCompactionOperationId())
	require.Equal(t, command.GetTeardownAggregateId(), compactReceipt.GetTeardownAggregateId())
	require.True(t, command.GetAggregateCompletedAt().AsTime().Equal(compactReceipt.GetAggregateCompletedAt().AsTime()))
	require.True(t, command.GetCompactionAuthorizedAt().AsTime().Equal(compactReceipt.GetCompactionAuthorizedAt().AsTime()))

	var storedCompactionRequest, storedCompactionReceipt, compactedCreateRequest, compactedCreateReceipt, compactedTeardownRequest, compactedTeardownReceipt []byte
	var storedCompactionOperation, storedAggregateID, storedCompactionReceiptID uuid.UUID
	var compactedAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT compaction_operation_id, teardown_aggregate_id, compaction_receipt_id, compaction_request_bytes, compaction_receipt_bytes, creation_request_bytes, creation_receipt_bytes, teardown_request_bytes, teardown_receipt_bytes, compacted_at FROM chat_match_squad_operations WHERE match_id=$1`, matchID).Scan(&storedCompactionOperation, &storedAggregateID, &storedCompactionReceiptID, &storedCompactionRequest, &storedCompactionReceipt, &compactedCreateRequest, &compactedCreateReceipt, &compactedTeardownRequest, &compactedTeardownReceipt, &compactedAt))
	require.Equal(t, command.GetOperationId(), storedCompactionOperation.String())
	require.Equal(t, command.GetTeardownAggregateId(), storedAggregateID.String())
	require.Equal(t, compactReceipt.GetReceiptId(), storedCompactionReceiptID.String())
	require.Equal(t, commandBytes, storedCompactionRequest, "the small permanent command is retained exactly")
	require.Equal(t, compactReceiptBytes, storedCompactionReceipt, "the compact receipt is retained exactly")
	require.Zero(t, compactedCreateRequest)
	require.Zero(t, compactedCreateReceipt)
	require.Zero(t, compactedTeardownRequest)
	require.Zero(t, compactedTeardownReceipt)
	require.False(t, compactedAt.IsZero())

	_, err = service.CompactMatchSquadChat(matchSquadContext(), proto.Clone(command).(*chatv1.CompactMatchSquadChatRequest))
	require.NoError(t, err, "lost response retry succeeds with the saved compact receipt")
	changed := proto.Clone(command).(*chatv1.CompactMatchSquadChatRequest)
	changed.CompactionAuthorizedAt = timestamppb.New(changed.GetCompactionAuthorizedAt().AsTime().Add(time.Second))
	_, err = service.CompactMatchSquadChat(matchSquadContext(), changed)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "changed bytes cannot replace the permanent compaction command")
	secondCommand := proto.Clone(command).(*chatv1.CompactMatchSquadChatRequest)
	secondCommand.OperationId = uuid.NewString()
	_, err = service.CompactMatchSquadChat(matchSquadContext(), secondCommand)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "a second command cannot issue another terminal receipt")
	_, err = service.CreateMatchSquadChat(matchSquadContext(), createRequest)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "compacted creation evidence cannot fabricate its original receipt")
	_, err = service.TeardownMatchSquadChat(matchSquadContext(), teardownRequest)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "compacted teardown evidence cannot fabricate its original receipt")
	_, err = pool.Exec(ctx, `UPDATE chat_match_squad_operations SET creation_request_bytes=$2 WHERE match_id=$1`, matchID, createRequestBytes)
	require.Error(t, err, "SQL guard must prevent reopening compacted payloads")
	_, err = pool.Exec(ctx, `UPDATE chat_match_squad_operations SET compaction_receipt_bytes=NULL WHERE match_id=$1`, matchID)
	require.Error(t, err, "SQL guard must protect the permanent compaction receipt")
	_, err = pool.Exec(ctx, `DELETE FROM chat_match_squad_operations WHERE match_id=$1`, matchID)
	require.Error(t, err, "SQL guard must prevent deleting terminal fences")
	_, err = pool.Exec(ctx, `DELETE FROM chats WHERE id=$1`, chatID)
	require.Error(t, err, "ownership trigger must protect durable history after compaction")
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chats WHERE id=$1", chatID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM chat_members WHERE chat_id=$1", chatID).Scan(&count))
	require.Zero(t, count)
	downMigration, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "chat_db", "000021_match_squad_operations.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(downMigration))
	require.Error(t, err, "DOWN must lock and refuse to erase pending or terminal evidence")
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
