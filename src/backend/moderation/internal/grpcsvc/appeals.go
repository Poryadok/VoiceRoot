package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/moderation/internal/store"

	moderationv1 "voice.app/voice/moderation/v1"
)

func (s *ModerationGRPC) SubmitAppeal(ctx context.Context, req *moderationv1.SubmitAppealRequest) (*moderationv1.SubmitAppealResponse, error) {
	if s == nil || s.Appeals == nil {
		return nil, status.Error(codes.FailedPrecondition, "appeal store is not configured")
	}
	accountID, err := profileIDFromMetadata(ctx)
	if err != nil {
		return nil, err
	}
	sanctionID, err := uuid.Parse(strings.TrimSpace(req.GetSanctionId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid sanction_id")
	}
	reason := strings.TrimSpace(req.GetReason())
	if reason == "" {
		return nil, status.Error(codes.InvalidArgument, "reason is required")
	}
	row, err := s.Appeals.InsertAppeal(ctx, sanctionID, accountID, reason)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &moderationv1.SubmitAppealResponse{Appeal: appealRowToProto(row)}, nil
}

func (s *ModerationGRPC) ReviewAppeal(ctx context.Context, req *moderationv1.ReviewAppealRequest) (*moderationv1.ReviewAppealResponse, error) {
	if s == nil || s.Appeals == nil {
		return nil, status.Error(codes.FailedPrecondition, "appeal store is not configured")
	}
	modProfile, err := requireInternalModerator(ctx)
	if err != nil {
		return nil, err
	}
	appealID, err := uuid.Parse(strings.TrimSpace(req.GetAppealId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid appeal_id")
	}
	statusVal := strings.TrimSpace(req.GetStatus())
	if statusVal != "approved" && statusVal != "denied" {
		return nil, status.Error(codes.InvalidArgument, "invalid appeal status")
	}
	var notes *string
	if req.GetModeratorNote() != "" {
		v := strings.TrimSpace(req.GetModeratorNote())
		notes = &v
	}
	row, err := s.Appeals.ReviewAppeal(ctx, appealID, statusVal, modProfile, notes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "appeal not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if statusVal == "approved" && s.Sanctions != nil {
		sanction, serr := s.Sanctions.GetByID(ctx, row.SanctionID)
		if serr == nil {
			_ = s.Sanctions.RevokeSanction(ctx, sanction.ID, modProfile)
			if s.Auth != nil && (sanction.Type == "temp_ban" || sanction.Type == "perm_ban") {
				_ = s.Auth.SetAccountStatus(ctx, sanction.TargetAccountID, "active", "appeal approved")
			}
		}
	}
	return &moderationv1.ReviewAppealResponse{Appeal: appealRowToProto(row)}, nil
}

func (s *ModerationGRPC) GetAppeal(ctx context.Context, req *moderationv1.GetAppealRequest) (*moderationv1.GetAppealResponse, error) {
	if s == nil || s.Appeals == nil {
		return nil, status.Error(codes.FailedPrecondition, "appeal store is not configured")
	}
	if _, err := requireInternalModerator(ctx); err != nil {
		return nil, err
	}
	appealID, err := uuid.Parse(strings.TrimSpace(req.GetAppealId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid appeal_id")
	}
	row, err := s.Appeals.GetByID(ctx, appealID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "appeal not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &moderationv1.GetAppealResponse{Appeal: appealRowToProto(row)}, nil
}

func appealRowToProto(row *store.AppealRow) *moderationv1.Appeal {
	if row == nil {
		return nil
	}
	out := &moderationv1.Appeal{
		Id:                  row.ID.String(),
		SanctionId:          row.SanctionID.String(),
		AppellantAccountId:  row.AppellantAccountID.String(),
		Reason:              row.Reason,
		Status:              row.Status,
		CreatedAt:           timestamppb.New(row.CreatedAt.UTC()),
	}
	if row.ReviewedBy != nil {
		v := row.ReviewedBy.String()
		out.ReviewedByProfileId = &v
	}
	return out
}
