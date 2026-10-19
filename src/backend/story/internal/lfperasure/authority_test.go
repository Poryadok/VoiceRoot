package lfperasure

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// These are synthetic authenticated-owner/view results, never installation or
// physical-erasure proof. The real owner/negative provider remains absent.
type ownerFixture struct {
	observation Observation
	err         error
}

func (f ownerFixture) ResolveFinal(context.Context, Request) (Observation, error) {
	return f.observation, f.err
}
func finalFixture(t *testing.T) (Binding, Request, Observation) {
	t.Helper()
	b := Binding{uuid.New(), uuid.New(), uuid.New()}
	r := Request{uuid.New(), uuid.New()}
	confirmed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	o := Observation{Version: 1, Purpose: Purpose, Participant: "story", Binding: b, Revision: 1, Cycle: r.Cycle, Decision: r.Decision, Fence: uuid.New(), Account: uuid.New(), Profiles: CanonicalProfiles([]uuid.UUID{uuid.New(), uuid.New()}), CompleteHistoricalScope: true, ConfirmedAt: confirmed, PurgeAt: confirmed.Add(30 * 24 * time.Hour), CommittedAt: confirmed.Add(31 * 24 * time.Hour), State: PurgeCommitted}
	h, e := ScopeHash(o.Account, o.Profiles)
	require.NoError(t, e)
	o.ScopeSHA256 = h
	return b, r, o
}

func TestFinalCutRequiresAuthenticatedFinalBoundary(t *testing.T) {
	b, r, o := finalFixture(t)
	_, e := ResolveFinal(context.Background(), b, r, nil)
	require.ErrorIs(t, e, ErrUnavailable)
	_, e = ResolveFinal(context.Background(), b, r, ownerFixture{o, errors.New("owner unavailable")})
	require.ErrorIs(t, e, ErrUnavailable)
	for _, state := range []State{0, Recoverable, Canceled} {
		o.State = state
		_, e = ResolveFinal(context.Background(), b, r, ownerFixture{o, nil})
		require.ErrorIs(t, e, ErrRejected, "elapsed31days is not finality")
	}
}

func TestFinalCutRejectsWrongBindingScopeAndCycle(t *testing.T) {
	for _, name := range []string{"version", "purpose", "participant", "environment", "installation", "owner", "revision", "cycle", "decision", "fence", "account", "partial scope", "scope hash", "duplicate profile", "unsorted profiles", "nil profile", "short recovery", "premature commit", "zero time"} {
		t.Run(name, func(t *testing.T) {
			b, r, o := finalFixture(t)
			switch name {
			case "version":
				o.Version = 2
			case "purpose":
				o.Purpose = "ACCOUNT_INACTIVE"
			case "participant":
				o.Participant = "subscription"
			case "environment":
				o.Binding.Environment = uuid.New()
			case "installation":
				o.Binding.Installation = uuid.New()
			case "owner":
				o.Binding.OwnerIncarnation = uuid.New()
			case "revision":
				o.Revision = 0
			case "cycle":
				o.Cycle = uuid.New()
			case "decision":
				o.Decision = uuid.New()
			case "fence":
				o.Fence = uuid.Nil
			case "account":
				o.Account = uuid.Nil
			case "partial scope":
				o.CompleteHistoricalScope = false
			case "scope hash":
				o.ScopeSHA256 = [32]byte{1}
			case "duplicate profile":
				o.Profiles[1] = o.Profiles[0]
			case "unsorted profiles":
				o.Profiles[0], o.Profiles[1] = o.Profiles[1], o.Profiles[0]
			case "nil profile":
				o.Profiles[0] = uuid.Nil
			case "short recovery":
				o.PurgeAt = o.PurgeAt.Add(-time.Second)
			case "premature commit":
				o.CommittedAt = o.PurgeAt.Add(-time.Nanosecond)
			case "zero time":
				o.ConfirmedAt = time.Time{}
			}
			_, e := ResolveFinal(context.Background(), b, r, ownerFixture{o, nil})
			require.ErrorIs(t, e, ErrRejected)
		})
	}
}

