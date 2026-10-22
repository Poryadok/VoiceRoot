//go:build spacebanintegration

package store

import (
	"context"
	"crypto/sha256"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	userv1 "voice.app/voice/user/v1"
)

// Explicit phase after real source migration/transition/enrollment. Controlled
// User facts qualify this component behavior only, never B15 real producers.
func TestAccountBanGIS50(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn)
	owner, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(owner.Close)
	serving := projectionServingLogin(t, ctx, owner)
	space := nativeBanSpace(8)
	var gisCalls atomic.Int32
	capture := nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		if r.Purpose == 3 {
			gisCalls.Add(1)
		}
		out := nativeBanOwnerFixture(r)
		for _, result := range out.Results {
			if result.Entry.ProfileId == nativeBanProfile(10).String() {
				// This new prospective alias is active throughout this fixture;
				// it never had the deleted legacy-profile fact used in B02.
				result.Frozen, result.Deleted = false, false
			}
		}
		return out, nil
	})
	st := &SpaceStore{Pool: serving, AccountCaptures: capture}
	allow := func(context.Context) error { return nil }
	require.NoError(t, nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), uuid.New(), nil, allow))
	digest := sha256.Sum256([]byte("component-gis-original-two-profiles-revision2"))
	input := CommunityRosterInput{OperationID: uuid.New(), ApplicationID: uuid.MustParse("40000000-0000-0000-0000-000000000001"), EnvironmentID: uuid.MustParse("40000000-0000-0000-0000-000000000002"), CorporationKey: "native7-fixture-8", SpaceID: space, OwnerGeneration: 1, SourceRevision: 2, SnapshotSHA256: digest[:], LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{nativeBanProfile(6), nativeBanProfile(10)}}
	receipt, err := st.ApplyCommunityRoster(ctx, input)
	require.NoError(t, err)
	require.EqualValues(t, 2, receipt.MemberCount, "claimed count remains the full input")
	require.Equal(t, input.ProfileIDs, receipt.ProfileIDs)
	require.Equal(t, []uuid.UUID{nativeBanProfile(6)}, receipt.EffectiveProfileIDs)
	require.EqualValues(t, 1, gisCalls.Load())
	var count int
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE'`, space, nativeBanProfile(10)).Scan(&count))
	require.Zero(t, count)
	var floor int64
	require.NoError(t, serving.QueryRow(ctx, `SELECT source_revision FROM space_account_ban_suppressions WHERE space_id=$1 AND profile_id=$2`, space, nativeBanProfile(10)).Scan(&floor))
	require.EqualValues(t, 2, floor)
	require.NoError(t, nativeUnbanExecute(t, st, space, nativeBanAccount(2), uuid.New(), allow))
	st.AccountCaptures = nil
	replay, err := st.ApplyCommunityRoster(ctx, input)
	require.NoError(t, err, "saved replay does not call unavailable User")
	require.True(t, replay.Replayed)
	require.Equal(t, receipt.ReceiptID, replay.ReceiptID)
	require.Equal(t, receipt.EffectiveProfileIDs, replay.EffectiveProfileIDs)
	sameRevision := input
	sameRevision.OperationID = uuid.New()
	replay, err = st.ApplyCommunityRoster(ctx, sameRevision)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, receipt.ReceiptID, replay.ReceiptID)
	var lease time.Time
	require.NoError(t, serving.QueryRow(ctx, `SELECT roster_lease_expires_at FROM community_owner_authority WHERE space_id=$1`, space).Scan(&lease))
	require.WithinDuration(t, input.LeaseExpiresAt, lease, time.Microsecond)
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE'`, space, nativeBanProfile(10)).Scan(&count))
	require.Zero(t, count, "same operation/revision cannot resurrect the suppressed source")
	st.AccountCaptures = capture
	newer := input
	newer.OperationID, newer.SourceRevision = uuid.New(), 3
	newDigest := sha256.Sum256([]byte("component-gis-qualified-revision3"))
	newer.SnapshotSHA256, newer.LeaseExpiresAt = newDigest[:], time.Now().Add(time.Minute)
	qualified, err := st.ApplyCommunityRoster(ctx, newer)
	require.NoError(t, err)
	require.Equal(t, input.ProfileIDs, qualified.EffectiveProfileIDs)
	require.EqualValues(t, 2, gisCalls.Load())
	var generation int64
	var episode uuid.UUID
	require.NoError(t, serving.QueryRow(ctx, `SELECT generation,episode_id FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE'`, space, nativeBanProfile(10)).Scan(&generation, &episode))
	require.Positive(t, generation)
	require.NotEqual(t, uuid.Nil, episode)
	require.NoError(t, serving.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space).Scan(&count))
	require.Equal(t, 3, count, "union contains owner, unrelated B, and one newly eligible alias")
}
