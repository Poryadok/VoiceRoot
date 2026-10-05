package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/integrationtest"
)

func TestMatchSquadCompactionIntentRequiresCompletedAggregateAndExactDueTime(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	matchID, _, _ := seedActiveDuoMatch(t, ctx, pool)
	operationID, chatOpID, voiceOpID := uuid.New(), uuid.New(), uuid.New()
	manifestHash := make([]byte, 32)
	_, err := pool.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_operations
		(match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes,
		 state, chat_operation_id, chat_request_sha256, chat_request_bytes,
		 voice_operation_id)
		VALUES ($1,$2,$3,$4,'provisioning',$5,$6,$7,$8)
	`, matchID, operationID, manifestHash, []byte("manifest"), chatOpID, manifestHash, []byte("chat request"), voiceOpID)
	require.NoError(t, err)

	aggregateID := uuid.New()
	aggregateCompletedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = pool.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_teardowns
		(aggregate_id, match_id, state, chat_teardown_operation_id,
		 chat_teardown_request_sha256, chat_teardown_request_bytes,
		 chat_teardown_receipt_id, chat_teardown_receipt_bytes,
		 voice_teardown_operation_id, voice_teardown_request_sha256,
		 voice_teardown_request_bytes, voice_teardown_receipt_id,
		 voice_teardown_receipt_bytes, aggregate_completed_at)
		VALUES ($1,$2,'complete',$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, aggregateID, matchID, uuid.New(), manifestHash, []byte("chat teardown"), uuid.New(), []byte("chat receipt"),
		uuid.New(), manifestHash, []byte("voice teardown"), uuid.New(), []byte("voice receipt"), aggregateCompletedAt)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_compaction_intents (aggregate_id, provider, operation_id, not_before)
		VALUES ($1,'chat',$2,$3)
	`, aggregateID, uuid.New(), aggregateCompletedAt.Add(30*24*time.Hour-time.Second))
	require.Error(t, err, "provider intent cannot use an early or provider-local clock")

	_, err = pool.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_compaction_intents (aggregate_id, provider, operation_id, not_before)
		VALUES ($1,'chat',$2,$3)
	`, aggregateID, uuid.New(), aggregateCompletedAt.Add(30*24*time.Hour))
	require.NoError(t, err)
}

func TestMatchSquadLifecycleDownRefusesToDropProvisioningEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	matchID, _, _ := seedActiveDuoMatch(t, ctx, pool)

	hash := make([]byte, 32)
	_, err := pool.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_operations
		(match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes,
		 state, chat_operation_id, chat_request_sha256, chat_request_bytes,
		 voice_operation_id)
		VALUES ($1,$2,$3,$4,'provisioning',$5,$6,$7,$8)
	`, matchID, uuid.New(), hash, []byte("manifest"), uuid.New(), hash, []byte("request"), uuid.New())
	require.NoError(t, err)

	root := repoRoot(t)
	down, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "matchmaking_db", "000016_match_squad_lifecycle.down.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "rollback must preserve admitted provisioning IDs and receipts")
	var operationCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_operations WHERE match_id=$1`, matchID).Scan(&operationCount))
	require.Equal(t, 1, operationCount)
}

func TestMatchSquadLifecycleDownRunnerPreservesEvidenceAndDirtyMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL and pinned golang-migrate container")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	directory := filepath.Join(repoRoot(t), "src", "backend", "migrations", "matchmaking_db")
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, directory)
	fixture.Run(true, "up")
	matchID, _, _ := seedActiveDuoMatch(t, ctx, fixture.Pool)
	hash := make([]byte, 32)
	_, err := fixture.Pool.Exec(ctx, `
		INSERT INTO matchmaking_match_squad_operations
		(match_id, operation_id, participant_manifest_sha256, participant_manifest_bytes,
		 state, chat_operation_id, chat_request_sha256, chat_request_bytes,
		 voice_operation_id)
		VALUES ($1,$2,$3,$4,'provisioning',$5,$6,$7,$8)
	`, matchID, uuid.New(), hash, []byte("manifest"), uuid.New(), hash, []byte("request"), uuid.New())
	require.NoError(t, err)

	// Exercise the real pinned migration runner and let its normal failed-Down
	// path record the dirty version. Do not force or clear the marker.
	fixture.Run(false, "down", "1")
	var version int64
	var dirty bool
	require.NoError(t, fixture.Pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.EqualValues(t, 16, version)
	require.True(t, dirty, "failed runner Down must leave its version dirty for operator repair")

	var operationCount int
	require.NoError(t, fixture.Pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_operations WHERE match_id=$1`, matchID).Scan(&operationCount))
	require.Equal(t, 1, operationCount, "runner refusal preserves provisioning evidence")
	var tableExists bool
	require.NoError(t, fixture.Pool.QueryRow(ctx, `SELECT to_regclass('public.matchmaking_match_squad_operations') IS NOT NULL`).Scan(&tableExists))
	require.True(t, tableExists, "runner refusal leaves lifecycle schema intact")
}

