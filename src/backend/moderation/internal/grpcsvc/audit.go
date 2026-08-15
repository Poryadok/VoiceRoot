package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	moderationv1 "voice.app/voice/moderation/v1"

	"voice/backend/moderation/internal/store"
)

func (s *ModerationGRPC) ExportAuditLog(ctx context.Context, _ *moderationv1.ExportAuditLogRequest) (*moderationv1.ExportAuditLogResponse, error) {
	if s == nil || s.AuditLog == nil {
		return nil, status.Error(codes.FailedPrecondition, "audit log store is not configured")
	}
	if _, err := requireInternalModerator(ctx); err != nil {
		return nil, err
	}
	rows, err := s.AuditLog.ListAuditLog(ctx, 1000)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	entries := make([]*moderationv1.AuditLogEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, auditRowToProto(row))
	}
	return &moderationv1.ExportAuditLogResponse{
		AuditLogExport: &moderationv1.AuditLogExport{Entries: entries},
	}, nil
}

func auditRowToProto(row store.AuditRow) *moderationv1.AuditLogEntry {
	return &moderationv1.AuditLogEntry{
		Id:              row.ID.String(),
		ActorProfileId:  row.ActorProfileID.String(),
		Action:          row.Action,
		TargetType:      row.TargetType,
		TargetId:        row.TargetID.String(),
		Details:         row.Details,
		CreatedAt:       timestamppb.New(row.CreatedAt.UTC()),
	}
}
