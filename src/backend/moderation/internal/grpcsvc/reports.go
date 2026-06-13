package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/moderation/internal/store"

	moderationv1 "voice.app/voice/moderation/v1"
)

var allowedCategories = map[string]struct{}{
	"spam":       {},
	"harassment": {},
	"offensive":  {},
	"fake":       {},
	"cheating":   {},
	"other":      {},
}

func (s *ModerationGRPC) CreateReport(ctx context.Context, req *moderationv1.CreateReportRequest) (*moderationv1.CreateReportResponse, error) {
	if s == nil || s.Reports == nil {
		return nil, status.Error(codes.FailedPrecondition, "report store is not configured")
	}

	reporterProfileID, err := profileIDFromMetadata(ctx)
	if err != nil {
		return nil, err
	}
	targetType := strings.TrimSpace(req.GetTargetType())
	if targetType == "" {
		return nil, status.Error(codes.InvalidArgument, "target_type is required")
	}
	switch targetType {
	case "user", "message", "space":
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid target_type")
	}

	targetID, err := uuid.Parse(strings.TrimSpace(req.GetTargetId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid target_id")
	}
	if targetType == "user" && targetID == reporterProfileID {
		return nil, status.Error(codes.PermissionDenied, "cannot report yourself")
	}

	category := strings.TrimSpace(req.GetCategory())
	if _, ok := allowedCategories[category]; !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid category")
	}

	var description *string
	desc := strings.TrimSpace(req.GetDescription())
	if desc != "" {
		description = &desc
	}
	if category == "other" && description == nil {
		return nil, status.Error(codes.InvalidArgument, "description is required for category=other")
	}

	evidenceJSON := strings.TrimSpace(req.GetEvidenceJson())
	if evidenceJSON == "" {
		evidenceJSON = "{}"
	}

	row, err := s.Reports.InsertReport(ctx, reporterProfileID, targetType, targetID, category, description, evidenceJSON)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if targetType == "user" {
		count, err := s.Reports.CountReports24h(ctx, targetID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if count == 11 {
			if err := s.Reports.InsertAutoModLog(ctx, targetID, "report_threshold", "shadow_ban", `{"window":"24h","count":11}`); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}

	return &moderationv1.CreateReportResponse{
		Report: reportRowToProto(row),
	}, nil
}

func (s *ModerationGRPC) ListReports(ctx context.Context, req *moderationv1.ListReportsRequest) (*moderationv1.ListReportsResponse, error) {
	if s == nil || s.Reports == nil {
		return nil, status.Error(codes.FailedPrecondition, "report store is not configured")
	}
	if !isInternalRequest(ctx) {
		return nil, status.Error(codes.PermissionDenied, "internal access required")
	}

	limit := int32(50)
	if req.GetPage() != nil && req.GetPage().GetPageSize() > 0 {
		limit = req.GetPage().GetPageSize()
	}
	rows, err := s.Reports.ListReports(ctx, strings.TrimSpace(req.GetStatusFilter()), limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*moderationv1.Report, 0, len(rows))
	for i := range rows {
		out = append(out, reportRowToProto(&rows[i]))
	}
	return &moderationv1.ListReportsResponse{
		ReportList: &moderationv1.ReportList{
			Reports: out,
		},
	}, nil
}

func profileIDFromMetadata(ctx context.Context) (uuid.UUID, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	vals := md.Get("x-voice-profile-id")
	if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
		return uuid.Nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	id, err := uuid.Parse(strings.TrimSpace(vals[0]))
	if err != nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "invalid profile")
	}
	return id, nil
}

func isInternalRequest(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	for _, v := range md.Get("x-voice-internal") {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes":
			return true
		}
	}
	return false
}

func reportRowToProto(row *store.ReportRow) *moderationv1.Report {
	if row == nil {
		return nil
	}
	out := &moderationv1.Report{
		Id:                row.ID.String(),
		ReporterProfileId: row.ReporterProfileID.String(),
		TargetType:        row.TargetType,
		TargetId:          row.TargetID.String(),
		Category:          row.Category,
		EvidenceJson:      row.EvidenceJSON,
		Status:            row.Status,
		CreatedAt:         timestamppb.New(row.CreatedAt.UTC()),
	}
	if row.Description != nil {
		out.Description = row.Description
	}
	if row.AssignedToProfile != nil {
		v := row.AssignedToProfile.String()
		out.AssignedToProfileId = &v
	}
	if row.ResolvedAt != nil {
		out.ResolvedAt = timestamppb.New(row.ResolvedAt.UTC())
	}
	if row.ResolutionJSON != nil {
		out.ResolutionJson = *row.ResolutionJSON
	}
	return out
}
