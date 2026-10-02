package principalruntime

import (
	"context"
	"crypto/tls"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func (s *recordingRoleServer) ApplySpaceLifecycleFence(ctx context.Context, _ *rolev1.ApplySpaceLifecycleFenceRequest) (*rolev1.ApplySpaceLifecycleFenceResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.ApplySpaceLifecycleFenceResponse{}, nil
}

func TestRuntimeListenerSpaceDeletionFenceRequiresBoundSpaceIdentity(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	req := &rolev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}}}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	method := rolev1.RoleService_ApplySpaceLifecycleFence_FullMethodName
	for _, tc := range []struct{ name, issuer, audience, rpc, digest string }{
		{"valid", "space", "role", method, hash}, {"gateway", "gateway", "role", method, hash}, {"foreign_audience", "space", "chat", method, hash}, {"foreign_rpc", "space", "role", rolev1.RoleService_RetireSpace_FullMethodName, hash}, {"changed_hash", "space", "role", method, runtimeHash}, {"raw_identity", "space", "role", method, hash},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.NewString()
			token := issueRuntimeToken(t, f.key, tc.issuer, "current", tc.audience, tc.rpc, id, tc.digest)
			md := metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)
			if tc.name == "raw_identity" {
				md.Set("x-voice-profile-id", uuid.NewString())
			}
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), md), 2*time.Second)
			defer cancel()
			_, err := client.ApplySpaceLifecycleFence(ctx, req)
			if tc.name == "valid" {
				require.NoError(t, err)
				verified := <-recorder.principals
				require.Equal(t, "service:space", verified.Subject)
				require.Equal(t, hash, verified.RequestHash)
				_, err = client.ApplySpaceLifecycleFence(ctx, req)
				require.Equal(t, codes.Unauthenticated, status.Code(err), "transport replay denied")
			} else {
				require.Equal(t, codes.Unauthenticated, status.Code(err))
			}
		})
	}
	require.Equal(t, int64(1), recorder.calls.Load())
	nocert := runtimeListenerClient(t, address, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}))
	id := uuid.NewString()
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, id, hash)
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), 2*time.Second)
	defer cancel()
	_, err = nocert.ApplySpaceLifecycleFence(ctx, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, int64(1), recorder.calls.Load())
}
