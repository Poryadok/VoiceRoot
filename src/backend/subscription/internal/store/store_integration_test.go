package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

const storeSchemaSQL = `
CREATE TABLE IF NOT EXISTS subscriptions (
	id UUID PRIMARY KEY,
	account_id UUID NOT NULL,
	plan TEXT NOT NULL,
	billing_period TEXT NOT NULL,
	status TEXT NOT NULL,
	provider TEXT NOT NULL,
	provider_subscription_id TEXT NOT NULL,
	current_period_start TIMESTAMPTZ NOT NULL,
	current_period_end TIMESTAMPTZ NOT NULL,
	grace_period_end TIMESTAMPTZ,
	cancelled_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS space_subscriptions (
	id UUID PRIMARY KEY,
	space_id UUID NOT NULL,
	purchaser_account_id UUID NOT NULL,
	plan TEXT NOT NULL,
	billing_period TEXT NOT NULL,
	status TEXT NOT NULL,
	provider TEXT NOT NULL,
	provider_subscription_id TEXT NOT NULL,
	current_period_start TIMESTAMPTZ NOT NULL,
	current_period_end TIMESTAMPTZ NOT NULL,
	grace_period_end TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS billing_events (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	subscription_id UUID REFERENCES subscriptions(id),
	space_subscription_id UUID REFERENCES space_subscriptions(id),
	type TEXT NOT NULL,
	amount NUMERIC(12,2),
	currency TEXT,
	provider TEXT NOT NULL,
	provider_event_id TEXT NOT NULL,
	details JSONB NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (provider, provider_event_id)
);
`

func startStorePostgres(t *testing.T, ctx context.Context) *SubscriptionStore {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "subscriptiondb", "")
	_, err := pool.Exec(ctx, storeSchemaSQL)
	require.NoError(t, err)
	return &SubscriptionStore{Pool: pool}
}

func TestActivatePremium_andEffectiveTier(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)
	accountID := uuid.New()

	row, err := st.ActivatePremium(ctx, accountID, "evt_store_premium", json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, "premium", row.Plan)
	require.Equal(t, "active", row.Status)

	tier, err := st.EffectiveAccountTier(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, "premium", tier)

	_, err = st.ActivatePremium(ctx, accountID, "evt_store_premium", json.RawMessage(`{}`))
	require.ErrorIs(t, err, ErrDuplicateBillingEvent)
}

func TestMarkPaymentFailed_setsGracePeriod(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)
	accountID := uuid.New()

	_, err := st.ActivatePremium(ctx, accountID, "evt_before_fail", json.RawMessage(`{}`))
	require.NoError(t, err)

	row, err := st.MarkPaymentFailed(ctx, accountID, "evt_failed", json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, "grace_period", row.Status)
	require.NotNil(t, row.GracePeriodEnd)

	tier, err := st.EffectiveAccountTier(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, "grace_period", tier)
}

func TestActivateSpacePro_andHasActiveSpacePro(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)
	spaceID := uuid.New()
	purchaserID := uuid.New()

	row, err := st.ActivateSpacePro(ctx, spaceID, purchaserID, "evt_space_pro", json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, "space_pro", row.Plan)

	got, err := st.GetSpaceSubscriptionBySpaceID(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, row.ID, got.ID)

	hasPro, err := st.HasActiveSpaceProForPurchaser(ctx, purchaserID)
	require.NoError(t, err)
	require.True(t, hasPro)
}

func TestGetSubscriptionByAccountID_nilWhenMissing(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)

	row, err := st.GetSubscriptionByAccountID(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, row)

	tier, err := st.EffectiveAccountTier(ctx, uuid.New())
	require.NoError(t, err)
	require.Equal(t, "free", tier)
}

func TestEffectiveAccountTier_cancelledIsFree(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)
	accountID := uuid.New()

	row, err := st.ActivatePremium(ctx, accountID, "evt_cancelled", json.RawMessage(`{}`))
	require.NoError(t, err)

	_, err = st.Pool.Exec(ctx, `UPDATE subscriptions SET status = 'cancelled' WHERE id = $1`, row.ID)
	require.NoError(t, err)

	tier, err := st.EffectiveAccountTier(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, "free", tier)
}

func TestMarkPaymentFailed_notFoundWhenNoSubscription(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)

	_, err := st.MarkPaymentFailed(ctx, uuid.New(), "evt_orphan_fail", json.RawMessage(`{}`))
	require.Error(t, err)
}

func TestHasActiveSpaceProForPurchaser_falseWhenAbsent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)

	hasPro, err := st.HasActiveSpaceProForPurchaser(ctx, uuid.New())
	require.NoError(t, err)
	require.False(t, hasPro)
}

func TestActivateSpacePro_duplicateEventRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)
	spaceID := uuid.New()
	purchaserID := uuid.New()
	eventID := "evt_space_dup"

	_, err := st.ActivateSpacePro(ctx, spaceID, purchaserID, eventID, json.RawMessage(`{}`))
	require.NoError(t, err)
	_, err = st.ActivateSpacePro(ctx, spaceID, purchaserID, eventID, json.RawMessage(`{}`))
	require.ErrorIs(t, err, ErrDuplicateBillingEvent)
}

func TestActivatePremium_replacesExistingSubscription(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)
	accountID := uuid.New()

	first, err := st.ActivatePremium(ctx, accountID, "evt_first", json.RawMessage(`{}`))
	require.NoError(t, err)

	second, err := st.ActivatePremium(ctx, accountID, "evt_second", json.RawMessage(`{}`))
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	require.Equal(t, "premium", second.Plan)
}

func TestGetSpaceSubscriptionBySpaceID_nilWhenMissing(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)

	row, err := st.GetSpaceSubscriptionBySpaceID(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, row)
}

func TestMarkPaymentFailed_duplicateEventRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startStorePostgres(t, ctx)
	accountID := uuid.New()
	eventID := "evt_dup_fail"

	_, err := st.ActivatePremium(ctx, accountID, "evt_before_dup_fail", json.RawMessage(`{}`))
	require.NoError(t, err)

	_, err = st.MarkPaymentFailed(ctx, accountID, eventID, json.RawMessage(`{}`))
	require.NoError(t, err)
	_, err = st.MarkPaymentFailed(ctx, accountID, eventID, json.RawMessage(`{}`))
	require.ErrorIs(t, err, ErrDuplicateBillingEvent)
}
