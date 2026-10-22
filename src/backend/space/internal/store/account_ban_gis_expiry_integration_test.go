//go:build spacebanintegration

package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	userv1 "voice.app/voice/user/v1"
)

// Explicit owned-clock phase after TestAccountBanGIS50; no journal aging or
// fabricated receipt. The actual expiry writer must capture its native frame.
func TestAccountBanGISExpiry50(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn)
	owner, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(owner.Close)
	clock := func(micros int64) {
		t.Helper()
		_, err := owner.Exec(ctx, fmt.Sprintf(`COPY (SELECT '%d') TO '/fixture-clock/walltime-us'`, micros))
		require.NoError(t, err)
	}
	clock(0)
	t.Cleanup(func() { clock(0) })
	serving := projectionServingLogin(t, ctx, owner)
	space := nativeBanSpace(8)
	st := &SpaceStore{Pool: serving, AccountCaptures: nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		out := nativeBanOwnerFixture(r)
		for _, result := range out.Results {
			if result.Entry.ProfileId == nativeBanProfile(10).String() {
				result.Frozen, result.Deleted = false, false
			}
		}
		return out, nil
	})}
	digest := sha256.Sum256([]byte("component-gis-revision4-before-expiry"))
	input := CommunityRosterInput{OperationID: uuid.New(), ApplicationID: uuid.MustParse("40000000-0000-0000-0000-000000000001"), EnvironmentID: uuid.MustParse("40000000-0000-0000-0000-000000000002"), CorporationKey: "native7-fixture-8", SpaceID: space, OwnerGeneration: 1, SourceRevision: 4, SnapshotSHA256: digest[:], LeaseExpiresAt: time.Now().Add(90 * time.Second), ProfileIDs: []uuid.UUID{nativeBanProfile(6), nativeBanProfile(10)}}
	receipt, err := st.ApplyCommunityRoster(ctx, input)
	require.NoError(t, err)
	var before, after uuid.UUID
	require.NoError(t, serving.QueryRow(ctx, `SELECT episode_id FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE'`, space, nativeBanProfile(6)).Scan(&before))
	require.NoError(t, st.RemoveMember(ctx, space, nativeBanProfile(6)), "ordinary-only removal uses its actual native producer")
	member, err := st.IsSpaceMember(ctx, space, nativeBanProfile(6))
	require.NoError(t, err)
	require.True(t, member, "ordinary leave preserves independently eligible community")
	require.NoError(t, serving.QueryRow(ctx, `SELECT episode_id FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE'`, space, nativeBanProfile(6)).Scan(&after))
	require.Equal(t, before, after, "continuous union membership keeps its effective episode")
	var expires time.Time
	require.NoError(t, serving.QueryRow(ctx, `SELECT roster_lease_expires_at FROM community_owner_authority WHERE space_id=$1`, space).Scan(&expires))
	clock(expires.UnixMicro() + 1)
	removed, err := st.RevokeExpiredCommunityRosterMemberships(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, removed)
	for _, profile := range input.ProfileIDs {
		member, err = st.IsSpaceMember(ctx, space, profile)
		require.NoError(t, err)
		require.False(t, member, "expired community and retired ordinary sources cannot grant")
	}
	st.AccountCaptures = nil
	replay, err := st.ApplyCommunityRoster(ctx, input)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.False(t, replay.Current)
	require.Equal(t, receipt.ReceiptID, replay.ReceiptID)
	require.Equal(t, receipt.EffectiveProfileIDs, replay.EffectiveProfileIDs, "historical projection is not current membership")
	var retained time.Time
	require.NoError(t, serving.QueryRow(ctx, `SELECT roster_lease_expires_at FROM community_owner_authority WHERE space_id=$1`, space).Scan(&retained))
	require.Equal(t, expires, retained)
	var count int
	require.NoError(t, serving.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space).Scan(&count))
	require.Equal(t, 1, count)
}
