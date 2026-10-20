package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func acceptedPurgeUninstall(t *testing.T, st *store.BotStore, bot, owner, actor, space uuid.UUID) (*store.InstallationIntent, *store.BotDeletion) {
	t.Helper()
	q := &botv1.UninstallBotFromSpaceRequest{BotId: bot.String(), SpaceId: space.String()}
	raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, e)
	hash, e := principal.RequestHash(q)
	require.NoError(t, e)
	i, e := st.BeginInstallationCancellationWithAdmission(t.Context(), bot, space, actor, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_UninstallBotFromSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: raw, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, e)
	root, e := st.UninstallCleanup(t.Context(), i.InstallationID)
	require.NoError(t, e)
	return i, root
}

func saveRetirementEvidence(t *testing.T, st *store.BotStore, c *store.BotPurgeClaim, bot uuid.UUID, native bool) {
	t.Helper()
	p := c.PurgeRequest()
	proof := purgeRecoveryRoleProof(t, p)
	batches, e := c.ChatBatches(bot)
	require.NoError(t, e)
	var proofs []store.BotPurgeCoverageProof
	domain := func(name string, raw []byte) []byte {
		sum := sha256.Sum256(append(append([]byte(name), 0), raw...))
		return sum[:]
	}
	var messaging *spacev1.BotPurgeMessagingHandoff
	for _, batch := range batches {
		v := proof
		if len(batch) > 0 {
			q := proto.Clone(p).(*commonv1.SpacePurgeRequest)
			q.ParticipantId = commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING
			raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(&messagingv1.PurgeSpaceRequest{Purge: q})
			require.NoError(t, e)
			hash := domain("voice.messaging.v1.PurgeSpaceRequest", raw)
			r := &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: hash, CompletedAt: proto.Clone(p.PurgeDecidedAt).(*timestamppb.Timestamp)}
			rawReceipt, e := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
			require.NoError(t, e)
			if messaging == nil {
				messaging = &spacev1.BotPurgeMessagingHandoff{OriginalReceipt: r, OriginalRequestBytes: raw, OriginalRequestSha256: hash, OriginalReceiptBytes: rawReceipt, OriginalReceiptSha256: domain("voice.common.v1.SpacePurgeReceipt", rawReceipt)}
			}
			v.Handoff = proto.Clone(proof.Handoff).(*spacev1.GetBotPurgeHandoffResponse)
			v.Handoff.Messaging = proto.Clone(messaging).(*spacev1.BotPurgeMessagingHandoff)
			v.CoverageRequest = &messagingv1.GetBotPurgeChatCoverageRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding), ChatIds: batch}
			h, e := principal.RequestHash(v.CoverageRequest)
			require.NoError(t, e)
			digest, e := hex.DecodeString(strings.TrimPrefix(h, "sha256:"))
			require.NoError(t, e)
			v.Coverage = &messagingv1.GetBotPurgeChatCoverageResponse{ProtocolVersion: 1, RequestId: v.CoverageRequest.RequestId, SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding), SourceScheduleGeneration: 7, RequestSha256: digest}
			for _, id := range batch {
				v.Coverage.Coverage = append(v.Coverage.Coverage, &messagingv1.BotPurgeChatCoverage{ChatId: id, Covered: true})
			}
		}
		proofs = append(proofs, v)
	}
	require.NoError(t, st.SaveBotPurgeCoverageObservation(t.Context(), c, bot, proofs))
	if !native {
		return
	}
	requests, e := c.NativeDrainRequests(bot)
	require.NoError(t, e)
	for _, q := range requests {
		op := uuid.MustParse(q.OperationId)
		sealed, e := st.SealBotPurgeNativeRequest(t.Context(), c, bot, op)
		require.NoError(t, e)
		r := controlledPurgeNativeResult(t, q)
		raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
		require.NoError(t, e)
		require.NoError(t, st.SaveBotPurgeNativeReceipt(t.Context(), c, bot, op, sealed.RequestBytes, raw))
	}
}

