package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNative45SpaceMetadataUpdatePreservesMemberCohort(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	owner := uuid.New()
	space, err := admin.CreateSpace(ctx, owner, "metadata before guard", "", "public")
	require.NoError(t, err)
	unknown, err := admin.CreateSpace(ctx, owner, "uninitialized metadata", "", "public")
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	db.Run(true, "goto", "45")
	_, err = admin.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	st := &SpaceStore{Pool: serving}
	const snapshot = `SELECT jsonb_build_object(
	 'ordinary',(SELECT jsonb_agg(to_jsonb(h) ORDER BY profile_id) FROM public.space_ordinary_member_heads h WHERE space_id=$1),
	 'history',(SELECT jsonb_agg(to_jsonb(h) ORDER BY profile_id) FROM public.space_effective_member_heads h WHERE space_id=$1),
	 'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY profile_id) FROM public.space_members m WHERE space_id=$1))::text`
	var before, after string
	require.NoError(t, db.Pool.QueryRow(ctx, snapshot, space.ID).Scan(&before))
	name := "metadata after guard"
	row, err := st.UpdateSpace(ctx, space.ID, UpdateSpaceInput{Name: &name})
	require.NoError(t, err, "owner-unchanged metadata is not a membership source mutation")
	require.Equal(t, name, row.Name)
	allow := false
	config := `{"enabled":false}`
	row, err = st.UpdateSpace(ctx, space.ID, UpdateSpaceInput{AllowGuests: &allow, MMConfigJSON: &config})
	require.NoError(t, err)
	require.False(t, row.AllowGuests)
	require.NoError(t, db.Pool.QueryRow(ctx, snapshot, space.ID).Scan(&after))
	require.Equal(t, before, after, "settings do not retire/rebind membership or invent another effective episode")
	_, err = st.UpdateSpace(ctx, unknown.ID, UpdateSpaceInput{Name: &name})
	require.NoError(t, err, "metadata is independent of membership serving readiness")
	var unchanged string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT name FROM public.spaces WHERE id=$1`, unknown.ID).Scan(&unchanged))
	require.Equal(t, name, unchanged)
	_, err = st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: unknown.ID.String(), ProfileId: owner.String()})
	require.Error(t, err, "metadata success must not repair or authorize unknown membership readiness")
	var registry int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.spaces WHERE space_id=$1`, unknown.ID).Scan(&registry))
	require.Zero(t, registry)
	bad := "unsupported visibility"
	_, err = st.UpdateSpace(ctx, space.ID, UpdateSpaceInput{Visibility: &bad})
	require.Error(t, err, "actual constraint failure must roll back its entire capture")
	require.NoError(t, db.Pool.QueryRow(ctx, snapshot, space.ID).Scan(&after))
	require.Equal(t, before, after)
	var frames int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames`).Scan(&frames))
	require.Zero(t, frames)
	require.NoError(t, nativeReadinessTx(ctx, serving, space.ID, func(tx pgx.Tx) error {
		return (&SpaceStore{Pool: serving, tx: tx}).RequireScopedNativeMembershipReadiness(ctx, space.ID)
	}))
}
