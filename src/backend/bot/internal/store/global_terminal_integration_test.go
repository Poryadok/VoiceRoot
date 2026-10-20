package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	botv1 "voice.app/voice/bot/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func TestBotDeletionZeroSpacesStillRequiresGlobalTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL global terminal obligation")
	}
	st := startBotStore(t)
	ctx := t.Context()
	owner, actor := uuid.New(), uuid.New()
	bot, token, err := st.CreateBot(ctx, owner, "Zero-space terminal", "", `[]`, actor)
	require.NoError(t, err)
	q := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	root, err := st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	// Mandatory first effects remain atomic even though terminal delivery is pending.
	disabled, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "disabled", disabled.Status)
	_, err = st.GetBotByTokenHash(ctx, store.HashToken(token))
	require.ErrorIs(t, err, store.ErrNotFound)
	require.NotEmpty(t, root.DecisionBytes)
	require.Len(t, root.DecisionSHA256, 32)
	require.Equal(t, "PENDING", root.State, "no Space targets does not prove an all-Add global fence")
	claim, err := st.ClaimBotDeletion(ctx, root.OperationID)
	require.NoError(t, err)
	require.ErrorIs(t, st.CompleteBotDeletion(ctx, claim), store.ErrDeletionPending, "empty per-Space effect set cannot discharge the independent terminal obligation")
}

func TestBotGlobalTerminalClaimExpiresBehindActualEffectLock(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL terminal row wait and claim expiry")
	}
	for _, mode := range []string{"seal", "save"} {
		t.Run(mode, func(t *testing.T) {
			st := startBotStore(t)
			ctx := t.Context()
			bot, _, err := st.CreateBot(ctx, uuid.New(), "Wait expiry", "", `[]`, uuid.New())
			require.NoError(t, err)
			root, err := st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
			require.NoError(t, err)
			claim, err := st.ClaimBotDeletion(ctx, root.OperationID)
			require.NoError(t, err)
			q := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: bot.ID.String(), DecisionId: root.OperationID.String()}
			body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
			require.NoError(t, err)
			r := &spacev1.RegisterBotGlobalTerminalResponse{Receipt: &spacev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, BotId: q.BotId, DecisionId: q.DecisionId, ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED}}
			receipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
			require.NoError(t, err)
			if mode == "save" {
				require.NoError(t, st.SealBotGlobalSpaceRequest(ctx, claim, body))
			}
			blocker, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback(context.Background()) }()
			_, err = blocker.Exec(ctx, `SELECT operation_id FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND participant='space' FOR UPDATE`, root.OperationID)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `UPDATE bot_deletion_operations SET claimed_until=clock_timestamp()+interval '500 milliseconds' WHERE operation_id=$1`, root.OperationID)
			require.NoError(t, err)
			done := make(chan error, 1)
			go func() {
				if mode == "seal" {
					done <- st.SealBotGlobalSpaceRequest(ctx, claim, body)
				} else {
					done <- st.SaveBotGlobalSpaceReceipt(ctx, claim, body, receipt)
				}
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'SELECT request_bytes,%bot_deletion_terminal_effects%')`).Scan(&waiting) == nil && waiting
			}, 2*time.Second, 5*time.Millisecond)
			time.Sleep(600 * time.Millisecond)
			require.NoError(t, blocker.Rollback(ctx))
			require.ErrorIs(t, <-done, store.ErrDeletionClaimLost)
			var savedRequest, savedReceipt []byte
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT request_bytes,receipt_bytes FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND participant='space'`, root.OperationID).Scan(&savedRequest, &savedReceipt))
			require.Nil(t, savedReceipt)
			if mode == "seal" {
				require.Nil(t, savedRequest)
			} else {
				require.Equal(t, body, savedRequest)
			}
		})
	}
}

func TestBotGlobalTerminalJournalReceiptRetryAndIndependentRoleGate(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL global receipt recovery")
	}
	st := startBotStore(t)
	ctx := t.Context()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Terminal journal", "", `[]`, uuid.New())
	require.NoError(t, err)
	root, err := st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
	require.NoError(t, err)
	claim, err := st.ClaimBotDeletion(ctx, root.OperationID)
	require.NoError(t, err)
	q := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: bot.ID.String(), DecisionId: root.OperationID.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	// Controlled trusted store caller only: this does not attest remote delivery.
	r := &spacev1.RegisterBotGlobalTerminalResponse{Receipt: &spacev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, BotId: q.BotId, DecisionId: q.DecisionId, ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED}}
	receipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	require.NoError(t, err)
	require.NoError(t, st.SealBotGlobalSpaceRequest(ctx, claim, body))
	_, err = st.Pool.Exec(ctx, `UPDATE bot_deletion_operations SET claimed_until=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, root.OperationID)
	require.NoError(t, err)
	next, err := st.ClaimBotDeletion(ctx, root.OperationID)
	require.NoError(t, err)
	require.ErrorIs(t, st.SaveBotGlobalSpaceReceipt(ctx, claim, body, receipt), store.ErrDeletionClaimLost)
	require.NoError(t, st.SealBotGlobalSpaceRequest(ctx, next, body))
	require.NoError(t, st.SaveBotGlobalSpaceReceipt(ctx, next, body, receipt))
	require.NoError(t, st.SaveBotGlobalSpaceReceipt(ctx, next, body, receipt))
	effects, err := st.GlobalTerminalEffects(ctx, next)
	require.NoError(t, err)
	require.Equal(t, "role", effects[0].Participant)
	require.Equal(t, "PENDING", effects[0].State)
	require.Equal(t, "COMPLETE", effects[1].State)
	require.Equal(t, receipt, effects[1].ReceiptBytes)
	require.ErrorIs(t, st.CompleteBotDeletion(ctx, next), store.ErrDeletionPending, "Space receipt does not fabricate independent Role fence")
	r.Receipt.ReceiptId = uuid.NewString()
	changed, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	require.NoError(t, err)
	require.ErrorIs(t, st.SaveBotGlobalSpaceReceipt(ctx, next, body, changed), store.ErrInstallationConflict)
	q.DecisionId = uuid.NewString()
	wrong, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	require.ErrorIs(t, st.SealBotGlobalSpaceRequest(ctx, next, wrong), store.ErrInstallationConflict)
	ordinary := *next
	ordinary.Purpose = "space_uninstall"
	require.ErrorIs(t, st.SealBotGlobalSpaceRequest(ctx, &ordinary, body), store.ErrInstallationConflict)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_deletion_terminal_effects SET receipt_bytes=$2 WHERE operation_id=$1 AND participant='space'`, root.OperationID, changed)
	require.NoError(t, err)
	_, err = st.GlobalTerminalEffects(ctx, next)
	require.ErrorIs(t, err, store.ErrInstallationConflict)
}
