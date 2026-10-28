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
	if e := s.requireInvokerMembership(ctx, chatID, invoker); e != nil {
		if status.Code(e) == codes.PermissionDenied {
			_, markErr := s.Store.FailSlashDelivery(ctx, event.ID, event.Attempts)
			return true, errors.Join(e, markErr)
		}
		_, markErr := s.Store.RetrySlashDelivery(ctx, event.ID, event.Attempts)
		return true, errors.Join(e, markErr)
	}
	var reply store.InteractionReply
	var notify bool
	applied, err := s.Store.DeliverSlashInteraction(ctx, event, func(call context.Context, bot *store.BotRow) (store.SlashResult, error) {
		if payload.ChatType == "CHAT_TYPE_DM" && !store.ScopeAllows(bot.ScopesJSON, "DM_SEND") {
			return store.SlashResult{Failed: true}, nil
		}
		if strings.TrimSpace(*bot.WebhookURL) == "" {
			return store.SlashResult{Retry: true}, nil
		}
		if e := s.requireInvokerMembership(call, chatID, invoker); e != nil {
			denied := status.Code(e) == codes.PermissionDenied
			return store.SlashResult{Failed: denied, Retry: !denied}, e
		}
		client := webhook.GuardedClient(s.HTTPClient, func(request context.Context) error { return s.Store.CheckSlashAttempt(request, event) })
		resp, e := webhook.DeliverPOST(call, client, strings.TrimSpace(*bot.WebhookURL), bot.WebhookSecret, payload, dispatch.DefaultTimeout())
		if e != nil {
			permanent := webhook.IsPermanentDeliveryError(e)
			if permanent {
				reply = store.InteractionReply{Err: e}
				notify = true
			}
			return store.SlashResult{Failed: permanent, Retry: !permanent}, e
		}
		reply = store.InteractionReply{Content: resp.Content, Ephemeral: resp.Ephemeral, Deferred: resp.Deferred}
		notify = true
		return store.SlashResult{Deferred: resp.Deferred}, nil
	})
	if !applied {
		return true, err
	}
	if s.Events != nil {
		_ = s.Events.PublishWebhookDelivered(ctx, event.BotID.String(), event.Token, err == nil)
		if err != nil {
			_ = s.Events.PublishWebhookFailed(ctx, event.BotID.String(), "interaction", err.Error())
		}
	}
	if notify {
		s.Hub.Complete(event.Token, store.Session{BotID: event.BotID, Epoch: event.Epoch}, reply)
	}
	if err == nil {
		s.touchPresence(ctx, event.BotID)
	}
	return true, err
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
