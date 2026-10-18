package grpcsvc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ownerv1 "voice.app/voice/story/v1"

	"voice/backend/pkg/reporttargetprincipal"
)

// ResolveReportTarget is a private existence decision, not viewer authorization.
func (s *StoryGRPC) ResolveReportTarget(ctx context.Context, req *ownerv1.ResolveReportTargetRequest) (*ownerv1.ResolveReportTargetResponse, error) {
	if err := reporttargetprincipal.Require(ctx, "story", req); err != nil {
		return nil, err
	}
	if _, err := reporttargetprincipal.CanonicalID(req.GetReportId()); err != nil {
		return nil, err
	}
	id, err := reporttargetprincipal.CanonicalID(req.GetStoryId())
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "report target authority unavailable")
	}
	state, err := s.Store.ResolveReportTarget(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "report target authority unavailable")
	}
	return &ownerv1.ResolveReportTargetResponse{State: state}, nil
}

// RegisterReportTargetServer exposes only this protected method.
func RegisterReportTargetServer(server grpc.ServiceRegistrar, service *StoryGRPC) {
	desc := ownerv1.StoryService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range ownerv1.StoryService_ServiceDesc.Methods {
		if method.MethodName == "ResolveReportTarget" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, service)
}
