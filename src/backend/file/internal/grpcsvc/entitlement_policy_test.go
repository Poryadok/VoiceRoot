package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/file/internal/r2file"
)

type entitlementResolverFunc func(context.Context, uuid.UUID, time.Time) (bool, error)

func (f entitlementResolverFunc) Premium(ctx context.Context, accountID uuid.UUID, now time.Time) (bool, error) {
	return f(ctx, accountID, now)
}

func TestUploadPolicy_FailsClosedWithoutVerifiedProjection(t *testing.T) {
	accountID := uuid.New()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-subscription-tier", "premium"))
	service := New(Deps{Clock: fixedPolicyClock{now: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)}})

	limit, premium := service.uploadPolicy(ctx, accountID)
	require.Equal(t, int64(r2file.MaxFreeFileBytes), limit)
	require.False(t, premium)

	service.entitlements = entitlementResolverFunc(func(context.Context, uuid.UUID, time.Time) (bool, error) {
		return false, errors.New("projection unavailable")
	})
	limit, premium = service.uploadPolicy(ctx, accountID)
	require.Equal(t, int64(r2file.MaxFreeFileBytes), limit)
	require.False(t, premium)
}

func TestUploadPolicy_UsesVerifiedPremiumProjectionAndKeepsE2ERetention(t *testing.T) {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	service := New(Deps{
		Clock: fixedPolicyClock{now: now},
		Entitlements: entitlementResolverFunc(func(context.Context, uuid.UUID, time.Time) (bool, error) {
			return true, nil
		}),
	})

	limit, premium := service.uploadPolicy(context.Background(), uuid.New())
	require.Equal(t, int64(r2file.MaxPremiumFileBytes), limit)
	require.True(t, premium)
	require.Nil(t, retentionExpiresAt(service.clock, premium, false))
	require.Equal(t, now.Add(defaultRetentionDays*24*time.Hour), *retentionExpiresAt(service.clock, premium, true))
}

type fixedPolicyClock struct{ now time.Time }

func (c fixedPolicyClock) Now() time.Time { return c.now }
