package grpcsvc

import (
	"context"
	"google.golang.org/grpc"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/socialprincipal"
)

// SocialPrivacyGRPC is a separate domain entrypoint from legacy callers.
type SocialPrivacyGRPC struct {
	spacev1.UnimplementedSpaceServiceServer
	Space *SpaceGRPC
}

func (s *SocialPrivacyGRPC) AreCoMembers(ctx context.Context, req *spacev1.AreCoMembersRequest) (*spacev1.AreCoMembersResponse, error) {
	if err := socialprincipal.RequireSocial(ctx, "space", req); err != nil {
		return nil, err
	}
	return s.Space.AreCoMembers(ctx, req)
}
func RegisterSocialPrivacyServer(server grpc.ServiceRegistrar, service *SpaceGRPC) {
	desc := spacev1.SpaceService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range spacev1.SpaceService_ServiceDesc.Methods {
		if method.MethodName == "AreCoMembers" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, &SocialPrivacyGRPC{Space: service})
}
