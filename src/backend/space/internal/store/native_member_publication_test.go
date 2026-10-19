package store

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
)

func TestNativeCommunityOwnerPublicationIsPositiveAtomicAndHistoricalReplayIsInert(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	for _, name := range []string{"000015_audit_ledger.up.sql", "000022_lifecycle_runtime.up.sql", "000023_lifecycle_evidence_retention.up.sql", "000024_authority_revision.up.sql", "000030_entry_policy.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	applyMemberAuthorityIfPresent(t, st)
	_, err := pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(33,false)`)
	require.NoError(t, err)
	in := CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(), OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "native-bootstrap", TemplateID: "guild-default"}
	type result struct {
		row     *SpaceRow
		created bool
		err     error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			row, created, err := st.CreateCommunityBootstrapSpace(ctx, in)
			results <- result{row, created, err}
		}()
	}
	wg.Wait()
	close(results)
	var row *SpaceRow
	createdCount := 0
	for result := range results {
		require.NoError(t, result.err)
		if row != nil {
			require.Equal(t, row.ID, result.row.ID)
		}
		row = result.row
		if result.created {
			createdCount++
		}
	}
	require.Equal(t, 1, createdCount)
	req := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: row.ID.String(), ProfileId: in.OwnerProfileID.String()}
	observed, err := st.ReadMemberAdmissionAuthority(ctx, req)
	require.NoError(t, err)
	require.EqualValues(t, 1, observed.Ordinary.HeadGeneration)
	require.False(t, observed.Ordinary.ExplicitLegacy)
	require.Empty(t, observed.Ordinary.RoleReceiptSha256, "native provenance is never fabricated Role READY")
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE, observed.Ordinary.PublicationKind)
	require.Len(t, observed.Ordinary.NativePublication.BindingSha256, 32)
	_, err = pool.Exec(ctx, `UPDATE community_owner_authority SET owner_profile_id=$2 WHERE space_id=$1`, row.ID, uuid.New())
	require.NoError(t, err)
	_, err = st.ReadMemberAdmissionAuthority(ctx, req)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "discordant owner authorities are unknown, never a positive native source")
	_, err = pool.Exec(ctx, `UPDATE community_owner_authority SET owner_profile_id=$2 WHERE space_id=$1`, row.ID, in.OwnerProfileID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE space_native_member_publications SET binding_bytes=$2 WHERE operation_id=$1`, in.OperationID, []byte("different provenance"))
	require.Error(t, err, "historical native proof cannot be rewritten in place")
	current, err := st.IsCommunityBootstrapPublicationCurrent(ctx, in.OperationID, row.ID)
	require.NoError(t, err)
	require.True(t, current)
	var proofs, members int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_native_member_publications WHERE operation_id=$1`, in.OperationID).Scan(&proofs))
	require.Equal(t, 1, proofs)
	require.NoError(t, pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, row.ID).Scan(&members))
	require.Equal(t, 1, members)
	require.NoError(t, st.RemoveMember(ctx, row.ID, in.OwnerProfileID))
	_, created, err := st.CreateCommunityBootstrapSpace(ctx, in)
	require.NoError(t, err)
	require.False(t, created)
	current, err = st.IsCommunityBootstrapPublicationCurrent(ctx, in.OperationID, row.ID)
	require.NoError(t, err)
	require.False(t, current, "historical bootstrap cannot request a new Owner baseline after member loss")
	member, err := st.IsSpaceMember(ctx, row.ID, in.OwnerProfileID)
	require.NoError(t, err)
	require.False(t, member, "old bootstrap must not resurrect an ordinary membership")
	require.NoError(t, pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, row.ID).Scan(&members))
	require.Zero(t, members)
	changed := in
	changed.OwnerAccountID = uuid.New()
	_, _, err = st.CreateCommunityBootstrapSpace(ctx, changed)
	require.ErrorIs(t, err, ErrCommunityBootstrapConflict)
	_, err = pool.Exec(ctx, `CREATE FUNCTION fail_native_owner_member() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture native owner member failure'; END $$;
	CREATE TRIGGER fail_native_owner_member BEFORE INSERT ON space_members FOR EACH ROW EXECUTE FUNCTION fail_native_owner_member()`)
	require.NoError(t, err)
	bad := in
	bad.OperationID, bad.OwnerProfileID = uuid.New(), uuid.New()
	bad.CorporationKey = "failed-native-bootstrap"
	_, _, err = st.CreateCommunityBootstrapSpace(ctx, bad)
	require.Error(t, err)
	var leaked bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM community_bootstrap_operations WHERE operation_id=$1)
	 OR EXISTS(SELECT 1 FROM spaces WHERE owner_profile_id=$2)
	 OR EXISTS(SELECT 1 FROM space_native_member_publications WHERE operation_id=$1)
	 OR EXISTS(SELECT 1 FROM space_ordinary_member_heads WHERE profile_id=$2)
	 OR EXISTS(SELECT 1 FROM space_membership_ownership_heads WHERE owner_profile_id=$2)`, bad.OperationID, bad.OwnerProfileID).Scan(&leaked))
	require.False(t, leaked, "failed native membership insert must roll back Space, operation, private proof and both authority heads")
}
