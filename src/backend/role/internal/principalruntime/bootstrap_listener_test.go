package principalruntime

import (
	"context"
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
	"voice/backend/role/internal/principalgrpc"
)

func (s *recordingRoleServer) BootstrapSpaceRoles(ctx context.Context, _ *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.BootstrapSpaceRolesResponse{}, nil
}

func TestCreatorBootstrapProtectedListener(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	method := rolev1.RoleService_BootstrapSpaceRoles_FullMethodName
	request := &rolev1.BootstrapSpaceRolesRequest{SpaceId: uuid.NewString(), OwnerProfileId: uuid.NewString()}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	validToken := issueRuntimeToken(t, f.key, "space", "current", "role", method, "valid", hash)
	for _, name := range []string{"valid", "replay", "wrong_issuer", "wrong_rpc", "wrong_hash", "raw_identity", "unknown_fields", "invalid_space", "nil_owner"} {
		t.Run(name, func(t *testing.T) {
			issuer, rpc, digest, requestID := "space", method, hash, name
			if name == "replay" {
				requestID = "valid"
			}
			if name == "wrong_issuer" {
				issuer = "voice"
			}
			if name == "wrong_rpc" {
				rpc = rolev1.RoleService_CheckPermission_FullMethodName
			}
			if name == "wrong_hash" {
				digest = runtimeHash
			}
			if name == "unknown_fields" {
				request.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
				defer request.ProtoReflect().SetUnknown(nil)
				digest, err = principal.RequestHash(request)
				require.NoError(t, err)
			}
			if name == "invalid_space" || name == "nil_owner" {
				space, owner := request.SpaceId, request.OwnerProfileId
				defer func() { request.SpaceId, request.OwnerProfileId = space, owner }()
				if name == "invalid_space" {
					request.SpaceId = "invalid-space"
				} else {
					request.OwnerProfileId = uuid.Nil.String()
				}
				digest, err = principal.RequestHash(request)
				require.NoError(t, err)
			}
			token := issueRuntimeToken(t, f.key, issuer, "current", "role", rpc, requestID, digest)
			if name == "valid" || name == "replay" {
				token = validToken
			}
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)
			if name == "raw_identity" {
				md.Set("x-voice-profile-id", request.OwnerProfileId)
			}
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md), 2*time.Second)
			defer cancel()
			_, err := client.BootstrapSpaceRoles(ctx, request)
			if name == "valid" {
				require.NoError(t, err)
				verified := <-recorder.principals
				require.Equal(t, "service:space", verified.Subject)
				require.Equal(t, hash, verified.RequestHash)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, int64(1), recorder.calls.Load(), "invalid caller must not reach bootstrap")
		})
	}
}

func TestCreatorBootstrapUnavailableOnOrdinaryListener(t *testing.T) {
	called := false
	_, err := principalgrpc.OwnershipUnaryInterceptor(nil)(context.Background(), &rolev1.BootstrapSpaceRolesRequest{}, &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_BootstrapSpaceRoles_FullMethodName}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, called)
}
