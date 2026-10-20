package grpcsvc_test

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

func TestPurgeWorkerCompletesExactPositivelyCoveredNonemptySource(t *testing.T) {
	if testing.Short() {
		t.Skip("real BotPG positive nonempty P3 retirement")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot_purge_positive", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	actor, space := uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Positive P3 worker", "", `[]`, actor)
	require.NoError(t, err)
	i, err := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, i, nil))
	f := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}}
	_, err = st.ApplySpaceLifecycleFence(ctx, f)
	require.NoError(t, err)
	f.Generation = 2
	f.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = st.ApplySpaceLifecycleFence(ctx, f)
	require.NoError(t, err)
	p := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: f.SpaceId, DeletionOperationId: f.DeletionOperationId, Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, Manifest: f.Manifest, PurgeDecidedAt: timestamppb.Now()}
	_, err = st.PurgeSpace(ctx, p)
	require.ErrorIs(t, err, store.ErrDeletionPending)
	role, _ := controlledPurgeOriginals(t, p)
	issuer, _ := installationTestSigningKey(t)
	client := purgeControlledSpace{call: func(context.Context, *spacev1.GetBotPurgeHandoffRequest) (*spacev1.GetBotPurgeHandoffResponse, error) {
		return &spacev1.GetBotPurgeHandoffResponse{ProtocolVersion: 1, SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: p.Manifest, Role: role}, nil
	}}
	// Echo the actual request-bound read ID, as the real protected provider does.
	client.call = func(_ context.Context, q *spacev1.GetBotPurgeHandoffRequest) (*spacev1.GetBotPurgeHandoffResponse, error) {
		return &spacev1.GetBotPurgeHandoffResponse{ProtocolVersion: 1, RequestId: q.RequestId, SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: p.Manifest, Role: role}, nil
	}
	client.native = func(_ context.Context, q *spacev1.ReconcileBotPurgeInstallationRequest) (*spacev1.ReconcileBotPurgeInstallationResponse, error) {
		raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
		require.NoError(t, e)
		sum := sha256.Sum256(raw)
		return &spacev1.ReconcileBotPurgeInstallationResponse{Receipt: &spacev1.BotPurgeInstallationReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: q.OperationId, RequestSha256: sum[:], SpaceId: q.SpaceId, DeletionOperationId: q.DeletionOperationId, Generation: q.Generation, Manifest: proto.Clone(q.Manifest).(*commonv1.ManifestBinding), BotId: q.BotId, InstallationId: q.InstallationId, NativeCancellation: proto.Clone(q.NativeCancellation).(*spacev1.RemoveBotMemberRequest), Disposition: spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED, CompletedAt: timestamppb.Now()}}, nil
	}
	svc := &grpcsvc.BotGRPC{Store: st, PrincipalIssuer: issuer, InstallationSpace: client}
	worked, err := svc.ProcessNextBotPurgeHandoff(ctx)
	require.True(t, worked)
	require.NoError(t, err, "all exact positive evidence must retire the nonempty source rather than stay pending forever")
	r, err := st.PurgeSpace(ctx, p)
	require.NoError(t, err)
	require.Equal(t, commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, r.State)
	var remaining int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_installation_intents WHERE space_id=$1`, space).Scan(&remaining))
	require.Zero(t, remaining)
	worked, err = svc.ProcessNextBotPurgeHandoff(ctx)
	require.NoError(t, err)
	require.False(t, worked)
}
