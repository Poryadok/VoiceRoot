package grpcsvc_test

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

func TestPurgeNativeWorkerPersistsExactTypedResultWithoutLocalCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("real BotPG P3 native persistence with controlled RSA Space receiver")
	}
	ctx := t.Context()
	pool := integrationtest.StartPostgres(t, ctx, "bot_purge_native", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	actor, space := uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "P3 native worker", "", `[]`, actor)
	require.NoError(t, err)
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	m := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}
	f := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: m}
	_, err = st.ApplySpaceLifecycleFence(ctx, f)
	require.NoError(t, err)
	f.Generation = 2
	f.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = st.ApplySpaceLifecycleFence(ctx, f)
	require.NoError(t, err)
	p := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: f.SpaceId, DeletionOperationId: f.DeletionOperationId, Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, Manifest: m, PurgeDecidedAt: timestamppb.Now()}
	_, err = st.PurgeSpace(ctx, p)
	require.ErrorIs(t, err, store.ErrDeletionPending)
	role, _ := controlledPurgeOriginals(t, p)
	issuer, key := installationTestSigningKey(t)
	seen := map[string]bool{}
	calls, reads := 0, 0
	var sealed []byte
	var good *spacev1.ReconcileBotPurgeInstallationResponse
	client := purgeControlledSpace{call: func(_ context.Context, q *spacev1.GetBotPurgeHandoffRequest) (*spacev1.GetBotPurgeHandoffResponse, error) {
		reads++
		return &spacev1.GetBotPurgeHandoffResponse{ProtocolVersion: 1, RequestId: q.RequestId, SpaceId: q.SpaceId, DeletionOperationId: q.DeletionOperationId, Generation: q.Generation, Manifest: proto.Clone(q.Manifest).(*commonv1.ManifestBinding), Role: proto.Clone(role).(*spacev1.BotPurgeRoleHandoff)}, nil
	}, native: func(call context.Context, q *spacev1.ReconcileBotPurgeInstallationRequest) (*spacev1.ReconcileBotPurgeInstallationResponse, error) {
		calls++
		md, ok := metadata.FromOutgoingContext(call)
		require.True(t, ok)
		require.Equal(t, []string{q.OperationId}, md.Get("x-request-id"))
		require.Empty(t, md.Get("x-voice-profile-id"))
		h, e := principal.RequestHash(q)
		require.NoError(t, e)
		proof, e := principal.VerifyService(call, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName, ExpectedRequestID: q.OperationId, ExpectedRequestHash: h, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return key, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			if seen[jti] {
				return errors.New("replay")
			}
			seen[jti] = true
			return nil
		}})
		require.NoError(t, e)
		require.Equal(t, "service:bot", proof.Subject)
		require.Empty(t, proof.AccountID)
		require.Empty(t, proof.ProfileID)
		require.Zero(t, proof.SessionEpoch)
		body, e := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
		require.NoError(t, e)
		if sealed == nil {
			sealed = bytes.Clone(body)
		}
		require.Equal(t, sealed, body)
		require.Equal(t, intent.InstallationID.String(), q.InstallationId)
		require.Equal(t, intent.OperationID.String(), q.NativeCancellation.InstallOperationId)
		require.Equal(t, uint64(intent.Generation), q.NativeCancellation.InstallationIntent.Generation)
		require.NotEqual(t, q.OperationId, q.NativeCancellation.InstallationIntent.OperationId)
		require.Equal(t, actor.String(), q.NativeCancellation.ProfileId)
		var persisted int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2 AND coverage_bytes IS NOT NULL AND jsonb_array_length(convert_from(coverage_bytes,'UTF8')::jsonb->'native')=1`, space, bot.ID).Scan(&persisted))
		require.Equal(t, 1, persisted)
		// The actual RPC callback can independently take the Space guard: no
		// network call runs inside the producer's source/claim transaction.
		tx, e := pool.Begin(ctx)
		require.NoError(t, e)
		var free bool
		require.NoError(t, tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1,71431))`, space.String()).Scan(&free))
		require.True(t, free)
		require.NoError(t, tx.Rollback(ctx))
		if good == nil {
			digest := sha256.Sum256(body)
			good = &spacev1.ReconcileBotPurgeInstallationResponse{Receipt: &spacev1.BotPurgeInstallationReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: q.OperationId, RequestSha256: digest[:], SpaceId: q.SpaceId, DeletionOperationId: q.DeletionOperationId, Generation: q.Generation, Manifest: proto.Clone(q.Manifest).(*commonv1.ManifestBinding), BotId: q.BotId, InstallationId: q.InstallationId, NativeCancellation: proto.Clone(q.NativeCancellation).(*spacev1.RemoveBotMemberRequest), Disposition: spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED, CompletedAt: timestamppb.Now()}}
		}
		if calls == 1 {
			return nil, status.Error(codes.DeadlineExceeded, "controlled P3 native committed result ACK lost")
		}
		if calls == 2 {
			bad := proto.Clone(good).(*spacev1.ReconcileBotPurgeInstallationResponse)
			bad.Receipt.NativeCancellation.InstallationIntent.Generation++
			return bad, nil
		}
		return proto.Clone(good).(*spacev1.ReconcileBotPurgeInstallationResponse), nil
	}}
	svc := &grpcsvc.BotGRPC{Store: st, PrincipalIssuer: issuer, InstallationSpace: client}
	for pass := 0; pass < 4; pass++ {
		_, err = pool.Exec(ctx, `UPDATE bot_space_purge_handoffs SET next_attempt_at=clock_timestamp() WHERE space_id=$1`, space)
		require.NoError(t, err)
		worked, e := svc.ProcessNextBotPurgeHandoff(ctx)
		require.True(t, worked)
		if pass == 0 {
			require.Equal(t, codes.DeadlineExceeded, status.Code(e))
		} else if pass == 1 {
			require.Equal(t, codes.Unavailable, status.Code(e))
		} else {
			require.ErrorIs(t, e, store.ErrDeletionPending)
		}
		var state string
		require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&state))
		require.Equal(t, "PENDING", state)
	}
	require.Equal(t, 3, calls, "the fourth pass validates the exact retained typed result without another RPC")
	require.Equal(t, 1, reads)
	var receiptCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_lifecycle_purge_receipts WHERE space_id=$1`, space).Scan(&receiptCount))
	require.Zero(t, receiptCount)
}
