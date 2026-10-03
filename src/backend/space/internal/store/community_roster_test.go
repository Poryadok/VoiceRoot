package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCommunityRosterProjectionIsGenerationAndLeaseFenced(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	applicationID, environmentID, ownerAccountID, ownerProfileID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: applicationID,
		EnvironmentID: environmentID, OwnerAccountID: ownerAccountID, OwnerProfileID: ownerProfileID,
		CorporationKey: "corp-test", TemplateID: "guild-default"})
	require.NoError(t, err)
	one, two, manual := uuid.New(), uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, manual)
	require.NoError(t, err)
	first := CommunityRosterInput{OperationID: uuid.New(), ApplicationID: applicationID, EnvironmentID: environmentID,
		CorporationKey: "corp-test", SpaceID: space.ID, OwnerGeneration: 1, SourceRevision: 1,
		SnapshotSHA256: []byte("11111111111111111111111111111111"), LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{one}}
	firstReceipt, err := st.ApplyCommunityRoster(ctx, first)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, firstReceipt.ReceiptID)
	replayed, err := st.ApplyCommunityRoster(ctx, first)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, firstReceipt.ReceiptID, replayed.ReceiptID)
	member, err := st.IsSpaceMember(ctx, space.ID, one)
	require.NoError(t, err)
	require.True(t, member)
	coMembers, err := st.AreCoMembers(ctx, ownerProfileID, one, []uuid.UUID{space.ID})
	require.NoError(t, err)
	require.True(t, coMembers, "Chat co-membership sees active source-owned Space membership")
	ownerAccount, memberAccount := uuid.New(), uuid.New()
	coMembers, err = st.AreCoMembersWithAccountBans(ctx, ownerProfileID, one, []uuid.UUID{ownerAccount, memberAccount}, []uuid.UUID{space.ID})
	require.NoError(t, err)
	require.True(t, coMembers, "trusted account lookup preserves active shared membership")
	_, err = pool.Exec(ctx, `INSERT INTO space_bans(space_id,account_id,banned_by_profile_id,reason) VALUES($1,$2,$3,'policy')`, space.ID, memberAccount, ownerProfileID)
	require.NoError(t, err)
	coMembers, err = st.AreCoMembersWithAccountBans(ctx, ownerProfileID, one, []uuid.UUID{ownerAccount, memberAccount}, []uuid.UUID{space.ID})
	require.NoError(t, err)
	require.False(t, coMembers, "Chat/Search shared audience excludes an account banned from the Space")
	_, err = pool.Exec(ctx, `DELETE FROM space_bans WHERE space_id=$1 AND account_id=$2`, space.ID, memberAccount)
	require.NoError(t, err)

	second := first
	second.OperationID, second.SourceRevision = uuid.New(), 2
	second.SnapshotSHA256 = []byte("22222222222222222222222222222222")
	second.LeaseExpiresAt = time.Now().Add(time.Minute)
	second.ProfileIDs = []uuid.UUID{two}
	_, err = st.ApplyCommunityRoster(ctx, second)
	require.NoError(t, err)
	member, err = st.IsSpaceMember(ctx, space.ID, one)
	require.NoError(t, err)
	require.False(t, member, "departed community member loses effective Space access")
	member, err = st.IsSpaceMember(ctx, space.ID, two)
	require.NoError(t, err)
	require.True(t, member)
	mySpaces, err := st.ListMySpacesPage(ctx, two, "", 5)
	require.NoError(t, err)
	require.Len(t, mySpaces.Rows, 1, "active managed members can find their community Space")
	require.Equal(t, space.ID, mySpaces.Rows[0].ID)
	var memberCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&memberCount))
	require.Equal(t, 3, memberCount, "Space count includes manual and active source membership exactly once")
	member, err = st.IsSpaceMember(ctx, space.ID, manual)
	require.NoError(t, err)
	require.True(t, member, "community replacement cannot remove independent/manual membership")

	_, err = pool.Exec(ctx, `UPDATE community_owner_authority SET owner_generation=2,roster_source_revision=0,roster_sha256=NULL,roster_lease_expires_at=NULL WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	stale := second
	stale.OperationID, stale.SourceRevision = uuid.New(), 3
	stale.SnapshotSHA256 = []byte("33333333333333333333333333333333")
	_, err = st.ApplyCommunityRoster(ctx, stale)
	require.ErrorIs(t, err, ErrCommunityRosterConflict, "stale Owner generation cannot reapply access")
	fresh := second
	fresh.OperationID, fresh.OwnerGeneration, fresh.SourceRevision = uuid.New(), 2, 1
	fresh.SnapshotSHA256 = []byte("44444444444444444444444444444444")
	fresh.LeaseExpiresAt = time.Now().Add(time.Minute)
	_, err = st.ApplyCommunityRoster(ctx, fresh)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE community_roster_members SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE community_owner_authority SET roster_lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	member, err = st.IsSpaceMember(ctx, space.ID, two)
	require.NoError(t, err)
	require.False(t, member, "lease boundary denies access before cleanup worker runs")
	coMembers, err = st.AreCoMembers(ctx, ownerProfileID, two, []uuid.UUID{space.ID})
	require.NoError(t, err)
	require.False(t, coMembers, "Chat privacy audience denies expired membership")
	_, err = st.RevokeExpiredCommunityRosterMemberships(ctx)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&memberCount))
	require.Equal(t, 2, memberCount, "expiry cleanup repairs the denormalized member count")
}
