package grpcsvc

import (
	"context"
	"log/slog"
	"time"
)

// ProcessExpiredTempBans revokes expired temp_ban sanctions and restores Auth account status.
func (s *ModerationGRPC) ProcessExpiredTempBans(ctx context.Context, limit int) (int, error) {
	if s == nil || s.Sanctions == nil {
		return 0, nil
	}
	rows, err := s.Sanctions.ListExpiredActiveTempBans(ctx, limit)
	if err != nil {
		return 0, err
	}
	systemIssuer := autoModIssuerProfileID()
	processed := 0
	for i := range rows {
		row := rows[i]
		if s.Auth != nil {
			if err := s.Auth.SetAccountStatus(ctx, row.TargetAccountID, "active", "temp ban expired"); err != nil {
				continue
			}
		}
		if err := s.Sanctions.RevokeSanction(ctx, row.ID, systemIssuer); err != nil {
			continue
		}
		processed++
	}
	return processed, nil
}

// RunTempBanExpirySweeper periodically restores Auth after temp ban expiry.
func RunTempBanExpirySweeper(ctx context.Context, svc *ModerationGRPC, logger *slog.Logger) {
	if svc == nil || svc.Sanctions == nil {
		return
	}
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := svc.ProcessExpiredTempBans(ctx, 50)
			if err != nil && logger != nil {
				logger.Warn("temp ban expiry sweeper failed", slog.Any("error", err))
				continue
			}
			if n > 0 && logger != nil {
				logger.Info("temp ban expiry processed", slog.Int("count", n))
			}
		}
	}
}
