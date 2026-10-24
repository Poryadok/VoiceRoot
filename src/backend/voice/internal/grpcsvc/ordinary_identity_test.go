package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/authctx"
)

func TestOrdinaryIdentityStoresOnlyVerifiedActorsOwnEpoch(t *testing.T) {
	account, profile, peerAccount, peer := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	s := &VoiceGRPC{AccountVoiceProfiles: accountResolverStub{profile: account, peer: peerAccount}}
	verified := principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "voice", AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 9}
	ctx := principal.WithVerified(context.Background(), verified)
	id, err := s.ordinaryIdentity(ctx, profile.String(), true)
	require.NoError(t, err)
	require.Equal(t, int64(9), id.SessionEpoch)
	invited, err := s.ordinaryIdentity(ctx, peer.String(), false)
	require.NoError(t, err)
	require.Equal(t, peerAccount, invited.AccountID)
	require.Zero(t, invited.SessionEpoch)
	_, err = s.ordinaryIdentity(ctx, peer.String(), true)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	verified.SessionEpoch = 0
	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderAccountID, account.String()))
	ctx = principal.WithVerified(ctx, verified)
	_, err = s.ordinaryIdentity(ctx, profile.String(), true)
	require.Error(t, err, "headers cannot repair an invalid verified principal")
}

func TestOrdinaryLegacyIdentityRemainsExplicitlyUnknownEpoch(t *testing.T) {
	account, profile := uuid.New(), uuid.New()
	s := &VoiceGRPC{AccountVoiceProfiles: accountResolverStub{profile: account}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderAccountID, account.String()))
	id, err := s.ordinaryIdentity(ctx, profile.String(), true)
	require.NoError(t, err)
	require.Zero(t, id.SessionEpoch, "trusted account metadata is not a signed admission epoch")
}
