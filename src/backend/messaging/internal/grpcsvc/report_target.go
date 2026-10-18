package grpcsvc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ownerv1 "voice.app/voice/messaging/v1"

	"voice/backend/pkg/reporttargetprincipal"
)

// ResolveReportTarget is a private existence decision, not viewer authorization.
func (s *MessagingGRPC) ResolveReportTarget(ctx context.Context, req *ownerv1.ResolveReportTargetRequest) (*ownerv1.ResolveReportTargetResponse, error) {
	if err := reporttargetprincipal.Require(ctx, "messaging", req); err != nil {
		return nil, err
	}
	if _, err := reporttargetprincipal.CanonicalID(req.GetReportId()); err != nil {
		return nil, err
	}
	id, err := reporttargetprincipal.CanonicalID(req.GetMessageId())
	if err != nil {
		return nil, err
	}
	if s == nil || s.Messages == nil {
		return nil, status.Error(codes.Unavailable, "report target authority unavailable")
	}
	state, err := s.Messages.ResolveReportTarget(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "report target authority unavailable")
	}
	return &ownerv1.ResolveReportTargetResponse{State: state}, nil
}

// RegisterReportTargetServer exposes only this protected method.
func RegisterReportTargetServer(server grpc.ServiceRegistrar, service *MessagingGRPC) {
	desc := ownerv1.MessagingService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range ownerv1.MessagingService_ServiceDesc.Methods {
		if method.MethodName == "ResolveReportTarget" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, service)
}
