package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/authctx"
	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/store"
	"voice/backend/bot/internal/webhook"

	chatv1 "voice.app/voice/chat/v1"
)

// requireInvokerMembership asks the authoritative Chat service under the
// caller's profile. Chat.ListMembers checks effective membership before reading.
func (s *BotGRPC) requireInvokerMembership(ctx context.Context, chatID, invoker uuid.UUID) error {
	if s.Chat == nil {
		return status.Error(codes.Unavailable, "chat membership authority unavailable")
	}
	chatCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs(authctx.HeaderProfileID, invoker.String()))
	_, err := s.Chat.ListMembers(chatCtx, &chatv1.ListMembersRequest{ChatId: chatID.String()})
	if err == nil {
		return nil
	}
	if code := status.Code(err); code == codes.PermissionDenied || code == codes.NotFound {
		return status.Error(codes.PermissionDenied, "invoker is not a chat member")
	}
	return status.Errorf(codes.Unavailable, "chat membership check failed: %v", err)
}

// ProcessNextSlashInteraction replays one accepted interaction independently
// of the request process or its Hub waiter.
func (s *BotGRPC) ProcessNextSlashInteraction(ctx context.Context) (bool, error) {
	return s.processSlashInteraction(ctx, uuid.Nil)
}

func (s *BotGRPC) processSlashInteraction(ctx context.Context, id uuid.UUID) (bool, error) {
	event, err := s.Store.ClaimSlashInteraction(ctx, id)
	if err != nil || event == nil {
		return false, err
	}
	var payload webhook.InteractionPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		_, markErr := s.Store.FailSlashDelivery(ctx, event.ID, event.Attempts)
		return true, errors.Join(err, markErr)
	}
	payload.Type = "slash_command"
	payload.InteractionToken = event.Token
	chatID, chatErr := uuid.Parse(payload.ChatID)
	invoker, invokerErr := uuid.Parse(payload.InvokerProfileID)
	if chatErr != nil || invokerErr != nil || strings.TrimSpace(payload.CommandName) == "" {
		_, markErr := s.Store.FailSlashDelivery(ctx, event.ID, event.Attempts)
		return true, errors.Join(fmt.Errorf("invalid persisted slash interaction"), markErr)
	}
	bot, err := s.Store.GetBotByID(ctx, event.BotID)
	if err != nil {
		_, retryErr := s.Store.RetrySlashDelivery(ctx, event.ID, event.Attempts)
		return true, errors.Join(err, retryErr)
	}
	allowed, err := s.Store.IsChatWhitelisted(ctx, event.BotID, chatID)
	if err != nil {
		_, retryErr := s.Store.RetrySlashDelivery(ctx, event.ID, event.Attempts)
		return true, errors.Join(err, retryErr)
	}
	if !allowed || bot.Status != "live" || !store.ScopeAllows(bot.ScopesJSON, "TEXT_CHAT_SEND_MESSAGES") ||
		(payload.ChatType == "CHAT_TYPE_DM" && !store.ScopeAllows(bot.ScopesJSON, "DM_SEND")) {
		_, markErr := s.Store.FailSlashDelivery(ctx, event.ID, event.Attempts)
		return true, markErr
	}
	if err := s.requireInvokerMembership(ctx, chatID, invoker); err != nil {
		if status.Code(err) == codes.PermissionDenied {
			_, markErr := s.Store.FailSlashDelivery(ctx, event.ID, event.Attempts)
			return true, markErr
		}
		_, retryErr := s.Store.RetrySlashDelivery(ctx, event.ID, event.Attempts)
		return true, errors.Join(err, retryErr)
	}
	if bot.WebhookURL == nil || strings.TrimSpace(*bot.WebhookURL) == "" || bot.IsPollingMode {
		_, retryErr := s.Store.RetrySlashDelivery(ctx, event.ID, event.Attempts)
		return true, retryErr
	}
	attemptCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	resp, err := webhook.DeliverPOST(attemptCtx, s.HTTPClient, strings.TrimSpace(*bot.WebhookURL), bot.WebhookSecret, payload, dispatch.DefaultTimeout())
	cancel()
	if err != nil {
		var applied bool
		var markErr error
		if webhook.IsPermanentDeliveryError(err) {
			applied, markErr = s.Store.FailSlashDelivery(ctx, event.ID, event.Attempts)
			if applied {
				s.Hub.Complete(event.Token, store.InteractionReply{Err: err})
			}
		} else {
			applied, markErr = s.Store.RetrySlashDelivery(ctx, event.ID, event.Attempts)
		}
		if applied && s.Events != nil {
			_ = s.Events.PublishWebhookDelivered(ctx, event.BotID.String(), event.Token, false)
			_ = s.Events.PublishWebhookFailed(ctx, event.BotID.String(), "interaction", err.Error())
		}
		return true, errors.Join(err, markErr)
	}
	applied, err := s.Store.CompleteSlashDelivery(ctx, event.ID, event.Attempts, resp.Deferred)
	if err != nil {
		return true, err
	}
	if !applied {
		return true, nil
	}
	if s.Events != nil {
		_ = s.Events.PublishWebhookDelivered(ctx, event.BotID.String(), event.Token, true)
	}
	s.Hub.Complete(event.Token, store.InteractionReply{Content: resp.Content, Ephemeral: resp.Ephemeral, Deferred: resp.Deferred})
	s.touchPresence(ctx, event.BotID)
	return true, nil
}

func (s *BotGRPC) RunSlashOutbox(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		for i := 0; i < 16; i++ {
			worked, err := s.ProcessNextSlashInteraction(ctx)
			if err != nil && ctx.Err() == nil {
				slog.Warn("slash outbox delivery failed", "error", err)
			}
			if !worked || ctx.Err() != nil {
				break
			}
		}
	}
}
