package terminalrouting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func fixtureSchedule() Schedule {
	return Schedule{
		FenceID: "fixture-auth-lifetime-fence-a", CycleID: "fixture-deletion-cycle-a1", Revision: 41,
		ScheduledAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		PurgeAt:     time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC),
	}
}

func fixtureRestored(schedule Schedule) Observation {
	return Observation{Schedule: schedule,
		Decision: &Decision{Kind: Restored, Revision: 42, At: schedule.PurgeAt.Add(-time.Second)},
		Current:  Current{Revision: 42, State: Active},
	}
}

func TestDelayedScheduleUsesOriginalDeadline(t *testing.T) {
	schedule := fixtureSchedule()
	life, err := New(schedule)
	require.NoError(t, err)
	// Receipt is already one hour late; it cannot create another 30-day window.
	life = life.Advance(schedule.PurgeAt.Add(time.Hour))
	require.True(t, life.RoutingErased)
	require.Equal(t, schedule.PurgeAt, life.Schedule.PurgeAt)
	require.False(t, life.CanProjectOldRouting(schedule.PurgeAt.Add(time.Hour)))
	require.False(t, life.FreshAdmission(41, true))
}

func TestDelayedPredeadlineRestoreAllowsFreshAdmissionWithoutResurrection(t *testing.T) {
	schedule := fixtureSchedule()
	life, err := New(schedule)
	require.NoError(t, err)
	life = life.Advance(schedule.PurgeAt) // Redis/authority are unavailable at D.
	require.True(t, life.RoutingErased)
	life, err = life.Reconcile(fixtureRestored(schedule), schedule.PurgeAt.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, life.RoutingErased, "account restore cannot undo irreversible local routing erasure")
	require.True(t, life.FreshAdmission(42, true))
	require.False(t, life.FreshAdmission(42, false), "current profile eligibility remains required")
	require.False(t, life.FreshAdmission(41, true), "an old current-authority observation is not admission")
	require.False(t, life.CanProjectOldRouting(schedule.PurgeAt.Add(time.Hour)))
}

func TestKnownPredeadlineRestoreCancelsOnlyItsPendingErasure(t *testing.T) {
	schedule := fixtureSchedule()
	life, err := New(schedule)
	require.NoError(t, err)
	life, err = life.Reconcile(fixtureRestored(schedule), schedule.PurgeAt.Add(-time.Second))
	require.NoError(t, err)
	life = life.Advance(schedule.PurgeAt)
	require.False(t, life.RoutingErased, "already verified exact-cycle restore cancels only pending erasure")
	require.True(t, life.FreshAdmission(42, true))
}

func TestOldRestoreCannotCancelNewerCurrentDeletion(t *testing.T) {
	schedule := fixtureSchedule()
	life, err := New(schedule)
	require.NoError(t, err)
	newer := Observation{Schedule: schedule, Current: Current{
		Revision: 43, State: DeleteScheduled, CycleID: "fixture-deletion-cycle-a2",
	}}
	life, err = life.Reconcile(newer, schedule.PurgeAt.Add(time.Hour))
	require.NoError(t, err)
	life, err = life.Reconcile(fixtureRestored(schedule), schedule.PurgeAt.Add(2*time.Hour))
	require.NoError(t, err)
	require.EqualValues(t, 43, life.Current.Revision)
	require.Equal(t, DeleteScheduled, life.Current.State)
	require.Equal(t, "fixture-deletion-cycle-a2", life.Current.CycleID)
	require.False(t, life.FreshAdmission(42, true))
	require.False(t, life.FreshAdmission(43, true))
	require.True(t, life.RoutingErased)
}

func TestAuthorityCollisionAndRestoreAtEqualityFailClosed(t *testing.T) {
	schedule := fixtureSchedule()
	life, err := New(schedule)
	require.NoError(t, err)
	changed := fixtureRestored(schedule)
	changed.Schedule.PurgeAt = changed.Schedule.PurgeAt.Add(24 * time.Hour)
	_, err = life.Reconcile(changed, schedule.PurgeAt)
	require.ErrorIs(t, err, ErrAuthorityConflict)
	changed = fixtureRestored(schedule)
	changed.Decision.At = schedule.PurgeAt
	_, err = life.Reconcile(changed, schedule.PurgeAt)
	require.ErrorIs(t, err, ErrAuthorityInvalid)
	valid, err := life.Reconcile(fixtureRestored(schedule), schedule.PurgeAt.Add(-time.Second))
	require.NoError(t, err)
	changed = fixtureRestored(schedule)
	changed.Decision.At = changed.Decision.At.Add(-time.Second)
	_, err = valid.Reconcile(changed, schedule.PurgeAt)
	require.ErrorIs(t, err, ErrAuthorityConflict, "immutable decision bytes cannot be rewritten")
}

func TestPredeleteBackupCannotServeWithoutReconciliation(t *testing.T) {
	schedule := fixtureSchedule()
	backup, err := New(schedule)
	require.NoError(t, err)
	blocked, err := RestoreForServing(backup, nil, schedule.PurgeAt.Add(time.Hour))
	require.ErrorIs(t, err, ErrReconciliationRequired)
	require.True(t, blocked.RoutingErased, "passed local deadline applies even when authority is unavailable")
	require.False(t, blocked.FreshAdmission(42, true))
	observation := fixtureRestored(schedule)
	recovered, err := RestoreForServing(blocked, &observation, schedule.PurgeAt.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, recovered.RoutingErased)
	require.True(t, recovered.FreshAdmission(42, true))
	require.False(t, recovered.CanProjectOldRouting(schedule.PurgeAt.Add(time.Hour)))
}

func TestCurrentActiveWithoutExactCycleDecisionCannotReopenAdmission(t *testing.T) {
	schedule := fixtureSchedule()
	life, err := New(schedule)
	require.NoError(t, err)
	life, err = life.Reconcile(Observation{
		Schedule: schedule, Current: Current{Revision: 42, State: Active},
	}, schedule.PurgeAt.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, life.FreshAdmission(42, true), "current active state alone does not prove this cycle restored before its deadline")
}
