package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
)

func TestProtectedPrivacyDomainRequiresVerifiedSocial(t *testing.T) {
	req := &userv1.GetPrivacySettingsRequest{ProfileId: "77e391a9-6fb2-48b8-ad2e-cbc6d33a8eaf"}
	svc := &SocialPrivacyGRPC{User: &UserGRPC{}}
	_, err := svc.GetPrivacySettings(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "social", Subject: "service:social", Audience: "user", RPC: socialprincipal.Method("user"), RequestHash: hash}
	_, err = svc.GetPrivacySettings(principal.WithVerified(context.Background(), p), req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "authenticated request reaches missing store")
	p.Issuer = "chat"
	p.Subject = "service:chat"
	_, err = svc.GetPrivacySettings(principal.WithVerified(context.Background(), p), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestProtectedSocialGetProfilesIsRequestBoundAndCapped(t *testing.T) {
	svc := &SocialPrivacyGRPC{User: &UserGRPC{}}
	emptyRequest := &userv1.GetProfilesRequest{}
	_, err := svc.GetProfiles(context.Background(), emptyRequest)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	hash, err := principal.RequestHash(emptyRequest)
	require.NoError(t, err)
	identity := principal.Principal{
		Kind: "service", Issuer: "social", Subject: "service:social",
		Audience: "user", RPC: socialprincipal.Method("user"), RequestHash: hash,
	}
	resp, err := svc.GetProfiles(principal.WithVerified(context.Background(), identity), emptyRequest)
	require.NoError(t, err, "a correctly request-bound Social principal can use the existing batch profile lookup")
	require.Empty(t, resp.GetProfileList().GetProfiles())

	tooMany := &userv1.GetProfilesRequest{ProfileIds: make([]string, socialGetProfilesMaxProfiles+1)}
	for i := range tooMany.ProfileIds {
		tooMany.ProfileIds[i] = uuid.NewString()
	}
	hash, err = principal.RequestHash(tooMany)
	require.NoError(t, err)
	identity.RequestHash = hash
	_, err = svc.GetProfiles(principal.WithVerified(context.Background(), identity), tooMany)
	require.Equal(t, codes.InvalidArgument, status.Code(err), "the protected batch is bounded before User store access")
}
