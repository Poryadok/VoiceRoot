package grpcsvc

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/spaceaudit"
	"voice/backend/space/internal/auditprincipal"
	"voice/backend/space/internal/store"
)

type AuditIngestionStore interface {
	ImportAuditEvent(context.Context, string, spaceaudit.ReplayEvent) error
}
type AuditIngestionGRPC struct {
	spacev1.UnimplementedSpaceServiceServer
	Store AuditIngestionStore
}

func (s *AuditIngestionGRPC) AppendAuditEvent(ctx context.Context, req *spacev1.AppendAuditEventRequest) (*spacev1.AppendAuditEventResponse, error) {
	producer, ok := auditprincipal.Producer(ctx, req)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "verified audit producer required")
	}
	event := spaceaudit.ReplayEvent{AuditEventID: req.AuditEventId, SpaceID: req.SpaceId, ActorProfileID: req.ActorProfileId, Action: req.Action, TargetType: req.TargetType, TargetID: req.TargetId, DetailsJSON: req.DetailsJson, OccurredAt: req.OccurredAt}
	if err := spaceaudit.ValidateEvent(producer, event); err != nil {
		if errors.Is(err, spaceaudit.ErrProducerAction) {
			return nil, status.Error(codes.PermissionDenied, "action is not owned by producer")
		}
		return nil, status.Error(codes.InvalidArgument, "invalid audit event")
	}
	if s.Store == nil {
		return nil, status.Error(codes.Unavailable, "audit storage unavailable")
	}
	if err := s.Store.ImportAuditEvent(ctx, producer, event); err != nil {
		if errors.Is(err, store.ErrAuditEventConflict) || errors.Is(err, store.ErrCompositeConflict) {
			return nil, status.Error(codes.AlreadyExists, "audit event ID already reserved")
		}
		if errors.Is(err, store.ErrLifecycleFrozen) || errors.Is(err, store.ErrCompositeResultExpired) {
			return nil, status.Error(codes.FailedPrecondition, "audit lifecycle admission refused")
		}
		if errors.Is(err, spaceaudit.ErrInvalidEvent) || errors.Is(err, spaceaudit.ErrInvalidDetails) {
			return nil, status.Error(codes.InvalidArgument, "invalid audit event")
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, status.Error(codes.NotFound, "audit space not found")
		}
		return nil, status.Error(codes.Unavailable, "audit storage unavailable")
	}
	return &spacev1.AppendAuditEventResponse{}, nil
}
func RegisterAuditIngestionServer(server grpc.ServiceRegistrar, service *AuditIngestionGRPC) {
	desc := spacev1.SpaceService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range spacev1.SpaceService_ServiceDesc.Methods {
		if method.MethodName == "AppendAuditEvent" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, service)
}
