package privacy

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCheckAllowed_granted(t *testing.T) {
	owner := uuid.New()
	viewer := uuid.New()
	m := Matcher{Social: stubSocial{friends: map[string]bool{pairKey(viewer, owner): true}}}
	err := CheckAllowed(m, context.Background(), owner, viewer, FriendsOnly(), false)
	require.NoError(t, err)
}

func TestCheckAllowed_denied(t *testing.T) {
	owner := uuid.New()
	viewer := uuid.New()
	m := Matcher{Social: stubSocial{}}
	err := CheckAllowed(m, context.Background(), owner, viewer, FriendsOnly(), false)
	require.ErrorIs(t, err, ErrDenied)
}
