package principalgrpc

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"strings"
	"testing"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestOwnershipUnaryInterceptor_ProtectsOnlyExactLifecycleMethods(t *testing.T) {
	for _, method := range []string{"/voice.role.v1.RoleService/ApplyOwnershipTransfer", "/voice.role.v1.RoleService/CompensateOwnershipTransfer"} {
		t.Run(method, func(t *testing.T) {
			called := false
			_, err := OwnershipUnaryInterceptor(nil)(context.Background(), &emptypb.Empty{}, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			if status.Code(err) != codes.Unavailable || called {
				t.Fatalf("unconfigured lifecycle boundary: err=%v handler=%v", err, called)
			}
		})
	}
	for _, method := range []string{"/voice.role.v1.RoleService/CheckPermission", "/voice.role.v1.RoleService/AssignRole", "/voice.role.v1.RoleService/ApplyOwnershipTransferOther"} {
		t.Run(method, func(t *testing.T) {
			called := false
			_, err := OwnershipUnaryInterceptor(nil)(context.Background(), &emptypb.Empty{}, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			if err != nil || !called {
				t.Fatalf("unmigrated method changed: err=%v handler=%v", err, called)
			}
		})
	}
}

func TestOwnershipUnaryInterceptor_ConfiguredVerifierProtectsBothLifecycleMethods(t *testing.T) {
	for _, method := range []string{"/voice.role.v1.RoleService/ApplyOwnershipTransfer", "/voice.role.v1.RoleService/CompensateOwnershipTransfer"} {
		t.Run(method, func(t *testing.T) {
			for _, tc := range []struct {
				name        string
				md          metadata.MD
				rejectToken bool
				want        codes.Code
				wantVerify  int
			}{
				{name: "valid", md: metadata.Pairs("authorization", "Bearer signed-token", "x-request-id", "request-1"), want: codes.OK, wantVerify: 1},
				{name: "invalid credential", md: metadata.Pairs("authorization", "Bearer bad-token", "x-request-id", "request-1"), rejectToken: true, want: codes.Unauthenticated, wantVerify: 1},
				{name: "duplicate credential", md: metadata.Pairs("authorization", "Bearer a", "authorization", "Bearer b", "x-request-id", "request-1"), want: codes.Unauthenticated},
				{name: "duplicate binding", md: metadata.Pairs("authorization", "Bearer a", "x-request-id", "request-1", "x-request-id", "request-2"), want: codes.Unauthenticated},
				{name: "missing binding", md: metadata.Pairs("authorization", "Bearer a"), want: codes.Unauthenticated},
				{name: "raw identity", md: metadata.Pairs("authorization", "Bearer a", "x-request-id", "request-1", "x-voice-profile-id", "forged"), want: codes.Unauthenticated},
			} {
				t.Run(tc.name, func(t *testing.T) {
					verifies, handles := 0, 0
					var req proto.Message = &rolev1.ApplyOwnershipTransferRequest{SpaceId: "space", OldOwnerProfileId: "old", NewOwnerProfileId: "new", OperationId: "operation"}
					if strings.Contains(method, "Compensate") {
						req = &rolev1.CompensateOwnershipTransferRequest{SpaceId: "space", OldOwnerProfileId: "old", NewOwnerProfileId: "new", OperationId: "operation"}
					}
					hash, err := principal.RequestHash(req)
					require.NoError(t, err)
					verified := principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: method, RequestID: "request-1", RequestHash: hash}
					interceptor := OwnershipUnaryInterceptor(verifierFunc(func(_ context.Context, token, rpc, requestID, requestHash string) (principal.Principal, error) {
						verifies++
						require.Equal(t, method, rpc)
						require.Equal(t, "request-1", requestID)
						require.Equal(t, hash, requestHash)
						if tc.rejectToken {
							return principal.Principal{}, errors.New("invalid signature")
						}
						require.Equal(t, "signed-token", token)
						return verified, nil
					}))
					_, err = interceptor(metadata.NewIncomingContext(context.Background(), tc.md), req, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, actual any) (any, error) {
						handles++
						got, ok := principal.FromContext(ctx)
						require.True(t, ok)
						require.Equal(t, verified, got)
						require.Same(t, req, actual)
						return &emptypb.Empty{}, nil
					})
					require.Equal(t, tc.want, status.Code(err))
					require.Equal(t, tc.wantVerify, verifies)
					if tc.want == codes.OK {
						require.Equal(t, 1, handles)
					} else {
						require.Zero(t, handles)
					}
				})
			}
		})
	}
}
