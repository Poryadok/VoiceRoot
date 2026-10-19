package grpcsvc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/memberauthorityprincipal"
)

type MemberAuthorityStore interface {
	ReadMemberAdmissionAuthority(context.Context, *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error)
}

type MemberAuthorityGRPC struct {
	spacev1.UnimplementedSpaceServiceServer
	Store MemberAuthorityStore
}

func (s *MemberAuthorityGRPC) GetMemberAdmissionAuthority(ctx context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
	if _, ok := memberauthorityprincipal.Producer(ctx, req); !ok {
		return nil, status.Error(codes.Unauthenticated, "verified Role authority reader required")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "member authority unavailable")
	}
	response, err := s.Store.ReadMemberAdmissionAuthority(ctx, req)
	if err != nil || response == nil {
		return nil, status.Error(codes.Unavailable, "member authority unavailable")
	}
	return response, nil
}

// RegisterMemberAuthorityServer registers this one method, not ordinary Space
// mutation or any other protected service on the Role authority listener.
func RegisterMemberAuthorityServer(server grpc.ServiceRegistrar, service *MemberAuthorityGRPC) {
	desc := spacev1.SpaceService_ServiceDesc
	desc.Methods, desc.Streams = nil, nil
	for _, method := range spacev1.SpaceService_ServiceDesc.Methods {
		if method.MethodName == "GetMemberAdmissionAuthority" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, service)
}
