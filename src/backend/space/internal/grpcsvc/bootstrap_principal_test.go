package grpcsvc

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
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

type bootstrapPrincipalProbe struct {
	publicCreatorRoleProbe
	t   *testing.T
	ids []string
}

func (p *bootstrapPrincipalProbe) BootstrapSpaceRoles(ctx context.Context, req *rolev1.BootstrapSpaceRolesRequest, options ...grpc.CallOption) (*rolev1.BootstrapSpaceRolesResponse, error) {
	p.t.Helper()
	md, _ := metadata.FromOutgoingContext(ctx)
	require.Len(p.t, md, 2, "inherited Gateway/GIS credentials and actor metadata must be discarded")
	deadline, ok := ctx.Deadline()
	require.True(p.t, ok, "bootstrap must bound its Role call")
	require.LessOrEqual(p.t, time.Until(deadline), 5*time.Second)
	p.ids = append(p.ids, md.Get("x-request-id")...)
	return p.publicCreatorRoleProbe.BootstrapSpaceRoles(ctx, req, options...)
}

// The component peer verifies the actual signed outbound request. Consumer62's
// source-built matrix supplies the actual Role persistence/composition proof.
func TestBootstrapSpaceRolesUsesProtectedSignedClient(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	space, owner := uuid.New(), uuid.New()
	protected := &bootstrapPrincipalProbe{t: t, publicCreatorRoleProbe: publicCreatorRoleProbe{key: &key.PublicKey, space: space.String(), profile: owner.String(), response: &rolev1.BootstrapSpaceRolesResponse{}}}
	ordinary := &publicCreatorRoleProbe{err: status.Error(codes.Unavailable, "protected bootstrap listener required")}
	svc := &SpaceGRPC{Roles: ordinary, OwnershipRoles: protected, PrincipalIssuer: issuer}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer inbound", "x-request-id", "inbound", "x-voice-profile-id", uuid.NewString()))
	for i := 0; i < 2; i++ {
		require.NoError(t, svc.bootstrapSpaceRoles(ctx, space, owner))
	}
	require.Zero(t, ordinary.calls, "ordinary Role listener must never be a fallback")
	require.Equal(t, 2, protected.calls)
	require.Len(t, protected.ids, 2)
	require.NotEqual(t, protected.ids[0], protected.ids[1], "durable retry requires fresh transport identity")
	for _, id := range protected.ids {
		_, err := uuid.Parse(id)
		require.NoError(t, err)
	}
	for _, failure := range []string{"missing protected client", "missing signer", "nil reply", "unknown reply", "Role unavailable"} {
		t.Run(failure, func(t *testing.T) {
			candidate := &SpaceGRPC{Roles: ordinary, OwnershipRoles: protected, PrincipalIssuer: issuer}
			protected.err = nil
			protected.response = &rolev1.BootstrapSpaceRolesResponse{}
			switch failure {
			case "missing protected client":
				candidate.OwnershipRoles = nil
			case "missing signer":
				candidate.PrincipalIssuer = nil
			case "nil reply":
				protected.response = nil
			case "unknown reply":
				protected.response.ProtoReflect().SetUnknown([]byte{0x08, 0x01})
			case "Role unavailable":
				protected.err = status.Error(codes.Unavailable, "unavailable")
			}
			require.Equal(t, codes.Unavailable, status.Code(candidate.bootstrapSpaceRoles(ctx, space, owner)))
			require.Zero(t, ordinary.calls)
		})
	}
}

// Legacy ledger tests can supply an explicit signed bootstrap peer without
// treating an absent production Role runtime as a successful bootstrap.
type bootstrapLedgerFixture struct {
	rolev1.RoleServiceClient
	key *rsa.PublicKey
}

func (p bootstrapLedgerFixture) GetMemberRoles(ctx context.Context, req *rolev1.GetMemberRolesRequest, options ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	if p.RoleServiceClient == nil {
		return nil, status.Error(codes.Unavailable, "legacy ledger fixture has no Role reader")
	}
	return p.RoleServiceClient.GetMemberRoles(ctx, req, options...)
}

func (p bootstrapLedgerFixture) BootstrapSpaceRoles(ctx context.Context, req *rolev1.BootstrapSpaceRolesRequest, options ...grpc.CallOption) (*rolev1.BootstrapSpaceRolesResponse, error) {
	probe := &publicCreatorRoleProbe{key: p.key, space: req.SpaceId, profile: req.OwnerProfileId, response: &rolev1.BootstrapSpaceRolesResponse{}}
	if _, err := probe.BootstrapSpaceRoles(ctx, req, options...); err != nil {
		return nil, err
	}
	if p.RoleServiceClient != nil {
		return p.RoleServiceClient.BootstrapSpaceRoles(ctx, req, options...)
	}
	return &rolev1.BootstrapSpaceRolesResponse{}, nil
}

func newBootstrapLedgerFixture(t *testing.T, delegate rolev1.RoleServiceClient) (rolev1.RoleServiceClient, *principal.Issuer) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	return bootstrapLedgerFixture{RoleServiceClient: delegate, key: &key.PublicKey}, issuer
}

func TestBootstrapSpaceRolesMissingRuntimeFailsClosed(t *testing.T) {
	var absent *SpaceGRPC
	for _, svc := range []*SpaceGRPC{absent, {}} {
		require.Equal(t, codes.Unavailable, status.Code(svc.bootstrapSpaceRoles(context.Background(), uuid.New(), uuid.New())))
	}
}
