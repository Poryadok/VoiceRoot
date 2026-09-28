package principalruntime

import (
	"context"
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

func (s *recordingRoleServer) ApplyGameSessionGrants(ctx context.Context, _ *rolev1.ApplyGameSessionGrantsRequest) (*rolev1.ApplyGameSessionGrantsResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.ApplyGameSessionGrantsResponse{Receipt: &rolev1.GameSessionGrantReceipt{}}, nil
}

func (s *recordingRoleServer) RevokeGameSessionGrants(ctx context.Context, _ *rolev1.RevokeGameSessionGrantsRequest) (*rolev1.RevokeGameSessionGrantsResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.RevokeGameSessionGrantsResponse{Receipt: &rolev1.GameSessionGrantReceipt{}}, nil
}

func (s *recordingRoleServer) CheckGameSessionGrant(ctx context.Context, _ *rolev1.CheckGameSessionGrantRequest) (*rolev1.CheckGameSessionGrantResponse, error) {
	s.calls.Add(1)
	p, _ := principal.FromContext(ctx)
	s.principals <- p
	return &rolev1.CheckGameSessionGrantResponse{Allowed: true}, nil
}

func TestRuntimeListener_AuthenticatesOnlyTheDocumentedGameGrantIssuers(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
	apply := &rolev1.ApplyGameSessionGrantsRequest{
		ApplicationId: uuid.NewString(), EnvironmentId: uuid.NewString(), SessionId: uuid.NewString(),
		VoiceRoomId: uuid.NewString(), OperationId: uuid.NewString(), RosterRevision: 1,
	}
	revoke := &rolev1.RevokeGameSessionGrantsRequest{
		ApplicationId: apply.ApplicationId, EnvironmentId: apply.EnvironmentId, SessionId: apply.SessionId,
		OperationId: uuid.NewString(),
	}
	check := &rolev1.CheckGameSessionGrantRequest{
		ApplicationId: apply.ApplicationId, EnvironmentId: apply.EnvironmentId, SessionId: apply.SessionId,
		VoiceRoomId: apply.VoiceRoomId, ProfileId: uuid.NewString(),
	}
	tests := []struct {
		name, issuer, requestID string
		request                 proto.Message
		call                    func(context.Context) error
	}{
		{"apply", "gameintegration", apply.OperationId, apply, func(ctx context.Context) error {
			_, err := client.ApplyGameSessionGrants(ctx, apply)
			return err
		}},
		{"revoke", "gameintegration", revoke.OperationId, revoke, func(ctx context.Context) error {
			_, err := client.RevokeGameSessionGrants(ctx, revoke)
			return err
		}},
		{"check", "voice", uuid.NewString(), check, func(ctx context.Context) error {
			response, err := client.CheckGameSessionGrant(ctx, check)
			if err == nil && !response.Allowed {
				t.Errorf("authenticated fixture check should be admitted")
			}
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := principal.RequestHash(tc.request)
			require.NoError(t, err)
			token := issueRuntimeToken(t, f.key, tc.issuer, "current", "role", grpcMethodForGrant(tc.name), tc.requestID, hash)
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
				"authorization", "Bearer "+token, "x-request-id", tc.requestID)), 2*time.Second)
			defer cancel()
			require.NoError(t, tc.call(ctx))
			verified := <-recorder.principals
			require.Equal(t, tc.issuer, verified.Issuer)
			require.Equal(t, "service:"+tc.issuer, verified.Subject)
			require.Equal(t, "role", verified.Audience)
			require.Equal(t, hash, verified.RequestHash)
		})
	}
	require.Equal(t, int64(len(tests)), recorder.calls.Load())

	// The same valid mTLS client certificate cannot select another workload issuer.
	for _, tc := range []struct {
		name, issuer, method, requestID string
		request                         proto.Message
		call                            func(context.Context) error
	}{
		{"voice_cannot_apply", "voice", rolev1.RoleService_ApplyGameSessionGrants_FullMethodName, apply.OperationId, apply, func(ctx context.Context) error {
			_, err := client.ApplyGameSessionGrants(ctx, apply)
			return err
		}},
		{"voice_cannot_revoke", "voice", rolev1.RoleService_RevokeGameSessionGrants_FullMethodName, revoke.OperationId, revoke, func(ctx context.Context) error {
			_, err := client.RevokeGameSessionGrants(ctx, revoke)
			return err
		}},
		{"gis_cannot_check", "gameintegration", rolev1.RoleService_CheckGameSessionGrant_FullMethodName, uuid.NewString(), check, func(ctx context.Context) error {
			_, err := client.CheckGameSessionGrant(ctx, check)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := principal.RequestHash(tc.request)
			require.NoError(t, err)
			token := issueRuntimeToken(t, f.key, tc.issuer, "next", "role", tc.method, tc.requestID, hash)
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
				"authorization", "Bearer "+token, "x-request-id", tc.requestID)), 2*time.Second)
			defer cancel()
			require.Equal(t, codes.Unauthenticated, status.Code(tc.call(ctx)))
		})
	}
	require.Equal(t, int64(len(tests)), recorder.calls.Load(), "wrong issuers must fail before handlers")
}

func grpcMethodForGrant(name string) string {
	switch name {
	case "apply":
		return rolev1.RoleService_ApplyGameSessionGrants_FullMethodName
	case "revoke":
		return rolev1.RoleService_RevokeGameSessionGrants_FullMethodName
	default:
		return rolev1.RoleService_CheckGameSessionGrant_FullMethodName
	}
}

func TestRuntimeListener_AllowsOnlyAuthenticatedV2OwnershipSurface(t *testing.T) {
	f := newRuntimeFixture(t, 2)
	address, recorder, roots := startRuntimeListener(t, f)
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
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
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
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
	client := runtimeListenerClient(t, address, runtimeTLSCredentials(f, roots))
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
