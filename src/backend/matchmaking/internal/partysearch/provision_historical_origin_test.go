package partysearch

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"testing"
	"time"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/matchmaking/internal/store"
)

func TestHistoricalDispatchOriginRequiresFreshProofAndRejectsLegacyActivation(t *testing.T) {
	for _, known := range []bool{false, true} {
		name := "unknown_chat"
		if known {
			name = "both_known"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool, st, p, intent := dispatchFixture(t)
			// Independent protocol fixture: preserve a freshly qualified private source
			// cut, remove only021 while its operation inventory is empty, then exercise
			// genuine old intent/receipt APIs and the actual021 backfill. This does not
			// retrofit private authority onto a pre022 capture or qualify a rollout.
			down, err := os.ReadFile("../../../migrations/matchmaking_db/000021_match_squad_dispatch_settlement.down.sql")
			require.NoError(t, err)
			_, err = pool.Exec(ctx, string(down))
			require.NoError(t, err)
			if known {
				historicalBothReceipts(t, st, p, intent, true)
			} else {
				_, err = st.SaveMatchSquadProvisioningIntent(ctx, intent)
				require.NoError(t, err)
			}
			up, err := os.ReadFile("../../../migrations/matchmaking_db/000021_match_squad_dispatch_settlement.up.sql")
			require.NoError(t, err)
			_, err = pool.Exec(ctx, string(up))
			require.NoError(t, err)
			var origin int16
			var first uuid.UUID
			require.NoError(t, pool.QueryRow(ctx, `SELECT dispatch_protocol_version,chat_first_dispatch_id FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&origin, &first))
			require.Zero(t, origin)
			candidate, proof := dispatchProof(t, st, intent.MatchID, p)
			if known {
				_, err = st.ActivateProvisionedMatch(ctx, intent.MatchID)
				require.Error(t, err, "upgraded historical rows cannot bypass the fresh private proof")
				_, err = st.ActivateValidatedProvisionedMatch(ctx, candidate, proof, intent.OperationID)
				require.NoError(t, err)
			} else {
				ticket, e := st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
				require.NoError(t, e)
				require.Equal(t, first, ticket.FirstDispatchID, "historical possible dispatch remains the same obligation")
				require.Equal(t, intent.ChatOperationID, ticket.Operation.ChatOperationID)
				require.Equal(t, intent.ChatRequestBytes, ticket.Operation.ChatRequestBytes)
			}
			require.NoError(t, pool.QueryRow(ctx, `SELECT dispatch_protocol_version FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&origin))
			require.Zero(t, origin, "historical origin is preserved, not rewritten into new dispatch evidence")
		})
	}
}

func TestProvisionOutcomeIncludesNonretryableTeardownMismatch(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		name := "both_known"
		if unknown {
			name = "voice_unknown"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool, st, p, intent := dispatchFixture(t)
			if unknown {
				down, e := os.ReadFile("../../../migrations/matchmaking_db/000021_match_squad_dispatch_settlement.down.sql")
				require.NoError(t, e)
				_, e = pool.Exec(ctx, string(down))
				require.NoError(t, e)
				historicalBothReceipts(t, st, p, intent, false)
				up, e := os.ReadFile("../../../migrations/matchmaking_db/000021_match_squad_dispatch_settlement.up.sql")
				require.NoError(t, e)
				_, e = pool.Exec(ctx, string(up))
				require.NoError(t, e)
			} else {
				dispatchBothReceipts(t, st, p, intent)
			}
			require.NoError(t, st.AbandonMatch(ctx, intent.MatchID))
			pending, err := st.ListPendingMatchSquadTeardownParticipants(ctx, 10)
			require.NoError(t, err)
			var chat store.MatchSquadTeardownParticipant
			for _, item := range pending {
				if item.Provider == "chat" {
					chat = item
				}
			}
			require.NotEqual(t, uuid.Nil, chat.AggregateID)
			claimed, ok, err := st.ClaimMatchSquadTeardownParticipant(ctx, chat, 30*time.Second)
			require.NoError(t, err)
			require.True(t, ok)
			require.NoError(t, st.MarkMatchSquadTeardownContractMismatch(ctx, claimed, "receipt_mismatch"))
			outcome, err := st.GetMatchSquadProvisioningOutcome(ctx, intent.MatchID)
			require.NoError(t, err)
			require.Equal(t, store.MatchSquadOutcomeContractMismatch, outcome.State, "nonretryable cleanup dominates even an unresolved other creation")
			require.Empty(t, outcome.ChatID)
			require.Empty(t, outcome.VoiceRoomID)
			if !unknown {
				aggregate, e := st.GetMatchSquadTeardown(ctx, chat.AggregateID)
				require.NoError(t, e)
				request := new(callsv1.TeardownMatchSquadRoomRequest)
				require.NoError(t, proto.Unmarshal(aggregate.VoiceRequestBytes, request))
				receiptID := uuid.New()
				raw, e := proto.MarshalOptions{Deterministic: true}.Marshal(&callsv1.MatchSquadRoomTeardownReceipt{ProtocolVersion: 1, ReceiptId: receiptID.String(),
					TeardownOperationId: request.TeardownOperationId, MatchId: request.MatchId, RoomId: request.RoomId, CreationReceiptId: request.CreationReceiptId,
					ParticipantManifestSha256: request.ParticipantManifestSha256, RequestSha256: dispatchDigest(aggregate.VoiceRequestBytes),
					Status: callsv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, CompletedAt: timestamppb.Now()})
				require.NoError(t, e)
				aggregate, e = st.RecordMatchSquadTeardownReceipt(ctx, aggregate.AggregateID, "voice", receiptID, raw)
				require.NoError(t, e)
				require.NotNil(t, aggregate.VoiceReceiptID)
				require.Equal(t, "pending", aggregate.State)
				outcome, e = st.GetMatchSquadProvisioningOutcome(ctx, intent.MatchID)
				require.NoError(t, e)
				require.Equal(t, store.MatchSquadOutcomeContractMismatch, outcome.State, "independent known cleanup cannot erase the failed participant")
			}
		})
	}
}

func historicalBothReceipts(t *testing.T, st *store.MatchStore, p store.CreateSourcePartyParams, intent store.MatchSquadProvisioningIntent, voiceKnown bool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	_, err := st.SaveMatchSquadProvisioningIntent(ctx, intent)
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
	if !voiceKnown {
		return chatID, uuid.Nil
	}
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
