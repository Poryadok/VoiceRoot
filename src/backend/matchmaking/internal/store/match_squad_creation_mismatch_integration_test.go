package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"path/filepath"
	"testing"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func TestCreationMismatchMigrationEmptyRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := rosterResetDB(t)
	up21, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/000021_match_squad_dispatch_settlement.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(up21))
	require.NoError(t, err)
	for _, direction := range []string{"up", "down", "up"} {
		raw, e := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/000023_match_squad_creation_mismatch."+direction+".sql"))
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(raw))
		require.NoError(t, e, direction)
	}
}

func TestCreationMismatchKeepsFinalCleanupEvidenceWithoutClosure(t *testing.T) {
	testCreationMismatchCleanup(t, false)
}
func TestLateCreationMismatchPreservesClosedStateAndSealedProviders(t *testing.T) {
	testCreationMismatchCleanup(t, true)
}
func testCreationMismatchCleanup(t *testing.T, late bool) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := rosterResetDB(t)
	st := &MatchStore{Pool: pool}
	_, members := capturedRosterParty(t, pool, nil, 0, uuid.New())
	s := members[0]
	proposal, err := st.CreateProposal(ctx, CreateProposalParams{GameID: s.GameID, Mode: "test", Region: "eu", Sessions: []ProposalSession{{SessionID: s.ID, ProfileID: s.ProfileID, PartyID: s.PartyID}}})
	require.NoError(t, err)
	_, err = st.SetProposalResponse(ctx, proposal.Match.ID, s.ProfileID, ProposalResponseAccepted)
	require.NoError(t, err)
	manifest := append([]byte(nil), s.ProfileID[:]...)
	chatOp, voiceOp := uuid.New(), uuid.New()
	participants := []*chatv1.MatchSquadParticipant{{ProfileId: s.ProfileID.String()}}
	chatRequest, err := proto.MarshalOptions{Deterministic: true}.Marshal(&chatv1.CreateMatchSquadChatRequest{
		ProtocolVersion: 1, OperationId: chatOp.String(), MatchId: proposal.Match.ID.String(),
		Participants: participants, ParticipantManifestSha256: sumSHA256(manifest)})
	require.NoError(t, err)
	_, err = st.SaveMatchSquadProvisioningIntent(ctx, MatchSquadProvisioningIntent{MatchID: proposal.Match.ID, OperationID: uuid.New(),
		ParticipantManifestHash: sumSHA256(manifest), ParticipantManifestBytes: manifest, ChatOperationID: chatOp,
		ChatRequestHash: sumSHA256(chatRequest), ChatRequestBytes: chatRequest, VoiceOperationID: voiceOp})
	require.NoError(t, err)
	chatID, chatReceiptID := uuid.New(), uuid.New()
	chatReceipt := &chatv1.MatchSquadChatReceipt{ProtocolVersion: 1, ReceiptId: chatReceiptID.String(), OperationId: chatOp.String(),
		MatchId: proposal.Match.ID.String(), ChatId: chatID.String(), ParticipantManifestSha256: sumSHA256(manifest),
		RequestSha256: sumSHA256(chatRequest), CreatedAt: timestamppb.Now()}
	chatReceiptBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(chatReceipt)
	require.NoError(t, err)
	voiceRequest, err := proto.MarshalOptions{Deterministic: true}.Marshal(&callsv1.CreateMatchSquadRoomRequest{
		ProtocolVersion: 1, OperationId: voiceOp.String(), MatchId: proposal.Match.ID.String(), Participants: participants,
		ParticipantManifestSha256: sumSHA256(manifest), ChatCreationReceipt: chatReceipt})
	require.NoError(t, err)
	_, err = st.RecordMatchSquadChatReceipt(ctx, proposal.Match.ID, chatReceiptID, chatID, chatReceiptBytes, sumSHA256(voiceRequest), voiceRequest)
	require.NoError(t, err)
	migration, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/000021_match_squad_dispatch_settlement.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)

	migration23, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/000023_match_squad_creation_mismatch.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration23))
	require.NoError(t, err)
	operationBefore, err := st.GetMatchSquadProvisioningOperation(ctx, proposal.Match.ID)
	require.NoError(t, err)
	witness := []byte("contradictory exact provider response")
	if !late {
		require.NoError(t, st.RecordMatchSquadCreationMismatch(ctx, proposal.Match.ID, operationBefore.OperationID, "voice", sumSHA256(voiceRequest), witness))
		var durableWitness []byte
		require.NoError(t, pool.QueryRow(ctx, `SELECT voice_creation_mismatch_bytes FROM matchmaking_match_squad_operations WHERE match_id=$1`, proposal.Match.ID).Scan(&durableWitness))
		require.Equal(t, witness, durableWitness)
		require.NoError(t, st.RecordMatchSquadCreationMismatch(ctx, proposal.Match.ID, operationBefore.OperationID, "voice", sumSHA256(voiceRequest), witness))
		require.ErrorIs(t, st.RecordMatchSquadCreationMismatch(ctx, proposal.Match.ID, operationBefore.OperationID, "voice", sumSHA256(voiceRequest), []byte("different contradiction")), ErrMatchSquadCreationMismatch)
		require.ErrorIs(t, st.RecordMatchSquadCreationMismatch(ctx, proposal.Match.ID, uuid.New(), "voice", sumSHA256(voiceRequest), witness), ErrMatchSquadConflict)
		// Ordinary retry diagnostics may change after restart, but cannot replace
		// the first negative fact or stop its immediate operator signal.
		_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET provision_last_error_class='unavailable' WHERE match_id=$1`, proposal.Match.ID)
		require.NoError(t, err)
		restarted := &MatchStore{Pool: pool}
		require.ErrorIs(t, restarted.CheckMatchSquadCreationEvidence(ctx, proposal.Match.ID), ErrMatchSquadCreationMismatch)
		_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET voice_creation_mismatch_bytes=NULL,voice_creation_mismatch_sha256=NULL,voice_creation_mismatch_at=NULL WHERE match_id=$1`, proposal.Match.ID)
		require.ErrorContains(t, err, "cannot rewrite creation mismatch evidence")
	}
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `UPDATE matches SET status='abandoned' WHERE id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	require.NoError(t, LatchMatchSquadAbandonmentTx(ctx, tx, proposal.Match.ID, "VOICE_ROSTER_CHANGED", [32]byte{1}))
	require.NoError(t, tx.Commit(ctx))
	var aggregateID uuid.UUID
	var required []string
	var chatTeardownBytes []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT aggregate_id,required_providers,chat_teardown_request_bytes FROM matchmaking_match_squad_teardowns WHERE match_id=$1`, proposal.Match.ID).Scan(&aggregateID, &required, &chatTeardownBytes))
	require.Equal(t, []string{"chat", "voice"}, required)
	require.NotEmpty(t, chatTeardownBytes, "known Chat cleanup must not wait for unknown Voice")
	chatTeardown := new(chatv1.TeardownMatchSquadChatRequest)
	require.NoError(t, proto.Unmarshal(chatTeardownBytes, chatTeardown))
	chatCleanupID := uuid.New()
	chatCleanup, err := proto.MarshalOptions{Deterministic: true}.Marshal(&chatv1.MatchSquadChatTeardownReceipt{
		ProtocolVersion: 1, ReceiptId: chatCleanupID.String(), TeardownOperationId: chatTeardown.GetTeardownOperationId(),
		MatchId: proposal.Match.ID.String(), ChatId: chatID.String(), CreationReceiptId: chatReceiptID.String(),
		ParticipantManifestSha256: sumSHA256(manifest), RequestSha256: sumSHA256(chatTeardownBytes),
		Status: chatv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, CompletedAt: timestamppb.Now()})
	require.NoError(t, err)
	pending, err := st.RecordMatchSquadTeardownReceipt(ctx, aggregateID, "chat", chatCleanupID, chatCleanup)
	require.NoError(t, err)
	require.Equal(t, "pending", pending.State)
	var waiting bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT state='WAITING_FOR_CREATION' AND request_bytes IS NULL FROM matchmaking_match_squad_teardown_participants WHERE aggregate_id=$1 AND provider='voice'`, aggregateID).Scan(&waiting))
	require.True(t, waiting)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_compaction_intents WHERE aggregate_id=$1`, aggregateID).Scan(&count))
	require.Zero(t, count, "unknown creation cannot age out through compaction")
	_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET state='closed' WHERE match_id=$1`, proposal.Match.ID)
	require.Error(t, err, "unknown Voice creation must keep settlement discoverable")
	roomID, voiceReceiptID := uuid.New(), uuid.New()
	voiceReceipt, err := proto.MarshalOptions{Deterministic: true}.Marshal(&callsv1.MatchSquadRoomReceipt{
		ProtocolVersion: 1, ReceiptId: voiceReceiptID.String(), OperationId: voiceOp.String(), MatchId: proposal.Match.ID.String(),
		RoomId: roomID.String(), ChatId: chatID.String(), ChatCreationReceiptId: chatReceiptID.String(),
		ChatCreationReceiptSha256: sumSHA256(chatReceiptBytes), ParticipantManifestSha256: sumSHA256(manifest),
		RequestSha256: sumSHA256(voiceRequest), CreatedAt: timestamppb.Now()})
	require.NoError(t, err)
	_, err = st.RecordMatchSquadVoiceReceipt(ctx, proposal.Match.ID, voiceReceiptID, roomID, voiceReceipt)
	require.NoError(t, err)
	aggregate, err := st.GetMatchSquadTeardown(ctx, aggregateID)
	require.NoError(t, err)
	voiceTeardown := new(callsv1.TeardownMatchSquadRoomRequest)
	require.NoError(t, proto.Unmarshal(aggregate.VoiceRequestBytes, voiceTeardown))
	voiceCleanupID := uuid.New()
	voiceCleanup, err := proto.MarshalOptions{Deterministic: true}.Marshal(&callsv1.MatchSquadRoomTeardownReceipt{
		ProtocolVersion: 1, ReceiptId: voiceCleanupID.String(), TeardownOperationId: voiceTeardown.GetTeardownOperationId(),
		MatchId: proposal.Match.ID.String(), RoomId: roomID.String(), CreationReceiptId: voiceReceiptID.String(),
		ParticipantManifestSha256: sumSHA256(manifest), RequestSha256: sumSHA256(aggregate.VoiceRequestBytes),
		Status: callsv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, CompletedAt: timestamppb.Now()})
	require.NoError(t, err)
	complete, err := st.RecordMatchSquadTeardownReceipt(ctx, aggregateID, "voice", voiceCleanupID, voiceCleanup)
	require.NoError(t, err)
	if late {
		require.Equal(t, "complete", complete.State, "ordinary valid path completes before the late contradiction")
		require.NoError(t, st.RecordMatchSquadCreationMismatch(ctx, proposal.Match.ID, operationBefore.OperationID, "voice", sumSHA256(voiceRequest), witness))
		require.ErrorIs(t, st.CheckMatchSquadCreationEvidence(ctx, proposal.Match.ID), ErrMatchSquadCreationMismatch)
		attention, e := st.MatchSquadProvisioningAttention(ctx)
		require.NoError(t, e)
		require.Equal(t, 1, attention.ContractMismatch, "late closed mismatch still pages")
		var sealed []string
		require.NoError(t, pool.QueryRow(ctx, `SELECT required_providers FROM matchmaking_match_squad_teardowns WHERE aggregate_id=$1`, aggregateID).Scan(&sealed))
		require.Equal(t, required, sealed, "late negative evidence cannot rewrite the sealed required set")
	} else {
		require.Equal(t, "pending", complete.State)
	}

	require.NotNil(t, complete.ChatReceiptID)
	require.NotNil(t, complete.VoiceReceiptID)
	var participantReceipts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_teardown_participants WHERE aggregate_id=$1 AND receipt_bytes IS NOT NULL`, aggregateID).Scan(&participantReceipts))
	require.Equal(t, 2, participantReceipts, "the last cleanup receipt must commit despite the mismatch")
	_, err = st.PrepareMatchSquadCompaction(ctx, aggregateID, "voice")
	require.ErrorIs(t, err, ErrMatchSquadCreationMismatch)
	if !late {
		_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_teardowns SET state='complete',aggregate_completed_at=clock_timestamp() WHERE aggregate_id=$1`, aggregateID)
		require.ErrorContains(t, err, "creation mismatch requires reconciliation")
	}
	down23, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/000023_match_squad_creation_mismatch.down.sql"))
	require.NoError(t, err)
	downTx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = downTx.Exec(ctx, string(down23))
	require.ErrorContains(t, err, "cannot remove creation mismatch evidence")
	require.NoError(t, downTx.Rollback(ctx))
	match, err := st.Get(ctx, proposal.Match.ID)
	require.NoError(t, err)
	require.Equal(t, MatchStatusAbandoned, match.Status)
	operation, err := st.GetMatchSquadProvisioningOperation(ctx, proposal.Match.ID)
	require.NoError(t, err)
	if late {
		require.Equal(t, "closed", operation.State)
	} else {
		require.Equal(t, "compensating", operation.State)
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_completion_events WHERE aggregate_id=$1`, aggregateID).Scan(&count))
	require.Zero(t, count)
	_, err = st.RecordMatchSquadVoiceReceipt(ctx, proposal.Match.ID, voiceReceiptID, roomID, voiceReceipt)
	require.NoError(t, err, "exact late creation replay after cleanup cannot reopen anything")
}
