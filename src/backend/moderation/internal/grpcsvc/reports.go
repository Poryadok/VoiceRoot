package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/moderation/internal/store"
	"voice/backend/pkg/principal"

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

var allowedReportStatuses = map[string]struct{}{
	"pending":   {},
	"reviewing": {},
	"resolved":  {},
	"dismissed": {},
}

func (s *ModerationGRPC) CreateReport(ctx context.Context, req *moderationv1.CreateReportRequest) (*moderationv1.CreateReportResponse, error) {
	if s == nil || s.Reports == nil {
		return nil, status.Error(codes.FailedPrecondition, "report store is not configured")
	}

	reporterProfileID, reporterAccountID, err := verifiedReportIdentity(ctx, req)
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
	if !json.Valid([]byte(evidenceJSON)) {
		return nil, status.Error(codes.InvalidArgument, "invalid evidence_json")
	}

	check, err := store.AcquireReportPrivacy(ctx, s.ReportPrivacy)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "report intake unavailable")
	}
	subject, epoch, err := store.ResolveReportSubject(ctx, s.ReportAuthority, reporterAccountID, uuid.Nil)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "report intake unavailable")
	}
	p, _ := principal.FromContext(ctx)
	if subject.State != "LIVE" || epoch != uint64(p.SessionEpoch) {
		return nil, status.Error(codes.Unauthenticated, "current reporter required")
	}
	row, err := s.Reports.InsertSubjectReport(ctx, reporterProfileID, subject, check, targetType, targetID, category, description, evidenceJSON)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "report intake unavailable")
	}

	// The committed receipt is independent of target validation and worker
	// progress. Never reread the row here or publish target-dependent effects.
	return &moderationv1.CreateReportResponse{
		Report: &moderationv1.Report{Id: row.ID.String()},
	}, nil
}

func verifiedReportIdentity(ctx context.Context, req *moderationv1.CreateReportRequest) (uuid.UUID, uuid.UUID, error) {
	p, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(req)
	if !ok || err != nil || p.Kind != "delegated_user" || p.Issuer != "gateway" || p.Audience != "moderation" || p.RPC != moderationv1.ModerationService_CreateReport_FullMethodName || p.RequestHash != hash || p.RequestID == "" || p.Subject != p.AccountID || p.SessionEpoch <= 0 {
		return uuid.Nil, uuid.Nil, status.Error(codes.Unauthenticated, "verified report principal required")
	}
	profile, profileErr := uuid.Parse(p.ProfileID)
	account, accountErr := uuid.Parse(p.AccountID)
	if profileErr != nil || accountErr != nil || profile == uuid.Nil || account == uuid.Nil {
		return uuid.Nil, uuid.Nil, status.Error(codes.Unauthenticated, "invalid verified reporter")
	}
	return profile, account, nil
}

func (s *ModerationGRPC) ListReports(ctx context.Context, req *moderationv1.ListReportsRequest) (*moderationv1.ListReportsResponse, error) {
	if s == nil || s.Reports == nil {
		return nil, status.Error(codes.FailedPrecondition, "report store is not configured")
	}
	if !isInternalRequest(ctx) {
		return nil, status.Error(codes.PermissionDenied, "internal access required")
	}

	limit := int32(50)
	cursor := ""
	if req.GetPage() != nil {
		if req.GetPage().GetPageSize() > 0 {
			limit = req.GetPage().GetPageSize()
		}
		cursor = strings.TrimSpace(req.GetPage().GetCursor())
	}
	page, err := s.Reports.ListReportsFilteredPage(ctx, strings.TrimSpace(req.GetStatusFilter()), strings.TrimSpace(req.GetQueueFilter()), cursor, limit)
	if err != nil {
		if errors.Is(err, store.ErrInvalidReportListCursor) {
			return nil, status.Error(codes.InvalidArgument, "invalid cursor")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*moderationv1.Report, 0, len(page.Rows))
	for i := range page.Rows {
		out = append(out, reportRowToProto(&page.Rows[i]))
	}
	return &moderationv1.ListReportsResponse{
		ReportList: &moderationv1.ReportList{
			Reports:    out,
			NextCursor: page.NextCursor,
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

func accountIDFromMetadata(ctx context.Context) (uuid.UUID, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	vals := md.Get("x-voice-user-id")
	if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
		return uuid.Nil, status.Error(codes.Unauthenticated, "missing account")
	}
	id, err := uuid.Parse(strings.TrimSpace(vals[0]))
	if err != nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "invalid account")
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
		Category:          row.Category,
		EvidenceJson:      row.EvidenceJSON,
		Status:            row.Status,
		CreatedAt:         timestamppb.New(row.CreatedAt.UTC()),
	}
	if row.TargetID != nil {
		out.TargetId = row.TargetID.String()
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
	if _, ok := allowedReportStatuses[newStatus]; !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid new_status")
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

// autoModIssuerProfileID is the well-known automod actor for system sanctions.
func autoModIssuerProfileID() uuid.UUID {
	return uuid.MustParse("00000000-0000-4000-8000-000000000001")
}
