package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOrdinaryLossPreservesLiveCommunityMemberAndCount(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL independent membership sources")
	}
	for _, action := range []string{"leave", "kick", "native_leave", "expired_community", "rollback"} {
		t.Run(action, func(t *testing.T) {
			ctx, st := authorityRevisionSpaceThrough(t, 34)
			app, environment, ownerAccount, owner := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			space, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{
				OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment,
				OwnerAccountID: ownerAccount, OwnerProfileID: owner,
				CorporationKey: "independent-loss", TemplateID: "guild-default",
			})
			require.NoError(t, err)
			// Roster snapshots require canonical, unique profile ordering.
			member := uuid.MustParse("11111111-1111-4111-8111-111111111111")
			other := uuid.MustParse("22222222-2222-4222-8222-222222222222")
			_, err = st.JoinSpace(ctx, space.ID, member, uuid.New())
			require.NoError(t, err)
			_, err = st.ApplyCommunityRoster(ctx, CommunityRosterInput{
				OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment,
				CorporationKey: "independent-loss", SpaceID: space.ID, OwnerGeneration: 1, SourceRevision: 1,
				SnapshotSHA256: []byte("11111111111111111111111111111111"),
				LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{member, other},
			})
			require.NoError(t, err)
			count := func() int {
				t.Helper()
				var n int
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&n))
				return n
			}
			require.Equal(t, 3, count())
			if action == "native_leave" {
				raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000039_native_membership_authority.up.sql"))
				require.NoError(t, err)
				_, err = st.Pool.Exec(ctx, string(raw))
				require.NoError(t, err)
			}
			if action == "expired_community" {
				_, err = st.Pool.Exec(ctx, `UPDATE community_owner_authority SET roster_lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1`, space.ID)
				require.NoError(t, err)
			}
			if action == "rollback" {
				abort := errors.New("caller rollback")
				err = st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
					require.NoError(t, scoped.RemoveMember(ctx, space.ID, member))
					return abort
				})
				require.ErrorIs(t, err, abort)
				var ordinary bool
				require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, space.ID, member).Scan(&ordinary))
				require.True(t, ordinary)
				require.Equal(t, 3, count())
				return
			}
			if action == "kick" {
				err = st.KickMember(ctx, space.ID, member, owner)
			} else {
				err = st.RemoveMember(ctx, space.ID, member)
			}
			require.NoError(t, err)
			active, err := st.IsSpaceMember(ctx, space.ID, member)
			require.NoError(t, err)
			if action == "expired_community" {
				require.False(t, active)
				require.Equal(t, 1, count(), "expired community rows cannot retain capacity")
				return
			}
			require.True(t, active, "ordinary loss cannot retire an independent live community source")
			require.Equal(t, 3, count(), "capacity counts effective profiles, not removed ordinary rows")
			rows, _, err := st.ListSpaceMembersPage(ctx, space.ID, 10, "")
			require.NoError(t, err)
			require.Len(t, rows, 3)
			if action == "native_leave" {
				var state string
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, member).Scan(&state))
				require.Equal(t, "RETIRED", state)
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, member).Scan(&state))
				require.Equal(t, "ACTIVE", state)
			}
		})
	}
}
