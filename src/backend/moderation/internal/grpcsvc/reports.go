package grpcsvc

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	case "user", "message", "space", "story":
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
	if strings.EqualFold(category, "mm_toxic") {
		category = "cheating"
	}
	if _, ok := allowedCategories[category]; !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid category")
	}

	var description *string
	desc := strings.TrimSpace(req.GetDescription())
	if desc != "" {
		if len([]rune(desc)) > 500 {
			return nil, status.Error(codes.InvalidArgument, "description must be at most 500 characters")
		}
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
		if err := s.maybeAutoShadowBan(ctx, targetID); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
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
	rows, err := s.Reports.ListReportsFiltered(ctx, strings.TrimSpace(req.GetStatusFilter()), strings.TrimSpace(req.GetQueueFilter()), limit)
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

func (s *ModerationGRPC) GetReport(ctx context.Context, req *moderationv1.GetReportRequest) (*moderationv1.GetReportResponse, error) {
	if s == nil || s.Reports == nil {
		return nil, status.Error(codes.FailedPrecondition, "report store is not configured")
	}
	if _, err := requireInternalModerator(ctx); err != nil {
		return nil, err
	}
	reportID, err := uuid.Parse(strings.TrimSpace(req.GetReportId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid report_id")
	}
	row, err := s.Reports.GetReportByID(ctx, reportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "report not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &moderationv1.GetReportResponse{Report: reportRowToProto(row)}, nil
}

func (s *ModerationGRPC) ResolveReport(ctx context.Context, req *moderationv1.ResolveReportRequest) (*moderationv1.ResolveReportResponse, error) {
	if s == nil || s.Reports == nil {
		return nil, status.Error(codes.FailedPrecondition, "report store is not configured")
	}
	modProfile, err := requireInternalModerator(ctx)
	if err != nil {
		return nil, err
	}
	reportID, err := uuid.Parse(strings.TrimSpace(req.GetReportId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid report_id")
	}
	newStatus := strings.TrimSpace(req.GetNewStatus())
	if newStatus == "" {
		return nil, status.Error(codes.InvalidArgument, "new_status is required")
	}
	var assignedTo *uuid.UUID
	if req.GetAssignedToProfileId() != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(req.GetAssignedToProfileId()))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid assigned_to_profile_id")
		}
		assignedTo = &parsed
	}
	var resolution *string
	if strings.TrimSpace(req.GetResolutionJson()) != "" {
		v := strings.TrimSpace(req.GetResolutionJson())
		resolution = &v
	}
	setResolved := newStatus == "resolved" || newStatus == "dismissed"
	row, err := s.Reports.UpdateReport(ctx, reportID, newStatus, assignedTo, resolution, setResolved)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "report not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.AuditLog != nil {
		details := `{"report_id":"` + reportID.String() + `","status":"` + newStatus + `"}`
		_ = s.AuditLog.InsertAudit(ctx, modProfile, "report_resolved", "report", reportID, details)
	}
	return &moderationv1.ResolveReportResponse{Report: reportRowToProto(row)}, nil
}

func (s *ModerationGRPC) reportThreshold() int {
	audience := s.PlatformAudienceSize
	if audience <= 0 {
		audience = 1000
	}
	threshold := int(math.Ceil(0.01 * float64(audience)))
	if threshold < 10 {
		threshold = 10
	}
	return threshold
}

func (s *ModerationGRPC) maybeAutoShadowBan(ctx context.Context, targetProfileID uuid.UUID) error {
	count, err := s.Reports.CountReports24h(ctx, targetProfileID)
	if err != nil {
		return err
	}
	threshold := s.reportThreshold()
	if count < threshold {
		return nil
	}
	targetAccountID := targetProfileID
	if s.Users != nil {
		resolved, err := s.Users.AccountIDForProfile(ctx, targetProfileID)
		if err != nil {
			return err
		}
		if resolved != uuid.Nil {
			targetAccountID = resolved
		}
	}
	details := `{"window":"24h","count":` + strconv.Itoa(count) + `,"threshold":` + strconv.Itoa(threshold) + `,"audience_source":"env"}`
	if err := s.Reports.InsertAutoModLog(ctx, targetProfileID, "report_threshold", "shadow_ban", details); err != nil {
		return err
	}
	_ = targetAccountID // app stack4 applies sanctions; app stack1 logs only (PLAN §11).
	return nil
}
