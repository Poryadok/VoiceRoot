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
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
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
	replayed, err = matches.ActivateProvisionedMatch(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, active.ChatID, replayed.ChatID)
	require.Equal(t, active.VoiceRoomID, replayed.VoiceRoomID)
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
	_, _, err = matches.CompleteMatchLeaveWithOperation(ctx, intent.MatchID, profileB, profileAOperation)
	require.ErrorIs(t, err, ErrMatchOperationConflict, "the same operation ID cannot be rebound to another actor")
	var teardownCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_teardowns WHERE match_id=$1`, intent.MatchID).Scan(&teardownCount))
	require.Zero(t, teardownCount)

	profileBOperation := uuid.New()
	completedMatch, completed, err := matches.CompleteMatchLeaveWithOperation(ctx, intent.MatchID, profileB, profileBOperation)
	require.NoError(t, err)
	require.True(t, completed, "the final confirmed Matchmaking leave may initiate aggregate teardown")
	require.Equal(t, MatchStatusCompleted, completedMatch.Status)
	var aggregateID uuid.UUID
	var aggregateState string
	var chatTeardownBytes, voiceTeardownBytes []byte
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT aggregate_id, state, chat_teardown_request_bytes, voice_teardown_request_bytes
		FROM matchmaking_match_squad_teardowns WHERE match_id=$1
	`, intent.MatchID).Scan(&aggregateID, &aggregateState, &chatTeardownBytes, &voiceTeardownBytes))
	require.NotEqual(t, uuid.Nil, aggregateID)
	require.Equal(t, "pending", aggregateState, "provider room absence is still pending after Matchmaking completion")
	completedReplay, replayTransition, err := matches.CompleteMatchLeaveWithOperation(ctx, intent.MatchID, profileB, profileBOperation)
	require.NoError(t, err)
	require.False(t, replayTransition, "replaying the final actor operation must not start a second teardown")
	require.Equal(t, completedMatch.CompletedAt, completedReplay.CompletedAt)
	var chatTeardown chatv1.TeardownMatchSquadChatRequest
	require.NoError(t, proto.Unmarshal(chatTeardownBytes, &chatTeardown))
	require.Equal(t, intent.MatchID.String(), chatTeardown.GetMatchId())
	require.Equal(t, chatID.String(), chatTeardown.GetChatId())
	require.Equal(t, chatReceiptID.String(), chatTeardown.GetCreationReceiptId())
	var voiceTeardown callsv1.TeardownMatchSquadRoomRequest
	require.NoError(t, proto.Unmarshal(voiceTeardownBytes, &voiceTeardown))
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
