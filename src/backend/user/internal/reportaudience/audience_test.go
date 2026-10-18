package reportaudience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type memorySource struct {
	snapshot Snapshot
	changed  bool
}

func (s *memorySource) Snapshot(context.Context, uuid.UUID) (Snapshot, error) { return s.snapshot, nil }
func (s *memorySource) Recheck(context.Context, Binding) error {
	if s.changed {
		return errors.New("binding changed")
	}
	return nil
}

type classifierFunc func(context.Context, uuid.UUID, []uuid.UUID) (Classification, error)

func (f classifierFunc) Classify(ctx context.Context, id uuid.UUID, ids []uuid.UUID) (Classification, error) {
	return f(ctx, id, ids)
}

func TestResolveRequiresCompleteFreshPositiveAuthority(t *testing.T) {
	now := time.Now().UTC()
	profile, account, live, deleted, missing := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, mode := range []string{"complete", "omission", "extra", "duplicate", "unknown", "expired", "future", "wrong evaluation", "outage", "changed binding"} {
		t.Run(mode, func(t *testing.T) {
			source := &memorySource{snapshot: Snapshot{Binding: Binding{ProfileID: profile, AccountID: account, Revision: 3, State: Live}, Candidates: []uuid.UUID{live, deleted, missing}}}
			source.changed = mode == "changed binding"
			service := Service{Source: source, Now: func() time.Time { return now }}
			service.Classifier = classifierFunc(func(_ context.Context, evaluation uuid.UUID, ids []uuid.UUID) (Classification, error) {
				require.Equal(t, []uuid.UUID{live, deleted, missing}, ids)
				out := Classification{EvaluationID: evaluation, SnapshotID: uuid.New(), StartedAt: now, CompletedAt: now, ExpiresAt: now.Add(30 * time.Second), Accounts: []Account{{live, Live}, {deleted, Deleted}, {missing, Missing}}}
				switch mode {
				case "omission":
					out.Accounts = out.Accounts[:2]
				case "extra":
					out.Accounts = append(out.Accounts, Account{uuid.New(), Live})
				case "duplicate":
					out.Accounts[1] = out.Accounts[0]
				case "unknown":
					out.Accounts[0].State = 0
				case "expired":
					out.StartedAt = now.Add(-31 * time.Second)
					out.ExpiresAt = now.Add(-time.Second)
				case "future":
					out.CompletedAt = now.Add(time.Second)
				case "wrong evaluation":
					out.EvaluationID = uuid.New()
				case "outage":
					return Classification{}, errors.New("auth unavailable")
				}
				return out, nil
			})
			got, err := service.Resolve(context.Background(), uuid.New(), profile, nil)
			if mode != "complete" {
				require.Error(t, err)
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, int64(1), got.Audience)
			require.Equal(t, MetricVersion, got.Metric)
			require.Equal(t, Live, got.State)
			require.Equal(t, now.Add(30*time.Second), got.ExpiresAt)
		})
	}
}

func TestResolveBatchesWithoutPartialFallback(t *testing.T) {
	now := time.Now().UTC()
	ids := make([]uuid.UUID, 1001)
	for i := range ids {
		ids[i] = uuid.New()
	}
	source := &memorySource{snapshot: Snapshot{Binding: Binding{ProfileID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: Live}, Candidates: ids}}
	calls := 0
	s := Service{Source: source, Now: func() time.Time { return now }, Classifier: classifierFunc(func(_ context.Context, id uuid.UUID, batch []uuid.UUID) (Classification, error) {
		calls++
		require.LessOrEqual(t, len(batch), 1000)
		out := Classification{EvaluationID: id, SnapshotID: uuid.New(), StartedAt: now, CompletedAt: now, ExpiresAt: now.Add(30 * time.Second)}
		for _, candidate := range batch {
			out.Accounts = append(out.Accounts, Account{candidate, Live})
		}
		return out, nil
	})}
	got, err := s.Resolve(context.Background(), uuid.New(), source.snapshot.ProfileID, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1001), got.Audience)
	require.Equal(t, 2, calls)
	// Reject duplicate candidates instead of turning malformed local authority into a denominator.
	source.snapshot.Candidates[1000] = ids[0]
	_, err = s.Resolve(context.Background(), uuid.New(), source.snapshot.ProfileID, nil)
	require.Error(t, err)
}

func TestResolveEmptyAndTerminalAudience(t *testing.T) {
	now := time.Now().UTC()
	for _, state := range []State{Live, Deleted, Missing} {
		source := &memorySource{snapshot: Snapshot{Binding: Binding{ProfileID: uuid.New(), AccountID: uuid.New(), Revision: 1, State: state}}}
		s := Service{Source: source, Now: func() time.Time { return now }, Classifier: classifierFunc(func(context.Context, uuid.UUID, []uuid.UUID) (Classification, error) {
			t.Fatal("empty/terminal must not request classification")
			return Classification{}, nil
		})}
		got, err := s.Resolve(context.Background(), uuid.New(), source.snapshot.ProfileID, nil)
		require.NoError(t, err)
		if state == Live {
			require.Equal(t, EmptyAudience, got.State)
		} else {
			require.Equal(t, state, got.State)
		}
		require.Zero(t, got.Audience)
	}
}
