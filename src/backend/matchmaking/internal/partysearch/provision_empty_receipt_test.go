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
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/squad"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/principal"
)

func TestPresentEmptyCreationReceiptKeepsWirePresenceAcrossRestart(t *testing.T) {
	for _, provider := range []string{"chat", "voice"} {
		t.Run(provider, func(t *testing.T) {
			ctx := context.Background()
			pool, st, p, intent := dispatchFixture(t)
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			require.NoError(t, err)
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "matchmaking", KeyID: "test", PrivateKey: key})
			require.NoError(t, err)
			chat := &gateChatClient{create: func(request *chatv1.CreateMatchSquadChatRequest) (*chatv1.CreateMatchSquadChatResponse, error) {
				if provider == "chat" {
					return &chatv1.CreateMatchSquadChatResponse{Receipt: &chatv1.MatchSquadChatReceipt{}}, nil
				}
				raw, e := proto.MarshalOptions{Deterministic: true}.Marshal(request)
				require.NoError(t, e)
				return &chatv1.CreateMatchSquadChatResponse{Receipt: &chatv1.MatchSquadChatReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: request.OperationId, MatchId: request.MatchId, ChatId: uuid.NewString(),
					ParticipantManifestSha256: request.ParticipantManifestSha256, RequestSha256: dispatchDigest(raw), CreatedAt: timestamppb.Now()}}, nil
			}}
			voice := &gateVoiceClient{create: func(*callsv1.CreateMatchSquadRoomRequest) (*callsv1.CreateMatchSquadRoomResponse, error) {
				return &callsv1.CreateMatchSquadRoomResponse{Receipt: &callsv1.MatchSquadRoomReceipt{}}, nil
			}}
			validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
				return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs,
					MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
			})}
			worker := &squad.MatchSquadProviderWorker{Store: st, Issuer: issuer, Chat: chat, Voice: voice, PartyValidator: validator}
			_, _, err = worker.Provision(ctx, intent.MatchID, p.MemberProfileIDs)
			require.ErrorIs(t, err, store.ErrMatchSquadCreationMismatch)
			var witness []byte
			require.NoError(t, pool.QueryRow(ctx, `SELECT CASE WHEN $2='chat' THEN chat_creation_mismatch_bytes ELSE voice_creation_mismatch_bytes END FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID, provider).Scan(&witness))
			var expected []byte
			if provider == "chat" {
				expected, err = proto.Marshal(&chatv1.CreateMatchSquadChatResponse{Receipt: &chatv1.MatchSquadChatReceipt{}})
			} else {
				expected, err = proto.Marshal(&callsv1.CreateMatchSquadRoomResponse{Receipt: &callsv1.MatchSquadRoomReceipt{}})
			}
			require.NoError(t, err)
			require.NotEmpty(t, expected)
			require.Equal(t, expected, witness, "outer response preserves positive receipt presence")
			calls := chat.calls + voice.calls
			_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET provision_next_attempt_at=clock_timestamp()-interval '1 second',provision_last_error_class='unavailable' WHERE match_id=$1`, intent.MatchID)
			require.NoError(t, err)
			restarted := *worker
			_, err = restarted.RunProvisioningOnce(ctx, 10)
			require.ErrorIs(t, err, squad.ErrProvisionOperatorAttention)
			require.Equal(t, calls, chat.calls+voice.calls, "restart cannot erase positive empty-receipt evidence")
			require.ErrorIs(t, st.CheckMatchSquadCreationEvidence(ctx, intent.MatchID), store.ErrMatchSquadCreationMismatch)
		})
	}
}
