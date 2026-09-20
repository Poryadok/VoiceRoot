package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/socialprincipal"

	userv1 "voice.app/voice/user/v1"
)

// FileOwnershipGRPC exposes only the profile-owner resolver to a verified File
// service credential. The ordinary listener retains legacy Messaging/Chat
// callers but never accepts an unverified File marker.
type FileOwnershipGRPC struct {
	userv1.UnimplementedUserServiceServer
	User *UserGRPC
}

func (s *FileOwnershipGRPC) ResolveAccountIDForProfile(ctx context.Context, req *userv1.ResolveAccountIDForProfileRequest) (*userv1.ResolveAccountIDForProfileResponse, error) {
	if err := socialprincipal.RequireSocialForMethod(ctx, "file", userv1.UserService_ResolveAccountIDForProfile_FullMethodName, req); err != nil {
		return nil, err
	}
	profileID, profileErr := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	actorID, actorErr := uuid.Parse(strings.TrimSpace(req.GetActorProfileId()))
	operationID, operationErr := uuid.Parse(strings.TrimSpace(req.GetOperationId()))
	if profileErr != nil || actorErr != nil || operationErr != nil || profileID == uuid.Nil || actorID == uuid.Nil || operationID == uuid.Nil || profileID != actorID {
		return nil, status.Error(codes.InvalidArgument, "invalid retention owner assertion")
	}
	if s == nil || s.User == nil {
		return nil, status.Error(codes.Unavailable, "user ownership service unavailable")
	}
	return s.User.resolveAccountIDForProfile(ctx, req)
}

func RegisterFileOwnershipServer(server grpc.ServiceRegistrar, service *UserGRPC) {
	desc := userv1.UserService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range userv1.UserService_ServiceDesc.Methods {
		if method.MethodName == "ResolveAccountIDForProfile" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, &FileOwnershipGRPC{User: service})
}
