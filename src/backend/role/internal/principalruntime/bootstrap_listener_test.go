package principalruntime

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func (s *recordingRoleServer) BootstrapSpaceRoles(ctx context.Context, _ *rolev1.BootstrapSpaceRolesRequest) (*rolev1.BootstrapSpaceRolesResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.BootstrapSpaceRolesResponse{}, nil
}

func TestRuntimeBootstrapDependencyOutageFailsClosed(t *testing.T) {
	for _, name := range []string{"jwks", "redis"} {
		t.Run(name, func(t *testing.T) {
			f := newRuntimeFixture(t, 2)
			if name == "jwks" {
				f.jwks.set(http.StatusServiceUnavailable, nil)
			}
			runtime, err := New(context.Background(), f.config)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, runtime.Close()) })
			address, recorder, roots := startRuntimeListenerWithRuntime(t, f, runtime)
			client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
			if name == "redis" {
				f.redis.Close()
			}
			req := &rolev1.BootstrapSpaceRolesRequest{SpaceId: uuid.NewString(), OwnerProfileId: uuid.NewString()}
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			method := rolev1.RoleService_BootstrapSpaceRoles_FullMethodName
			id := uuid.NewString()
			token := issueRuntimeToken(t, f.key, "space", "current", "role", method, id, hash)
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 3*time.Second)
			defer cancel()
			_, err = client.BootstrapSpaceRoles(ctx, req)
			require.NoError(t, ctx.Err())
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Zero(t, recorder.calls.Load())
		})
	}
}

func TestRuntimeBootstrapRejectsSubstitutionsBeforeHandler(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	method := rolev1.RoleService_BootstrapSpaceRoles_FullMethodName
	base := &rolev1.BootstrapSpaceRolesRequest{SpaceId: uuid.NewString(), OwnerProfileId: uuid.NewString()}
	for _, name := range []string{"issuer", "audience", "method", "hash", "unknown", "space", "owner", "operation", "actor", "duplicate", "mixed"} {
		t.Run(name, func(t *testing.T) {
			req := proto.Clone(base).(*rolev1.BootstrapSpaceRolesRequest)
			issuer, audience, signedMethod, id := "space", "role", method, uuid.NewString()
			want := codes.Unauthenticated
			switch name {
			case "issuer":
				issuer = "voice"
			case "audience":
				audience = "space"
			case "method":
				signedMethod = rolev1.RoleService_CheckPermission_FullMethodName
			case "unknown":
				req.ProtoReflect().SetUnknown([]byte{0x78, 1})
				want = codes.InvalidArgument
			case "space":
				req.SpaceId = " " + req.SpaceId
				want = codes.InvalidArgument
			case "owner":
				req.OwnerProfileId = uuid.Nil.String()
				want = codes.InvalidArgument
			case "operation":
				id = "unbound"
				want = codes.InvalidArgument
			}
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			if name == "hash" {
				hash = runtimeHash
			}
			token := issueRuntimeToken(t, f.key, issuer, "current", audience, signedMethod, id, hash)
			if name == "mixed" {
				token = withSignedRuntimeClaim(t, token, f.key, "account_type")
			}
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)
			if name == "actor" {
				md.Set("x-voice-profile-id", req.OwnerProfileId)
			}
			if name == "duplicate" {
				md.Append("authorization", "Bearer "+token)
			}
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md), 2*time.Second)
			defer cancel()
			_, err = client.BootstrapSpaceRoles(ctx, req)
			require.NoError(t, ctx.Err())
			require.Equal(t, want, status.Code(err))
		})
	}
	require.Zero(t, recorder.calls.Load())
}

// This proves only transport admission to a recorder, never Owner qualification.
func TestRuntimeBootstrapExactSpaceTransport(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	req := &rolev1.BootstrapSpaceRolesRequest{SpaceId: uuid.NewString(), OwnerProfileId: uuid.NewString()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	method := rolev1.RoleService_BootstrapSpaceRoles_FullMethodName
	id := uuid.NewString()
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, id, hash)
	md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)
	call := func() error {
		ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md.Copy()), 2*time.Second)
		defer cancel()
		_, err := client.BootstrapSpaceRoles(ctx, req)
		require.NoError(t, ctx.Err(), "auth status must not be a dead RPC context")
		return err
	}
	require.NoError(t, call())
	p := <-recorder.principals
	require.Equal(t, "service:space", p.Subject)
	require.Equal(t, method, p.RPC)
	require.Equal(t, id, p.RequestID)
	require.Equal(t, hash, p.RequestHash)
	require.Equal(t, codes.Unauthenticated, status.Code(call()), "spent JTI must not reach handler")
	require.Equal(t, int64(1), recorder.calls.Load())
}
