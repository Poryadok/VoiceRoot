package store

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNativeReadersRequireFreshEvidenceAt21(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, owner := uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	_, err := st.GetMemberRoles(ctx, space, owner)
	require.ErrorIs(t, err, ErrScopeUnavailable, "actual native reader must not export a raw Owner tuple at21 without Space authority")
	_, err = st.GetEffectiveMask(ctx, space, owner, nil, nil)
	require.ErrorIs(t, err, ErrScopeUnavailable)
	_, err = st.CanCreateRole(ctx, space, owner, 0)
	require.ErrorIs(t, err, ErrScopeUnavailable, "hierarchy must consume the same qualified view")
}
