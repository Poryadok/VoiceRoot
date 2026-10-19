package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/pkg/principal"
	"voice/backend/role/permissions"
	"voice/backend/space/internal/store"

	spacev1 "voice.app/voice/space/v1"
)

// GetAuditLog returns administrative actions visible to roles with the exact
// SPACE_VIEW_AUDIT_LOG permission. Without Role Service, only the owner may read.
func (s *SpaceGRPC) GetAuditLog(ctx context.Context, req *spacev1.GetAuditLogRequest) (*spacev1.GetAuditLogResponse, error) {
	identity, err := principal.RequireDelegatedUser(ctx, "space", spacev1.SpaceService_GetAuditLog_FullMethodName, req)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "verified audit reader required")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	caller := identity.ProfileID
	release, err := s.lockSpaceMutation(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	defer release()

	member, err := s.Store.IsSpaceMember(ctx, spaceID, caller)
	if err != nil {
		return nil, mapSpaceStoreError(err)
	}
	if !member {
		return nil, status.Error(codes.PermissionDenied, "not a space member")
	}
	if err := s.requireSpacePermissionForProfile(ctx, spaceID, caller, permissions.SpaceViewAuditLog); err != nil {
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
	query := store.AuditLogQuery{SpaceID: spaceID, ViewerProfileID: caller,
		PageSize: limit, Cursor: cursor, AccessScope: "space.audit.read.v1"}
	if req.ActorProfileId != nil {
		actor, err := parseUUIDField("actor_profile_id", *req.ActorProfileId)
		if err != nil || actor == uuid.Nil {
			return nil, status.Error(codes.InvalidArgument, "invalid actor_profile_id")
		}
		query.ActorProfileID = &actor
	}
	if req.Action != nil {
		if strings.TrimSpace(*req.Action) == "" {
			return nil, status.Error(codes.InvalidArgument, "invalid action")
		}
		query.Action = req.Action
	}
	if req.From != nil {
		if req.From.CheckValid() != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid from")
		}
		from := req.From.AsTime().UTC()
		query.From = &from
	}
	if req.To != nil {
		if req.To.CheckValid() != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid to")
		}
		to := req.To.AsTime().UTC()
		query.To = &to
	}
	page, err := s.Store.ListAuditLogQueryPage(ctx, query, s.AuditCursorKey)
	if errors.Is(err, store.ErrInvalidAuditCursor) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err != nil {
		return nil, mapSpaceStoreError(err)
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
