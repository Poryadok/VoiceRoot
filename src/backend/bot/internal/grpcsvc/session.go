package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/bot/internal/authctx"
	"voice/backend/bot/internal/store"
)

func sessionError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrSessionRevoked) {
		return status.Error(codes.Unauthenticated, "Bot session revoked")
	}
	if errors.Is(err, store.ErrMessageSendDenied) {
		return status.Error(codes.PermissionDenied, "Bot authority denied")
	}
	if status.Code(err) != codes.Unknown {
		return err
	}
	return status.Error(codes.Unavailable, "Bot authority unavailable")
}

func requestSessionError(ctx context.Context, err error) error {
	if errors.Is(err, store.ErrSessionRevoked) {
		if _, botToken := authctx.BotToken(ctx); !botToken {
			return status.Error(codes.NotFound, "Bot interaction not found")
		}
	}
	return sessionError(err)
}

func (s *BotGRPC) withBotSession(ctx context.Context, bot *store.BotRow, chat uuid.UUID, effect func(context.Context, pgx.Tx, *store.BotRow) error) error {
	return requestSessionError(ctx, s.Store.WithSession(ctx, bot.Authority(), chat, func(call context.Context, tx pgx.Tx, current *store.BotRow) error {
		if s.Hub != nil {
			s.Hub.PurgeBot(current.ID, current.SessionEpoch)
		}
		return effect(call, tx, current)
	}))
}

func runtimeCall[T any](ctx context.Context, s *BotGRPC, bot *store.BotRow, effect func(context.Context, *store.BotRow) (T, error)) (T, error) {
	return runtimeScopedCall(ctx, s, bot, uuid.Nil, nil, effect)
}

func runtimeSpaceCall[T any](ctx context.Context, s *BotGRPC, bot *store.BotRow, spaceID string, effect func(context.Context, *store.BotRow) (T, error)) (T, error) {
	space, err := parseUUID("space_id", spaceID)
	if err != nil {
		var result T
		return result, err
	}
	return runtimeScopedCall(ctx, s, bot, uuid.Nil, []uuid.UUID{space}, effect)
}

func runtimeChatCall[T any](ctx context.Context, s *BotGRPC, bot *store.BotRow, chatID string, effect func(context.Context, *store.BotRow) (T, error)) (T, error) {
	chat, err := parseUUID("chat.id", chatID)
	if err != nil {
		var result T
		return result, err
	}
	return runtimeScopedCall(ctx, s, bot, chat, nil, effect)
}

func runtimeScopedCall[T any](ctx context.Context, s *BotGRPC, bot *store.BotRow, chat uuid.UUID, spaces []uuid.UUID, effect func(context.Context, *store.BotRow) (T, error)) (T, error) {
	var result T
	err := s.Store.WithSessionSpaces(ctx, bot.Authority(), chat, spaces, func(call context.Context, _ pgx.Tx, current *store.BotRow) error {
		if s.Hub != nil {
			s.Hub.PurgeBot(current.ID, current.SessionEpoch)
		}
		var err error
		result, err = effect(call, current)
		return err
	})
	return result, requestSessionError(ctx, err)
}