func retirementFixture(t *testing.T) (*store.BotStore, uuid.UUID, *store.BotDeletion, *store.BotPurgeClaim, *commonv1.SpacePurgeRequest) {
	t.Helper()
	st := startBotLifecycleStore(t)
	owner, actor, space := uuid.New(), uuid.New(), uuid.New()
	bot, _, e := st.CreateBot(t.Context(), owner, "Local P3 target", "", `[]`, actor)
	require.NoError(t, e)
	i, e := st.PrepareInstallation(t.Context(), bot.ID, space, actor, nil)
	require.NoError(t, e)
	require.NoError(t, st.CompleteInstallation(t.Context(), i, nil))
	_, root := acceptedPurgeUninstall(t, st, bot.ID, owner, actor, space)
	p := preparePurgeHandoffDecision(t, st, space)
	_, e = st.PurgeSpace(t.Context(), p)
	require.ErrorIs(t, e, store.ErrDeletionPending)
	c, e := st.ClaimNextBotPurgeHandoff(t.Context())
	require.NoError(t, e)
	return st, bot.ID, root, c, p
}

func TestLocalP3TargetRetirementPreservesOpaqueLedgerAfterIntentAndProofRemoval(t *testing.T) {
	if testing.Short() {
		t.Skip("real BotPG accepted local P3 target retirement")
	}
	st, bot, root, c, p := retirementFixture(t)
	ctx := t.Context()
	saveRetirementEvidence(t, st, c, bot, true)
	var target uuid.UUID
	var effects []uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT target_id,effect_ids FROM bot_uninstall_installations WHERE operation_id=$1`, root.OperationID).Scan(&target, &effects))
	require.Len(t, effects, 2)
	r, e := st.RetireBotPurgeHandoff(ctx, c)
	require.NoError(t, e)
	require.Equal(t, commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, r.State)
	var result uuid.UUID
	var kept []uuid.UUID
	var compact bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT local_result_id,effect_ids,disposition='LOCAL_P3_COVERED' AND bot_id IS NULL AND space_id IS NULL AND actor_profile_id IS NULL AND installation_id IS NULL AND install_operation_id IS NULL AND bot_generation IS NULL AND cancel_operation_id IS NULL AND chat_ids IS NULL AND legacy_evidence IS NULL AND scope_bytes IS NULL AND scope_sha256 IS NULL FROM bot_uninstall_installations WHERE target_id=$1`, target).Scan(&result, &kept, &compact))
	require.NotEqual(t, uuid.Nil, result)
	require.True(t, compact)
	require.Equal(t, effects, kept)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_effects WHERE target_id=$1 AND state='LOCAL_P3_COVERED' AND kind IS NULL AND chat_id IS NULL AND request_bytes IS NULL AND receipt_bytes IS NULL AND request_sha256 IS NULL AND receipt_sha256 IS NULL`, target).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_installation_intents WHERE space_id=$1`, p.SpaceId).Scan(&count))
	require.Zero(t, count)
	// Model full evidence already removed by its separate bounded compactor;
	// this is not an execution claim for the clock/compactor itself.
	_, e = st.Pool.Exec(ctx, `DELETE FROM bot_space_purge_sources WHERE space_id=$1;`, p.SpaceId)
	require.NoError(t, e)
	_, e = st.Pool.Exec(ctx, `DELETE FROM bot_space_purge_handoffs WHERE space_id=$1`, p.SpaceId)
	require.NoError(t, e)
	claimed, e := st.ClaimUninstallCleanup(ctx, root.OperationID)
	require.NoError(t, e)
	require.NoError(t, st.CompleteBotDeletion(ctx, claimed), "qualified local result completes accepted work without reconstructing a missing live intent or full proof")
	var saved uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT local_result_id FROM bot_uninstall_installations WHERE target_id=$1`, target).Scan(&saved))
	require.Equal(t, result, saved)
	replay, e := st.PurgeSpace(ctx, p)
	require.NoError(t, e)
	require.True(t, proto.Equal(r, replay))
}

func TestLocalP3RetirementRefusesLostAcceptedMembershipAndMissingNative(t *testing.T) {
	if testing.Short() {
		t.Skip("real BotPG no completion by missing ledger")
	}
	for _, mode := range []string{"missing native", "missing effect", "missing target", "extra effect"} {
		t.Run(mode, func(t *testing.T) {
			st, bot, root, c, p := retirementFixture(t)
			ctx := t.Context()
			saveRetirementEvidence(t, st, c, bot, mode != "missing native")
			var target uuid.UUID
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT target_id FROM bot_uninstall_installations WHERE operation_id=$1`, root.OperationID).Scan(&target))
			switch mode {
			case "missing effect":
				_, e := st.Pool.Exec(ctx, `DELETE FROM bot_uninstall_effects WHERE effect_id=(SELECT effect_id FROM bot_uninstall_effects WHERE target_id=$1 LIMIT 1)`, target)
				require.NoError(t, e)
			case "missing target":
				_, e := st.Pool.Exec(ctx, `DELETE FROM bot_uninstall_effects WHERE target_id=$1`, target)
				require.NoError(t, e)
				_, e = st.Pool.Exec(ctx, `DELETE FROM bot_uninstall_installations WHERE target_id=$1`, target)
				require.NoError(t, e)
			case "extra effect":
				_, e := st.Pool.Exec(ctx, `INSERT INTO bot_uninstall_effects(effect_id,target_id,kind,chat_id) VALUES($1,$2,'MESSAGING_UNPIN',$3)`, uuid.New(), target, uuid.New())
				require.NoError(t, e)
			}
			_, e := st.RetireBotPurgeHandoff(ctx, c)
			require.Error(t, e)
			var intents, receipts int
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_installation_intents WHERE space_id=$1`, p.SpaceId).Scan(&intents))
			require.Equal(t, 1, intents)
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_lifecycle_purge_receipts WHERE space_id=$1`, p.SpaceId).Scan(&receipts))
			require.Zero(t, receipts)
		})
	}
}