func TestMatchSquadLifecycleDownRefusesConcurrentWriterWithoutAbortingConnection(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	matchID, profileA, _ := seedActiveDuoMatch(t, ctx, pool)

	writerConn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer writerConn.Release()
	writerTx, err := writerConn.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writerTx.Rollback(ctx) }()
	opID := uuid.New()
	_, err = writerTx.Exec(ctx, `INSERT INTO matchmaking_match_leave_operations(actor_profile_id,operation_id,match_id) VALUES($1,$2,$3)`, profileA, opID, matchID)
	require.NoError(t, err)

	root := repoRoot(t)
	down, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "matchmaking_db", "000016_match_squad_lifecycle.down.sql"))
	require.NoError(t, err)
	downConn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer downConn.Release()
	_, err = downConn.Exec(ctx, string(down))
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "55P03", pgErr.Code, "DOWN must fail immediately at its NOWAIT evidence lock")
	var one int
	require.NoError(t, downConn.QueryRow(ctx, `SELECT 1`).Scan(&one), "a failed autocommit DO leaves the pooled connection usable")
	require.Equal(t, 1, one)
	require.NoError(t, writerTx.Commit(ctx))

	var leaveCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_leave_operations WHERE actor_profile_id=$1 AND operation_id=$2 AND match_id=$3`, profileA, opID, matchID).Scan(&leaveCount))
	require.Equal(t, 1, leaveCount, "refusal preserves the concurrent actor-scoped fence")
	var tableExists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.matchmaking_match_squad_operations') IS NOT NULL`).Scan(&tableExists))
	require.True(t, tableExists, "refusal leaves the lifecycle schema intact")
}

func TestMatchSquadLifecycleDownLateLockRefusalReleasesEarlierLocks(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	root := repoRoot(t)
	down, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "matchmaking_db", "000016_match_squad_lifecycle.down.sql"))
	require.NoError(t, err)

	writer, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer writer.Release()
	writerTx, err := writer.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writerTx.Rollback(ctx) }()
	_, err = writerTx.Exec(ctx, `LOCK TABLE matchmaking_match_squad_completion_events IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)

	downConn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer downConn.Release()
	_, err = downConn.Exec(ctx, string(down))
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "55P03", pgErr.Code, "a later-listed table also refuses without waiting")

	var leakedLocks int
	// The query runs on another backend; explicitly inspect the DOWN backend
	// while it is idle to prove the failed statement released its earlier locks.
	var downPID int32
	require.NoError(t, downConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&downPID))
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_locks l
		JOIN pg_class c ON c.oid=l.relation
		WHERE l.pid=$1 AND c.relname IN (
			'matches','matchmaking_match_leave_operations','matchmaking_match_squad_operations',
			'matchmaking_match_squad_teardowns','matchmaking_match_squad_teardown_participants',
			'matchmaking_match_squad_compaction_intents','matchmaking_match_squad_completion_events')
	`, downPID).Scan(&leakedLocks))
	require.Zero(t, leakedLocks, "NOWAIT failure releases every earlier table lock")
	var one int
	require.NoError(t, downConn.QueryRow(ctx, `SELECT 1`).Scan(&one))
	require.Equal(t, 1, one)
}

