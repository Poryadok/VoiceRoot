package partysearch

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
)

func dispatchDigest(raw []byte) []byte {
	hash := sha256.Sum256(raw)
	return hash[:]
}

func dispatchFixture(t *testing.T) (*pgxpool.Pool, *store.MatchStore, store.CreateSourcePartyParams, store.MatchSquadProvisioningIntent) {
	t.Helper()
	ctx := context.Background()
	pool := partyDB(t)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	sessions := make([]store.ProposalSession, 0, len(party.Sessions))
	for _, session := range party.Sessions {
		sessions = append(sessions, store.ProposalSession{SessionID: session.ID, ProfileID: session.ProfileID, PartyID: session.PartyID})
	}
	st := &store.MatchStore{Pool: pool}
	proposal, err := st.CreateProposal(ctx, store.CreateProposalParams{GameID: p.GameID, Mode: p.Mode, Region: "eu", Sessions: sessions})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE match_proposals SET response='accepted' WHERE match_id=$1`, proposal.Match.ID)
	require.NoError(t, err)
	chatOp := uuid.New()
	manifest := []byte{}
	participants := make([]*chatv1.MatchSquadParticipant, 0, len(p.MemberProfileIDs))
	for _, profile := range p.MemberProfileIDs {
		manifest = append(manifest, profile[:]...)
		participants = append(participants, &chatv1.MatchSquadParticipant{ProfileId: profile.String()})
	}
	request, err := proto.MarshalOptions{Deterministic: true}.Marshal(&chatv1.CreateMatchSquadChatRequest{
		ProtocolVersion: 1, OperationId: chatOp.String(), MatchId: proposal.Match.ID.String(),
		Participants: participants, ParticipantManifestSha256: dispatchDigest(manifest)})
	require.NoError(t, err)
	return pool, st, p, store.MatchSquadProvisioningIntent{MatchID: proposal.Match.ID, OperationID: uuid.New(),
		ParticipantManifestBytes: manifest, ParticipantManifestHash: dispatchDigest(manifest), ChatOperationID: chatOp,
		ChatRequestBytes: request, ChatRequestHash: dispatchDigest(request), VoiceOperationID: uuid.New()}
}

func dispatchProof(t *testing.T, st *store.MatchStore, id uuid.UUID, p store.CreateSourcePartyParams) (store.ProvisionCandidate, *partysource.ValidatedAttempt) {
	t.Helper()
	ctx := context.Background()
	candidate, err := st.CaptureProvisionCandidate(ctx, id)
	require.NoError(t, err)
	validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{ProtocolVersion: 1, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion,
			MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}
	result, err := validator.ValidateCapturedParties(ctx, candidate)
	require.NoError(t, err)
	require.NotNil(t, result.Attempt)
	require.True(t, result.Attempt.Matches(candidate))
	return candidate, result.Attempt
}

func TestValidatedDispatchMarkerSurvivesAbandonmentAndStaleWorker(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := dispatchFixture(t)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
	require.NoError(t, err)
	var chatState, voiceState string
	var protocol int
	require.NoError(t, pool.QueryRow(ctx, `SELECT dispatch_protocol_version,chat_dispatch_state,voice_dispatch_state FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&protocol, &chatState, &voiceState))
	require.Equal(t, 1, protocol)
	require.Equal(t, "NOT_ISSUED", chatState)
	require.Equal(t, "NOT_ISSUED", voiceState)
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	ticket, err := st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, ticket.FirstDispatchID)
	require.Equal(t, intent.ChatRequestBytes, ticket.Operation.ChatRequestBytes)
	require.NoError(t, pool.QueryRow(ctx, `SELECT chat_dispatch_state FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&chatState))
	require.Equal(t, "MAY_HAVE_CREATED", chatState, "marker committed before caller receives permission for any RPC")
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	retry, err := st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.NoError(t, err)
	require.Equal(t, ticket.FirstDispatchID, retry.FirstDispatchID)
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.ErrorIs(t, err, store.ErrProvisionCandidateChanged, "each retry consumes its proof even when the first-dispatch marker already exists")
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `UPDATE matches SET status='abandoned' WHERE id=$1`, intent.MatchID)
	require.NoError(t, err)
	require.NoError(t, store.LatchMatchSquadAbandonmentTx(ctx, tx, intent.MatchID, "VOICE_ROSTER_CHANGED", [32]byte{1}))
	require.NoError(t, tx.Commit(ctx))
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.Error(t, err, "stale validated cut cannot authorize a forward call after abandonment")
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "voice")
	require.Error(t, err)
	var required []string
	require.NoError(t, pool.QueryRow(ctx, `SELECT required_providers FROM matchmaking_match_squad_teardowns WHERE match_id=$1`, intent.MatchID).Scan(&required))
	require.Equal(t, []string{"chat"}, required)
	require.NoError(t, pool.QueryRow(ctx, `SELECT voice_dispatch_state FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&voiceState))
	require.Equal(t, "NOT_ISSUED", voiceState)
}

