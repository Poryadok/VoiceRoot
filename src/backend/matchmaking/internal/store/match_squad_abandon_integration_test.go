package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
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
}
