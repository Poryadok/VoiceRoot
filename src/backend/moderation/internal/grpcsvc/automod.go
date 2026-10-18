package grpcsvc

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	moderationv1 "voice.app/voice/moderation/v1"
)

func isSpamPattern(content string) bool {
	normalized := strings.ToLower(strings.TrimSpace(content))
	if normalized == "" {
		return false
	}
	if strings.Count(normalized, "http://") >= 3 || strings.Count(normalized, "https://") >= 3 {
		return true
	}
	return false
}

func (s *ModerationGRPC) CheckMessage(ctx context.Context, req *moderationv1.CheckMessageRequest) (*moderationv1.CheckMessageResponse, error) {
	if s == nil || s.AutoMod == nil {
		return &moderationv1.CheckMessageResponse{
			CheckResult: &moderationv1.CheckResult{Allowed: true},
		}, nil
	}
	defer s.recordScanBestEffort(ctx)
	content := strings.TrimSpace(req.GetContent())
	if !isSpamPattern(content) {
		return &moderationv1.CheckMessageResponse{
			CheckResult: &moderationv1.CheckResult{Allowed: true},
		}, nil
	}
	senderProfile, err := parseUUIDField("sender_profile_id", req.GetSenderProfileId())
	if err != nil {
		return nil, err
	}
	offenses, err := s.AutoMod.CountSpamOffenses(ctx, senderProfile)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if offenses == 0 {
		_ = s.AutoMod.InsertSpamOffense(ctx, senderProfile, "mute", `{"duration":"1h"}`)
		reason := "spam_mute"
		return &moderationv1.CheckMessageResponse{
			CheckResult: &moderationv1.CheckResult{Allowed: false, BlockReason: &reason},
		}, nil
	}
	_ = s.AutoMod.InsertSpamOffense(ctx, senderProfile, "mute_permanent", `{}`)
	reason := "spam_mute_permanent"
	return &moderationv1.CheckMessageResponse{
		CheckResult: &moderationv1.CheckResult{Allowed: false, BlockReason: &reason},
	}, nil
}

// Metrics run only after the business decision, with a separate cancellation
// budget. A locked counter must neither expire later business queries nor use
// the remaining transport deadline before the computed result can be returned.
func (s *ModerationGRPC) recordScanBestEffort(ctx context.Context) {
	outcome := "acknowledged"
	defer func() {
		metrics := s.ScanMetrics
		if metrics == nil {
			metrics = defaultScanMetrics
		}
		metrics.outcomes.WithLabelValues(outcome).Inc()
	}()
	if ctx.Err() != nil {
		outcome = "skipped_canceled"
		return
	}
	budget := 50 * time.Millisecond
	if deadline, ok := ctx.Deadline(); ok {
		budget = min(budget, time.Until(deadline)/4)
	}
	if budget < time.Millisecond {
		outcome = "skipped_budget"
		return
	}
	metricCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	record := s.scanRecorder
	if record == nil {
		record = s.AutoMod.RecordScan
	}
	if err := record(metricCtx); err != nil {
		outcome = "unconfirmed"
		slog.WarnContext(ctx, "automod scan counter update failed")
	}
}

func (s *ModerationGRPC) GetAutoModStats(ctx context.Context, _ *moderationv1.GetAutoModStatsRequest) (*moderationv1.GetAutoModStatsResponse, error) {
	if s == nil || s.AutoMod == nil {
		return &moderationv1.GetAutoModStatsResponse{
			AutoModStats: &moderationv1.AutoModStats{},
		}, nil
	}
	if _, err := requireInternalModerator(ctx); err != nil {
		return nil, err
	}
	checked, blocked, err := s.AutoMod.Stats(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &moderationv1.GetAutoModStatsResponse{
		AutoModStats: &moderationv1.AutoModStats{
			MessagesChecked: checked,
			Blocked:         blocked,
		},
	}, nil
}

func parseUUIDField(field, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, "invalid "+field)
	}
	return id, nil
}