func TestLocalP3RetirementLinksLateGlobalTargetAndPreservesOtherObligations(t *testing.T) {
	if testing.Short() {
		t.Skip("real BotPG late accepted global lineage")
	}
	st := startBotLifecycleStore(t)
	ctx := t.Context()
	owner, actor, space, other := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, _, e := st.CreateBot(ctx, owner, "Late global target", "", `[]`, actor)
	require.NoError(t, e)
	for _, id := range []uuid.UUID{space, other} {
		i, e := st.PrepareInstallation(ctx, bot.ID, id, actor, nil)
		require.NoError(t, e)
		require.NoError(t, st.CompleteInstallation(ctx, i, nil))
	}
	p := preparePurgeHandoffDecision(t, st, space)
	_, e = st.PurgeSpace(ctx, p)
	require.ErrorIs(t, e, store.ErrDeletionPending)
	c, e := st.ClaimNextBotPurgeHandoff(ctx)
	require.NoError(t, e)
	saveRetirementEvidence(t, st, c, bot.ID, true)
	q := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	body, e := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, e)
	hash, e := principal.RequestHash(q)
	require.NoError(t, e)
	root, e := st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, e)
	_, e = st.RetireBotPurgeHandoff(ctx, c)
	require.NoError(t, e)
	var local, active, pending int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE disposition='LOCAL_P3_COVERED'),count(*) FILTER(WHERE disposition='ACTIVE' AND space_id=$2 AND scope_bytes IS NOT NULL) FROM bot_deletion_installations WHERE operation_id=$1`, root.OperationID, other).Scan(&local, &active))
	require.Equal(t, 1, local)
	require.Equal(t, 1, active)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_terminal_effects WHERE operation_id=$1 AND state='PENDING'`, root.OperationID).Scan(&pending))
	require.Equal(t, 2, pending)
	claimed, e := st.ClaimBotDeletion(ctx, root.OperationID)
	require.NoError(t, e)
	require.ErrorIs(t, st.CompleteBotDeletion(ctx, claimed), store.ErrDeletionPending)
}

func TestLocalP3RetirementRechecksClaimAfterAcceptedRootWait(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL accepted-root wait expires lease")
	}
	st, bot, root, c, p := retirementFixture(t)
	ctx := t.Context()
	saveRetirementEvidence(t, st, c, bot, true)
	blocker, e := st.Pool.Begin(ctx)
	require.NoError(t, e)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, e = blocker.Exec(ctx, `SELECT operation_id FROM bot_uninstall_operations WHERE operation_id=$1 FOR UPDATE`, root.OperationID)
	require.NoError(t, e)
	_, e = st.Pool.Exec(ctx, `UPDATE bot_space_purge_handoffs SET claimed_until=clock_timestamp()+interval '750 milliseconds' WHERE space_id=$1`, p.SpaceId)
	require.NoError(t, e)
	done := make(chan error, 1)
	go func() { _, e := st.RetireBotPurgeHandoff(ctx, c); done <- e }()
	require.Eventually(t, func() bool {
		var waiting bool
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE query LIKE '%FROM bot_uninstall_operations o JOIN bot_uninstall_installations%' AND wait_event_type='Lock')`).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	time.Sleep(800 * time.Millisecond)
	require.NoError(t, blocker.Commit(ctx))
	require.ErrorIs(t, <-done, store.ErrDeletionClaimLost)
	var active int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_installations WHERE operation_id=$1 AND disposition='ACTIVE' AND scope_bytes IS NOT NULL`, root.OperationID).Scan(&active))
	require.Equal(t, 1, active)
}

