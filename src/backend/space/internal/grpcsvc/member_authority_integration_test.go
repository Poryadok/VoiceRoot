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
	"voice/backend/pkg/roleoverrideprincipal"
	"voice/backend/space/internal/store"
)

type memberAuthorityVerifier struct {
	key  *rsa.PublicKey
	seen map[string]bool
}

func (v *memberAuthorityVerifier) Verify(ctx context.Context, token, method, id, hash string) (principal.Principal, error) {
	return principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "role", ExpectedAudience: "space", ExpectedRPC: method, ExpectedRequestID: id, ExpectedRequestHash: hash,
		KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return v.key, nil },
		ReplayGuard: func(_ context.Context, _, jti string, _ time.Time) error {
			if v.seen[jti] {
				return errors.New("replay")
			}
			v.seen[jti] = true
			return nil
		}})
}

func TestMemberAdmissionAuthorityProtectedNativePreparing(t *testing.T) {
	f := newNativeModerationFixture(t)
	ctx := context.Background()
	invite, err := f.st.GetInviteByCode(ctx, f.invite(t))
	require.NoError(t, err)
	input := store.MemberAdmissionInput{OperationID: uuid.New(), SpaceID: f.space, ProfileID: uuid.New(), AccountID: uuid.New(), InviteID: &invite.ID}
	intent, err := f.st.PrepareMemberAdmission(ctx, input)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "role", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	operation := input.OperationID.String()
	req := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: f.space.String(), ProfileId: input.ProfileID.String(), ExpectedGeneration: &intent.Generation, ExpectedAdmissionOperationId: &operation}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName, RequestID: id, RequestHash: hash})
	require.NoError(t, err)
	signed := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	service := &SpaceGRPC{Store: f.st}
	info := &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName}
	handler := func(call context.Context, request any) (any, error) {
		return service.GetMemberAdmissionAuthority(call, request.(*spacev1.GetMemberAdmissionAuthorityRequest))
	}
	_, err = service.GetMemberAdmissionAuthority(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = roleoverrideprincipal.OrdinaryUnaryInterceptor("space")(signed, req, info, handler)
	require.Equal(t, codes.Unavailable, status.Code(err))
	interceptor := roleoverrideprincipal.StrictUnaryInterceptor("space", &memberAuthorityVerifier{key: &key.PublicKey, seen: map[string]bool{}})
	result, err := interceptor(signed, req, info, handler)
	require.NoError(t, err)
	response := result.(*spacev1.GetMemberAdmissionAuthorityResponse)
	require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_PREPARING, response.Ordinary.State)
	require.Equal(t, intent.Generation, response.Ordinary.PreparingBinding.GetOrdinary().MembershipGeneration)
	require.Equal(t, operation, response.Ordinary.PreparingBinding.GetOrdinary().PublicationIdentity)
	require.Equal(t, intent.BindingSHA256, response.Ordinary.PreparingAdmissionBindingSha256)
	require.Positive(t, response.ValidityRemaining.AsDuration())
	require.LessOrEqual(t, response.ValidityRemaining.AsDuration(), 5*time.Second)
	_, err = interceptor(signed, req, info, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	member, err := f.st.IsSpaceMember(ctx, f.space, input.ProfileID)
	require.NoError(t, err)
	require.False(t, member, "an authority read and inert preparation do not publish membership")
}
