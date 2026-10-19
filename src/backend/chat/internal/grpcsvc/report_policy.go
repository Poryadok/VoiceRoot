package grpcsvc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/reportprincipal"
)

type ReportPolicyStore interface {
	ManagedChatMessageEntitled(context.Context, uuid.UUID, uuid.UUID, time.Time) (bool, error)
}

// ReportPolicyGRPC uses the native managed interval ledger. Its unmanaged true
// result is only an additional restriction check, never Space/member authority.
type ReportPolicyGRPC struct {
	chatv1.UnimplementedChatServiceServer
	Store ReportPolicyStore
}

func (s *ReportPolicyGRPC) CheckMessageReadEntitlement(ctx context.Context, r *chatv1.CheckMessageReadEntitlementRequest) (*chatv1.CheckMessageReadEntitlementResponse, error) {
	if !reportprincipal.Producer(ctx, r) {
		return nil, status.Error(codes.Unauthenticated, "verified report policy producer required")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "report policy unavailable")
	}
	chat, _ := uuid.Parse(r.ChatId)
	profile, _ := uuid.Parse(r.ProfileId)
	entitled, err := s.Store.ManagedChatMessageEntitled(ctx, chat, profile, r.MessageCreatedAt.AsTime())
	if err != nil {
		return nil, status.Error(codes.Unavailable, "report policy unavailable")
	}
	return &chatv1.CheckMessageReadEntitlementResponse{Entitled: entitled}, nil
}
func RegisterReportPolicyServer(s grpc.ServiceRegistrar, handler *ReportPolicyGRPC) {
	desc := chatv1.ChatService_ServiceDesc
	desc.Methods = nil
	for _, method := range chatv1.ChatService_ServiceDesc.Methods {
		if method.MethodName == "CheckMessageReadEntitlement" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	desc.Streams = nil
	s.RegisterService(&desc, handler)
}
