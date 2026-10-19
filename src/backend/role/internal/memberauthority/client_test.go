package memberauthority

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type authorityFunc func(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error)

func (f authorityFunc) GetMemberAdmissionAuthority(ctx context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest, _ ...grpc.CallOption) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
	return f(ctx, req)
}

func TestAuthorityClientSignsFreshExactRoleRequests(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "role", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	req, reply, _ := observationFixture()
	requestIDs, jtis := map[string]bool{}, map[string]bool{}
	c := &Client{Issuer: issuer, Space: authorityFunc(func(ctx context.Context, got *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
		require.Equal(t, req, got)
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Len(t, md, 2, "user credentials and profile metadata must not propagate")
		transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), md))
		require.NoError(t, err)
		hash, err := principal.RequestHash(got)
		require.NoError(t, err)
		p, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "role", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }})
		require.NoError(t, err)
		require.False(t, requestIDs[p.RequestID])
		requestIDs[p.RequestID] = true
		require.False(t, jtis[p.JWTID])
		jtis[p.JWTID] = true
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
		return reply, nil
	})}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer user-secret", "x-voice-profile-id", "spoof", "x-request-id", "inherited"))
	for range 2 {
		observation, err := c.ObserveMember(ctx, req)
		require.NoError(t, err)
		require.True(t, observation.Available(time.Now()))
	}
	c.Space = authorityFunc(func(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
		return nil, nil
	})
	_, err = c.ObserveMember(ctx, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = c.ObserveMember(cancelled, req)
	require.Equal(t, codes.Canceled, status.Code(err))
}
