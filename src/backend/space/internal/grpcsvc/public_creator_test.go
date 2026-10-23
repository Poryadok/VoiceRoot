package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

type publicCreatorWriterProbe struct {
	row   *store.SpaceRow
	err   error
	calls int
	actor principal.Principal
}

func (p *publicCreatorWriterProbe) CreateSpaceWithCreator(_ context.Context, creator *lifecycleprincipal.VerifiedCreator, req *spacev1.CreateSpaceRequest) (*store.SpaceRow, error) {
	p.calls++
	actor, err := creator.ActorFor(req)
	if err != nil {
		return nil, err
	}
	p.actor = actor
	return p.row, p.err
}

type publicCreatorRoleProbe struct {
	rolev1.RoleServiceClient
	key            *rsa.PublicKey
	space, profile string
	calls          int
	err            error
	response       *rolev1.BootstrapSpaceRolesResponse
}

func (p *publicCreatorRoleProbe) BootstrapSpaceRoles(ctx context.Context, req *rolev1.BootstrapSpaceRolesRequest, _ ...grpc.CallOption) (*rolev1.BootstrapSpaceRolesResponse, error) {
	p.calls++
	md, _ := metadata.FromOutgoingContext(ctx)
	auth, ids := md.Get("authorization"), md.Get("x-request-id")
	if len(auth) != 1 || len(ids) != 1 || req.SpaceId != p.space || req.OwnerProfileId != p.profile {
		return nil, errors.New("bootstrap binding mismatch")
	}
	hash, _ := principal.RequestHash(req)
	_, err := principal.VerifyService(ctx, strings.TrimPrefix(auth[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_BootstrapSpaceRoles_FullMethodName, ExpectedRequestID: ids[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return p.key, nil }})
	if err != nil {
		return nil, err
	}
	return p.response, p.err
}

// A real signed ingress capability is used; the writer/Role peers are component
// probes only, never production publication or provider readiness evidence.
func TestPublicCreatorRequiresTypedCapabilityAndProtectedRoleOutcome(t *testing.T) {
	gatewayKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	spaceKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: gatewayKey})
	require.NoError(t, err)
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: spaceKey})
	require.NoError(t, err)
	account, profile, id := uuid.New(), uuid.New(), uuid.New()
	request := &spacev1.CreateSpaceRequest{Name: "typed creator", Visibility: "private"}
	writer := &publicCreatorWriterProbe{row: &store.SpaceRow{ID: id, OwnerProfileID: profile, Name: request.Name, Visibility: "private"}}
	roles := &publicCreatorRoleProbe{key: &spaceKey.PublicKey, space: id.String(), profile: profile.String(), response: &rolev1.BootstrapSpaceRolesResponse{}}
	service := &PublicLifecycleSpace{SpaceGRPC: &SpaceGRPC{OwnershipRoles: roles, PrincipalIssuer: signer}}
	_, err = service.createSpaceWithVerifiedCreator(context.Background(), request, writer)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, writer.calls)
	_, err = service.CreateSpace(context.Background(), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, roles.calls)
	floors := 0
	invoke := func() (any, error) {
		hash, err := principal.RequestHash(request)
		require.NoError(t, err)
		requestID := uuid.NewString()
		token, err := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, err)
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
		verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &gatewayKey.PublicKey, nil }, CreatorEpoch: func(context.Context, string, int64) error { floors++; return nil }, CreatorReplay: func(context.Context, string, string, time.Time) error { return nil }}
		return verifier.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateSpace_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
			return service.createSpaceWithVerifiedCreator(ctx, request, writer)
		})
	}
	result, err := invoke()
	require.NoError(t, err)
	require.Equal(t, profile.String(), result.(*spacev1.CreateSpaceResponse).Space.OwnerProfileId)
	require.Equal(t, account.String(), writer.actor.AccountID)
	require.Equal(t, profile.String(), writer.actor.ProfileID)
	require.Equal(t, 1, writer.calls)
	require.Equal(t, 1, roles.calls)
	require.Equal(t, 2, floors)
	for _, failure := range []string{"missing role", "missing signer", "nil role response", "unknown role response", "role unavailable", "writer unavailable", "owner mismatch"} {
		t.Run(failure, func(t *testing.T) {
			service.OwnershipRoles = roles
			service.PrincipalIssuer = signer
			roles.response = &rolev1.BootstrapSpaceRolesResponse{}
			roles.err = nil
			writer.err = nil
			writer.row.OwnerProfileID = profile
			switch failure {
			case "missing role":
				service.OwnershipRoles = nil
			case "missing signer":
				service.PrincipalIssuer = nil
			case "nil role response":
				roles.response = nil
			case "unknown role response":
				roles.response.ProtoReflect().SetUnknown([]byte{0x08, 0x01})
			case "role unavailable":
				roles.err = status.Error(codes.Unavailable, "unavailable")
			case "writer unavailable":
				writer.err = store.ErrMemberAuthorityUnavailable
			case "owner mismatch":
				writer.row.OwnerProfileID = uuid.New()
			}
			before := writer.calls
			_, err := invoke()
			require.Equal(t, codes.Unavailable, status.Code(err))
			if failure == "missing role" || failure == "missing signer" {
				require.Equal(t, before, writer.calls, "missing authority must not write")
			}
		})
	}
}
