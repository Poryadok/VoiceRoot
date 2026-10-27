package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/socialprincipal"
)

// NotificationPrimaryGRPC is registered only on the separate mTLS principal
// listener. It does not expose profile creation or general account enumeration.
type NotificationPrimaryGRPC struct {
	userv1.UnimplementedUserServiceServer
	User *UserGRPC
}

func (s *NotificationPrimaryGRPC) ResolvePrimaryProfileIDs(ctx context.Context, req *userv1.ResolvePrimaryProfileIDsRequest) (*userv1.ResolvePrimaryProfileIDsResponse, error) {
	if err := socialprincipal.RequireNotificationPrimary(ctx, req); err != nil {
		return nil, err
	}
	if req == nil || len(req.AccountIds) != 1 || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "one payer account required")
	}
	raw := req.AccountIds[0]
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return nil, status.Error(codes.InvalidArgument, "canonical payer account required")
	}
	if s == nil || s.User == nil {
		return nil, status.Error(codes.Unavailable, "primary authority unavailable")
	}
	return s.User.resolvePrimaryProfileIDs(ctx, req)
}

func RegisterNotificationPrimaryServer(server grpc.ServiceRegistrar, service *UserGRPC) {
	desc := userv1.UserService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range userv1.UserService_ServiceDesc.Methods {
		if method.MethodName == "ResolvePrimaryProfileIDs" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, &NotificationPrimaryGRPC{User: service})
}
