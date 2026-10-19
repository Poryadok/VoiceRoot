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

type protectedPermissionClient struct {
	rolev1.RoleServiceClient
	key     *rsa.PublicKey
	seen    map[string]bool
	allowed bool
	empty   bool
	calls   int
}

func (c *protectedPermissionClient) CheckPermission(ctx context.Context, request *rolev1.CheckPermissionRequest, _ ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok || len(md) != 2 {
		return nil, status.Error(codes.Unauthenticated, "raw metadata escaped")
	}
	transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(ctx, md))
	if err != nil {
		return nil, err
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, err
	}
	if c.key == nil {
		return nil, status.Error(codes.Internal, "ordinary Role client must not be called")
	}
	verified, err := principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_CheckPermission_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return c.key, nil }})
	if err != nil {
		return nil, err
	}
	if verified.Subject != "service:bot" || request.GetPermissionName() != "SPACE_MANAGE_BOTS" || request.GetChat() != nil || request.GetVoiceRoomId() != "" {
		return nil, status.Error(codes.PermissionDenied, "invalid decision scope")
	}
	if c.seen == nil {
		c.seen = map[string]bool{}
	}
	if c.seen[verified.JWTID] {
		return nil, status.Error(codes.Unauthenticated, "replay")
	}
	c.seen[verified.JWTID] = true
	c.calls++
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 3*time.Second {
		return nil, status.Error(codes.Internal, "decision must be bounded")
	}
	if c.empty {
		return nil, nil
	}
	return &rolev1.CheckPermissionResponse{Allowed: c.allowed}, nil
}

func TestInstallationPermissionUsesOnlySignedProtectedRoleDecision(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	ordinary := &protectedPermissionClient{allowed: true}
	protected := &protectedPermissionClient{key: &key.PublicKey, allowed: true}
	svc := &BotGRPC{Role: ordinary, PrincipalIssuer: issuer}
	space, profile := uuid.New(), uuid.New()
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("x-voice-user-id", uuid.NewString(), "x-voice-profile-id", profile.String()))
	require.Equal(t, codes.FailedPrecondition, status.Code(svc.checkInstallationPermission(ctx, space, profile)))
	require.Zero(t, ordinary.calls)
	svc.InstallationRole = protected
	require.NoError(t, svc.checkInstallationPermission(ctx, space, profile))
	require.NoError(t, svc.checkInstallationPermission(ctx, space, profile))
	require.Equal(t, 2, protected.calls)
	protected.allowed = false
	require.Equal(t, codes.PermissionDenied, status.Code(svc.checkInstallationPermission(ctx, space, profile)))
	protected.empty = true
	require.Equal(t, codes.Unavailable, status.Code(svc.checkInstallationPermission(ctx, space, profile)))
	svc.PrincipalIssuer = nil
	require.Equal(t, codes.FailedPrecondition, status.Code(svc.checkInstallationPermission(ctx, space, profile)))
}
