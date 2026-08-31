package grpcsvc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/moderation/internal/store"

	moderationv1 "voice.app/voice/moderation/v1"
)

const appealSubmissionWindow = 7 * 24 * time.Hour

func (s *ModerationGRPC) SubmitAppeal(ctx context.Context, req *moderationv1.SubmitAppealRequest) (*moderationv1.SubmitAppealResponse, error) {
	if s == nil || s.Appeals == nil || s.Sanctions == nil {
		return nil, status.Error(codes.FailedPrecondition, "appeal store is not configured")
	}
	accountID, err := accountIDFromMetadata(ctx)
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
	sanction, err := s.Sanctions.GetByID(ctx, sanctionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "sanction not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if sanction.TargetAccountID != accountID {
		return nil, status.Error(codes.PermissionDenied, "sanction does not belong to caller")
	}
	if time.Since(sanction.CreatedAt.UTC()) > appealSubmissionWindow {
		return nil, status.Error(codes.FailedPrecondition, "appeal submission window expired")
	}
	if existing, err := s.Appeals.GetBySanctionID(ctx, sanctionID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	} else if existing != nil {
		return nil, status.Error(codes.AlreadyExists, "appeal already submitted for this sanction")
	}
	row, err := s.Appeals.InsertAppeal(ctx, sanctionID, accountID, reason)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, status.Error(codes.AlreadyExists, "appeal already submitted for this sanction")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.DomainEvents != nil {
		_ = s.DomainEvents.PublishAppealSubmitted(ctx, row.ID.String(), sanctionID.String())
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
			if s.Matchmaking != nil && sanction.Type == "mm_ban" {
				_ = s.Matchmaking.RevokePlatformMMBan(ctx, sanction.TargetAccountID)
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

func (s *ModerationGRPC) ListAppeals(ctx context.Context, req *moderationv1.ListAppealsRequest) (*moderationv1.ListAppealsResponse, error) {
	if s == nil || s.Appeals == nil {
		return nil, status.Error(codes.FailedPrecondition, "appeal store is not configured")
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
	page, err := s.Appeals.ListAppealsPage(ctx, strings.TrimSpace(req.GetStatusFilter()), cursor, limit)
	if err != nil {
		if errors.Is(err, store.ErrInvalidAppealListCursor) {
			return nil, status.Error(codes.InvalidArgument, "invalid cursor")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*moderationv1.Appeal, 0, len(page.Rows))
	for i := range page.Rows {
		out = append(out, appealRowToProto(&page.Rows[i]))
	}
	return &moderationv1.ListAppealsResponse{
		AppealList: &moderationv1.AppealList{
			Appeals:    out,
			NextCursor: page.NextCursor,
		},
	}, nil
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
