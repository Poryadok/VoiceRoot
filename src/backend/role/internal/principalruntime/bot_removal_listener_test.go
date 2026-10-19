package principalruntime

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalgrpc"
)

func (s *recordingRoleServer) ApplyBotInstallationRemoval(ctx context.Context, _ *rolev1.ApplyBotInstallationRemovalRequest) (*rolev1.ApplyBotInstallationRemovalResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.ApplyBotInstallationRemovalResponse{}, nil
}

func TestBotRemovalProtectedListener(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	f.config.BotLifecycleEnabled = true
	f.config.JWKSURLs["bot"] = f.config.JWKSURLs["space"]
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	method := rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName
	request := &rolev1.ApplyBotInstallationRemovalRequest{OperationId: uuid.NewString(), Evidence: &rolev1.ApplyBotInstallationRemovalRequest_Native{Native: &rolev1.BotRemovalNativeEvidence{InstallationId: uuid.NewString()}}}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	token := issueRuntimeToken(t, f.key, "bot", "current", "role", method, request.OperationId, hash)
	for _, name := range []string{"valid", "replay", "wrong_issuer", "wrong_rpc", "wrong_hash", "wrong_operation", "nested_unknown", "raw_identity"} {
		t.Run(name, func(t *testing.T) {
			r := proto.Clone(request).(*rolev1.ApplyBotInstallationRemovalRequest)
			issuer, rpc, requestID, digest := "bot", method, request.OperationId, hash
			if name == "wrong_issuer" {
				issuer = "space"
			}
			if name == "wrong_rpc" {
				rpc = rolev1.RoleService_CheckPermission_FullMethodName
			}
			if name == "wrong_hash" {
				digest = runtimeHash
			}
			if name == "wrong_operation" {
				requestID = uuid.NewString()
			}
			if name == "nested_unknown" {
				r.GetNative().ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
				digest, err = principal.RequestHash(r)
				require.NoError(t, err)
			}
			signed := issueRuntimeToken(t, f.key, issuer, "current", "role", rpc, requestID, digest)
			if name == "valid" || name == "replay" {
				signed = token
			}
			md := metadata.Pairs("authorization", "Bearer "+signed, "x-request-id", requestID)
			if name == "raw_identity" {
				md.Set("x-voice-profile-id", uuid.NewString())
			}
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md), 2*time.Second)
			defer cancel()
			_, err := client.ApplyBotInstallationRemoval(ctx, r)
			if name == "valid" {
				require.NoError(t, err)
				p := <-recorder.principals
				require.Equal(t, "service:bot", p.Subject)
			} else {
				require.Error(t, err)
			}
			require.EqualValues(t, 1, recorder.calls.Load())
		})
	}
}

func TestBotRemovalOrdinaryListenerRejectsBeforeHandler(t *testing.T) {
	called := false
	_, err := principalgrpc.OwnershipUnaryInterceptor(nil)(context.Background(), &rolev1.ApplyBotInstallationRemovalRequest{}, &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	require.Error(t, err)
	require.False(t, called)
}
