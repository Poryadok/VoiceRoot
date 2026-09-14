package grpcsvc

import (
	"context"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
)

func TestProtectedPrivacyDomainRequiresVerifiedSocial(t *testing.T) {
	req := &spacev1.AreCoMembersRequest{ProfileIdA: "77e391a9-6fb2-48b8-ad2e-cbc6d33a8eaf", ProfileIdB: "263c36fa-80de-45f1-8c39-00216ce1f3e8"}
	svc := &SocialPrivacyGRPC{Space: &SpaceGRPC{}}
	_, err := svc.AreCoMembers(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "social", Subject: "service:social", Audience: "space", RPC: socialprincipal.Method("space"), RequestHash: hash}
	_, err = svc.AreCoMembers(principal.WithVerified(context.Background(), p), req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "authenticated request reaches missing store")
	p.Issuer = "chat"
	p.Subject = "service:chat"
	_, err = svc.AreCoMembers(principal.WithVerified(context.Background(), p), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