// An early membership check is only discovery. Final event and epoch authority
// are locked together before notification or the actual external sink.
func (s *BotGRPC) withInteractionSession(ctx context.Context, bot *store.BotRow, token string, requested *chatv1.ChatRef, effect func(context.Context, pgx.Tx, *store.BotRow, *interactionAuthority) error) error {
	permission, err := s.authorizeInteraction(ctx, bot, token, requested)
	if err != nil {
		return err
	}
	chat, err := uuid.Parse(permission.chat.GetId())
	if err != nil {
		return status.Error(codes.NotFound, "interaction not found")
	}
	if permission.space == nil || *permission.space == uuid.Nil {
		return status.Error(codes.NotFound, "interaction not found")
	}
	err = s.Store.WithSessionSpaces(ctx, bot.Authority(), chat, []uuid.UUID{*permission.space}, func(call context.Context, tx pgx.Tx, current *store.BotRow) error {
		if s.Hub != nil {
			s.Hub.PurgeBot(current.ID, current.SessionEpoch)
		}
		var state, payload string
		var epoch *int64
		var space *uuid.UUID
		err := tx.QueryRow(call, `SELECT delivery_status,payload::text,session_epoch,space_id FROM bot_event_log
WHERE bot_id=$1 AND interaction_token=$2 AND event_type='interaction'
AND delivery_status IN ('pending','deferred','delivered') AND interaction_completed_at IS NULL FOR UPDATE`, bot.ID, token).Scan(&state, &payload, &epoch, &space)
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "interaction not found")
		}
		if err != nil {
			return err
		}
		if epoch == nil || *epoch != bot.SessionEpoch {
			return status.Error(codes.NotFound, "interaction not found")
		}
		guardedSpace, held := store.GuardedChatSpace(call)
		if !held || space == nil || *space != *permission.space || guardedSpace != *space {
			return status.Error(codes.NotFound, "interaction not found")
		}
		var binding map[string]any
		if json.Unmarshal([]byte(payload), &binding) != nil || stringValue(binding["chat_id"]) != permission.chat.GetId() || stringValue(binding["invoker_profile_id"]) != permission.invoker.String() || persistedInteractionChatType(binding["chat_type"]) != permission.chat.GetType() {
			return status.Error(codes.NotFound, "interaction not found")
		}
		if err = s.requireScope(current, "TEXT_CHAT_SEND_MESSAGES"); err != nil {
			return err
		}
		if chatRefIsDM(permission.chat) {
			if err = s.requireScope(current, "DM_SEND"); err != nil {
				return err
			}
		}
		if err = s.requireInvokerMembership(call, chat, permission.invoker); err != nil {
			return err
		}
		permission.state = state
		return effect(call, tx, current, permission)
	})
	return requestSessionError(ctx, err)
}

func (s *BotGRPC) withAutocompleteSession(ctx context.Context, bot *store.BotRow, requestID string, effect func(context.Context, pgx.Tx, *store.BotRow, map[string]any) error) error {
	var payload string
	var space *uuid.UUID
	err := s.Store.Pool.QueryRow(ctx, `SELECT payload::text,space_id FROM bot_event_log WHERE bot_id=$1 AND interaction_token=$2 AND event_type='autocomplete' AND session_epoch=$3 AND delivery_status IN ('pending','delivered')`, bot.ID, requestID, bot.SessionEpoch).Scan(&payload, &space)
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, "autocomplete not found")
	}
	if err != nil {
		return requestSessionError(ctx, err)
	}
	var binding map[string]any
	if json.Unmarshal([]byte(payload), &binding) != nil || stringValue(binding["request_id"]) != requestID || space == nil || *space == uuid.Nil {
		return status.Error(codes.NotFound, "autocomplete not found")
	}
	chat, e := uuid.Parse(stringValue(binding["chat_id"]))
	if e != nil || chat == uuid.Nil {
		return status.Error(codes.NotFound, "autocomplete not found")
	}
	invoker, e := uuid.Parse(stringValue(binding["invoker_profile_id"]))
	if e != nil || invoker == uuid.Nil {
		return status.Error(codes.NotFound, "autocomplete not found")
	}
	err = s.Store.WithSessionSpaces(ctx, bot.Authority(), chat, []uuid.UUID{*space}, func(call context.Context, tx pgx.Tx, current *store.BotRow) error {
		var currentPayload string
		var currentSpace *uuid.UUID
		var epoch *int64
		e := tx.QueryRow(call, `SELECT payload::text,space_id,session_epoch FROM bot_event_log WHERE bot_id=$1 AND interaction_token=$2 AND event_type='autocomplete' AND delivery_status IN ('pending','delivered') FOR UPDATE`, bot.ID, requestID).Scan(&currentPayload, &currentSpace, &epoch)
		if errors.Is(e, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "autocomplete not found")
		}
		if e != nil {
			return e
		}
		actual, held := store.GuardedChatSpace(call)
		if epoch == nil || *epoch != bot.SessionEpoch || currentSpace == nil || *currentSpace != *space || !held || actual != *space || currentPayload != payload {
			return status.Error(codes.NotFound, "autocomplete not found")
		}
		if e = s.requireScope(current, "TEXT_CHAT_SEND_MESSAGES"); e != nil {
			return e
		}
		if persistedInteractionChatType(binding["chat_type"]) == chatv1.ChatType_CHAT_TYPE_DM {
			if e = s.requireScope(current, "DM_SEND"); e != nil {
				return e
			}
		}
		if e = s.requireInvokerMembership(call, chat, invoker); e != nil {
			return e
		}
		if s.Hub != nil {
			s.Hub.PurgeBot(current.ID, current.SessionEpoch)
		}
		return effect(call, tx, current, binding)
	})
	return requestSessionError(ctx, err)
}
