package providerlifecycle

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
)

func TestPersonalAndSpaceGraceCycleDoesNotRestart(t *testing.T) {
	for _, kind := range []eventsv1.SubscriptionAggregateKind{1, 2} {
		t.Run(kind.String(), func(t *testing.T) {
			c := fakeCommand(kind)
			s, err := reduce(nil, c)
			require.NoError(t, err)
			require.Equal(t, eventsv1.EntitlementState_ENTITLEMENT_STATE_ACTIVE, s.State)
			cancel := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_CANCEL_SCHEDULED, c.EffectiveAt.Add(time.Hour))
			scheduled, err := reduce(s, cancel)
			require.NoError(t, err)
			require.True(t, scheduled.CancelAtPeriodEnd)
			require.NotNil(t, scheduled.DowngradeCycleId)
			require.Nil(t, s.DowngradeCycleId, "caller snapshot must not be mutated")
			failure := nextCommand(cancel, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED, c.PeriodEnd)
			grace, err := reduce(scheduled, failure)
			require.NoError(t, err)
			require.Equal(t, eventsv1.EntitlementState_ENTITLEMENT_STATE_GRACE_PERIOD, grace.State)
			require.Equal(t, failure.EffectiveAt.Add(7*24*time.Hour), grace.EntitledUntil.AsTime())
			require.Equal(t, scheduled.DowngradeCycleId, grace.DowngradeCycleId)
			repeated := nextCommand(failure, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED, failure.EffectiveAt.Add(3*24*time.Hour))
			again, err := reduce(grace, repeated)
			require.NoError(t, err)
			require.True(t, proto.Equal(grace, again), "same unresolved payment cannot restart or replace its grace snapshot")
			expiry := nextCommand(repeated, eventsv1.EntitlementReason_ENTITLEMENT_REASON_GRACE_EXPIRED, grace.EntitledUntil.AsTime())
			inactive, err := reduce(grace, expiry)
			require.NoError(t, err)
			require.Equal(t, eventsv1.EntitlementState_ENTITLEMENT_STATE_INACTIVE, inactive.State)
			require.Equal(t, grace.DowngradeCycleId, inactive.DowngradeCycleId)
			require.Equal(t, expiry.EffectiveAt, inactive.EntitledUntil.AsTime())
			for _, reason := range []eventsv1.EntitlementReason{eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_RECOVERED, eventsv1.EntitlementReason_ENTITLEMENT_REASON_RENEWED} {
				recovery := nextCommand(failure, reason, failure.EffectiveAt.Add(24*time.Hour))
				recovery.PeriodStart = c.PeriodEnd
				recovery.PeriodEnd = c.PeriodEnd.AddDate(0, 1, 0)
				active, err := reduce(grace, recovery)
				require.NoError(t, err)
				require.Equal(t, eventsv1.EntitlementState_ENTITLEMENT_STATE_ACTIVE, active.State)
				require.Nil(t, active.DowngradeCycleId)
				require.Nil(t, active.GracePeriodEnd)
				require.False(t, active.CancelAtPeriodEnd)
			}
		})
	}
}

func TestCancelResumeAndPeriodBoundary(t *testing.T) {
	c := fakeCommand(1)
	active, err := reduce(nil, c)
	require.NoError(t, err)
	cancel := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_CANCEL_SCHEDULED, c.EffectiveAt.Add(time.Hour))
	scheduled, err := reduce(active, cancel)
	require.NoError(t, err)
	resume := nextCommand(cancel, eventsv1.EntitlementReason_ENTITLEMENT_REASON_CANCEL_RESUMED, c.EffectiveAt.Add(2*time.Hour))
	resumed, err := reduce(scheduled, resume)
	require.NoError(t, err)
	require.Nil(t, resumed.DowngradeCycleId)
	require.False(t, resumed.CancelAtPeriodEnd)
	early := nextCommand(cancel, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PERIOD_ENDED, c.PeriodEnd.Add(-time.Microsecond))
	_, err = reduce(scheduled, early)
	require.ErrorIs(t, err, ErrNeedsReconciliation)
	early.EffectiveAt = c.PeriodEnd
	expired, err := reduce(scheduled, early)
	require.NoError(t, err)
	require.Equal(t, early.EffectiveAt, expired.EntitledUntil.AsTime())
	purchase := nextCommand(early, eventsv1.EntitlementReason_ENTITLEMENT_REASON_STARTED, c.PeriodEnd)
	purchase.PeriodStart = c.PeriodEnd
	purchase.PeriodEnd = c.PeriodEnd.AddDate(0, 1, 0)
	bought, err := reduce(expired, purchase)
	require.NoError(t, err)
	require.Nil(t, bought.DowngradeCycleId)
	require.Equal(t, expired.DeletionFenceId, bought.DeletionFenceId)
	require.NotEqual(t, expired.EntitlementId, bought.EntitlementId)
}

func TestAmbiguousCommandsAndDeletionNeverGainAccess(t *testing.T) {
	c := fakeCommand(1)
	for _, change := range []func(*Command){
		func(c *Command) { c.Version = 0 }, func(c *Command) { c.Kind = 99 }, func(c *Command) { c.AggregateID = uuid.Nil.String() },
		func(c *Command) { c.Provider = "" }, func(c *Command) { c.EventID = "" }, func(c *Command) { c.RawBody = nil }, func(c *Command) { c.BillingPeriod = "" },
		func(c *Command) { c.PeriodEnd = c.PeriodStart }, func(c *Command) { c.Reason = 99 }, func(c *Command) { c.PurchaserID = uuid.NewString() },
	} {
		bad := c
		change(&bad)
		require.Error(t, validateCommand(bad))
	}
	active, err := reduce(nil, c)
	require.NoError(t, err)
	deleted := proto.Clone(active).(*eventsv1.EntitlementChanged)
	cycle := uuid.NewString()
	deleted.DeletionCycleId = &cycle
	renewed := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_RENEWED, c.EffectiveAt.Add(time.Hour))
	_, err = reduce(deleted, renewed)
	require.ErrorIs(t, err, ErrNeedsReconciliation)
	changedTarget := c
	changedTarget.AggregateID = uuid.NewString()
	_, err = reduce(active, changedTarget)
	require.ErrorIs(t, err, ErrContractMismatch)
	futureFailure := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED, c.EffectiveAt.Add(time.Hour))
	_, err = reduce(active, futureFailure)
	require.ErrorIs(t, err, ErrNeedsReconciliation, "failure before paid boundary needs authoritative reconciliation")
}
