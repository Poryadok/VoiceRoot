package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/moderation/internal/store"

	moderationv1 "voice.app/voice/moderation/v1"
)

var allowedSanctionTypes = map[string]struct{}{
	"warning": {}, "temp_ban": {}, "perm_ban": {}, "shadow_ban": {}, "mm_ban": {},
}

// AccountStatusClient syncs platform bans with Auth (optional).
type AccountStatusClient interface {
	SetAccountStatus(ctx context.Context, accountID uuid.UUID, accountStatus, reason string) error
}

func (s *ModerationGRPC) ApplySanction(ctx context.Context, req *moderationv1.ApplySanctionRequest) (*moderationv1.ApplySanctionResponse, error) {
	if s == nil || s.Sanctions == nil {
		return nil, status.Error(codes.FailedPrecondition, "sanction store is not configured")
	}
	modProfile, err := requireInternalModerator(ctx)
	if err != nil {
		return nil, err
	}
	targetAccount, err := uuid.Parse(strings.TrimSpace(req.GetTargetAccountId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid target_account_id")
	}
	sanctionType := strings.TrimSpace(req.GetType())
	if _, ok := allowedSanctionTypes[sanctionType]; !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid sanction type")
	}
	reason := strings.TrimSpace(req.GetReason())
	if reason == "" {
		return nil, status.Error(codes.InvalidArgument, "reason is required")
	}
	var reportID *uuid.UUID
	if req.GetReportId() != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(req.GetReportId()))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid report_id")
		}
		reportID = &parsed
	}
	var expiresAt *time.Time
	if req.GetExpiresAt() != nil {
		t := req.GetExpiresAt().AsTime().UTC()
		expiresAt = &t
	}
	row, err := s.Sanctions.InsertSanction(ctx, targetAccount, sanctionType, reason, reportID, modProfile, expiresAt)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.AuditLog != nil {
		details, _ := json.Marshal(map[string]string{
			"sanction_id": row.ID.String(),
			"type":        sanctionType,
			"reason":      reason,
		})
		_ = s.AuditLog.InsertAudit(ctx, modProfile, "sanction_applied", "account", targetAccount, string(details))
	}
	if s.Auth != nil && (sanctionType == "temp_ban" || sanctionType == "perm_ban") {
		if err := s.Auth.SetAccountStatus(ctx, targetAccount, "suspended", reason); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("auth sync: %v", err))
		}
	}
	if s.Analytics != nil {
		_ = s.Analytics.Publish(ctx, "analytics.moderation.sanction_applied", "moderation", "sanction_applied", map[string]any{
			"sanction_id":       row.ID.String(),
			"target_account_id": targetAccount.String(),
			"type":              sanctionType,
		})
	}
	return &moderationv1.ApplySanctionResponse{Sanction: sanctionRowToProto(row)}, nil
}

func (s *ModerationGRPC) RevokeSanction(ctx context.Context, req *moderationv1.RevokeSanctionRequest) (*moderationv1.RevokeSanctionResponse, error) {
	if s == nil || s.Sanctions == nil {
		return nil, status.Error(codes.FailedPrecondition, "sanction store is not configured")
	}
	modProfile, err := requireInternalModerator(ctx)
	if err != nil {
		return nil, err
	}
	sanctionID, err := uuid.Parse(strings.TrimSpace(req.GetSanctionId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sanction_id")
	}
	before, err := s.Sanctions.GetByID(ctx, sanctionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "sanction not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := s.Sanctions.RevokeSanction(ctx, sanctionID, modProfile); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "sanction not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.AuditLog != nil {
		details, _ := json.Marshal(map[string]string{"sanction_id": sanctionID.String()})
		_ = s.AuditLog.InsertAudit(ctx, modProfile, "sanction_revoked", "account", before.TargetAccountID, string(details))
	}
	if s.Auth != nil && (before.Type == "temp_ban" || before.Type == "perm_ban") {
		if err := s.Auth.SetAccountStatus(ctx, before.TargetAccountID, "active", "sanction revoked"); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("auth sync: %v", err))
		}
	}
	return &moderationv1.RevokeSanctionResponse{}, nil
}

func (s *ModerationGRPC) GetAccountSanctions(ctx context.Context, req *moderationv1.GetAccountSanctionsRequest) (*moderationv1.GetAccountSanctionsResponse, error) {
	if s == nil || s.Sanctions == nil {
		return nil, status.Error(codes.FailedPrecondition, "sanction store is not configured")
	}
	if _, err := requireInternalModerator(ctx); err != nil {
		return nil, err
	}
	accountID, err := uuid.Parse(strings.TrimSpace(req.GetAccountId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid account_id")
	}
	rows, err := s.Sanctions.ListByAccount(ctx, accountID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*moderationv1.Sanction, 0, len(rows))
	for i := range rows {
		out = append(out, sanctionRowToProto(&rows[i]))
	}
	return &moderationv1.GetAccountSanctionsResponse{
		SanctionList: &moderationv1.SanctionList{Sanctions: out},
	}, nil
}

func (s *ModerationGRPC) GetActiveSanction(ctx context.Context, req *moderationv1.GetActiveSanctionRequest) (*moderationv1.GetActiveSanctionResponse, error) {
	if s == nil || s.Sanctions == nil {
		return nil, status.Error(codes.FailedPrecondition, "sanction store is not configured")
	}
	if _, err := requireInternalModerator(ctx); err != nil {
		return nil, err
	}
	accountID, err := uuid.Parse(strings.TrimSpace(req.GetAccountId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid account_id")
	}
	row, err := s.Sanctions.GetActiveSanction(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "no active sanction")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &moderationv1.GetActiveSanctionResponse{Sanction: sanctionRowToProto(row)}, nil
}

func (s *ModerationGRPC) IsShadowBanned(ctx context.Context, req *moderationv1.IsShadowBannedRequest) (*moderationv1.IsShadowBannedResponse, error) {
	if s == nil || s.Sanctions == nil {
		return nil, status.Error(codes.FailedPrecondition, "sanction store is not configured")
	}
	accountID, err := uuid.Parse(strings.TrimSpace(req.GetAccountId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid account_id")
	}
	banned, err := s.Sanctions.IsShadowBanned(ctx, accountID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &moderationv1.IsShadowBannedResponse{ShadowBanned: banned}, nil
}

func requireInternalModerator(ctx context.Context) (uuid.UUID, error) {
	if !isInternalRequest(ctx) {
		return uuid.Nil, status.Error(codes.PermissionDenied, "internal access required")
	}
	return profileIDFromMetadata(ctx)
}

func sanctionRowToProto(row *store.SanctionRow) *moderationv1.Sanction {
	if row == nil {
		return nil
	}
	out := &moderationv1.Sanction{
		Id:                 row.ID.String(),
		TargetAccountId:    row.TargetAccountID.String(),
		Type:               row.Type,
		Reason:             row.Reason,
		IssuedByProfileId:  row.IssuedBy.String(),
		CreatedAt:          timestamppb.New(row.CreatedAt.UTC()),
	}
	if row.ReportID != nil {
		v := row.ReportID.String()
		out.ReportId = &v
	}
	if row.ExpiresAt != nil {
		out.ExpiresAt = timestamppb.New(row.ExpiresAt.UTC())
	}
	if row.RevokedAt != nil {
		out.RevokedAt = timestamppb.New(row.RevokedAt.UTC())
	}
	return out
}
