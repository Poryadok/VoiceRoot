package principalgrpc

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	rolev1 "voice.app/voice/role/v1"
)

func TestAdmissionMethods_NeverReachOrdinaryListener(t *testing.T) {
	for _, method := range []string{rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName, rolev1.RoleService_RemoveMemberRoles_FullMethodName} {
		t.Run(method, func(t *testing.T) {
			called := false
			_, err := OwnershipUnaryInterceptor(nil)(context.Background(), &emptypb.Empty{}, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			if called || status.Code(err) != codes.Unavailable {
				t.Fatalf("protected admission reached ordinary listener: called=%v err=%v", called, err)
			}
		})
	}
}
