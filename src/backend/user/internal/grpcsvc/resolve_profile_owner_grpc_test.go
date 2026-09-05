package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

// TestResolveAccountIDForProfile_InternalOwnerLookup exercises the Messaging-only
// ownership seam over a real bufconn gRPC boundary.  Visibility lookups must not
// replace this: a soft-deleted profile still resolves to its owning account.
func TestResolveAccountIDForProfile_InternalOwnerLookup(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli := startUserSettingsTestServer(t, profiles, store.NewPrivacyStore(pool))

	accountID := uuid.New()
	created, err := cli.EnsurePrimaryProfile(withInternalUserCtx(ctx), &userv1.EnsurePrimaryProfileRequest{AccountId: accountID.String()})
	require.NoError(t, err)
	profileID := created.GetProfile().GetId()

	messaging := metadata.AppendToOutgoingContext(ctx, authctx.HeaderInternalCaller, "messaging")
	resp, err := cli.ResolveAccountIDForProfile(messaging, &userv1.ResolveAccountIDForProfileRequest{ProfileId: profileID})
	require.NoError(t, err)
	require.Equal(t, accountID.String(), resp.GetAccountId())
	_, err = pool.Exec(ctx, "UPDATE profiles SET deleted_at = now() WHERE id = $1", uuid.MustParse(profileID))
	require.NoError(t, err)
	resp, err = cli.ResolveAccountIDForProfile(messaging, &userv1.ResolveAccountIDForProfileRequest{ProfileId: profileID})
	require.NoError(t, err)
	require.Equal(t, accountID.String(), resp.GetAccountId())

	for _, caller := range []context.Context{
		ctx,
		metadata.AppendToOutgoingContext(ctx, authctx.HeaderInternalCaller, "auth"),
		metadata.AppendToOutgoingContext(ctx, authctx.HeaderInternalCaller, " messaging "),
		metadata.AppendToOutgoingContext(ctx, authctx.HeaderInternalCaller, "messaging", authctx.HeaderInternalCaller, "messaging"),
	} {
		_, err := cli.ResolveAccountIDForProfile(caller, &userv1.ResolveAccountIDForProfileRequest{ProfileId: profileID})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}

	_, err = cli.ResolveAccountIDForProfile(messaging, &userv1.ResolveAccountIDForProfileRequest{ProfileId: "not-a-uuid"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = cli.ResolveAccountIDForProfile(messaging, &userv1.ResolveAccountIDForProfileRequest{ProfileId: uuid.NewString()})
	require.Equal(t, codes.NotFound, status.Code(err))

	pool.Close()
	_, err = cli.ResolveAccountIDForProfile(messaging, &userv1.ResolveAccountIDForProfileRequest{ProfileId: profileID})
	require.Equal(t, codes.Internal, status.Code(err))
}
