package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func TestMatchSquadUnknownChatAbandonmentSharesRollbackAndFreezesObligations(t *testing.T) {
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
	chatOp := uuid.New()
	manifest := append([]byte(nil), s.ProfileID[:]...)
	request, err := proto.MarshalOptions{Deterministic: true}.Marshal(&chatv1.CreateMatchSquadChatRequest{
		ProtocolVersion: 1, OperationId: chatOp.String(), MatchId: proposal.Match.ID.String(),
		Participants: []*chatv1.MatchSquadParticipant{{ProfileId: s.ProfileID.String()}}, ParticipantManifestSha256: sumSHA256(manifest),
	})
	require.NoError(t, err)
	_, err = st.SaveMatchSquadProvisioningIntent(ctx, MatchSquadProvisioningIntent{MatchID: proposal.Match.ID, OperationID: uuid.New(), ParticipantManifestHash: sumSHA256(manifest), ParticipantManifestBytes: manifest, ChatOperationID: chatOp, ChatRequestHash: sumSHA256(request), ChatRequestBytes: request, VoiceOperationID: uuid.New()})
	require.NoError(t, err)
	migration, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/000021_match_squad_dispatch_settlement.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	var chatState, voiceState, disposition string
	require.NoError(t, pool.QueryRow(ctx, `SELECT chat_dispatch_state,voice_dispatch_state FROM matchmaking_match_squad_operations WHERE match_id=$1`, proposal.Match.ID).Scan(&chatState, &voiceState))
	require.Equal(t, "MAY_HAVE_CREATED", chatState, "legacy response loss must not become no-create")
	require.Equal(t, "NOT_ISSUED", voiceState)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `UPDATE matches SET status='abandoned' WHERE id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	require.NoError(t, LatchMatchSquadAbandonmentTx(ctx, tx, proposal.Match.ID, "VOICE_ROSTER_CHANGED", [32]byte{1}))
	require.NoError(t, tx.QueryRow(ctx, `SELECT provision_disposition FROM matchmaking_match_squad_operations WHERE match_id=$1`, proposal.Match.ID).Scan(&disposition))
	require.Equal(t, "ABANDONED", disposition)
	var required []string
	var waiting bool
	require.NoError(t, tx.QueryRow(ctx, `SELECT a.required_providers,p.state='WAITING_FOR_CREATION' AND p.request_bytes IS NULL FROM matchmaking_match_squad_teardowns a JOIN matchmaking_match_squad_teardown_participants p USING(aggregate_id) WHERE a.match_id=$1 AND p.provider='chat'`, proposal.Match.ID).Scan(&required, &waiting))
	require.Equal(t, []string{"chat"}, required)
	require.True(t, waiting)
	require.NoError(t, tx.Rollback(ctx))
	require.NoError(t, pool.QueryRow(ctx, `SELECT provision_disposition FROM matchmaking_match_squad_operations WHERE match_id=$1`, proposal.Match.ID).Scan(&disposition))
	require.Equal(t, "LIVE", disposition)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_teardowns WHERE match_id=$1`, proposal.Match.ID).Scan(&count))
	require.Zero(t, count)
	// A committed reset is replayable but can never return to LIVE or drop an
	// issued provider from its immutable obligation set.
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE matches SET status='abandoned' WHERE id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	require.NoError(t, LatchMatchSquadAbandonmentTx(ctx, tx, proposal.Match.ID, "VOICE_ROSTER_CHANGED", [32]byte{1}))
	require.NoError(t, tx.Commit(ctx))
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, LatchMatchSquadAbandonmentTx(ctx, tx, proposal.Match.ID, "REPLAY_MUST_NOT_REWRITE", [32]byte{2}))
	require.NoError(t, tx.Commit(ctx))
	_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET provision_disposition='LIVE',provision_abandoned_at=NULL,provision_abandon_reason=NULL,provision_abandon_evidence_sha256=NULL WHERE match_id=$1`, proposal.Match.ID)
	require.Error(t, err, "abandonment is monotone even for a direct storage caller")
	_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET chat_dispatch_state='NOT_ISSUED',chat_first_dispatch_id=NULL,chat_first_dispatch_at=NULL WHERE match_id=$1`, proposal.Match.ID)
	require.Error(t, err, "possible dispatch cannot be forgotten")
	_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET voice_dispatch_state='MAY_HAVE_CREATED',voice_first_dispatch_id=gen_random_uuid(),voice_first_dispatch_at=clock_timestamp(),voice_request_sha256=$2,voice_request_bytes=$3 WHERE match_id=$1`, proposal.Match.ID, sumSHA256([]byte("late")), []byte("late"))
	require.Error(t, err, "abandonment cannot dispatch a previously unissued provider")
	creationID, chatID := uuid.New(), uuid.New()
	creation := &chatv1.MatchSquadChatReceipt{ProtocolVersion: 1, ReceiptId: creationID.String(),
		OperationId: chatOp.String(), MatchId: proposal.Match.ID.String(), ChatId: chatID.String(),
		ParticipantManifestSha256: sumSHA256(manifest), RequestSha256: sumSHA256(request), CreatedAt: timestamppb.Now()}
	creationBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(creation)
	require.NoError(t, err)
	op, err := st.GetMatchSquadProvisioningOperation(ctx, proposal.Match.ID)
	require.NoError(t, err)
	voiceRequest, err := proto.MarshalOptions{Deterministic: true}.Marshal(&callsv1.CreateMatchSquadRoomRequest{
		ProtocolVersion: 1, OperationId: op.VoiceOperationID.String(), MatchId: proposal.Match.ID.String(),
		Participants:              []*chatv1.MatchSquadParticipant{{ProfileId: s.ProfileID.String()}},
		ParticipantManifestSha256: sumSHA256(manifest), ChatCreationReceipt: creation})
	require.NoError(t, err)
	_, err = st.RecordMatchSquadChatReceipt(ctx, proposal.Match.ID, creationID, chatID, creationBytes, sumSHA256(voiceRequest), voiceRequest)
	require.NoError(t, err, "valid late creation receipt must settle the already-issued call")
	var teardownBytes []byte
	var participantState string
	require.NoError(t, pool.QueryRow(ctx, `SELECT p.state,p.request_bytes FROM matchmaking_match_squad_teardowns a
		JOIN matchmaking_match_squad_teardown_participants p USING(aggregate_id) WHERE a.match_id=$1 AND p.provider='chat'`, proposal.Match.ID).Scan(&participantState, &teardownBytes))
	require.Equal(t, "NOT_STARTED", participantState)
	teardown := new(chatv1.TeardownMatchSquadChatRequest)
	require.NoError(t, proto.Unmarshal(teardownBytes, teardown))
	require.Equal(t, chatID.String(), teardown.GetChatId())
	require.Equal(t, creationID.String(), teardown.GetCreationReceiptId())
	_, err = st.RecordMatchSquadChatReceipt(ctx, proposal.Match.ID, creationID, chatID, creationBytes, sumSHA256(voiceRequest), voiceRequest)
	require.NoError(t, err, "exact late receipt replay is idempotent")
	require.NoError(t, pool.QueryRow(ctx, `SELECT provision_disposition,chat_dispatch_state,voice_dispatch_state FROM matchmaking_match_squad_operations WHERE match_id=$1`, proposal.Match.ID).Scan(&disposition, &chatState, &voiceState))
	require.Equal(t, "ABANDONED", disposition)
	require.Equal(t, "RECEIPT_KNOWN", chatState)
	require.Equal(t, "NOT_ISSUED", voiceState, "late Chat receipt cannot dispatch Voice")
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/000021_match_squad_dispatch_settlement.down.sql"))
	require.NoError(t, err)
	// Explicit caller transaction is important: a failed multi-statement DOWN
	// must be rolled back before returning this pooled connection.
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, string(down))
	require.ErrorContains(t, err, "cannot remove MatchSquad dispatch or settlement evidence")
	require.NoError(t, tx.Rollback(ctx))
}

func TestMatchSquadSettlementMigrationEmptyRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	ctx := context.Background()
	pool := rosterResetDB(t)
	for _, direction := range []string{"up", "down", "up"} {
		migration, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/000021_match_squad_dispatch_settlement."+direction+".sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(migration))
		require.NoError(t, err, direction)
	}
}

func TestMatchSquadUnissuedAbandonmentClosesWithoutSyntheticTeardown(t *testing.T) {
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
	migration, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/matchmaking_db/000021_match_squad_dispatch_settlement.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	// Seed the new protocol's exact, never-dispatched intent. Coordinator
	// validation/dispatch behavior is outside this storage-only proof.
	manifest := append([]byte(nil), s.ProfileID[:]...)
	chatOp := uuid.New()
	request, err := proto.MarshalOptions{Deterministic: true}.Marshal(&chatv1.CreateMatchSquadChatRequest{
		ProtocolVersion: 1, OperationId: chatOp.String(), MatchId: proposal.Match.ID.String(),
		Participants: []*chatv1.MatchSquadParticipant{{ProfileId: s.ProfileID.String()}}, ParticipantManifestSha256: sumSHA256(manifest),
	})
	require.NoError(t, err)
	_, err = st.SaveMatchSquadProvisioningIntent(ctx, MatchSquadProvisioningIntent{MatchID: proposal.Match.ID, OperationID: uuid.New(),
		ParticipantManifestHash: sumSHA256(manifest), ParticipantManifestBytes: manifest, ChatOperationID: chatOp,
		ChatRequestHash: sumSHA256(request), ChatRequestBytes: request, VoiceOperationID: uuid.New()})
	require.Error(t, err, "upgraded schema rejects new unmarked legacy intent")
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_match_squad_operations
		(match_id,operation_id,participant_manifest_sha256,participant_manifest_bytes,state,chat_operation_id,chat_request_sha256,chat_request_bytes,voice_operation_id,dispatch_protocol_version)
		VALUES ($1,$2,$3,$4,'provisioning',$5,$6,$7,$8,1)`, proposal.Match.ID, uuid.New(), sumSHA256(manifest), manifest, chatOp, sumSHA256(request), request, uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO matchmaking_match_squad_teardowns
		(aggregate_id,match_id,state,purpose,required_providers,chat_teardown_operation_id)
		VALUES ($1,$2,'pending','PROVISION_COMPENSATION',ARRAY['chat']::TEXT[],$3)`, uuid.New(), proposal.Match.ID, uuid.New())
	require.Error(t, err, "unresolved compensation must have a real abandoned dispatch obligation")
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `UPDATE matches SET status='abandoned' WHERE id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	require.NoError(t, LatchMatchSquadAbandonmentTx(ctx, tx, proposal.Match.ID, "VOICE_ROSTER_CHANGED", [32]byte{1}))
	require.NoError(t, tx.Commit(ctx))
	var disposition, state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT provision_disposition,state FROM matchmaking_match_squad_operations WHERE match_id=$1`, proposal.Match.ID).Scan(&disposition, &state))
	require.Equal(t, "ABANDONED", disposition)
	require.Equal(t, "closed", state)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_teardowns WHERE match_id=$1`, proposal.Match.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_completion_events`).Scan(&count))
	require.Zero(t, count)
}

func TestMatchSquadKnownChatCleanupDoesNotCompleteUnknownVoice(t *testing.T) {
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
	require.Equal(t, "complete", complete.State)
	match, err := st.Get(ctx, proposal.Match.ID)
	require.NoError(t, err)
	require.Equal(t, MatchStatusAbandoned, match.Status)
	operation, err := st.GetMatchSquadProvisioningOperation(ctx, proposal.Match.ID)
	require.NoError(t, err)
	require.Equal(t, "closed", operation.State)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_completion_events WHERE aggregate_id=$1`, aggregateID).Scan(&count))
	require.Zero(t, count)
	_, err = st.RecordMatchSquadVoiceReceipt(ctx, proposal.Match.ID, voiceReceiptID, roomID, voiceReceipt)
	require.NoError(t, err, "exact late creation replay after cleanup cannot reopen anything")
}
