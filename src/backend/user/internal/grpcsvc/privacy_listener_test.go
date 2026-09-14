package grpcsvc

import (
	"context"
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
