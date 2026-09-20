package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/principal"
	"voice/backend/user/internal/authctx"

	userv1 "voice.app/voice/user/v1"
)

func TestFileOwnershipResolver_RejectsUnboundOrMalformedAuthority(t *testing.T) {
	profileID, operationID := uuid.New(), uuid.New()
	req := &userv1.ResolveAccountIDForProfileRequest{ProfileId: profileID.String(), ActorProfileId: profileID.String(), OperationId: operationID.String()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	valid := principal.Principal{Kind: "service", Issuer: "file", Subject: "service:file", Audience: "file", RPC: userv1.UserService_ResolveAccountIDForProfile_FullMethodName, RequestHash: hash}
	service := &FileOwnershipGRPC{}

	for name, tc := range map[string]struct {
		mutate func(*principal.Principal)
		want   codes.Code
	}{
		"issuer":   {func(p *principal.Principal) { p.Issuer = "social" }, codes.PermissionDenied},
		"audience": {func(p *principal.Principal) { p.Audience = "user" }, codes.Unauthenticated},
		"rpc":      {func(p *principal.Principal) { p.RPC = userv1.UserService_GetProfile_FullMethodName }, codes.Unauthenticated},
		"hash":     {func(p *principal.Principal) { p.RequestHash = "sha256:bad" }, codes.Unauthenticated},
		"profile":  {func(p *principal.Principal) { p.ProfileID = uuid.NewString() }, codes.Unauthenticated},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			tc.mutate(&candidate)
			_, err := service.ResolveAccountIDForProfile(principal.WithVerified(context.Background(), candidate), req)
			require.Equal(t, tc.want, status.Code(err), name)
		})
	}

	_, err = service.ResolveAccountIDForProfile(principal.WithVerified(context.Background(), valid), &userv1.ResolveAccountIDForProfileRequest{ProfileId: profileID.String(), ActorProfileId: uuid.NewString(), OperationId: operationID.String()})
	require.Equal(t, codes.Unauthenticated, status.Code(err), "changed request must fail its signed hash before lookup")

	_, err = service.ResolveAccountIDForProfile(principal.WithVerified(context.Background(), valid), &userv1.ResolveAccountIDForProfileRequest{ProfileId: profileID.String(), ActorProfileId: profileID.String(), OperationId: "not-a-uuid"})
	require.Equal(t, codes.Unauthenticated, status.Code(err), "operation id is part of the signed request")

	for name, malformed := range map[string]*userv1.ResolveAccountIDForProfileRequest{
		"actor mismatch": {ProfileId: profileID.String(), ActorProfileId: uuid.NewString(), OperationId: operationID.String()},
		"operation UUID": {ProfileId: profileID.String(), ActorProfileId: profileID.String(), OperationId: "not-a-uuid"},
	} {
		t.Run(name, func(t *testing.T) {
			hash, err := principal.RequestHash(malformed)
			require.NoError(t, err)
			candidate := valid
			candidate.RequestHash = hash
			_, err = service.ResolveAccountIDForProfile(principal.WithVerified(context.Background(), candidate), malformed)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestUserGRPCResolveAccountIDForProfile_RejectsRawFileCaller(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderInternalCaller, "file"))
	_, err := (&UserGRPC{}).ResolveAccountIDForProfile(ctx, &userv1.ResolveAccountIDForProfileRequest{ProfileId: uuid.NewString()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
