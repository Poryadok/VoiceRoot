package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/moderation/internal/store"
)

func TestProcessExpiredTempBans_AuthFailureLeavesSanctionEligibleForRetry(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startModerationPostgresPlatform(t, ctx)
	sanctions := &store.SanctionStore{Pool: pool}
	accountID := uuid.New()
	row, err := sanctions.InsertSanction(
		ctx,
		accountID,
		"temp_ban",
		"expired temporary ban",
		nil,
		autoModIssuerProfileID(),
		ptr(time.Now().UTC().Add(-time.Minute)),
	)
	require.NoError(t, err)

	auth := &failOnceAccountStatusClient{err: errors.New("auth unavailable")}
	svc := &ModerationGRPC{Sanctions: sanctions, Auth: auth}

	processed, err := svc.ProcessExpiredTempBans(ctx, 10)
	require.NoError(t, err)
	require.Zero(t, processed)
	require.Equal(t, 1, auth.calls)
	require.Equal(t, "active", auth.status)
	require.Equal(t, accountID, auth.accountID)

	afterFailure, err := sanctions.GetByID(ctx, row.ID)
	require.NoError(t, err)
	require.Nil(t, afterFailure.RevokedAt, "failed Auth reactivation must leave the sanction eligible for retry")
	eligible, err := sanctions.ListExpiredActiveTempBans(ctx, 10)
	require.NoError(t, err)
	require.Len(t, eligible, 1)
	require.Equal(t, row.ID, eligible[0].ID)

	processed, err = svc.ProcessExpiredTempBans(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Equal(t, 2, auth.calls)

	afterSuccess, err := sanctions.GetByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, afterSuccess.RevokedAt)
	eligible, err = sanctions.ListExpiredActiveTempBans(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, eligible)
	var auditCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM moderation_audit_log`).Scan(&auditCount))
	require.Zero(t, auditCount, "automatic expiry must not create a false manual-revocation audit completion")

	processed, err = svc.ProcessExpiredTempBans(ctx, 10)
	require.NoError(t, err)
	require.Zero(t, processed)
	require.Equal(t, 2, auth.calls, "a completed expiry must not reactivate or revoke again")
}

type failOnceAccountStatusClient struct {
	err       error
	calls     int
	accountID uuid.UUID
	status    string
	reason    string
}

func (c *failOnceAccountStatusClient) SetAccountStatus(_ context.Context, accountID uuid.UUID, status, reason string) error {
	c.calls++
	c.accountID = accountID
	c.status = status
	c.reason = reason
	if c.calls == 1 {
		return c.err
	}
	return nil
}

func ptr[T any](v T) *T {
	return &v
}
