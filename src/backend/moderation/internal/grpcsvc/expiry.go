package grpcsvc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ProcessExpiredTempBans revokes expired temp_ban sanctions and restores Auth account status.
func (s *ModerationGRPC) ProcessExpiredTempBans(ctx context.Context, limit int) (int, error) {
	if s == nil || s.Sanctions == nil {
		return 0, nil
	}
	_, syncErr := s.processPendingAccountStatusSync(ctx, limit)
	rows, err := s.Sanctions.ListExpiredActiveTempBans(ctx, limit)
	if err != nil {
		return 0, errors.Join(syncErr, err)
	}
	systemIssuer := autoModIssuerProfileID()
	expired := 0
	for i := range rows {
		row := rows[i]
		didExpire := false
		err := s.Sanctions.WithAccountLock(ctx, row.TargetAccountID, func(tx pgx.Tx) error {
			current, err := s.Sanctions.GetByIDTx(ctx, tx, row.ID)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			if current.TargetAccountID != row.TargetAccountID || current.Type != "temp_ban" || current.RevokedAt != nil || current.ExpiresAt == nil || !current.ExpiresAt.Before(time.Now().UTC()) {
				return nil
			}
			if err := s.Sanctions.RevokeSanctionTx(ctx, tx, row.ID, systemIssuer); err != nil {
				return err
			}
			if s.Auth != nil {
				if err := s.Sanctions.QueueAccountStatusSyncTx(ctx, tx, row.TargetAccountID); err != nil {
					return err
				}
			}
			didExpire = true
			return nil
		})
		if err != nil {
			syncErr = errors.Join(syncErr, err)
			continue
		}
		if !didExpire {
			continue
		}
		expired++
		if s.Auth != nil {
			if err := s.reconcileAccountStatus(ctx, row.TargetAccountID, "temp ban expired"); err != nil {
				syncErr = errors.Join(syncErr, err)
			}
		}
	}
	return expired, syncErr
}

// processPendingAccountStatusSync retries durable work left by Auth or PostgreSQL failures.
func (s *ModerationGRPC) processPendingAccountStatusSync(ctx context.Context, limit int) (int, error) {
	if s == nil || s.Sanctions == nil || s.Auth == nil {
		return 0, nil
	}
	accounts, err := s.Sanctions.ListPendingAccountStatusSync(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	var syncErr error
	for _, accountID := range accounts {
		if err := s.reconcileAccountStatus(ctx, accountID, "moderation sanction status reconciliation"); err != nil {
			syncErr = errors.Join(syncErr, err)
			continue
		}
		processed++
	}
	return processed, syncErr
}

// queueAccountStatusSync commits retry work independently after an ambiguous apply transaction.
func (s *ModerationGRPC) queueAccountStatusSync(ctx context.Context, accountID uuid.UUID) error {
	if s == nil || s.Sanctions == nil || s.Auth == nil {
		return nil
	}
	return s.Sanctions.WithAccountLock(ctx, accountID, func(tx pgx.Tx) error {
		return s.Sanctions.QueueAccountStatusSyncTx(ctx, tx, accountID)
	})
}

// reconcileAccountStatus re-reads authoritative bans under the account lock. The
// sanction mutation and pending intent are already committed before Auth is called.
func (s *ModerationGRPC) reconcileAccountStatus(ctx context.Context, accountID uuid.UUID, reason string) error {
	if s == nil || s.Sanctions == nil || s.Auth == nil {
		return nil
	}
	return s.Sanctions.WithAccountLock(ctx, accountID, func(tx pgx.Tx) error {
		pending, err := s.Sanctions.HasPendingAccountStatusSyncTx(ctx, tx, accountID)
		if err != nil || !pending {
			return err
		}
		effective, err := s.Sanctions.HasEffectiveAccountBanTx(ctx, tx, accountID)
		if err != nil {
			return err
		}
		desired := "active"
		if effective {
			desired = "suspended"
		}
		if err := s.Auth.SetAccountStatus(ctx, accountID, desired, reason); err != nil {
			return err
		}
		return s.Sanctions.CompleteAccountStatusSyncTx(ctx, tx, accountID)
	})
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
