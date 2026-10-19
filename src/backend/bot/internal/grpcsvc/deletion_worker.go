package grpcsvc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
)

// A historical COMPLETE row is revalidated against its frozen typed source on
// every recovery pass; it is not trusted solely because its state bit is set.
func validateCompletedDeletionEffect(claim *store.BotDeletion, effect store.BotDeletionEffect) error {
	var request proto.Message
	switch effect.Kind {
	case "SPACE_CLEANUP":
		if len(effect.ReceiptBytes) == 0 {
			return store.ErrDeletionPending
		}
		q, err := buildSpaceDeletionRequest(claim, effect)
		if err != nil {
			return err
		}
		request = q
		r := new(spacev1.ApplyBotInstallationRemovalResponse)
		if proto.Unmarshal(effect.ReceiptBytes, r) != nil || validateSpaceDeletionReceipt(q, r) != nil {
			return errDeletionEvidence
		}
	case "ROLE_REMOVE":
		q, err := buildRoleDeletionRequest(claim, effect)
		if err != nil {
			return err
		}
		request = q
		r := new(rolev1.ApplyBotInstallationRemovalResponse)
		if proto.Unmarshal(effect.ReceiptBytes, r) != nil || validateRoleDeletionReceipt(q, r) != nil {
			return errDeletionEvidence
		}
	case "MESSAGING_UNPIN":
		q, err := buildMessagingDeletionRequest(claim, effect)
		if err != nil {
			return err
		}
		request = q
		r := new(messagingv1.ApplyBotChatPinRemovalResponse)
		if proto.Unmarshal(effect.ReceiptBytes, r) != nil || validateMessagingDeletionReceipt(q, r) != nil {
			return errDeletionEvidence
		}
	default:
		// Native Empty cancellation alone cannot attest the global terminal fence.
		return store.ErrDeletionPending
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil || !bytes.Equal(body, effect.RequestBytes) {
		return errDeletionEvidence
	}
	return nil
}

func (s *BotGRPC) processDeletionClaim(ctx context.Context, claim *store.BotDeletion) error {
	effects, err := s.Store.BotDeletionEffects(ctx, claim.OperationID)
	if err != nil {
		return err
	}
	var pending error
	for _, effect := range effects {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if effect.State == "COMPLETE" {
			if err := validateCompletedDeletionEffect(claim, effect); err != nil {
				return err
			}
			continue
		}
		switch effect.Kind {
		case "SPACE_CLEANUP":
			err = s.applySpaceDeletion(ctx, claim, effect)
		case "ROLE_REMOVE":
			err = s.applyRoleDeletion(ctx, claim, effect)
		case "MESSAGING_UNPIN":
			err = s.applyMessagingDeletion(ctx, claim, effect)
		default:
			err = store.ErrDeletionPending
		}
		if err != nil {
			if errors.Is(err, store.ErrDeletionClaimLost) {
				return err
			}
			if pending == nil {
				pending = err
			}
		}
	}
	if pending != nil {
		return pending
	}
	return s.Store.CompleteBotDeletion(ctx, claim)
}

func (s *BotGRPC) ProcessNextBotDeletion(ctx context.Context) (bool, error) {
	if s == nil || s.Store == nil || s.Store.Pool == nil {
		return false, status.Error(codes.Unavailable, "Bot deletion store unavailable")
	}
	claim, err := s.Store.ClaimNextBotDeletion(ctx)
	if errors.Is(err, store.ErrDeletionClaimLost) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	workCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	err = s.processDeletionClaim(workCtx, claim)
	cancel()
	if err == nil {
		return true, nil
	}
	// Release only this still-live generation. Cancellation or a lost claim must
	// never erase sealed bytes, receipts, or a successor worker's ownership.
	retryCtx, retryCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer retryCancel()
	if retryErr := s.Store.RetryBotDeletion(retryCtx, claim); retryErr != nil {
		return true, retryErr
	}
	return true, err
}

func (s *BotGRPC) RunBotDeletionRecovery(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		_, err := s.ProcessNextBotDeletion(ctx)
		if err != nil && ctx.Err() == nil && !errors.Is(err, store.ErrDeletionPending) {
			slog.Warn("Bot deletion cleanup remains pending", "code", status.Code(err).String())
		}
	}
}
