package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNative45CommunityBootstrapBornPublicationAndReplay(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "up")
	_, err := (&SpaceStore{Pool: db.Pool}).TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	st := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	in := CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(), OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "qualified-community", TemplateID: "guild-default"}
	space, created, err := st.CreateCommunityBootstrapSpace(ctx, in)
	require.NoError(t, err, "real GIS operation must capture its born identity before the first guarded source SQL")
	require.True(t, created)
	facts, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: in.OwnerProfileID.String()})
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE, facts.Ordinary.PublicationKind)
	var before, after string
	const snapshot = `SELECT jsonb_build_object('head',(SELECT to_jsonb(h) FROM public.space_effective_member_heads h WHERE space_id=$1 AND profile_id=$2),'registry',(SELECT to_jsonb(r) FROM native_membership_guard.spaces r WHERE space_id=$1))::text`
	require.NoError(t, db.Pool.QueryRow(ctx, snapshot, space.ID, in.OwnerProfileID).Scan(&before))
	var complete bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1 AND state='COMPLETE')
 AND EXISTS(SELECT 1 FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE' AND boundary_state='KNOWN' AND generation=1)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$1)`, space.ID, in.OwnerProfileID).Scan(&complete))
	require.True(t, complete)
	replayed, created, err := st.CreateCommunityBootstrapSpace(ctx, in)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, space.ID, replayed.ID)
	require.NoError(t, db.Pool.QueryRow(ctx, snapshot, space.ID, in.OwnerProfileID).Scan(&after))
	require.Equal(t, before, after, "completed replay never writes source/history/registry")
	changed := in
	changed.OwnerProfileID = uuid.New()
	_, _, err = st.CreateCommunityBootstrapSpace(ctx, changed)
	require.ErrorIs(t, err, ErrCommunityBootstrapConflict)
	changed = in
	changed.OperationID = uuid.New()
	_, _, err = st.CreateCommunityBootstrapSpace(ctx, changed)
	require.ErrorIs(t, err, ErrCommunityBootstrapConflict, "one business identity cannot create a second Space")
	var spaces, operations, frames int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.spaces),(SELECT count(*) FROM public.community_bootstrap_operations),(SELECT count(*) FROM native_membership_guard.frames)`).Scan(&spaces, &operations, &frames))
	require.Equal(t, 1, spaces)
	require.Equal(t, 1, operations)
	require.Zero(t, frames)
	t.Run("concurrent exact operation creates once", func(t *testing.T) {
		in.OperationID, in.CorporationKey = uuid.New(), "concurrent-community"
		type outcome struct {
			row     *SpaceRow
			created bool
			err     error
		}
		results := make(chan outcome, 2)
		for range 2 {
			go func() {
				row, created, err := st.CreateCommunityBootstrapSpace(ctx, in)
				results <- outcome{row, created, err}
			}()
		}
		first, second := <-results, <-results
		require.NoError(t, first.err)
		require.NoError(t, second.err)
		require.Equal(t, first.row.ID, second.row.ID)
		require.NotEqual(t, first.created, second.created)
	})
	t.Run("actual publication conflict rolls back born transaction", func(t *testing.T) {
		create, _, _ := signedNativeCreatorCurrent(t, st, &spacev1.CreateSpaceRequest{Name: "Actual publication uniqueness control", Visibility: "private"})
		creatorSpace, err := create()
		require.NoError(t, err)
		var occupied uuid.UUID
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT operation_id FROM public.space_native_creator_operations WHERE space_id=$1`, creatorSpace.ID).Scan(&occupied))
		failed := in
		failed.OperationID, failed.CorporationKey = occupied, "publication-conflict"
		_, _, err = st.CreateCommunityBootstrapSpace(ctx, failed)
		require.Error(t, err, "real native publication uniqueness rejects after born Space and journal INSERTs")
		var constraint *pgconn.PgError
		require.ErrorAs(t, err, &constraint, "a guard/deadline/setup failure cannot satisfy the publication rollback oracle")
		require.Equal(t, "23505", constraint.Code)
		require.Equal(t, "space_native_member_publications_pkey", constraint.ConstraintName)
		var clean bool
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.community_bootstrap_operations WHERE operation_id=$1)
 AND (SELECT count(*) FROM public.spaces)=3 AND (SELECT count(*) FROM native_membership_guard.spaces)=3
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, occupied).Scan(&clean))
		require.True(t, clean, "failed native publication cannot retain journal, Space, registry or frame")
	})
}

func TestNative39CommunityBootstrapHistoricalPublication(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "39")
	st := &SpaceStore{Pool: db.Pool}
	in := CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(), OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "historical39", TemplateID: "guild-default"}
	space, created, err := st.CreateCommunityBootstrapSpace(ctx, in)
	require.NoError(t, err)
	require.True(t, created)
	facts, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: in.OwnerProfileID.String()})
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE, facts.Ordinary.PublicationKind)
	replayed, created, err := st.CreateCommunityBootstrapSpace(ctx, in)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, space.ID, replayed.ID)
}