func TestMatchSquadLifecycleDownAtomicRollbackAndEmptySuccess(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	root := repoRoot(t)
	down, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "matchmaking_db", "000016_match_squad_lifecycle.down.sql"))
	require.NoError(t, err)

	// A dependent view forces a late DDL failure after the DO has executed its
	// drops. The statement must restore all prior objects atomically.
	_, err = pool.Exec(ctx, `CREATE VIEW match_squad_down_failure_probe AS SELECT * FROM matchmaking_match_leave_operations`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err)
	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.matchmaking_match_leave_operations') IS NOT NULL`).Scan(&exists))
	require.True(t, exists, "DDL failure rolls back all earlier drops")
	_, err = pool.Exec(ctx, `DROP VIEW match_squad_down_failure_probe`)
	require.NoError(t, err)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, string(down))
	require.NoError(t, err, "empty lifecycle evidence permits DOWN")
	require.NoError(t, tx.Rollback(ctx), "a caller-owned transaction may roll back the successful atomic block")
	require.NoError(t, conn.QueryRow(ctx, `SELECT to_regclass('public.matchmaking_match_leave_operations') IS NOT NULL`).Scan(&exists))
	require.True(t, exists, "outer rollback preserves the schema")

	_, err = conn.Exec(ctx, string(down))
	require.NoError(t, err, "empty DOWN succeeds in autocommit")
	require.NoError(t, conn.QueryRow(ctx, `SELECT to_regclass('public.matchmaking_match_leave_operations') IS NULL`).Scan(&exists))
	require.True(t, exists)
}

