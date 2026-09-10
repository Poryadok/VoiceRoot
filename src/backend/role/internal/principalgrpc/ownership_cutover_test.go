package principalgrpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestOwnershipUnaryInterceptor_DrainsEveryOwnershipProtocolOnLegacyListener(t *testing.T) {
	methods := []string{
		rolev1.RoleService_GetOwnershipTransferCapabilities_FullMethodName,
		rolev1.RoleService_PrepareOwnershipTransfer_FullMethodName,
		rolev1.RoleService_FinalizeOwnershipTransfer_FullMethodName,
		rolev1.RoleService_AbortOwnershipTransfer_FullMethodName,
		rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName,
		rolev1.RoleService_CompensateOwnershipTransfer_FullMethodName,
	}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			verified, handled := 0, 0
			interceptor := OwnershipUnaryInterceptor(verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) {
				verified++
				return principal.Principal{}, errors.New("must not verify a drained method")
			}))
			_, err := interceptor(context.Background(), &emptypb.Empty{}, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
				handled++
				return &emptypb.Empty{}, nil
			})
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.Zero(t, verified, "legacy ownership denial must not consume verifier dependencies")
			require.Zero(t, handled, "legacy ownership denial must precede handler entry")
		})
	}
}

func TestOwnershipUnaryInterceptor_PreservesNonOwnershipMethods(t *testing.T) {
	for _, method := range []string{
		rolev1.RoleService_CheckPermission_FullMethodName,
		rolev1.RoleService_AssignRole_FullMethodName,
		rolev1.RoleService_ListRoles_FullMethodName,
		rolev1.RoleService_ApplyOwnershipTransfer_FullMethodName + "Other",
	} {
		t.Run(method, func(t *testing.T) {
			handled := 0
			_, err := OwnershipUnaryInterceptor(nil)(context.Background(), &emptypb.Empty{}, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
				handled++
				return &emptypb.Empty{}, nil
			})
			require.NoError(t, err)
			require.Equal(t, 1, handled)
		})
	}
}
