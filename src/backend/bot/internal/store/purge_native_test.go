package store_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func controlledPurgeNativeResult(t *testing.T, q *spacev1.ReconcileBotPurgeInstallationRequest) *spacev1.ReconcileBotPurgeInstallationResponse {
	t.Helper()
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	digest := sha256.Sum256(body)
	return &spacev1.ReconcileBotPurgeInstallationResponse{Receipt: &spacev1.BotPurgeInstallationReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: q.OperationId, RequestSha256: digest[:], SpaceId: q.SpaceId, DeletionOperationId: q.DeletionOperationId, Generation: q.Generation, Manifest: proto.Clone(q.Manifest).(*commonv1.ManifestBinding), BotId: q.BotId, InstallationId: q.InstallationId, NativeCancellation: proto.Clone(q.NativeCancellation).(*spacev1.RemoveBotMemberRequest), Disposition: spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED, CompletedAt: timestamppb.Now()}}
}

func TestBotPurgeNativeSealingRequiresCoverageAndOwnsTypedReceiptBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("real BotPG native P3 request/receipt CAS")
	}
	st := startBotLifecycleStore(t)
	ctx := t.Context()
	bot, purge := pendingPurgeRecoverySource(t, st)
	claim, err := st.ClaimNextBotPurgeHandoff(ctx)
	require.NoError(t, err)
	requests, err := claim.NativeDrainRequests(bot)
	require.NoError(t, err)
	require.Len(t, requests, 1)
	q := requests[0]
	operation := uuid.MustParse(q.OperationId)
	_, err = st.SealBotPurgeNativeRequest(ctx, claim, bot, operation)
	require.ErrorIs(t, err, store.ErrDeletionPending, "native source cannot bypass required own Role/Messaging P3 evidence")
	require.NoError(t, st.SaveBotPurgeCoverageObservation(ctx, claim, bot, []store.BotPurgeCoverageProof{purgeRecoveryRoleProof(t, purge)}))
	sealed, err := st.SealBotPurgeNativeRequest(ctx, claim, bot, operation)
	require.NoError(t, err)
	require.Empty(t, sealed.ReceiptBytes)
	good := controlledPurgeNativeResult(t, q)
	for _, mode := range []string{"missing receipt", "other child", "other Space", "other P3 decision", "other P3 generation", "other Bot", "other installation", "other native cancel", "other native add", "other Bot generation", "bad request SHA", "bad manifest", "zero disposition", "missing time", "unknown nested"} {
		t.Run(mode, func(t *testing.T) {
			r := proto.Clone(good).(*spacev1.ReconcileBotPurgeInstallationResponse)
			switch mode {
			case "missing receipt":
				r.Receipt = nil
			case "other child":
				r.Receipt.OperationId = uuid.NewString()
			case "other Space":
				r.Receipt.SpaceId = uuid.NewString()
			case "other P3 decision":
				r.Receipt.DeletionOperationId = uuid.NewString()
			case "other P3 generation":
				r.Receipt.Generation++
			case "other Bot":
				r.Receipt.BotId = uuid.NewString()
			case "other installation":
				r.Receipt.InstallationId = uuid.NewString()
			case "other native cancel":
				r.Receipt.NativeCancellation.InstallationIntent.OperationId = uuid.NewString()
			case "other native add":
				r.Receipt.NativeCancellation.InstallOperationId = uuid.NewString()
			case "other Bot generation":
				r.Receipt.NativeCancellation.InstallationIntent.Generation++
			case "bad request SHA":
				r.Receipt.RequestSha256[0] ^= 1
			case "bad manifest":
				r.Receipt.Manifest.ManifestSha256[0] ^= 1
			case "zero disposition":
				r.Receipt.Disposition = spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_UNSPECIFIED
			case "missing time":
				r.Receipt.CompletedAt = nil
			case "unknown nested":
				r.Receipt.NativeCancellation.ProtoReflect().SetUnknown([]byte{0x78, 1})
			}
			require.ErrorIs(t, store.ValidateBotPurgeNativeReceipt(q, r), store.ErrSpaceLifecycleConflict)
		})
	}
	request := bytes.Clone(sealed.RequestBytes)
	receipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(good)
	require.NoError(t, err)
	originalReceipt := bytes.Clone(receipt)
	blocker, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer blocker.Release()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,71431))`, purge.SpaceId)
	require.NoError(t, err)
	defer func() {
		_, _ = blocker.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,71431))`, purge.SpaceId)
	}()
	done := make(chan error, 1)
	go func() { done <- st.SaveBotPurgeNativeReceipt(ctx, claim, bot, operation, request, receipt) }()
	require.Eventually(t, func() bool {
		var waiting bool
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted)`).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	request[0] ^= 1
	receipt[0] ^= 1
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_unlock(hashtextextended($1,71431))`, purge.SpaceId)
	require.NoError(t, err)
	require.NoError(t, <-done)
	replay, err := st.SealBotPurgeNativeRequest(ctx, claim, bot, operation)
	require.NoError(t, err)
	require.Equal(t, sealed.RequestBytes, replay.RequestBytes)
	require.Equal(t, originalReceipt, replay.ReceiptBytes)
	_, err = st.PurgeSpace(ctx, purge)
	require.ErrorIs(t, err, store.ErrDeletionPending, "remote native proof alone is not local journal retirement")
}

func TestBotPurgeNativeRequestPreservesAcceptedCanceledInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("real accepted CANCELED installation recovery source")
	}
	st := startBotLifecycleStore(t)
	ctx := t.Context()
	owner, actor, space := uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "P3 canceled native plan", "", `[]`, actor)
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	q := &botv1.UninstallBotFromSpaceRequest{BotId: bot.ID.String(), SpaceId: space.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	i, err := st.BeginInstallationCancellationWithAdmission(ctx, bot.ID, space, actor, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_UninstallBotFromSpace_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallationCancellation(ctx, i))
	purge := preparePurgeHandoffDecision(t, st, space)
	_, err = st.PurgeSpace(ctx, purge)
	require.ErrorIs(t, err, store.ErrDeletionPending)
	claim, err := st.ClaimNextBotPurgeHandoff(ctx)
	require.NoError(t, err)
	require.Nil(t, claim.Sources()[0].Intent, "disposable CANCELED intent is not consulted as a new live source")
	plans, err := claim.NativeDrainRequests(bot.ID)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Equal(t, i.CancelOperationID.String(), plans[0].NativeCancellation.InstallationIntent.OperationId)
	require.Equal(t, intent.OperationID.String(), plans[0].NativeCancellation.InstallOperationId)
	require.Equal(t, intent.InstallationID.String(), plans[0].InstallationId)
	require.NotEqual(t, plans[0].OperationId, plans[0].NativeCancellation.InstallationIntent.OperationId)
}