func TestFinalCutIsImmutableTransientPersonalData(t *testing.T) {
	b, r, o := finalFixture(t)
	original := o.Profiles[0]
	c, e := ResolveFinal(context.Background(), b, r, ownerFixture{o, nil})
	require.NoError(t, e)
	require.True(t, c.Valid())
	o.Profiles[0] = uuid.New()
	require.Equal(t, original, c.Profiles()[0])
	p := c.Profiles()
	p[0] = uuid.New()
	require.Equal(t, original, c.Profiles()[0])
	require.Equal(t, "[private Story LFP final cut]", c.String())
	require.False(t, (FinalCut{}).Valid())
}

type negativeFixture struct {
	view View
	err  error
}

func (f negativeFixture) Enter(context.Context) (View, error) { return f.view, f.err }

type viewFixture struct {
	verdict  Verdict
	releases int
}

func (f *viewFixture) Check(References) Verdict { return f.verdict }
func (f *viewFixture) Release()                 { f.releases++ }

func TestMissingNegativeAuthorityDoesNotTurnAbsenceIntoNew(t *testing.T) {
	_, e := Enter(context.Background(), nil)
	require.ErrorIs(t, e, ErrUnavailable)
	_, e = Enter(context.Background(), negativeFixture{})
	require.ErrorIs(t, e, ErrUnavailable)
	v := &viewFixture{verdict: LiveOrUnseen}
	_, e = Enter(context.Background(), negativeFixture{v, errors.New("truncated snapshot")})
	require.ErrorIs(t, e, ErrUnavailable)
	require.Equal(t, 1, v.releases)
}

func TestNegativeViewTerminalDiffersFromAbsence(t *testing.T) {
	r := References{Operations: []uuid.UUID{uuid.New()}, CohortEnvironment: uuid.New(), CohortInstallation: uuid.New(), CohortIncarnation: uuid.New(), DatabaseBinding: [32]byte{1}}
	for _, test := range []struct {
		v   Verdict
		err error
	}{{Unknown, ErrUnavailable}, {LiveOrUnseen, nil}, {ErasedOrRetired, ErrErased}, {99, ErrUnavailable}} {
		e := Check(&viewFixture{verdict: test.v}, r)
		if test.err == nil {
			require.NoError(t, e)
		} else {
			require.ErrorIs(t, e, test.err)
		}
	}
	require.ErrorIs(t, Check(&viewFixture{verdict: LiveOrUnseen}, References{}), ErrUnavailable)
	r.Operations = []uuid.UUID{uuid.Nil}
	require.ErrorIs(t, Check(&viewFixture{verdict: LiveOrUnseen}, r), ErrUnavailable)
}

type snapshotFixture struct {
	s   SnapshotSeal
	err error
}

func (f snapshotFixture) ReconcileAndSeal(context.Context, Binding) (SnapshotSeal, error) {
	return f.s, f.err
}
func TestMissingOrIncompleteSnapshotCannotOpenRestore(t *testing.T) {
	b, _, _ := finalFixture(t)
	_, e := ReconcileAndSeal(context.Background(), b, nil)
	require.ErrorIs(t, e, ErrUnavailable)
	for _, s := range []SnapshotSeal{{}, {Binding: b}, {Binding: b, SnapshotID: uuid.New(), Revision: 1}} {
		_, e = ReconcileAndSeal(context.Background(), b, snapshotFixture{s, nil})
		require.ErrorIs(t, e, ErrUnavailable)
	}
	s := SnapshotSeal{Binding: b, SnapshotID: uuid.New(), Revision: 1, ManifestSHA256: [32]byte{1}}
	_, e = ReconcileAndSeal(context.Background(), b, snapshotFixture{s, errors.New("late cut cannot seal")})
	require.ErrorIs(t, e, ErrUnavailable)
	got, e := ReconcileAndSeal(context.Background(), b, snapshotFixture{s, nil})
	require.NoError(t, e)
	require.Equal(t, s, got)
}
