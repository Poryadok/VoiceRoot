package grpcsvc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spacev1 "voice.app/voice/space/v1"
	subscriptionv1 "voice.app/voice/subscription/v1"
)

// This is the current GetLimits/CheckLimit projection contract, not provider
// lifecycle or membership authorization coverage. Fixtures represent persisted
// entitlement outcomes, including a completed expiry (cancelled).
func TestScopeSpaceContract(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startSubscriptionPostgres(t, ctx)
	client, cleanup := startSubscriptionGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	purchaser, premiumAccount, otherAccount := uuid.New(), uuid.New(), uuid.New()
	proSpace, graceSpace, expiredSpace, freeSpace := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, fixture := range []struct {
		space  uuid.UUID
		status string
	}{
		{proSpace, "active"},
		{graceSpace, "grace_period"},
		{expiredSpace, "cancelled"},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO space_subscriptions
			(id, space_id, purchaser_account_id, plan, billing_period, status, provider,
			 provider_subscription_id, current_period_start, current_period_end, grace_period_end)
			VALUES ($1, $2, $3, 'space_pro', 'monthly', $4, 'paddle', $5,
			 now() - interval '1 month',
			 CASE WHEN $4 = 'cancelled' THEN now() - interval '1 day' ELSE now() + interval '1 day' END,
			 CASE WHEN $4 = 'grace_period' THEN now() + interval '7 days' ELSE NULL END)`,
			uuid.New(), fixture.space, purchaser, fixture.status, uuid.NewString())
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO subscriptions
		(id, account_id, plan, billing_period, status, provider, provider_subscription_id,
		 current_period_start, current_period_end)
		VALUES ($1, $2, 'premium', 'monthly', 'active', 'paddle', $3,
		 now() - interval '1 day', now() + interval '1 month')`,
		uuid.New(), premiumAccount, uuid.NewString())
	require.NoError(t, err)

	freeLimits := map[string]int64{
		"space_member_count": 50, "voice_room_participants": 32,
		"space_tree_nodes": 50, "space_banner": 0, "space_custom_emoji": 0,
	}
	proLimits := map[string]int64{
		"space_member_count": 5000, "voice_room_participants": 128,
		"space_tree_nodes": 500, "space_banner": 1, "space_custom_emoji": 1,
	}
	for _, tc := range []struct {
		name    string
		account uuid.UUID
		space   uuid.UUID
		pro     bool
	}{
		{"purchaser_pro_space", purchaser, proSpace, true},
		{"purchaser_other_free_space", purchaser, freeSpace, false},
		{"purchaser_expired_space", purchaser, expiredSpace, false},
		{"purchaser_grace_space", purchaser, graceSpace, true},
		{"premium_account_free_space", premiumAccount, freeSpace, false},
		{"other_account_pro_space", otherAccount, proSpace, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope := &spacev1.SpaceRef{Id: tc.space.String()}
			response, err := client.GetLimits(ctx, &subscriptionv1.GetLimitsRequest{
				AccountId: tc.account.String(), ScopeSpace: scope,
			})
			require.NoError(t, err)
			var got map[string]int64
			require.NoError(t, json.Unmarshal([]byte(response.GetLimits().GetLimitsJson()), &got))
			want := freeLimits
			if tc.pro {
				want = proLimits
			}
			require.Equal(t, want, got, "Space projection must contain only exact-Space benefits")

			check, err := client.CheckLimit(ctx, &subscriptionv1.CheckLimitRequest{
				AccountId: tc.account.String(), ScopeSpace: scope, LimitName: "space_member_count",
			})
			require.NoError(t, err)
			require.Equal(t, tc.pro, check.GetAllowed())
			remaining := int64(0)
			if tc.pro {
				remaining = 5000
			}
			require.Equal(t, remaining, check.GetRemaining())
		})
	}

	for _, tc := range []struct {
		name     string
		account  uuid.UUID
		upload   int64
		profiles int64
	}{
		{"space_pro_purchaser_is_personally_free", purchaser, 52428800, 2},
		{"personal_premium", premiumAccount, 209715200, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, scope := range []*spacev1.SpaceRef{nil, {}, {Id: "  "}} {
				response, err := client.GetLimits(ctx, &subscriptionv1.GetLimitsRequest{
					AccountId: tc.account.String(), ScopeSpace: scope,
				})
				require.NoError(t, err)
				var got map[string]int64
				require.NoError(t, json.Unmarshal([]byte(response.GetLimits().GetLimitsJson()), &got))
				require.Equal(t, tc.upload, got["file_upload_bytes"])
				require.Equal(t, tc.profiles, got["profile_count"])
				require.NotContains(t, got, "space_member_count")
				require.NotContains(t, got, "space_banner")
			}
		})
	}

	t.Run("check_requires_scope", func(t *testing.T) {
		for _, scope := range []*spacev1.SpaceRef{nil, {}, {Id: "  "}} {
			_, err := client.CheckLimit(ctx, &subscriptionv1.CheckLimitRequest{
				AccountId: purchaser.String(), ScopeSpace: scope, LimitName: "space_member_count",
			})
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		}
	})
	t.Run("malformed_scope_rejected_by_both_handlers", func(t *testing.T) {
		for _, id := range []string{"not-a-uuid", uuid.Nil.String()} {
			scope := &spacev1.SpaceRef{Id: id}
			_, err := client.GetLimits(ctx, &subscriptionv1.GetLimitsRequest{
				AccountId: purchaser.String(), ScopeSpace: scope,
			})
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			_, err = client.CheckLimit(ctx, &subscriptionv1.CheckLimitRequest{
				AccountId: purchaser.String(), ScopeSpace: scope, LimitName: "space_member_count",
			})
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		}
	})
}
