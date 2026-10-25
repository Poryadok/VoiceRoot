package store

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNativeInstallerSourceRequiresOwnPoolAndExactStage(t *testing.T) {
	for _, s := range []*SpaceStore{nil, {}} {
		for _, v := range []int64{0, 43, 44, 45, 50, 51} {
			_, err := s.InspectNativeInstallerSource(context.Background(), v)
			require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
		}
	}
}

func TestNativeInstallerSourceUnknownStageRejectsBeforePoolAccess(t *testing.T) {
	// A zero pool cannot open a connection. Unsupported versions must deny at
	// the API boundary before touching it; admitting a future version fails here.
	s := &SpaceStore{Pool: &pgxpool.Pool{}}
	for _, version := range []int64{0, 43, 46, 49, 52, 100} {
		_, err := s.InspectNativeInstallerSource(context.Background(), version)
		require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
	}
}
