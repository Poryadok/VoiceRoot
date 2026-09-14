package subscriptionconsume

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTierCacheApplyHigherRevisionAndRejectStaleSnapshot(t *testing.T) {
	cache := NewTierCache()
	deadline := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	require.Equal(t, ApplyApplied, cache.Apply(EntitlementSnapshot{
		AccountID:     "acc-1",
		Revision:      1,
		State:         StateInactive,
		EntitledUntil: deadline,
	}))
	require.Equal(t, "free", cache.TierAt("acc-1", deadline.Add(-time.Nanosecond)))
	require.Equal(t, ApplyApplied, cache.Apply(EntitlementSnapshot{
		AccountID:     "acc-1",
		Revision:      2,
		State:         StateActive,
		EntitledUntil: deadline,
	}))
	require.Equal(t, ApplyStale, cache.Apply(EntitlementSnapshot{
		AccountID:     "acc-1",
		Revision:      1,
		State:         StateInactive,
		EntitledUntil: deadline.Add(-time.Hour),
	}))
	require.Equal(t, "premium", cache.TierAt("acc-1", deadline.Add(-time.Nanosecond)))
}

func TestTierCacheFailsClosedAtEntitlementDeadline(t *testing.T) {
	deadline := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	for _, state := range []EntitlementState{StateActive, StateGracePeriod} {
		t.Run(string(state), func(t *testing.T) {
			cache := NewTierCache()
			require.Equal(t, ApplyApplied, cache.Apply(EntitlementSnapshot{
				AccountID:     "acc-1",
				Revision:      1,
				State:         state,
				EntitledUntil: deadline,
			}))
			require.Equal(t, "premium", cache.TierAt("acc-1", deadline.Add(-time.Nanosecond)))
			require.Equal(t, "free", cache.TierAt("acc-1", deadline))
			require.Equal(t, "free", cache.TierAt("acc-1", deadline.Add(time.Nanosecond)))
		})
	}
}

func TestTierCacheRejectsConflictingSameRevisionWithoutMutation(t *testing.T) {
	cache := NewTierCache()
	deadline := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	snapshot := EntitlementSnapshot{
		AccountID:     "acc-1",
		Revision:      3,
		State:         StateActive,
		EntitledUntil: deadline,
	}
	require.Equal(t, ApplyApplied, cache.Apply(snapshot))
	require.Equal(t, ApplyDuplicate, cache.Apply(snapshot))
	require.Equal(t, ApplyDuplicate, cache.Apply(EntitlementSnapshot{
		AccountID:     "ACC-1",
		Revision:      3,
		State:         StateActive,
		EntitledUntil: deadline.In(time.FixedZone("UTC+3", 3*60*60)),
	}))

	for _, conflicting := range []EntitlementSnapshot{
		{AccountID: "acc-1", Revision: 3, State: StateInactive, EntitledUntil: deadline},
		{AccountID: "acc-1", Revision: 3, State: StateActive, EntitledUntil: deadline.Add(time.Hour)},
	} {
		result := cache.Apply(conflicting)
		require.Equal(t, ApplyConflict, result)
		require.True(t, errors.Is(result.Err(), ErrConflictingRevision))
		require.Equal(t, "premium", cache.TierAt("acc-1", deadline.Add(-time.Nanosecond)))
	}
}

func TestTierCacheRejectsInvalidSnapshotWithoutMutatingExistingEntitlement(t *testing.T) {
	cache := NewTierCache()
	deadline := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	require.Equal(t, ApplyApplied, cache.Apply(EntitlementSnapshot{
		AccountID:     "acc-1",
		Revision:      1,
		State:         StateActive,
		EntitledUntil: deadline,
	}))

	for _, snapshot := range []EntitlementSnapshot{
		{AccountID: "acc-1", State: StateActive, EntitledUntil: deadline},
		{AccountID: "acc-1", Revision: 2, State: StateActive},
		{Revision: 2, State: StateActive, EntitledUntil: deadline},
		{AccountID: "acc-1", Revision: 2, State: "UNKNOWN", EntitledUntil: deadline},
	} {
		result := cache.Apply(snapshot)
		require.Equal(t, ApplyInvalid, result)
		require.True(t, errors.Is(result.Err(), ErrInvalidSnapshot))
		require.Equal(t, "premium", cache.TierAt("acc-1", deadline.Add(-time.Nanosecond)))
	}
}
