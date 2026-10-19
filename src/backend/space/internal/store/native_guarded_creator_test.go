package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNativeReadinessBornCreatorCapturesBeforeInsert(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "up")
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	st := &SpaceStore{Pool: serving}
	create, _, profile := signedNativeCreatorCurrent(t, st, &spacev1.CreateSpaceRequest{Name: "born under native44", Visibility: "private"})
	space, err := create()
	require.NoError(t, err, "actual signed Creator must begin its born frame before spaces INSERT")
	var valid bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE' AND boundary_state='KNOWN' AND generation=1 AND history_visible_since IS NOT NULL)
 AND EXISTS(SELECT 1 FROM public.space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE' AND publication_kind=3 AND NOT legacy AND generation=1)
 AND EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1 AND state='COMPLETE')
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$1)`, space.ID, profile).Scan(&valid))
	require.True(t, valid, "source publication/history/registry and frame completion commit atomically")
	require.NoError(t, nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
		return (&SpaceStore{Pool: serving, tx: tx}).RequireScopedNativeMembershipReadiness(ctx, space.ID)
	}))
	response, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: profile.String()})
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE, response.GetOrdinary().GetPublicationKind())
	_, err = create()
	require.Error(t, err, "the real durable Creator JTI replay cannot create another Space")
	failed, _, _ := signedNativeCreatorCurrent(t, st, &spacev1.CreateSpaceRequest{Name: "invalid born rollback", Visibility: "invalid-mode"})
	_, err = failed()
	require.Error(t, err, "real spaces CHECK rejects after DB begin, not a fabricated publication failure")
	var spaces, registries, frames int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.spaces),(SELECT count(*) FROM native_membership_guard.spaces),(SELECT count(*) FROM native_membership_guard.frames)`).Scan(&spaces, &registries, &frames))
	require.Equal(t, 1, spaces)
	require.Equal(t, 1, registries, "failed born transaction cannot leave a ready registry")
	require.Zero(t, frames)
}