func TestMatchSquadProvisioningRequestsAndReceiptsAreDurableAndImmutable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	games := &GameStore{Pool: pool}
	gamePage, err := games.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, gamePage.Games)
	sessions := &SessionStore{Pool: pool}
	profileA, profileB := uuid.New(), uuid.New()
	searchA, err := sessions.Create(ctx, CreateSessionParams{ProfileID: profileA, GameID: gamePage.Games[0].ID, Mode: "Duo", Criteria: `{"region":"eu"}`, TimeoutAt: time.Now().Add(30 * time.Minute)})
	require.NoError(t, err)
	searchB, err := sessions.Create(ctx, CreateSessionParams{ProfileID: profileB, GameID: gamePage.Games[0].ID, Mode: "Duo", Criteria: `{"region":"eu"}`, TimeoutAt: time.Now().Add(30 * time.Minute)})
	require.NoError(t, err)
	matches := &MatchStore{Pool: pool}
	proposal, err := matches.CreateProposal(ctx, CreateProposalParams{GameID: gamePage.Games[0].ID, Mode: "Duo", Region: "eu", Sessions: []ProposalSession{{SessionID: searchA.ID, ProfileID: profileA}, {SessionID: searchB.ID, ProfileID: profileB}}})
	require.NoError(t, err)
	_, err = matches.SetProposalResponse(ctx, proposal.Match.ID, profileA, ProposalResponseAccepted)
	require.NoError(t, err)
	_, err = matches.SetProposalResponse(ctx, proposal.Match.ID, profileB, ProposalResponseAccepted)
	require.NoError(t, err)

	profileIDs := []uuid.UUID{profileA, profileB}
	sort.Slice(profileIDs, func(i, j int) bool { return bytes.Compare(profileIDs[i][:], profileIDs[j][:]) < 0 })
	manifest := make([]byte, 0, len(profileIDs)*16)
	participants := make([]*chatv1.MatchSquadParticipant, 0, len(profileIDs))
	for _, profileID := range profileIDs {
		manifest = append(manifest, profileID[:]...)
		participants = append(participants, &chatv1.MatchSquadParticipant{ProfileId: profileID.String()})
	}
	manifestHash := sumSHA256(manifest)
	chatOpID, voiceOpID := uuid.New(), uuid.New()
	chatRequest, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&chatv1.CreateMatchSquadChatRequest{
		ProtocolVersion: 1, OperationId: chatOpID.String(), MatchId: proposal.Match.ID.String(),
		Participants: participants, ParticipantManifestSha256: manifestHash,
	})
	require.NoError(t, err)
	intent := MatchSquadProvisioningIntent{
		MatchID: proposal.Match.ID, OperationID: uuid.New(), ParticipantManifestBytes: manifest,
		ParticipantManifestHash: manifestHash, ChatOperationID: chatOpID,
		ChatRequestBytes: chatRequest, ChatRequestHash: sumSHA256(chatRequest), VoiceOperationID: voiceOpID,
	}
	saved, err := matches.SaveMatchSquadProvisioningIntent(ctx, intent)
	require.NoError(t, err)
	require.Equal(t, intent.ChatRequestBytes, saved.ChatRequestBytes)
	replayed, err := matches.SaveMatchSquadProvisioningIntent(ctx, intent)
	require.NoError(t, err)
	require.Equal(t, saved.OperationID, replayed.OperationID)

	chatReceiptID, chatID := uuid.New(), uuid.New()
	chatReceipt := &chatv1.MatchSquadChatReceipt{
		ProtocolVersion: 1, ReceiptId: chatReceiptID.String(), OperationId: chatOpID.String(),
		MatchId: proposal.Match.ID.String(), ChatId: chatID.String(), ParticipantManifestSha256: manifestHash,
		RequestSha256: sumSHA256(chatRequest), CreatedAt: timestamppb.Now(),
	}
	chatReceiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(chatReceipt)
	require.NoError(t, err)
	voiceRequest, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&callsv1.CreateMatchSquadRoomRequest{
		ProtocolVersion: 1, OperationId: voiceOpID.String(), MatchId: proposal.Match.ID.String(),
		Participants: participants, ParticipantManifestSha256: manifestHash, ChatCreationReceipt: chatReceipt,
	})
	require.NoError(t, err)
	withChat, err := matches.RecordMatchSquadChatReceipt(ctx, intent.MatchID, chatReceiptID, chatID, chatReceiptBytes, sumSHA256(voiceRequest), voiceRequest)
	require.NoError(t, err)
	require.Equal(t, voiceRequest, withChat.VoiceRequestBytes)
	withChatReplay, err := matches.RecordMatchSquadChatReceipt(ctx, intent.MatchID, chatReceiptID, chatID, chatReceiptBytes, sumSHA256(voiceRequest), voiceRequest)
	require.NoError(t, err)
	require.Equal(t, withChat.ChatReceiptBytes, withChatReplay.ChatReceiptBytes)
	_, err = matches.RecordMatchSquadChatReceipt(ctx, intent.MatchID, chatReceiptID, chatID, []byte("changed receipt"), sumSHA256(voiceRequest), voiceRequest)
	require.ErrorIs(t, err, ErrMatchSquadConflict)

	voiceReceiptID, roomID := uuid.New(), uuid.New()
	voiceReceipt := &callsv1.MatchSquadRoomReceipt{
		ProtocolVersion: 1, ReceiptId: voiceReceiptID.String(), OperationId: voiceOpID.String(),
		MatchId: proposal.Match.ID.String(), RoomId: roomID.String(), ChatId: chatID.String(),
		ChatCreationReceiptId: chatReceiptID.String(), ChatCreationReceiptSha256: sumSHA256(chatReceiptBytes),
		ParticipantManifestSha256: manifestHash, RequestSha256: sumSHA256(voiceRequest), CreatedAt: timestamppb.Now(),
	}
	voiceReceiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(voiceReceipt)
	require.NoError(t, err)
	withVoice, err := matches.RecordMatchSquadVoiceReceipt(ctx, intent.MatchID, voiceReceiptID, roomID, voiceReceiptBytes)
	require.NoError(t, err)
	require.Equal(t, roomID, *withVoice.VoiceRoomID)
	active, err := matches.ActivateProvisionedMatch(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, MatchStatusActive, active.Status)
	require.Equal(t, chatID.String(), *active.ChatID)
	require.Equal(t, roomID.String(), *active.VoiceRoomID)
	replayedActive, err := matches.ActivateProvisionedMatch(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, active.ChatID, replayedActive.ChatID)
	require.Equal(t, active.VoiceRoomID, replayedActive.VoiceRoomID)
	for _, sessionID := range []uuid.UUID{searchA.ID, searchB.ID} {
		var sessionStatus string
		require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM search_sessions WHERE id=$1`, sessionID).Scan(&sessionStatus))
		require.Equal(t, SessionStatusMatched, sessionStatus)
	}

	profileAOperation := uuid.New()
	_, completed, err := matches.CompleteMatchLeaveWithOperation(ctx, intent.MatchID, profileA, profileAOperation)
	require.NoError(t, err)
	require.False(t, completed, "one member's confirmed Voice Leave must not start shared-room teardown")
	replayedLeave, replayedTransition, err := matches.CompleteMatchLeaveWithOperation(ctx, intent.MatchID, profileA, profileAOperation)
	require.NoError(t, err)
	require.False(t, replayedTransition, "an actor-scoped operation replay cannot repeat the transition")
	require.True(t, replayedLeave.HasLeft(profileA))
	completedMatch, completed, err := matches.CompleteMatchLeaveWithOperation(ctx, intent.MatchID, profileB, profileAOperation)
	require.NoError(t, err, "the same UUID is an independent operation in another actor's namespace")
	require.True(t, completed, "the other participant's actor-scoped leave completes the match")
	require.Equal(t, MatchStatusCompleted, completedMatch.Status)

	// Once profile B has used this key on the first match, rebinding that same
	// actor-scoped key to another authorized match must conflict. The second
	// match is active but has no provider resources, so this negative only
	// exercises the idempotency binding and cannot start a second teardown.
	otherProfile := uuid.New()
	secondSearchB, err := sessions.Create(ctx, CreateSessionParams{ProfileID: profileB, GameID: gamePage.Games[0].ID, Mode: "Duo", Criteria: `{"region":"eu"}`, TimeoutAt: time.Now().Add(30 * time.Minute)})
	require.NoError(t, err)
	secondSearchOther, err := sessions.Create(ctx, CreateSessionParams{ProfileID: otherProfile, GameID: gamePage.Games[0].ID, Mode: "Duo", Criteria: `{"region":"eu"}`, TimeoutAt: time.Now().Add(30 * time.Minute)})
	require.NoError(t, err)
	secondProposal, err := matches.CreateProposal(ctx, CreateProposalParams{GameID: gamePage.Games[0].ID, Mode: "Duo", Region: "eu", Sessions: []ProposalSession{{SessionID: secondSearchB.ID, ProfileID: profileB}, {SessionID: secondSearchOther.ID, ProfileID: otherProfile}}})
	require.NoError(t, err)
	_, err = matches.SetProposalResponse(ctx, secondProposal.Match.ID, profileB, ProposalResponseAccepted)
	require.NoError(t, err)
	_, err = matches.SetProposalResponse(ctx, secondProposal.Match.ID, otherProfile, ProposalResponseAccepted)
	require.NoError(t, err)
	_, err = matches.ActivateMatch(ctx, secondProposal.Match.ID, uuid.NewString(), uuid.NewString())
	require.NoError(t, err)
	_, _, err = matches.CompleteMatchLeaveWithOperation(ctx, secondProposal.Match.ID, profileB, profileAOperation)
	require.ErrorIs(t, err, ErrMatchOperationConflict, "one actor's key cannot be rebound to another match")
	secondMatchAfterConflict, err := matches.Get(ctx, secondProposal.Match.ID)
	require.NoError(t, err)
	require.False(t, secondMatchAfterConflict.HasLeft(profileB), "conflicting reuse cannot mutate the other match")
	var teardownCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_teardowns WHERE match_id=$1`, intent.MatchID).Scan(&teardownCount))
	require.Equal(t, 1, teardownCount, "the other actor's final accepted leave creates the original match aggregate exactly once")

	var aggregateID uuid.UUID
	var aggregateState string
	var chatTeardownBytes, voiceTeardownBytes []byte
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT aggregate_id, state, chat_teardown_request_bytes, voice_teardown_request_bytes
		FROM matchmaking_match_squad_teardowns WHERE match_id=$1
	`, intent.MatchID).Scan(&aggregateID, &aggregateState, &chatTeardownBytes, &voiceTeardownBytes))
	require.NotEqual(t, uuid.Nil, aggregateID)
	require.Equal(t, "pending", aggregateState, "provider room absence is still pending after Matchmaking completion")
	var chatTeardown chatv1.TeardownMatchSquadChatRequest
	require.NoError(t, proto.Unmarshal(chatTeardownBytes, &chatTeardown))
	var voiceTeardown callsv1.TeardownMatchSquadRoomRequest
	require.NoError(t, proto.Unmarshal(voiceTeardownBytes, &voiceTeardown))
	teardownParticipants, err := matches.ListPendingMatchSquadTeardownParticipants(ctx, 100)
	require.NoError(t, err)
	require.Len(t, teardownParticipants, 2, "Chat and Voice provider work has independent durable rows")
	participantByProvider := map[string]MatchSquadTeardownParticipant{}
	for _, participant := range teardownParticipants {
		participantByProvider[participant.Provider] = participant
		require.Equal(t, aggregateID, participant.AggregateID)
		require.Equal(t, "NOT_STARTED", participant.State)
		require.Equal(t, sumSHA256(participant.Request), participant.RequestHash)
	}
	require.Equal(t, chatTeardown.GetTeardownOperationId(), participantByProvider["chat"].OperationID.String())
	require.Equal(t, chatTeardownBytes, participantByProvider["chat"].Request)
	require.Equal(t, voiceTeardown.GetTeardownOperationId(), participantByProvider["voice"].OperationID.String())
	require.Equal(t, voiceTeardownBytes, participantByProvider["voice"].Request)
	require.NoError(t, matches.SetMatchSquadTeardownParticipantState(ctx, aggregateID, "chat", "IN_FLIGHT"))
	require.NoError(t, matches.SetMatchSquadTeardownParticipantState(ctx, aggregateID, "chat", "RETRYABLE_FAILURE"))
	retryable, err := matches.ListPendingMatchSquadTeardownParticipants(ctx, 100)
	require.NoError(t, err)
	require.Len(t, retryable, 2, "a retryable provider request stays discoverable with the same request bytes")
	completedReplay, replayTransition, err := matches.CompleteMatchLeaveWithOperation(ctx, intent.MatchID, profileB, profileAOperation)
	require.NoError(t, err)
	require.False(t, replayTransition, "replaying the final actor operation must not start a second teardown")
	require.Equal(t, completedMatch.CompletedAt, completedReplay.CompletedAt)
	require.Equal(t, intent.MatchID.String(), chatTeardown.GetMatchId())
	require.Equal(t, chatID.String(), chatTeardown.GetChatId())
	require.Equal(t, chatReceiptID.String(), chatTeardown.GetCreationReceiptId())
	require.Equal(t, intent.MatchID.String(), voiceTeardown.GetMatchId())
	require.Equal(t, roomID.String(), voiceTeardown.GetRoomId())
	require.Equal(t, voiceReceiptID.String(), voiceTeardown.GetCreationReceiptId())

	chatTeardownReceiptID := uuid.New()
	chatTeardownReceipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&chatv1.MatchSquadChatTeardownReceipt{
		ProtocolVersion: 1, ReceiptId: chatTeardownReceiptID.String(), TeardownOperationId: chatTeardown.GetTeardownOperationId(),
		MatchId: intent.MatchID.String(), ChatId: chatID.String(), CreationReceiptId: chatReceiptID.String(),
		ParticipantManifestSha256: manifestHash, RequestSha256: sumSHA256(chatTeardownBytes),
		Status: chatv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, CompletedAt: timestamppb.Now(),
	})
	require.NoError(t, err)
	pending, err := matches.RecordMatchSquadTeardownReceipt(ctx, aggregateID, "chat", chatTeardownReceiptID, chatTeardownReceipt)
	require.NoError(t, err)
	require.Equal(t, "pending", pending.State, "one provider receipt cannot claim aggregate room absence")

	voiceTeardownReceiptID := uuid.New()
	voiceTeardownReceipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&callsv1.MatchSquadRoomTeardownReceipt{
		ProtocolVersion: 1, ReceiptId: voiceTeardownReceiptID.String(), TeardownOperationId: voiceTeardown.GetTeardownOperationId(),
		MatchId: intent.MatchID.String(), RoomId: roomID.String(), CreationReceiptId: voiceReceiptID.String(),
		ParticipantManifestSha256: manifestHash, RequestSha256: sumSHA256(voiceTeardownBytes),
		Status: callsv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, CompletedAt: timestamppb.Now(),
	})
	require.NoError(t, err)
	completedAggregate, err := matches.RecordMatchSquadTeardownReceipt(ctx, aggregateID, "voice", voiceTeardownReceiptID, voiceTeardownReceipt)
	require.NoError(t, err)
	require.Equal(t, "complete", completedAggregate.State)
	require.NotNil(t, completedAggregate.AggregateCompletedAt)
	remainingParticipants, err := matches.ListPendingMatchSquadTeardownParticipants(ctx, 100)
	require.NoError(t, err)
	require.Empty(t, remainingParticipants, "only exact receipts complete provider participants")
	var chatParticipantState, voiceParticipantState string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM matchmaking_match_squad_teardown_participants WHERE aggregate_id=$1 AND provider='chat'`, aggregateID).Scan(&chatParticipantState))
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM matchmaking_match_squad_teardown_participants WHERE aggregate_id=$1 AND provider='voice'`, aggregateID).Scan(&voiceParticipantState))
	require.Equal(t, "COMPLETE", chatParticipantState)
	require.Equal(t, "COMPLETE", voiceParticipantState)
	pendingEvents, err := matches.ListPendingMatchSquadCompletionEvents(ctx, 100)
	require.NoError(t, err)
	require.Len(t, pendingEvents, 1, "both exact provider receipts atomically admit one completion event")
	completionEvent := pendingEvents[0]
	require.Equal(t, aggregateID, completionEvent.AggregateID)
	require.NotEqual(t, uuid.Nil, completionEvent.EventID)
	require.Equal(t, intent.MatchID, completionEvent.MatchID)
	require.Equal(t, int64(completedMatch.CompletedAt.Sub(completedMatch.CreatedAt).Seconds()), completionEvent.DurationSeconds)
	require.ElementsMatch(t, []string{profileA.String(), profileB.String()}, completionEvent.ProfileIDs)
	require.True(t, completionEvent.OccurredAt.Equal(*completedMatch.CompletedAt))
	replayedEvent, err := matches.ListPendingMatchSquadCompletionEvents(ctx, 100)
	require.NoError(t, err)
	require.Equal(t, completionEvent.EventID, replayedEvent[0].EventID, "delivery retries retain the event ID used for broker deduplication")
	require.NoError(t, matches.MarkMatchSquadCompletionEventPublished(ctx, aggregateID, completionEvent.EventID))
	noPendingEvents, err := matches.ListPendingMatchSquadCompletionEvents(ctx, 100)
	require.NoError(t, err)
	require.Empty(t, noPendingEvents)
	var intentCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_compaction_intents WHERE aggregate_id=$1`, aggregateID).Scan(&intentCount))
	require.Equal(t, 2, intentCount)
	var notBefore, aggregateCompletedAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT i.not_before, t.aggregate_completed_at
		FROM matchmaking_match_squad_compaction_intents i
		JOIN matchmaking_match_squad_teardowns t USING (aggregate_id)
		WHERE i.aggregate_id=$1 AND i.provider='chat'
	`, aggregateID).Scan(&notBefore, &aggregateCompletedAt))
	require.True(t, notBefore.Equal(aggregateCompletedAt.Add(30*24*time.Hour)))
	_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_teardowns SET chat_teardown_receipt_bytes=$2 WHERE aggregate_id=$1`, aggregateID, []byte("changed"))
	require.Error(t, err, "completed provider evidence is immutable")
	_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_compaction_intents SET operation_id=$2 WHERE aggregate_id=$1 AND provider='chat'`, aggregateID, uuid.New())
	require.Error(t, err, "the due operation identity is immutable")
	_, err = matches.PrepareMatchSquadCompaction(ctx, aggregateID, "chat")
	require.ErrorIs(t, err, ErrMatchSquadPending, "no provider compaction request may be prepared before the aggregate deadline")
	var persistedCompactionRequests int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_compaction_intents WHERE aggregate_id=$1 AND request_bytes IS NOT NULL`, aggregateID).Scan(&persistedCompactionRequests))
	require.Zero(t, persistedCompactionRequests)
}
