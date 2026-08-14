package grpcsvc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	moderationv1 "voice.app/voice/moderation/v1"
)

type recordingMMClient struct {
	mu      sync.Mutex
	revoked []uuid.UUID
}

func (c *recordingMMClient) ApplyPlatformMMBan(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string, _ *time.Time) error {
	return nil
}

func (c *recordingMMClient) RevokePlatformMMBan(_ context.Context, targetAccountID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revoked = append(c.revoked, targetAccountID)
	return nil
}

func (c *recordingMMClient) lastRevoked() uuid.UUID {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.revoked) == 0 {
		return uuid.Nil
	}
	return c.revoked[len(c.revoked)-1]
}

func TestReviewAppeal_ApprovedRevokesMMBanInMatchmaking(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	mm := &recordingMMClient{}
	client, cleanup := startModerationGRPCTestServer(t, pool, func(s *ModerationGRPC) {
		s.Matchmaking = mm
	})
	t.Cleanup(cleanup)

	modProfile := uuid.New()
	targetAccount := uuid.New()
	modCtx := withInternalModCtx(ctx, modProfile)
	appellantCtx := withAccountCtx(ctx, targetAccount)

	applied, err := client.ApplySanction(modCtx, &moderationv1.ApplySanctionRequest{
		TargetAccountId: targetAccount.String(),
		Type:            "mm_ban",
		Reason:          "appeal mm ban",
	})
	require.NoError(t, err)

	submitted, err := client.SubmitAppeal(appellantCtx, &moderationv1.SubmitAppealRequest{
		SanctionId: applied.GetSanction().GetId(),
		Reason:     "unfair queue ban",
	})
	require.NoError(t, err)

	_, err = client.ReviewAppeal(modCtx, &moderationv1.ReviewAppealRequest{
		AppealId: submitted.GetAppeal().GetId(),
		Status:   "approved",
	})
	require.NoError(t, err)
	require.Equal(t, targetAccount, mm.lastRevoked())
}

func TestSubmitAppeal_ExpiredWindow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	client, cleanup := startModerationGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	modProfile := uuid.New()
	targetAccount := uuid.New()
	modCtx := withInternalModCtx(ctx, modProfile)
	appellantCtx := withAccountCtx(ctx, targetAccount)

	applied, err := client.ApplySanction(modCtx, &moderationv1.ApplySanctionRequest{
		TargetAccountId: targetAccount.String(),
		Type:            "temp_ban",
		Reason:          "old sanction",
		ExpiresAt:       timestamppb.New(time.Now().UTC().Add(24 * time.Hour)),
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
UPDATE sanctions SET created_at = now() - interval '8 days'
WHERE id = $1`, applied.GetSanction().GetId())
	require.NoError(t, err)

	_, err = client.SubmitAppeal(appellantCtx, &moderationv1.SubmitAppealRequest{
		SanctionId: applied.GetSanction().GetId(),
		Reason:     "too late",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func withAccountCtx(ctx context.Context, accountID uuid.UUID) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-user-id", accountID.String()))
}
