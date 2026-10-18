package automod

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestObserve_IndependentEvidenceAndApplicationTimeMute(t *testing.T) {
	source := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	applied := source.Add(24 * time.Hour)
	var account Account
	var pattern Pattern
	var actions []Action
	for sequence := uint64(1); sequence <= 10; sequence++ {
		decision, err := Observe(account, pattern, Event{Sequence: sequence, RecordedAt: source.Add(time.Duration(sequence) * time.Second), Eligibility: Eligible}, applied)
		require.NoError(t, err)
		account, pattern = decision.Account, decision.Pattern
		actions = append(actions, decision.Actions...)
		if sequence == 5 || sequence == 6 {
			require.Len(t, actions, 1, "the sixth message cannot reuse the first five")
			require.Equal(t, Mute, actions[0].Kind)
			require.Equal(t, applied.Add(time.Hour), account.MuteUntil)
		}
	}
	require.Len(t, actions, 2)
	require.Equal(t, PermanentBan, actions[1].Kind)
	require.True(t, account.PermanentActive)
	require.Equal(t, uint64(10), pattern.ConsumedThrough)
}

func TestObserve_RollingWindowAndInclusiveStrikeBoundary(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		gap  time.Duration
		want ActionKind
	}{{"inclusive", 30 * 24 * time.Hour, PermanentBan}, {"outside", 30*24*time.Hour + time.Microsecond, Mute}} {
		t.Run(tc.name, func(t *testing.T) {
			first, err := Observe(Account{}, Pattern{}, Event{Sequence: 1, RecordedAt: t0, Eligibility: Eligible, LinkCount: 3}, t0)
			require.NoError(t, err)
			next, err := Observe(first.Account, first.Pattern, Event{Sequence: 2, RecordedAt: t0.Add(tc.gap), Eligibility: Eligible, LinkCount: 3}, t0.Add(tc.gap))
			require.NoError(t, err)
			require.Len(t, next.Actions, 1)
			require.Equal(t, tc.want, next.Actions[0].Kind)
		})
	}
	var account Account
	var pattern Pattern
	for seq := uint64(1); seq <= 5; seq++ {
		at := t0.Add(time.Duration(seq-1) * time.Second)
		if seq == 5 {
			at = t0.Add(10*time.Minute + time.Microsecond)
		}
		decision, err := Observe(account, pattern, Event{Sequence: seq, RecordedAt: at, Eligibility: Eligible}, at)
		require.NoError(t, err)
		require.Empty(t, decision.Actions, "expired oldest sample cannot contribute to five")
		account, pattern = decision.Account, decision.Pattern
	}
}

func TestObserve_MetadataCountersDoNotDisableDetection(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	account := Account{MuteUntil: t0}
	var pattern Pattern
	for seq := uint64(1); seq <= 15; seq++ {
		kind := Eligible
		if seq <= 5 {
			kind = NoText
		} else if seq <= 10 {
			kind = E2E
		}
		decision, err := Observe(account, pattern, Event{Sequence: seq, RecordedAt: t0.Add(time.Duration(seq) * time.Second), Eligibility: kind}, t0.Add(time.Minute))
		require.NoError(t, err)
		account, pattern = decision.Account, decision.Pattern
		if seq <= 10 {
			require.Empty(t, decision.Actions)
			require.Equal(t, uint32(seq), account.PostMuteCount)
		}
		if seq == 15 {
			require.Len(t, decision.Actions, 1)
			require.Equal(t, Mute, decision.Actions[0].Kind)
		}
	}
}

func TestObserve_ExpiryAndSequenceFailuresNeverCreateHits(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	decision, err := Observe(Account{}, Pattern{}, Event{Sequence: 1, RecordedAt: t0, Eligibility: Eligible, LinkCount: 3}, t0.Add(30*24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, ExpiredEvidence, decision.Inspection)
	require.Empty(t, decision.Actions)
	require.Equal(t, uint64(1), decision.Account.LastSequence)
	_, err = Observe(Account{}, Pattern{}, Event{Sequence: 2, RecordedAt: t0, Eligibility: Eligible}, t0)
	require.ErrorIs(t, err, ErrSequence)
	_, err = Observe(Account{}, Pattern{}, Event{Sequence: 1, RecordedAt: t0, Eligibility: E2E, LinkCount: 3}, t0)
	require.ErrorIs(t, err, ErrEvidence)
}
