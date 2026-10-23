package store

import (
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestBotPinCoverageRejectsDifferentExplicitIncarnation(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL retained coverage incarnation")
	}
	for _, mixed := range []bool{false, true} {
		t.Run(map[bool]string{false: "covered_only", true: "mixed_uncovered"}[mixed], func(t *testing.T) {
			s, first, message := newBotPinRemovalFixture(t)
			ctx := t.Context()
			_, err := s.ApplyBotChatPinRemoval(ctx, first)
			require.NoError(t, err)
			require.NoError(t, s.UpsertPin(ctx, uuid.MustParse(first.ChatId), message, uuid.New()))
			next := nextBotPinDecision(first, "global_delete")
			next.Attachments[0].CurrentScope.Response.Scope.ChatScopeIncarnation = uuid.NewString()
			if mixed {
				newTarget := unqualifiedPinAttachment()
				newTarget.CurrentScope = proto.Clone(first.Attachments[0].CurrentScope).(*messagingv1.BotRemovalChatScopeObservation)
				next.Attachments = append(next.Attachments, newTarget)
				sort.Slice(next.Attachments, func(i, j int) bool { return next.Attachments[i].AttachmentId < next.Attachments[j].AttachmentId })
			}
			_, err = s.ApplyBotChatPinRemoval(ctx, next)
			require.ErrorIs(t, err, ErrBotPinRemovalConflict, "explicit new incarnation cannot borrow historical coverage or fall back to DELETE")
			pins, err := s.ListPins(ctx, uuid.MustParse(first.ChatId))
			require.NoError(t, err)
			require.Len(t, pins, 1)
			var revision, receipts, decisions int
			require.NoError(t, s.Pool.QueryRow(ctx, `SELECT revision FROM bot_pin_removal_revisions WHERE chat_id=$1 AND actor_profile_id=$2`, first.ChatId, first.ActorProfileId).Scan(&revision))
			require.Equal(t, 1, revision)
			require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_pin_removal_receipts WHERE operation_id=$1`, next.OperationId).Scan(&receipts))
			require.Zero(t, receipts)
			require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_pin_removal_decisions WHERE decision_id=$1`, next.DecisionId).Scan(&decisions))
			require.Zero(t, decisions)
			// A newer read operation/revision of the SAME incarnation is eligible,
			// and historical adoption without a current observation needs no datasource.
			for _, absent := range []bool{false, true} {
				accepted := nextBotPinDecision(first, "global_delete")
				if absent {
					accepted.Attachments[0].CurrentScope = nil
				} else {
					prepareBotPinCurrentScope(t, s, accepted)
					accepted.Attachments[0].CurrentScope.Response.Scope.ChatAuthorityRevision++
				}
				s.BotPinChatScope = nil
				response, err := s.ApplyBotChatPinRemoval(ctx, accepted)
				require.NoError(t, err)
				require.Zero(t, response.Receipt.RemovedPinCount)
				require.Zero(t, response.Receipt.ExecutionCleanupRevision)
			}
		})
	}
}
