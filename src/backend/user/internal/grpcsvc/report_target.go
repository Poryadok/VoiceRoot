package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	commonv1 "voice.app/voice/common/v1"
	ownerv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/reporttargetprincipal"
)

// ResolveReportTarget is a private existence decision, not viewer authorization.
func (s *UserGRPC) ResolveReportTarget(ctx context.Context, req *ownerv1.ResolveReportTargetRequest) (*ownerv1.ResolveReportTargetResponse, error) {
	if err := reporttargetprincipal.Require(ctx, "user", req); err != nil {
		return nil, err
	}
	if _, err := reporttargetprincipal.CanonicalID(req.GetReportId()); err != nil {
		return nil, err
	}
	id, err := reporttargetprincipal.CanonicalID(req.GetProfileId())
	if err != nil {
		return nil, err
	}
	if s == nil || s.Profiles == nil {
		return nil, status.Error(codes.Unavailable, "report target authority unavailable")
	}
	state, account, err := s.Profiles.ResolveReportTarget(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "report target authority unavailable")
	}
	if state == commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE {
		// The event overlay can lag Auth; current deletion authority is required.
		if s.DeletedAccounts == nil {
			return nil, status.Error(codes.Unavailable, "report target authority unavailable")
		}
		deleted, checkErr := s.DeletedAccounts.DeletedAmong(ctx, []uuid.UUID{account})
		if checkErr != nil {
			return nil, status.Error(codes.Unavailable, "report target authority unavailable")
		}
		for found := range deleted {
			if found != account {
				return nil, status.Error(codes.Unavailable, "report target authority unavailable")
			}
		}
		if _, inactive := deleted[account]; inactive {
			state = commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED
		}
	}
	out := &ownerv1.ResolveReportTargetResponse{State: state}
	if state == commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE {
		out.AccountId = account.String()
	}
	return out, nil
}

// RegisterReportTargetServer exposes only this protected method.
func RegisterReportTargetServer(server grpc.ServiceRegistrar, service *UserGRPC) {
	desc := ownerv1.UserService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range ownerv1.UserService_ServiceDesc.Methods {
		if method.MethodName == "ResolveReportTarget" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, service)
}
