package principalruntime

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalgrpc"
)

func (s *recordingRoleServer) GetOwnershipTransferCapabilities(ctx context.Context, _ *rolev1.GetOwnershipTransferCapabilitiesRequest) (*rolev1.GetOwnershipTransferCapabilitiesResponse, error) {
	if !principalgrpc.OwnershipV2CapabilitiesActive(ctx) {
		return nil, status.Error(codes.Unavailable, "ownership v2 activation hold")
	}
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.GetOwnershipTransferCapabilitiesResponse{}, nil
}

func (s *recordingRoleServer) PrepareOwnershipTransfer(ctx context.Context, _ *rolev1.PrepareOwnershipTransferRequest) (*rolev1.PrepareOwnershipTransferResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.PrepareOwnershipTransferResponse{}, nil
}

func (s *recordingRoleServer) FinalizeOwnershipTransfer(ctx context.Context, _ *rolev1.FinalizeOwnershipTransferRequest) (*rolev1.FinalizeOwnershipTransferResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.FinalizeOwnershipTransferResponse{}, nil
}

func (s *recordingRoleServer) AbortOwnershipTransfer(ctx context.Context, _ *rolev1.AbortOwnershipTransferRequest) (*rolev1.AbortOwnershipTransferResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.AbortOwnershipTransferResponse{}, nil
}

func (s *recordingRoleServer) ResolveVoiceRoomGrants(ctx context.Context, _ *rolev1.ResolveVoiceRoomGrantsRequest) (*rolev1.ResolveVoiceRoomGrantsResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.ResolveVoiceRoomGrantsResponse{}, nil
}

func TestRuntimeListener_AllowsOnlyAuthenticatedV2OwnershipSurface(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}))
	intent := &rolev1.OwnershipTransferIntent{
		ProtocolVersion: 2,
		SpaceId:         uuid.NewString(), OldOwnerProfileId: uuid.NewString(),
		NewOwnerProfileId: uuid.NewString(), OperationId: uuid.NewString(),
	}
	tests := []struct {
		name   string
		method string
		req    proto.Message
		call   func(context.Context) error
	}{
		{"capabilities", rolev1.RoleService_GetOwnershipTransferCapabilities_FullMethodName, &rolev1.GetOwnershipTransferCapabilitiesRequest{}, func(ctx context.Context) error {
			_, err := client.GetOwnershipTransferCapabilities(ctx, &rolev1.GetOwnershipTransferCapabilitiesRequest{})
			return err
		}},
		{"prepare", rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName, &rolev1.PrepareOwnershipTransferRequest{Intent: intent}, func(ctx context.Context) error {
			_, err := client.PrepareOwnershipTransfer(ctx, &rolev1.PrepareOwnershipTransferRequest{Intent: intent})
			return err
		}},
		{"finalize", rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName, &rolev1.FinalizeOwnershipTransferRequest{Intent: intent}, func(ctx context.Context) error {
			_, err := client.FinalizeOwnershipTransfer(ctx, &rolev1.FinalizeOwnershipTransferRequest{Intent: intent})
			return err
		}},
		{"abort", rolev1.RoleService_AbortOwnershipTransfer_FullMethodName, &rolev1.AbortOwnershipTransferRequest{Intent: intent}, func(ctx context.Context) error {
			_, err := client.AbortOwnershipTransfer(ctx, &rolev1.AbortOwnershipTransferRequest{Intent: intent})
			return err
		}},
	}
	for index, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := principal.RequestHash(tc.req)
			require.NoError(t, err)
			requestID := "v2-listener-" + tc.name
			key, kid := f.key, "current"
			if index%2 == 1 {
				key, kid = f.next, "next"
			}
			token := issueRuntimeToken(t, key, "space", kid, "role", tc.method, requestID, hash)
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), 2*time.Second)
			defer cancel()
			require.NoError(t, tc.call(ctx))
			verified := <-recorder.principals
			require.Equal(t, "service:space", verified.Subject)
			require.Equal(t, tc.method, verified.RPC)
			require.Equal(t, hash, verified.RequestHash)
			require.Empty(t, verified.AccountID)
			require.Empty(t, verified.ProfileID)
			require.Zero(t, verified.SessionEpoch)
		})
	}
	require.Equal(t, int64(len(tests)), recorder.calls.Load())
}

func TestRuntimeListener_DeniesLegacyAndUnrelatedMethodsBeforeHandler(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.ApplyOwnershipTransfer(ctx, &rolev1.ApplyOwnershipTransferRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = client.CompensateOwnershipTransfer(ctx, &rolev1.CompensateOwnershipTransferRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = client.ResolveVoiceRoomGrants(ctx, &rolev1.ResolveVoiceRoomGrantsRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = client.ListRoles(ctx, &rolev1.ListRolesRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, recorder.calls.Load())
	require.Empty(t, recorder.principals)
}

func TestRuntimeListener_V2RejectsChangedUnknownFieldsAndReplayBeforeHandler(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}))
	request := &rolev1.PrepareOwnershipTransferRequest{Intent: &rolev1.OwnershipTransferIntent{
		ProtocolVersion: 2, SpaceId: uuid.NewString(), OldOwnerProfileId: uuid.NewString(),
		NewOwnerProfileId: uuid.NewString(), OperationId: uuid.NewString(),
	}}
	request.ProtoReflect().SetUnknown([]byte{0xa8, 0x06, 0x01})
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	method := rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName
	token := issueRuntimeToken(t, f.key, "space", "current", "role", method, "v2-unknown", hash)
	metadataContext := func() context.Context {
		return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", "v2-unknown"))
	}

	changed := proto.Clone(request).(*rolev1.PrepareOwnershipTransferRequest)
	changed.ProtoReflect().SetUnknown([]byte{0xa8, 0x06, 0x02})
	_, err = client.PrepareOwnershipTransfer(metadataContext(), changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, recorder.calls.Load())

	_, err = client.PrepareOwnershipTransfer(metadataContext(), request)
	require.NoError(t, err)
	<-recorder.principals
	_, err = client.PrepareOwnershipTransfer(metadataContext(), request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, int64(1), recorder.calls.Load())
}
