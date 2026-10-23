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

func TestNativeMembershipACLRequiresExactSpaceFence(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, owner, profile, other := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	binding := ensureMemberBinding(space, profile, 1)
	receipt, err := st.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	observed := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	require.NoError(t, st.WithinSpaces(observed, []uuid.UUID{space}, func(scoped *RoleStore) error {
		present, err := scoped.QualifiedMemberPresent(observed, space, profile)
		require.NoError(t, err)
		require.True(t, present)
		return nil
	}))
	err = st.WithinSpaces(observed, []uuid.UUID{other}, func(scoped *RoleStore) error {
		_, err := scoped.QualifiedMemberPresent(observed, space, profile)
		return err
	})
	require.ErrorIs(t, err, ErrScopeUnavailable, "another Space's transaction cannot qualify this membership")
}
