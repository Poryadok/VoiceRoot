package sweeper

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
	"voice/backend/subscription/internal/store"
)

type captureGraceEvents struct {
	reminders []graceReminderCall
	expired   []string
}

type graceReminderCall struct {
	AccountID string
	Plan      string
	Day       int32
}

func (c *captureGraceEvents) PublishPlanExpired(context.Context, string, string) error { return nil }
func (c *captureGraceEvents) PublishDowngrade(context.Context, string, string) error   { return nil }
func (c *captureGraceEvents) PublishSpaceProExpired(_ context.Context, spaceID string) error {
	c.expired = append(c.expired, spaceID)
	return nil
}
func (c *captureGraceEvents) PublishGraceReminder(_ context.Context, accountID, plan string, day int32) error {
	c.reminders = append(c.reminders, graceReminderCall{AccountID: accountID, Plan: plan, Day: day})
	return nil
}

const sweeperSchemaSQL = `
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
	grace_reminders_sent INT[] NOT NULL DEFAULT '{}',
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
	grace_reminders_sent INT[] NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

// TestRunner_EmitsGraceRemindersOnDays1_3_7 documents subscription.md grace notifications.
func TestRunner_EmitsGraceRemindersOnDays1_3_7(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "subgrace", "")
	_, err := pool.Exec(ctx, sweeperSchemaSQL)
	require.NoError(t, err)
	st := &store.SubscriptionStore{Pool: pool}
	events := &captureGraceEvents{}

	accountID := uuid.New()
	subID := uuid.New()
	graceStart := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	graceEnd := graceStart.Add(7 * 24 * time.Hour)
	_, err = pool.Exec(ctx, `
INSERT INTO subscriptions (
	id, account_id, plan, billing_period, status, provider, provider_subscription_id,
	current_period_start, current_period_end, grace_period_end
) VALUES ($1,$2,'premium','monthly','grace_period','paddle',$3,$4,$5,$6)`,
		subID, accountID, "paddle_grace_"+subID.String(), graceStart, graceEnd, graceEnd)
	require.NoError(t, err)

	runner := &Runner{Store: st, DomainEvents: events}

	for _, tc := range []struct {
		now     time.Time
		wantDay int32
	}{
		{graceStart.Add(30 * time.Minute), 1},
		{graceStart.Add(2*24*time.Hour + time.Hour), 3},
		{graceStart.Add(6*24*time.Hour + time.Hour), 7},
	} {
		events.reminders = nil
		runner.Now = func() time.Time { return tc.now }
		require.NoError(t, runner.RunOnce(ctx))
		require.Len(t, events.reminders, 1, "day %d", tc.wantDay)
		require.Equal(t, accountID.String(), events.reminders[0].AccountID)
		require.Equal(t, "premium", events.reminders[0].Plan)
		require.Equal(t, tc.wantDay, events.reminders[0].Day)

		// Idempotent: same day must not re-emit.
		events.reminders = nil
		require.NoError(t, runner.RunOnce(ctx))
		require.Empty(t, events.reminders, "day %d duplicate", tc.wantDay)
	}
}
