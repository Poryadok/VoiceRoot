package messageviews

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

type compactStore struct {
	profiles []uuid.UUID
	deleted  []uuid.UUID
}

func (s *compactStore) MessageViewProfiles(_ context.Context, after uuid.UUID, limit int) ([]uuid.UUID, error) {
	if after != uuid.Nil {
		return nil, nil
	}
	return s.profiles, nil
}
func (s *compactStore) DeleteProfileMessageViews(_ context.Context, ids []uuid.UUID) error {
	s.deleted = append(s.deleted, ids...)
	return nil
}

type deletionCheck func(context.Context, []uuid.UUID) (map[uuid.UUID]DeletionState, error)

func (f deletionCheck) ProfileDeletionStates(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]DeletionState, error) {
	return f(ctx, ids)
}

func TestCompactorDeletesOnlyExplicitSoftDeletedProfiles(t *testing.T) {
	active, deleted, unknown := uuid.New(), uuid.New(), uuid.New()
	s := &compactStore{profiles: []uuid.UUID{active, deleted, unknown}}
	c := &Compactor{Store: s, Checker: deletionCheck(func(context.Context, []uuid.UUID) (map[uuid.UUID]DeletionState, error) {
		return map[uuid.UUID]DeletionState{active: NotDeleted, deleted: SoftDeleted, unknown: Unknown}, nil
	})}
	require.NoError(t, c.Sweep(context.Background()))
	require.Equal(t, []uuid.UUID{deleted}, s.deleted)
}
func TestCompactorRejectsOutageAndMalformedWholeChunk(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	for _, result := range []map[uuid.UUID]DeletionState{nil, {a: SoftDeleted}, {a: SoftDeleted, b: DeletionState(99)}, {a: SoftDeleted, b: NotDeleted, uuid.New(): SoftDeleted}} {
		s := &compactStore{profiles: []uuid.UUID{a, b}}
		c := &Compactor{Store: s, Checker: deletionCheck(func(context.Context, []uuid.UUID) (map[uuid.UUID]DeletionState, error) { return result, nil })}
		require.Error(t, c.Sweep(context.Background()))
		require.Empty(t, s.deleted)
	}
	s := &compactStore{profiles: []uuid.UUID{a, b}}
	c := &Compactor{Store: s, Checker: deletionCheck(func(context.Context, []uuid.UUID) (map[uuid.UUID]DeletionState, error) {
		return nil, errors.New("offline")
	})}
	require.Error(t, c.Sweep(context.Background()))
	require.Empty(t, s.deleted)
}
