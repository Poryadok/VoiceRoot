package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

// Producer-owned counterpart of183's actual Space9096->Role25 protected
// transport discriminator. Raw head/catalog presence cannot ready this route.
func TestNativeAuthority44RequiresCompletedRegistry(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	space, err := admin.CreateSpace(ctx, uuid.New(), "authority registry gate", "", "public")
	require.NoError(t, err)
	db.Run(true, "up")
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	st := &SpaceStore{Pool: serving}
	req := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: space.OwnerProfileID.String()}
	response, err := st.ReadMemberAdmissionAuthority(ctx, req)
	t.Logf("uninitialized route response=%v error=%v", response, err)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "protected producer must refuse before full administrative cutover")
	require.Nil(t, response)
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.spaces WHERE space_id=$1`, space.ID).Scan(&count))
	require.Zero(t, count, "a read must never initialize or repair its registry")
	require.Error(t, st.InitializeNativeMembership(ctx, space.ID), "ordinary LOGIN cannot become a deployment initializer")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.spaces WHERE space_id=$1`, space.ID).Scan(&count))
	require.Zero(t, count, "refused app initializer leaves no partial cutover")
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID), "real producer owns its own RC/fenced committed transaction")
	response, err = st.ReadMemberAdmissionAuthority(ctx, req)
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE, response.GetOrdinary().GetState())
	response, err = admin.ReadMemberAdmissionAuthority(ctx, req)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "an installer LOGIN is not a serving principal")
	require.Nil(t, response)
	require.Error(t, admin.InitializeNativeMembership(ctx, space.ID), "completed initialization is not a repair/re-cutover API")
	require.Error(t, admin.InitializeNativeMembership(ctx, uuid.New()), "missing Space cannot acquire a COMPLETE registry")
}