func TestValidatedIntentMissingProofOrChangedCutWritesNothing(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := dispatchFixture(t)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, nil, intent)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET recovery_generation=recovery_generation+1 WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	_, err = st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
	require.Error(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&count))
	require.Zero(t, count)
}

func TestValidatedForwardBoundariesCheckNonInitiatingMemberBan(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := dispatchFixture(t)
	bans := &store.BanStore{Pool: pool}
	memberAccount := p.MemberAuthorities[1].AccountID
	require.NotEqual(t, p.InitiatorAccountID, memberAccount)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	ban := store.InsertPlatformMMBanParams{AccountID: memberAccount, Reason: "test member ban", BannedByProfileID: uuid.New()}
	require.NoError(t, bans.InsertPlatformMMBan(ctx, ban))
	_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
	require.ErrorIs(t, err, store.ErrPlatformBanDenied)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, bans.RevokePlatformMMBan(ctx, memberAccount))
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	_, err = st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
	require.NoError(t, err)
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	require.NoError(t, bans.InsertPlatformMMBan(ctx, ban))
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.ErrorIs(t, err, store.ErrPlatformBanDenied)
	var chatState, voiceState string
	require.NoError(t, pool.QueryRow(ctx, `SELECT chat_dispatch_state,voice_dispatch_state FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&chatState, &voiceState))
	require.Equal(t, "NOT_ISSUED", chatState)
	require.Equal(t, "NOT_ISSUED", voiceState)
	require.NoError(t, bans.RevokePlatformMMBan(ctx, memberAccount))
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	ticket, err := st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, ticket.FirstDispatchID)
}

func dispatchBothReceipts(t *testing.T, st *store.MatchStore, p store.CreateSourcePartyParams, intent store.MatchSquadProvisioningIntent) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
	require.NoError(t, err)
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.NoError(t, err)
	chatID, chatReceiptID := uuid.New(), uuid.New()
	chatReceipt := &chatv1.MatchSquadChatReceipt{ProtocolVersion: 1, ReceiptId: chatReceiptID.String(), OperationId: intent.ChatOperationID.String(),
		MatchId: intent.MatchID.String(), ChatId: chatID.String(), ParticipantManifestSha256: intent.ParticipantManifestHash,
		RequestSha256: intent.ChatRequestHash, CreatedAt: timestamppb.Now()}
	chatBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(chatReceipt)
	require.NoError(t, err)
	participants := make([]*chatv1.MatchSquadParticipant, 0, len(p.MemberProfileIDs))
	for _, profile := range p.MemberProfileIDs {
		participants = append(participants, &chatv1.MatchSquadParticipant{ProfileId: profile.String()})
	}
	voiceBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(&callsv1.CreateMatchSquadRoomRequest{
		ProtocolVersion: 1, OperationId: intent.VoiceOperationID.String(), MatchId: intent.MatchID.String(), Participants: participants,
		ParticipantManifestSha256: intent.ParticipantManifestHash, ChatCreationReceipt: chatReceipt})
	require.NoError(t, err)
	_, err = st.RecordMatchSquadChatReceipt(ctx, intent.MatchID, chatReceiptID, chatID, chatBytes, dispatchDigest(voiceBytes), voiceBytes)
	require.NoError(t, err)
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "voice")
	require.NoError(t, err)
	roomID, voiceReceiptID := uuid.New(), uuid.New()
	voiceReceipt, err := proto.MarshalOptions{Deterministic: true}.Marshal(&callsv1.MatchSquadRoomReceipt{
		ProtocolVersion: 1, ReceiptId: voiceReceiptID.String(), OperationId: intent.VoiceOperationID.String(), MatchId: intent.MatchID.String(),
		RoomId: roomID.String(), ChatId: chatID.String(), ChatCreationReceiptId: chatReceiptID.String(), ChatCreationReceiptSha256: dispatchDigest(chatBytes),
		ParticipantManifestSha256: intent.ParticipantManifestHash, RequestSha256: dispatchDigest(voiceBytes), CreatedAt: timestamppb.Now()})
	require.NoError(t, err)
	_, err = st.RecordMatchSquadVoiceReceipt(ctx, intent.MatchID, voiceReceiptID, roomID, voiceReceipt)
	require.NoError(t, err)
	return chatID, roomID
}

func TestValidatedActivationChecksFreshCutAndAllMemberBan(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := dispatchFixture(t)
	chatID, roomID := dispatchBothReceipts(t, st, p, intent)
	_, legacyErr := st.ActivateProvisionedMatch(ctx, intent.MatchID)
	require.Error(t, legacyErr, "legacy API cannot bypass protocol1's fresh private proof")
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	bans := &store.BanStore{Pool: pool}
	memberAccount := p.MemberAuthorities[1].AccountID
	require.NoError(t, bans.InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: memberAccount, Reason: "test final gate", BannedByProfileID: uuid.New()}))
	_, err := st.ActivateValidatedProvisionedMatch(ctx, candidate, proof, intent.OperationID)
	require.ErrorIs(t, err, store.ErrPlatformBanDenied)
	require.NoError(t, bans.RevokePlatformMMBan(ctx, memberAccount))
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET recovery_generation=recovery_generation+1 WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	_, err = st.ActivateValidatedProvisionedMatch(ctx, candidate, proof, intent.OperationID)
	require.ErrorIs(t, err, store.ErrProvisionCandidateChanged)
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	active, err := st.ActivateValidatedProvisionedMatch(ctx, candidate, proof, intent.OperationID)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusActive, active.Status)
	require.NotNil(t, active.ChatID)
	require.NotNil(t, active.VoiceRoomID)
	require.Equal(t, chatID.String(), *active.ChatID)
	require.Equal(t, roomID.String(), *active.VoiceRoomID)
	var partyID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM parties WHERE search_operation_id=$1`, p.SearchOperationID).Scan(&partyID))
	_, err = st.ResetVoiceParties(ctx, []uuid.UUID{partyID})
	require.NoError(t, err)
	match, err := st.Get(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusActive, match.Status, "pre-match reset cannot auto-end an active winner")
}

func TestValidatedActivationLosesToCommittedAbandonment(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := dispatchFixture(t)
	dispatchBothReceipts(t, st, p, intent)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `UPDATE matches SET status='abandoned' WHERE id=$1`, intent.MatchID)
	require.NoError(t, err)
	require.NoError(t, store.LatchMatchSquadAbandonmentTx(ctx, tx, intent.MatchID, "VOICE_ROSTER_CHANGED", [32]byte{1}))
	require.NoError(t, tx.Commit(ctx))
	_, err = st.ActivateValidatedProvisionedMatch(ctx, candidate, proof, intent.OperationID)
	require.Error(t, err)
	var disposition, state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT provision_disposition,state FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&disposition, &state))
	require.Equal(t, "ABANDONED", disposition)
	require.Equal(t, "compensating", state)
	var required []string
	require.NoError(t, pool.QueryRow(ctx, `SELECT required_providers FROM matchmaking_match_squad_teardowns WHERE match_id=$1`, intent.MatchID).Scan(&required))
	require.Equal(t, []string{"chat", "voice"}, required)
}
