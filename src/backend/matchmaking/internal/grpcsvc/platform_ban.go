package grpcsvc

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// ApplyPlatformMMBan records a moderation platform MM ban (S2S from Moderation).
func (s *MatchmakingGRPC) ApplyPlatformMMBan(ctx context.Context, req *matchmakingv1.ApplyPlatformMMBanRequest) (*matchmakingv1.ApplyPlatformMMBanResponse, error) {
	if s.Bans == nil {
		return nil, status.Error(codes.Unavailable, "ban unavailable")
	}
	accountID, err := uuid.Parse(strings.TrimSpace(req.GetTargetAccountId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid target_account_id")
	}
	reason := strings.TrimSpace(req.GetReason())
	if reason == "" {
		return nil, status.Error(codes.InvalidArgument, "reason required")
	}
	bannedBy, err := uuid.Parse(strings.TrimSpace(req.GetBannedByProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid banned_by_profile_id")
	}
	var expiresAt *time.Time
	if req.GetExpiresAt() != nil {
		t := req.GetExpiresAt().AsTime().UTC()
		expiresAt = &t
	}
	if err := s.Bans.InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{
		AccountID:         accountID,
		Reason:            reason,
		BannedByProfileID: bannedBy,
		ExpiresAt:         expiresAt,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "platform ban: %v", err)
	}
	return &matchmakingv1.ApplyPlatformMMBanResponse{}, nil
}

// RevokePlatformMMBan lifts a moderation platform MM ban.
func (s *MatchmakingGRPC) RevokePlatformMMBan(ctx context.Context, req *matchmakingv1.RevokePlatformMMBanRequest) (*matchmakingv1.RevokePlatformMMBanResponse, error) {
	if s.Bans == nil {
		return nil, status.Error(codes.Unavailable, "ban unavailable")
	}
	accountID, err := uuid.Parse(strings.TrimSpace(req.GetTargetAccountId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid target_account_id")
	}
	if err := s.Bans.RevokePlatformMMBan(ctx, accountID); err != nil {
		return nil, status.Errorf(codes.Internal, "revoke platform ban: %v", err)
	}
	return &matchmakingv1.RevokePlatformMMBanResponse{}, nil
}
