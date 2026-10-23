package store

import (
	"context"
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
