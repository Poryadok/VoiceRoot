package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/role/permissions"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"

	spacev1 "voice.app/voice/space/v1"
)

// GetAuditLog returns administrative actions visible to roles with the exact
// SPACE_VIEW_AUDIT_LOG permission. Without Role Service, only the owner may read.
func (s *SpaceGRPC) GetAuditLog(ctx context.Context, req *spacev1.GetAuditLogRequest) (*spacev1.GetAuditLogResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	release, err := s.lockSpaceMutation(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	defer release()

	member, err := s.Store.IsSpaceMember(ctx, spaceID, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !member {
		return nil, status.Error(codes.PermissionDenied, "not a space member")
	}
	if err := s.requireSpacePermission(ctx, spaceID, permissions.SpaceViewAuditLog); err != nil {
		return nil, err
	}

	limit := 50
	cursor := ""
	if page := req.GetPage(); page != nil {
		cursor = page.GetCursor()
		if page.GetPageSize() > 0 {
			limit = int(page.GetPageSize())
		}
	}
	if limit > 100 {
		limit = 100
	}
	page, err := s.Store.ListAuditLogPage(ctx, spaceID, cursor, limit)
	if errors.Is(err, store.ErrInvalidAuditCursor) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	entries := make([]*spacev1.AuditLogEntry, 0, len(page.Rows))
	for _, row := range page.Rows {
		entries = append(entries, &spacev1.AuditLogEntry{
			Id:             row.ID.String(),
			SpaceId:        row.SpaceID.String(),
			ActorProfileId: row.ActorProfileID.String(),
			Action:         row.Action,
			TargetType:     row.TargetType,
			TargetId:       row.TargetID.String(),
			DetailsJson:    row.DetailsJSON,
			CreatedAt:      timestamppb.New(row.CreatedAt),
		})
	}
	return &spacev1.GetAuditLogResponse{AuditLogList: &spacev1.AuditLogList{
		Entries: entries, NextCursor: page.NextCursor,
	}}, nil
}
