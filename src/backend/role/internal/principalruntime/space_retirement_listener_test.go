package principalruntime

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func (s *recordingRoleServer) RetireSpace(ctx context.Context, _ *rolev1.RetireSpaceRequest) (*rolev1.RetireSpaceResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.RetireSpaceResponse{}, nil
}

func r23RuntimeRetirementRequest() *rolev1.RetireSpaceRequest {
	return &rolev1.RetireSpaceRequest{
		ProtocolVersion:     1,
		SpaceId:             uuid.NewString(),
		DeletionOperationId: uuid.NewString(),
		Generation:          1,
		PurgeDecidedAt:      timestamppb.New(time.Now().UTC()),
		Manifest:            &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32), ItemCount: 1},
	}
}

func TestRuntimeListener_AllowsOnlyBoundSpacePrincipalToRetireSpace(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}))
	req := r23RuntimeRetirementRequest()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	method := rolev1.RoleService_RetireSpace_FullMethodName
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "r23-retire", hash)
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "r23-retire")), 2*time.Second)
	defer cancel()
	_, err = client.RetireSpace(ctx, req)
	require.NoError(t, err)
	require.Equal(t, int64(1), recorder.calls.Load())
	verified := <-recorder.principals
	require.Equal(t, "service:space", verified.Subject)
	require.Equal(t, method, verified.RPC)
	require.Equal(t, hash, verified.RequestHash)
	require.Empty(t, verified.AccountID)
	require.Empty(t, verified.ProfileID)
	require.Zero(t, verified.SessionEpoch)
}

func TestRuntimeListener_RetireSpaceRejectsWrongBindingBeforeHandler(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}))
	request := r23RuntimeRetirementRequest()
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := rolev1.RoleService_RetireSpace_FullMethodName
	for _, tc := range []struct {
		name, issuer, audience, rpc, signedHash string
	}{
		{"gateway", "gateway", "role", method, hash},
		{"wrong_audience", "space", "space", method, hash},
		{"wrong_rpc", "space", "role", rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, hash},
		{"wrong_hash", "space", "role", method, "sha256:" + strings.Repeat("0", 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := issueRuntimeToken(t, f.key, tc.issuer, "current", tc.audience, tc.rpc, "r23-"+tc.name, tc.signedHash)
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "r23-"+tc.name)), 2*time.Second)
			defer cancel()
			_, err := client.RetireSpace(ctx, request)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
	changed := proto.Clone(request).(*rolev1.RetireSpaceRequest)
	changed.Generation++
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "r23-changed", hash)
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "r23-changed")), 2*time.Second)
	defer cancel()
	_, err = client.RetireSpace(ctx, changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, recorder.calls.Load(), "invalid principal or request binding reached retirement handler")
}
