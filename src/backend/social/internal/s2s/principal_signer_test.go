package s2s

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

func testSocialIssuer(t *testing.T) (*principal.Issuer, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "social", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	return issuer, &key.PublicKey
}

type principalCapture struct {
	metadata metadata.MD
	request  proto.Message
}

func (c *principalCapture) Invoke(ctx context.Context, _ string, req, reply any, _ ...grpc.CallOption) error {
	c.metadata, _ = metadata.FromOutgoingContext(ctx)
	c.request = req.(proto.Message)
	switch r := reply.(type) {
	case *userv1.GetPrivacySettingsResponse:
		r.PrivacySettings = &userv1.PrivacySettings{}
	case *spacev1.AreCoMembersResponse:
		r.CoMembers = true
	}
	return nil
}
func (*principalCapture) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("unexpected stream")
}

func TestPrivacyAdaptersSignFreshBoundCredentials(t *testing.T) {
	issuer, key := testSocialIssuer(t)
	capture := &principalCapture{}
	user := &GRPCUserPrivacy{Client: userv1.NewUserServiceClient(capture), Issuer: issuer}
	space := &GRPCSpaceCoMembership{Client: spacev1.NewSpaceServiceClient(capture), Issuer: issuer}
	ctx := metadata.NewOutgoingContext(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer attacker", "x-request-id", "attacker")), metadata.Pairs("x-voice-internal-caller", "attacker", "x-other", "strip-me"))
	seen := map[string]bool{}
	for _, tc := range []struct {
		aud, rpc string
		call     func() error
	}{
		{"user", userv1.UserService_GetPrivacySettings_FullMethodName, func() error { _, err := user.AllowFriendRequestsAudience(ctx, uuid.New()); return err }},
		{"user", userv1.UserService_GetPrivacySettings_FullMethodName, func() error { _, err := user.AllowPhoneSearchAudience(ctx, uuid.New()); return err }},
		{"space", spacev1.SpaceService_AreCoMembers_FullMethodName, func() error {
			_, err := space.AreCoMembers(ctx, uuid.New(), uuid.New(), []string{uuid.NewString()})
			return err
		}},
	} {
		require.NoError(t, tc.call())
		require.Len(t, capture.metadata, 2)
		md, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), capture.metadata))
		require.NoError(t, err)
		require.NotEqual(t, "attacker", md.RequestID)
		hash, err := principal.RequestHash(capture.request)
		require.NoError(t, err)
		p, err := principal.VerifyService(ctx, md.BearerToken, principal.VerifyConfig{ExpectedIssuer: "social", ExpectedAudience: tc.aud, ExpectedRPC: tc.rpc, ExpectedRequestID: md.RequestID, ExpectedRequestHash: hash, KeyResolver: func(_ context.Context, iss, kid string) (*rsa.PublicKey, error) {
			require.Equal(t, "social", iss)
			require.Equal(t, "current", kid)
			return key, nil
		}})
		require.NoError(t, err)
		require.Equal(t, "service:social", p.Subject)
		require.LessOrEqual(t, p.ExpiresAt.Sub(p.IssuedAt), 30*time.Second)
		require.False(t, seen[p.JWTID])
		seen[p.JWTID] = true
	}
}

func TestPrivacyAdaptersFailClosedWithoutSigner(t *testing.T) {
	_, missingErr := (&GRPCUserPrivacy{}).AllowFriendRequestsAudience(context.Background(), uuid.New())
	require.Equal(t, codes.Unavailable, status.Code(missingErr))
	_, missingErr = (&GRPCUserPrivacy{}).AllowPhoneSearchAudience(context.Background(), uuid.New())
	require.Equal(t, codes.Unavailable, status.Code(missingErr))
	_, missingErr = (&GRPCSpaceCoMembership{}).AreCoMembers(context.Background(), uuid.New(), uuid.New(), nil)
	require.Equal(t, codes.Unavailable, status.Code(missingErr))
	capture := &principalCapture{}
	_, err := (&GRPCUserPrivacy{Client: userv1.NewUserServiceClient(capture)}).AllowFriendRequestsAudience(context.Background(), uuid.New())
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Nil(t, capture.request)
	_, err = (&GRPCSpaceCoMembership{Client: spacev1.NewSpaceServiceClient(capture)}).AreCoMembers(context.Background(), uuid.New(), uuid.New(), nil)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Nil(t, capture.request)
}
