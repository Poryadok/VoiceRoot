package grpcsvc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/socialprincipal"
)

const socialGetProfilesMaxProfiles = 500

// SocialPrivacyGRPC is a separate domain entrypoint from legacy callers.
type SocialPrivacyGRPC struct {
	userv1.UnimplementedUserServiceServer
	User *UserGRPC
}

func (s *SocialPrivacyGRPC) GetPrivacySettings(ctx context.Context, req *userv1.GetPrivacySettingsRequest) (*userv1.GetPrivacySettingsResponse, error) {
	if err := socialprincipal.RequireSocial(ctx, "user", req); err != nil {
		return nil, err
	}
	return s.User.GetPrivacySettings(ctx, req)
}

func (s *SocialPrivacyGRPC) GetProfile(ctx context.Context, req *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error) {
	if err := socialprincipal.RequireSocialForMethod(ctx, "user", userv1.UserService_GetProfile_FullMethodName, req); err != nil {
		return nil, err
	}
	return s.User.GetProfile(ctx, req)
}

func (s *SocialPrivacyGRPC) GetProfiles(ctx context.Context, req *userv1.GetProfilesRequest) (*userv1.GetProfilesResponse, error) {
	if err := socialprincipal.RequireSocialForMethod(ctx, "user", userv1.UserService_GetProfiles_FullMethodName, req); err != nil {
		return nil, err
	}
	if len(req.GetProfileIds()) > socialGetProfilesMaxProfiles {
		return nil, status.Errorf(codes.InvalidArgument, "at most %d profile IDs may be resolved per protected request", socialGetProfilesMaxProfiles)
	}
	return s.User.GetProfiles(ctx, req)
}

func (s *SocialPrivacyGRPC) ListProfileIDsForAccount(ctx context.Context, req *userv1.ListProfileIDsForAccountRequest) (*userv1.ListProfileIDsForAccountResponse, error) {
	if err := socialprincipal.RequireSocialForMethod(ctx, "user", userv1.UserService_ListProfileIDsForAccount_FullMethodName, req); err != nil {
		return nil, err
	}
	return s.User.listProfileIDsForAccount(ctx, req)
}

func RegisterSocialPrivacyServer(server grpc.ServiceRegistrar, service *UserGRPC) {
	desc := userv1.UserService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range userv1.UserService_ServiceDesc.Methods {
		if method.MethodName == "GetPrivacySettings" || method.MethodName == "GetProfile" || method.MethodName == "GetProfiles" || method.MethodName == "ListProfileIDsForAccount" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, &SocialPrivacyGRPC{User: service})
}

// NotificationPresenceGRPC is a separate authenticated User entrypoint with a
// single routing-only method. It is never registered on the public or Social
// listener.
type NotificationPresenceGRPC struct {
	userv1.UnimplementedUserServiceServer
	User *UserGRPC
}

func (s *NotificationPresenceGRPC) GetNotificationRoutingPresence(ctx context.Context, req *userv1.GetNotificationRoutingPresenceRequest) (*userv1.GetNotificationRoutingPresenceResponse, error) {
	if err := socialprincipal.RequireNotificationPresence(ctx, userv1.UserService_GetNotificationRoutingPresence_FullMethodName, req); err != nil {
		return nil, err
	}
	return s.User.notificationRoutingPresence(ctx, req)
}

func RegisterNotificationPresenceServer(server grpc.ServiceRegistrar, service *UserGRPC) {
	desc := userv1.UserService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range userv1.UserService_ServiceDesc.Methods {
		if method.MethodName == "GetNotificationRoutingPresence" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	desc.Metadata = "voice/user/v1/user.proto"
	server.RegisterService(&desc, &NotificationPresenceGRPC{User: service})
}
