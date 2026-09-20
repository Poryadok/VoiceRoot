package grpcsvc

import (
	"context"
	"google.golang.org/grpc"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/socialprincipal"
)

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
		if method.MethodName == "GetPrivacySettings" || method.MethodName == "GetProfile" || method.MethodName == "ListProfileIDsForAccount" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, &SocialPrivacyGRPC{User: service})
}
