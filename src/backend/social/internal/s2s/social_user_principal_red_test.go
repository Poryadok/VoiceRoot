package s2s

import (
	"context"
	"crypto/rsa"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"voice/backend/pkg/principal"

	userv1 "voice.app/voice/user/v1"
)

type socialUserPrincipalCapture struct {
	metadata metadata.MD
	request  proto.Message
	account  uuid.UUID
	profile  uuid.UUID
}

func (c *socialUserPrincipalCapture) Invoke(ctx context.Context, _ string, req, reply any, _ ...grpc.CallOption) error {
	c.metadata, _ = metadata.FromOutgoingContext(ctx)
	c.request = req.(proto.Message)
	switch out := reply.(type) {
	case *userv1.ListProfileIDsForAccountResponse:
		out.ProfileIds = []string{c.profile.String()}
	case *userv1.GetProfileResponse:
		out.Profile = &userv1.Profile{Id: c.profile.String(), AccountId: c.account.String()}
	}
	return nil
}

func (*socialUserPrincipalCapture) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("unexpected stream")
}

// This is the Social-side regression proof for the final User lookup cutover:
// friend, contact, and account-block lookups must never use ordinary 9090
// caller metadata.
func TestSocialUserLookupAdaptersUseFreshBoundPrincipal(t *testing.T) {
	issuer, key := testSocialIssuer(t)
	accountID, profileID := uuid.New(), uuid.New()
	capture := &socialUserPrincipalCapture{account: accountID, profile: profileID}
	client := userv1.NewUserServiceClient(capture)
	poisoned := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer attacker",
		"x-request-id", "attacker-request-id",
		"x-voice-internal-caller", "attacker",
	))

	lookups := []struct {
		rpc  string
		call func() error
	}{
		{
			rpc: userv1.UserService_ListProfileIDsForAccount_FullMethodName,
			call: func() error {
				got, err := (&GRPCAccountProfiles{Client: client, Issuer: issuer}).ProfileIDsForAccount(poisoned, accountID)
				require.Equal(t, []uuid.UUID{profileID}, got)
				return err
			},
		},
		{
			rpc: userv1.UserService_GetProfile_FullMethodName,
			call: func() error {
				got, err := (&GRPCProfileAccounts{Client: client, Issuer: issuer}).AccountIDByProfileID(poisoned, profileID)
				require.Equal(t, accountID, got)
				return err
			},
		},
	}
	for _, lookup := range lookups {
		require.NoError(t, lookup.call())
		require.Len(t, capture.metadata, 2)
		md, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), capture.metadata))
		require.NoError(t, err)
		require.NotEqual(t, "attacker-request-id", md.RequestID)
		hash, err := principal.RequestHash(capture.request)
		require.NoError(t, err)
		verifySocialLookupPrincipal(t, key, md, lookup.rpc, hash)
	}
}

func TestSocialUserLookupAdaptersFailClosedWithoutSigner(t *testing.T) {
	client := userv1.NewUserServiceClient(&socialUserPrincipalCapture{account: uuid.New(), profile: uuid.New()})
	_, err := (&GRPCAccountProfiles{Client: client}).ProfileIDsForAccount(context.Background(), uuid.New())
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = (&GRPCProfileAccounts{Client: client}).AccountIDByProfileID(context.Background(), uuid.New())
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func verifySocialLookupPrincipal(t *testing.T, key *rsa.PublicKey, md principal.TransportMetadata, rpc, hash string) {
	t.Helper()
	p, err := principal.VerifyService(context.Background(), md.BearerToken, principal.VerifyConfig{
		ExpectedIssuer:      "social",
		ExpectedAudience:    "user",
		ExpectedRPC:         rpc,
		ExpectedRequestID:   md.RequestID,
		ExpectedRequestHash: hash,
		KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
			require.Equal(t, "social", issuer)
			require.Equal(t, "current", kid)
			return key, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "service:social", p.Subject)
}