func TestLocalP3RetirementRollsBackEarlierTargetWhenLaterSourceUncovered(t *testing.T) {
	if testing.Short() {
		t.Skip("real multi-Bot retirement atomic rollback")
	}
	st := startBotLifecycleStore(t)
	ctx := t.Context()
	space := uuid.New()
	for n := 0; n < 2; n++ {
		owner, actor := uuid.New(), uuid.New()
		bot, _, e := st.CreateBot(ctx, owner, "Atomic P3 source", "", `[]`, actor)
		require.NoError(t, e)
		i, e := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
		require.NoError(t, e)
		require.NoError(t, st.CompleteInstallation(ctx, i, nil))
		acceptedPurgeUninstall(t, st, bot.ID, owner, actor, space)
	}
	p := preparePurgeHandoffDecision(t, st, space)
	_, e := st.PurgeSpace(ctx, p)
	require.ErrorIs(t, e, store.ErrDeletionPending)
	c, e := st.ClaimNextBotPurgeHandoff(ctx)
	require.NoError(t, e)
	sources := c.Sources()
	require.Len(t, sources, 2)
	for n, v := range sources {
		saveRetirementEvidence(t, st, c, v.BotID, n == 0)
	}
	_, e = st.RetireBotPurgeHandoff(ctx, c)
	require.ErrorIs(t, e, store.ErrDeletionPending)
	var rich, covered, intents int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_uninstall_installations WHERE space_id=$1 AND disposition='ACTIVE' AND scope_bytes IS NOT NULL`, space).Scan(&rich))
	require.Equal(t, 2, rich)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_sources WHERE space_id=$1 AND state='COVERED'`, space).Scan(&covered))
	require.Zero(t, covered)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_installation_intents WHERE space_id=$1`, space).Scan(&intents))
	require.Equal(t, 2, intents)
}

func TestLocalP3RetirementRetainsLateAcceptedUnseenChatObligation(t *testing.T) {
	if testing.Short() {
		t.Skip("real late accepted source set conflict")
	}
	st := startBotLifecycleStore(t)
	ctx := t.Context()
	owner, actor, space, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, _, e := st.CreateBot(ctx, owner, "Unseen late P3 target", "", `[]`, actor)
	require.NoError(t, e)
	i, e := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, e)
	require.NoError(t, st.CompleteInstallation(ctx, i, nil))
	p := preparePurgeHandoffDecision(t, st, space)
	_, e = st.PurgeSpace(ctx, p)
	require.ErrorIs(t, e, store.ErrDeletionPending)
	c, e := st.ClaimNextBotPurgeHandoff(ctx)
	require.NoError(t, e)
	saveRetirementEvidence(t, st, c, bot.ID, true)
	// Deliberately model an unaccounted old writer, not a permitted current
	// public admission through the irreversible Space fence.
	_, e = st.Pool.Exec(ctx, `INSERT INTO bot_chat_whitelist(bot_id,chat_id,space_id,enabled,added_by_profile_id) VALUES($1,$2,$3,false,$4)`, bot.ID, chat, space, actor)
	require.NoError(t, e)
	q := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, e)
	hash, e := principal.RequestHash(q)
	require.NoError(t, e)
	root, e := st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: raw, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, e)
	_, e = st.RetireBotPurgeHandoff(ctx, c)
	require.ErrorIs(t, e, store.ErrSpaceLifecycleConflict)
	var retained, receipts int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_installations WHERE operation_id=$1 AND disposition='ACTIVE' AND scope_bytes IS NOT NULL AND chat_ids ? $2`, root.OperationID, chat.String()).Scan(&retained))
	require.Equal(t, 1, retained)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_lifecycle_purge_receipts WHERE space_id=$1`, space).Scan(&receipts))
	require.Zero(t, receipts)
}
