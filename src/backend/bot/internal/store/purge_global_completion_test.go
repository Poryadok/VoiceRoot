package store_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	botv1 "voice.app/voice/bot/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

// These are controlled typed participant results saved through the production
// seal/validation/CAS methods, not a claim of remote terminal delivery.
func savePurgeGlobalTerminals(t *testing.T, st *store.BotStore, claim *store.BotDeletion, skipSpace bool) {
	t.Helper()
	ctx := t.Context()
	marshal := func(v proto.Message) []byte {
		b, e := (proto.MarshalOptions{Deterministic: true}).Marshal(v)
		require.NoError(t, e)
		return b
	}
	role := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: claim.BotID.String(), DecisionId: claim.OperationID.String()}
	roleBody := marshal(role)
	require.NoError(t, st.SealBotGlobalRoleRequest(ctx, claim, roleBody))
	require.NoError(t, st.SaveBotGlobalRoleReceipt(ctx, claim, roleBody, marshal(&rolev1.RegisterBotGlobalTerminalResponse{Receipt: &rolev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, Participant: "role", BotId: role.BotId, DecisionId: role.DecisionId, ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED}})))
	if skipSpace {
		return
	}
	space := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: claim.BotID.String(), DecisionId: claim.OperationID.String()}
	spaceBody := marshal(space)
	require.NoError(t, st.SealBotGlobalSpaceRequest(ctx, claim, spaceBody))
	require.NoError(t, st.SaveBotGlobalSpaceReceipt(ctx, claim, spaceBody, marshal(&spacev1.RegisterBotGlobalTerminalResponse{Receipt: &spacev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, BotId: space.BotId, DecisionId: space.DecisionId, ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED}})))
}

func TestGlobalDeletionCompletesQualifiedLocalP3TargetsOnlyWithAllObligations(t *testing.T) {
	if testing.Short() {
		t.Skip("real PG final global reducer after qualified local P3 retirement")
	}
	for _, mode := range []string{"complete", "other target pending", "missing target", "missing effect", "pending terminal", "missing terminal"} {
		t.Run(mode, func(t *testing.T) {
			st := startBotLifecycleStore(t)
			ctx := t.Context()
			owner, actor := uuid.New(), uuid.New()
			bot, _, e := st.CreateBot(ctx, owner, "Global local retirement", "", `[]`, actor)
			require.NoError(t, e)
			spaces := []uuid.UUID{uuid.New(), uuid.New()}
			for _, space := range spaces {
				i, e := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
				require.NoError(t, e)
				require.NoError(t, st.CompleteInstallation(ctx, i, nil))
			}
			q := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
			body, e := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
			require.NoError(t, e)
			hash, e := principal.RequestHash(q)
			require.NoError(t, e)
			root, e := st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
			require.NoError(t, e)
			var target uuid.UUID
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT target_id FROM bot_deletion_installations WHERE operation_id=$1 AND space_id=$2`, root.OperationID, spaces[0]).Scan(&target))
			for n, space := range spaces {
				if n == 1 && mode == "other target pending" {
					break
				}
				p := preparePurgeHandoffDecision(t, st, space)
				_, e = st.PurgeSpace(ctx, p)
				require.ErrorIs(t, e, store.ErrDeletionPending)
				c, e := st.ClaimNextBotPurgeHandoff(ctx)
				require.NoError(t, e)
				saveRetirementEvidence(t, st, c, bot.ID, true)
				_, e = st.RetireBotPurgeHandoff(ctx, c)
				require.NoError(t, e)
			}
			claim, e := st.ClaimBotDeletion(ctx, root.OperationID)
			require.NoError(t, e)
			savePurgeGlobalTerminals(t, st, claim, mode == "pending terminal")
			switch mode {
			case "missing target":
				_, e = st.Pool.Exec(ctx, `DELETE FROM bot_deletion_effects WHERE target_id=$1`, target)
				require.NoError(t, e)
				_, e = st.Pool.Exec(ctx, `DELETE FROM bot_deletion_installations WHERE target_id=$1`, target)
			case "missing effect":
				_, e = st.Pool.Exec(ctx, `DELETE FROM bot_deletion_effects WHERE effect_id=(SELECT effect_id FROM bot_deletion_effects WHERE target_id=$1 LIMIT 1)`, target)
			case "missing terminal":
				_, e = st.Pool.Exec(ctx, `DELETE FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND participant='space'`, root.OperationID)
			}
			require.NoError(t, e)
			e = st.CompleteBotDeletion(ctx, claim)
			if mode == "complete" {
				require.NoError(t, e, "all exact local targets and both own typed terminals must allow final global completion")
			} else if mode == "missing target" || mode == "missing effect" {
				require.ErrorIs(t, e, store.ErrInstallationConflict)
			} else {
				require.ErrorIs(t, e, store.ErrDeletionPending)
			}
			var state, status string
			var unresolved bool
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT o.state,b.status,b.retired_cleanup_unresolved FROM bot_deletion_operations o JOIN bots b ON b.id=o.bot_id WHERE o.operation_id=$1`, root.OperationID).Scan(&state, &status, &unresolved))
			require.Equal(t, "disabled", status)
			require.False(t, unresolved)
			if mode == "complete" {
				require.Equal(t, "COMPLETE", state)
			} else {
				require.NotEqual(t, "COMPLETE", state)
			}
		})
	}
}
