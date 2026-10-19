package grpcsvc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaccountcapture"
)

type SpaceAccountCaptureAuthority interface {
	CaptureSpaceMemberAccountBindings(context.Context, *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error)
}

type SpaceAccountCaptureGRPC struct {
	userv1.UnimplementedUserServiceServer
	Authority SpaceAccountCaptureAuthority
}

func (s *SpaceAccountCaptureGRPC) CaptureSpaceMemberAccountBindings(ctx context.Context, req *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	p, ok := principal.FromContext(ctx)
	hash, hashErr := principal.RequestHash(req)
	if !ok || hashErr != nil || p.Kind != "service" || p.Issuer != "space" || p.Subject != "service:space" || p.Audience != "user" || p.RPC != userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName || p.RequestHash != hash || p.RequestID == "" || p.JWTID == "" || !time.Now().Before(p.ExpiresAt) || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return nil, status.Error(codes.Unauthenticated, "invalid Space capture principal")
	}
	if spaceaccountcapture.ValidateRequest(req) != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Space capture request")
	}
	if s == nil || s.Authority == nil {
		return nil, status.Error(codes.Unavailable, "Space capture authority unavailable")
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := s.Authority.CaptureSpaceMemberAccountBindings(bounded, proto.Clone(req).(*userv1.CaptureSpaceMemberAccountBindingsRequest))
	if err != nil || bounded.Err() != nil || spaceaccountcapture.ValidateResponse(req, out) != nil {
		return nil, status.Error(codes.Unavailable, "Space capture authority unavailable")
	}
	return out, nil
}

func RegisterSpaceAccountCaptureServer(s grpc.ServiceRegistrar, authority SpaceAccountCaptureAuthority) {
	desc := userv1.UserService_ServiceDesc
	desc.Methods, desc.Streams = nil, nil
	for _, method := range userv1.UserService_ServiceDesc.Methods {
		if method.MethodName == "CaptureSpaceMemberAccountBindings" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	s.RegisterService(&desc, &SpaceAccountCaptureGRPC{Authority: authority})
}
