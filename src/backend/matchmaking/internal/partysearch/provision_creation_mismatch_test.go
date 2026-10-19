package partysearch

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/squad"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/principal"
)

func TestProviderContradictoryCreationRemainsStickyAcrossRetryAndFreshCut(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := dispatchFixture(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "matchmaking", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	var witness []byte
	chat := &gateChatClient{create: func(request *chatv1.CreateMatchSquadChatRequest) (*chatv1.CreateMatchSquadChatResponse, error) {
		raw, e := proto.MarshalOptions{Deterministic: true}.Marshal(request)
		require.NoError(t, e)
		receipt := &chatv1.MatchSquadChatReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: request.OperationId, MatchId: uuid.NewString(), ChatId: uuid.NewString(),
			ParticipantManifestSha256: request.ParticipantManifestSha256, RequestSha256: dispatchDigest(raw), CreatedAt: timestamppb.Now()}
		witness, e = proto.MarshalOptions{Deterministic: true}.Marshal(&chatv1.CreateMatchSquadChatResponse{Receipt: receipt})
		require.NoError(t, e)
		return &chatv1.CreateMatchSquadChatResponse{Receipt: receipt}, nil
	}}
	voice := &gateVoiceClient{}
	validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion,
			MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}
	worker := &squad.MatchSquadProviderWorker{Store: st, Issuer: issuer, Chat: chat, Voice: voice, PartyValidator: validator}
	_, _, err = worker.Provision(ctx, intent.MatchID, p.MemberProfileIDs)
	require.ErrorIs(t, err, store.ErrMatchSquadCreationMismatch)
	require.ErrorIs(t, err, squad.ErrProvisionOperatorAttention)
	require.Equal(t, 1, chat.calls)
	require.Zero(t, voice.calls)
	var actual []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT chat_creation_mismatch_bytes FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&actual))
	require.Equal(t, witness, actual)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.ErrorIs(t, err, store.ErrMatchSquadCreationMismatch, "a new valid source cut cannot erase a provider contradiction")
	_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET provision_next_attempt_at=clock_timestamp()-interval '1 second',provision_last_error_class='unavailable' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	restarted := *worker
	_, err = restarted.RunProvisioningOnce(ctx, 10)
	require.ErrorIs(t, err, squad.ErrProvisionOperatorAttention)
	require.Equal(t, 1, chat.calls, "LIVE mismatch cannot dispatch another forward request")
	require.Zero(t, voice.calls)
	attention, err := st.MatchSquadProvisioningAttention(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, attention.ContractMismatch)
}
