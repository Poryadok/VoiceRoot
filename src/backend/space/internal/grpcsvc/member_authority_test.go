package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/memberauthorityprincipal"
)

type memberAuthorityStoreSpy struct {
	calls int
	err   error
}

func (s *memberAuthorityStoreSpy) ReadMemberAdmissionAuthority(_ context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
	s.calls++
	return &spacev1.GetMemberAdmissionAuthorityResponse{ProtocolVersion: 1, SpaceId: req.SpaceId, ProfileId: req.ProfileId}, s.err
}

func TestMemberAuthorityHandlerRequiresPrivateExactProofAndMapsUnknown(t *testing.T) {
	req := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), ProfileId: uuid.NewString()}
	spy := &memberAuthorityStoreSpy{}
	service := &MemberAuthorityGRPC{Store: spy}
	for _, ctx := range []context.Context{context.Background(), metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-profile-id", req.ProfileId)), principal.WithVerified(context.Background(), principal.Principal{Issuer: "role", Subject: "service:role", Kind: "service"})} {
		_, err := service.GetMemberAdmissionAuthority(ctx, req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	require.Zero(t, spy.calls)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "role", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	requestID := uuid.NewString()
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName, RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	v := memberauthorityprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	_, err = v.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
		changed := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: req.SpaceId, ProfileId: uuid.NewString()}
		_, changedErr := service.GetMemberAdmissionAuthority(ctx, changed)
		require.Equal(t, codes.Unauthenticated, status.Code(changedErr))
		require.Zero(t, spy.calls)
		response, readErr := service.GetMemberAdmissionAuthority(ctx, req)
		require.NoError(t, readErr)
		require.Equal(t, req.ProfileId, response.ProfileId)
		spy.err = errors.New("private database failure")
		_, readErr = service.GetMemberAdmissionAuthority(ctx, req)
		require.Equal(t, codes.Unavailable, status.Code(readErr))
		require.NotContains(t, readErr.Error(), spy.err.Error())
		return response, nil
	})
	require.NoError(t, err)
	require.Equal(t, 2, spy.calls)
}

func TestMemberAuthorityRegistrationExposesOnlyOneRead(t *testing.T) {
	server := grpc.NewServer()
	t.Cleanup(server.Stop)
	RegisterMemberAuthorityServer(server, &MemberAuthorityGRPC{})
	info := server.GetServiceInfo()
	require.Len(t, info, 1)
	methods := info[spacev1.SpaceService_ServiceDesc.ServiceName].Methods
	require.Len(t, methods, 1)
	require.Equal(t, "GetMemberAdmissionAuthority", methods[0].Name)
}
