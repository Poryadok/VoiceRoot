package grpcsvc

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/authctx"
	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/store"
	"voice/backend/bot/internal/webhook"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
)

func (s *BotGRPC) InstallBotInSpace(ctx context.Context, req *botv1.InstallBotInSpaceRequest) (*botv1.InstallBotInSpaceResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profile, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Role != nil {
		resp, err := s.Role.CheckPermission(ctx, &rolev1.CheckPermissionRequest{
			SpaceId:        spaceID.String(),
			ProfileId:      profile.String(),
			PermissionName: "SPACE_MANAGE_BOTS",
		})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !resp.GetAllowed() {
			return nil, status.Error(codes.PermissionDenied, "SPACE_MANAGE_BOTS required")
		}
	}
	chats := make([]uuid.UUID, 0, len(req.GetAllowedChats()))
	for _, ref := range req.GetAllowedChats() {
		cid, err := parseUUID("chat.id", ref.GetId())
		if err != nil {
			return nil, err
		}
		chats = append(chats, cid)
	}
	botRow, err := s.Store.GetBotByID(ctx, botID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	if s.Chat != nil {
		actorID := botRow.ActorProfileID.String()
		chatCtx := s.userCallCtx(ctx)
		for _, cid := range chats {
			if _, err := s.Chat.AddMembers(chatCtx, &chatv1.AddMembersRequest{
				ChatId:     cid.String(),
				ProfileIds: []string{actorID},
			}); err != nil {
				return nil, status.Errorf(codes.FailedPrecondition, "add bot actor to chat: %v", err)
			}
		}
	}
	installID, err := s.Store.InstallInSpace(ctx, botID, spaceID, profile, chats)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &botv1.InstallBotInSpaceResponse{InstallationId: installID.String()}, nil
}

func (s *BotGRPC) UninstallBotFromSpace(ctx context.Context, req *botv1.UninstallBotFromSpaceRequest) (*botv1.UninstallBotFromSpaceResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profile, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Role != nil {
		resp, err := s.Role.CheckPermission(ctx, &rolev1.CheckPermissionRequest{
			SpaceId:        spaceID.String(),
			ProfileId:      profile.String(),
			PermissionName: "SPACE_MANAGE_BOTS",
		})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !resp.GetAllowed() {
			return nil, status.Error(codes.PermissionDenied, "SPACE_MANAGE_BOTS required")
		}
	}
	if err := s.Store.UninstallFromSpace(ctx, botID, spaceID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &botv1.UninstallBotFromSpaceResponse{}, nil
}

func (s *BotGRPC) ListInstalledBots(ctx context.Context, req *botv1.ListInstalledBotsRequest) (*botv1.ListInstalledBotsResponse, error) {
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.Pool.Query(ctx, `
SELECT i.id, b.id, b.owner_account_id, b.name, b.description, b.avatar_url, b.webhook_url,
	b.is_polling_mode, b.scopes::text, b.status, b.actor_profile_id, b.slug, b.created_at, b.updated_at
FROM bot_space_installations i
JOIN bots b ON b.id = i.bot_id
WHERE i.space_id = $1 AND b.status = 'live'`, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer rows.Close()
	var out []*botv1.InstalledBot
	for rows.Next() {
		var installID uuid.UUID
		var row store.BotRow
		if err := rows.Scan(
			&installID, &row.ID, &row.OwnerAccountID, &row.Name, &row.Description, &row.AvatarURL, &row.WebhookURL,
			&row.IsPollingMode, &row.ScopesJSON, &row.Status, &row.ActorProfileID, &row.Slug, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		whitelist, _ := s.Store.Pool.Query(ctx, `SELECT chat_id::text FROM bot_chat_whitelist WHERE bot_id = $1 AND space_id = $2 AND enabled = true`, row.ID, spaceID)
		var refs []*chatv1.ChatRef
		if whitelist != nil {
			for whitelist.Next() {
				var cid string
				_ = whitelist.Scan(&cid)
				refs = append(refs, &chatv1.ChatRef{Id: cid, Type: chatTypePtr(chatv1.ChatType_CHAT_TYPE_CHANNEL)})
			}
			whitelist.Close()
		}
		out = append(out, &botv1.InstalledBot{
			Bot:            botToProto(row),
			InstallationId: installID.String(),
			AllowedChats:   refs,
		})
	}
	return &botv1.ListInstalledBotsResponse{InstalledBots: out}, nil
}

func (s *BotGRPC) ListSlashCommandsForChat(ctx context.Context, req *botv1.ListSlashCommandsForChatRequest) (*botv1.ListSlashCommandsForChatResponse, error) {
	chatID, err := parseUUID("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.Pool.Query(ctx, `
SELECT c.bot_id, c.name, c.description, c.parameters::text, b.name
FROM bot_commands c
JOIN bots b ON b.id = c.bot_id
JOIN bot_chat_whitelist w ON w.bot_id = c.bot_id
WHERE w.chat_id = $1 AND w.enabled = true AND b.status = 'live'
ORDER BY b.name, c.name`, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer rows.Close()
	var commands []*botv1.SlashCommand
	for rows.Next() {
		var botID uuid.UUID
		var name, desc, params, botName string
		if err := rows.Scan(&botID, &name, &desc, &params, &botName); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		cmd := &botv1.SlashCommand{
			BotId:       botID.String(),
			BotName:     botName,
			Name:        name,
			Description: desc,
		}
		if parts := strings.SplitN(strings.TrimSpace(name), " ", 2); len(parts) == 2 {
			group := parts[0]
			cmd.GroupName = &group
			cmd.Name = parts[1]
		}
		var opts []manifestOption
		_ = json.Unmarshal([]byte(params), &opts)
		for _, o := range opts {
			cmd.Options = append(cmd.Options, &botv1.SlashCommandOption{
				Name:         o.Name,
				Type:         o.Type,
				Required:     o.Required,
				Autocomplete: o.Autocomplete,
			})
		}
		commands = append(commands, cmd)
	}
	return &botv1.ListSlashCommandsForChatResponse{Commands: commands}, nil
}

type manifestOption struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Required     bool   `json:"required"`
	Autocomplete bool   `json:"autocomplete"`
}

func (s *BotGRPC) ExecuteSlashInteraction(ctx context.Context, req *botv1.ExecuteSlashInteractionRequest) (*botv1.ExecuteSlashInteractionResponse, error) {
	invoker, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	chatID, err := parseUUID("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	allowed, err := s.Store.IsChatWhitelisted(ctx, botID, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !allowed {
		return nil, status.Error(codes.PermissionDenied, "bot not enabled in chat")
	}
	botRow, err := s.Store.GetBotByID(ctx, botID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	if !store.ScopeAllows(botRow.ScopesJSON, "TEXT_CHAT_SEND_MESSAGES") {
		return nil, status.Error(codes.PermissionDenied, "bot lacks TEXT_CHAT_SEND_MESSAGES")
	}
	cmdName := strings.TrimPrefix(strings.TrimSpace(req.GetCommandName()), "/")
	commands, err := s.Store.ListCommands(ctx, botID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	found := false
	for _, c := range commands {
		if c.Name == cmdName {
			found = true
			break
		}
	}
	if !found {
		return nil, status.Error(codes.NotFound, "unknown command")
	}

	if s.Events != nil {
		_ = s.Events.PublishCommandExecuted(ctx, botID.String(), cmdName, chatID.String())
	}

	token := uuid.NewString()
	var options map[string]any
	_ = json.Unmarshal([]byte(req.GetOptionsJson()), &options)
	payload := webhook.InteractionPayload{
		Type:             "slash_command",
		InteractionToken: token,
		CommandName:      cmdName,
		Options:          options,
		ChatID:           chatID.String(),
		ChatType:         req.GetChat().GetType().String(),
		InvokerProfileID: invoker.String(),
	}
	ch := s.Hub.Register(token)

	eventPayload := map[string]any{
		"interaction_token":  token,
		"command_name":       cmdName,
		"options":            options,
		"chat_id":            chatID.String(),
		"invoker_profile_id": invoker.String(),
	}
	_, _ = s.Store.EnqueueEvent(ctx, botID, "interaction", eventPayload, token)

	if botRow.IsPollingMode || botRow.WebhookURL == nil || strings.TrimSpace(*botRow.WebhookURL) == "" {
		// polling delivery via event queue
	} else {
		url := strings.TrimSpace(*botRow.WebhookURL)
		go func() {
			resp, err := webhook.DeliverPOST(context.Background(), s.HTTPClient, url, botRow.WebhookSecret, payload, dispatch.DefaultTimeout)
			if err != nil {
				_ = s.Store.MarkEventFailed(context.Background(), botID, token)
				if s.Events != nil {
					_ = s.Events.PublishWebhookDelivered(context.Background(), botID.String(), token, false)
					_ = s.Events.PublishWebhookFailed(context.Background(), botID.String(), "interaction", err.Error())
				}
				s.Hub.Complete(token, store.InteractionReply{Err: err})
				return
			}
			if s.Events != nil {
				_ = s.Events.PublishWebhookDelivered(context.Background(), botID.String(), token, true)
			}
			s.Hub.Complete(token, store.InteractionReply{
				Content:   resp.Content,
				Ephemeral: resp.Ephemeral,
				Deferred:  resp.Deferred,
			})
		}()
	}

	reply, ok := s.Hub.Wait(ch, dispatch.DefaultTimeout)
	if !ok || reply.Err == dispatch.ErrTimeout {
		s.Hub.Cancel(token)
		_ = s.Store.MarkEventTimeout(ctx, botID, token)
		code := "bot_timeout"
		msg := "Bot did not respond in time. Try again later."
		return &botv1.ExecuteSlashInteractionResponse{
			InteractionToken: token,
			ErrorCode:        &code,
			ErrorMessage:     &msg,
		}, nil
	}
	if reply.Err != nil {
		s.Hub.Cancel(token)
		_ = s.Store.MarkEventFailed(ctx, botID, token)
		return nil, status.Error(codes.Unavailable, reply.Err.Error())
	}
	if reply.Deferred {
		def := true
		return &botv1.ExecuteSlashInteractionResponse{
			InteractionToken: token,
			Deferred:         def,
		}, nil
	}
	resp, err := s.finishInteraction(ctx, botRow, req.GetChat(), invoker, token, reply.Content, reply.Ephemeral)
	if err != nil {
		s.Hub.Cancel(token)
		return nil, err
	}
	_ = s.Store.MarkInteractionDelivered(ctx, botID, token)
	s.Hub.Cancel(token)
	return resp, nil
}

func (s *BotGRPC) CompleteInteraction(ctx context.Context, req *botv1.CompleteInteractionRequest) (*botv1.CompleteInteractionResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(req.GetInteractionToken())
	reply := store.InteractionReply{
		Content:   strings.TrimSpace(req.GetContent()),
		Ephemeral: req.GetIsEphemeral(),
		Deferred:  req.GetDeferred(),
	}
	if s.Hub.Complete(token, reply) {
		if !reply.Deferred {
			_ = s.Store.MarkInteractionDelivered(ctx, botRow.ID, token)
		}
		return &botv1.CompleteInteractionResponse{}, nil
	}
	if reply.Content != "" && !reply.Deferred && !reply.Ephemeral {
		_, _, ref, lerr := s.lookupInteraction(ctx, botRow.ID, token)
		if lerr != nil {
			return nil, lerr
		}
		if _, err := s.postMessage(ctx, botRow, ref, reply.Content); err != nil {
			return nil, err
		}
		s.Hub.FinishDeferred(token)
		_ = s.Store.MarkInteractionDelivered(ctx, botRow.ID, token)
		return &botv1.CompleteInteractionResponse{}, nil
	}
	return nil, status.Error(codes.NotFound, "unknown interaction token")
}

func (s *BotGRPC) lookupInteraction(ctx context.Context, botID uuid.UUID, token string) (uuid.UUID, uuid.UUID, *chatv1.ChatRef, error) {
	var payload string
	err := s.Store.Pool.QueryRow(ctx, `
SELECT payload::text FROM bot_event_log WHERE bot_id = $1 AND interaction_token = $2 ORDER BY created_at DESC LIMIT 1`,
		botID, token).Scan(&payload)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, status.Error(codes.NotFound, "interaction not found")
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(payload), &m)
	chatID, _ := uuid.Parse(stringValue(m["chat_id"]))
	invoker, _ := uuid.Parse(stringValue(m["invoker_profile_id"]))
	return chatID, invoker, &chatv1.ChatRef{Id: chatID.String(), Type: chatTypePtr(chatv1.ChatType_CHAT_TYPE_CHANNEL)}, nil
}

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (s *BotGRPC) SendBotMessage(ctx context.Context, req *botv1.SendBotMessageRequest) (*botv1.SendBotMessageResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		// allow owner path via bot_id for tests
		bid, perr := parseUUID("bot_id", req.GetBotId())
		if perr != nil {
			return nil, err
		}
		if e := s.ensureOwner(ctx, bid); e != nil {
			return nil, err
		}
		botRow, err = s.Store.GetBotByID(ctx, bid)
		if err != nil {
			return nil, mapStoreErr(err)
		}
	}
	if req.GetInteractionToken() != "" {
		token := strings.TrimSpace(req.GetInteractionToken())
		if s.Hub.IsDeferred(token) {
			_, _, ref, lerr := s.lookupInteraction(ctx, botRow.ID, token)
			if lerr != nil {
				return nil, lerr
			}
			content := strings.TrimSpace(req.GetContent())
			if content == "" {
				return nil, status.Error(codes.InvalidArgument, "content required")
			}
			msg, perr := s.postMessage(ctx, botRow, ref, content)
			if perr != nil {
				return nil, perr
			}
			s.Hub.FinishDeferred(token)
			_ = s.Store.MarkInteractionDelivered(ctx, botRow.ID, token)
			return &botv1.SendBotMessageResponse{Message: msg}, nil
		}
		reply := store.InteractionReply{Content: req.GetContent()}
		if s.Hub.Complete(token, reply) {
			return &botv1.SendBotMessageResponse{}, nil
		}
	}
	chatID, err := parseUUID("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	allowed, err := s.Store.IsChatWhitelisted(ctx, botRow.ID, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !allowed {
		return nil, status.Error(codes.PermissionDenied, "bot not enabled in chat")
	}
	msg, err := s.postMessage(ctx, botRow, req.GetChat(), req.GetContent())
	if err != nil {
		return nil, err
	}
	return &botv1.SendBotMessageResponse{Message: msg}, nil
}

func (s *BotGRPC) lookupInteractionMeta(ctx context.Context, botID uuid.UUID, token string) (uuid.UUID, *chatv1.ChatRef, error) {
	_, invoker, ref, err := s.lookupInteraction(ctx, botID, token)
	return invoker, ref, err
}

func (s *BotGRPC) SendEphemeral(ctx context.Context, req *botv1.SendEphemeralRequest) (*botv1.SendEphemeralResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	target, err := parseUUID("target_profile_id", req.GetTargetProfileId())
	if err != nil {
		return nil, err
	}
	chatID, err := parseUUID("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	allowed, err := s.Store.IsChatWhitelisted(ctx, botRow.ID, chatID)
	if err != nil || !allowed {
		return nil, status.Error(codes.PermissionDenied, "bot not enabled in chat")
	}
	_, err = s.finishInteraction(ctx, botRow, req.GetChat(), target, "", req.GetContent(), true)
	if err != nil {
		return nil, err
	}
	return &botv1.SendEphemeralResponse{}, nil
}

func (s *BotGRPC) EditBotMessage(ctx context.Context, req *botv1.EditBotMessageRequest) (*botv1.EditBotMessageResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	messageID := strings.TrimSpace(req.GetMessageId())
	if messageID == "" {
		return nil, status.Error(codes.InvalidArgument, "message_id required")
	}
	content := strings.TrimSpace(req.GetContent())
	if content == "" {
		return nil, status.Error(codes.InvalidArgument, "content required")
	}
	if s.Messaging == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging client not configured")
	}
	ctx = metadata.AppendToOutgoingContext(ctx,
		authctx.HeaderProfileID, botRow.ActorProfileID.String(),
		authctx.HeaderUserID, botRow.OwnerAccountID.String(),
	)
	resp, err := s.Messaging.EditMessage(ctx, &messagingv1.EditMessageRequest{
		MessageId: messageID,
		Content:   content,
	})
	if err != nil {
		return nil, err
	}
	return &botv1.EditBotMessageResponse{Message: resp.GetMessage()}, nil
}

func (s *BotGRPC) finishInteraction(ctx context.Context, botRow *store.BotRow, chat *chatv1.ChatRef, invoker uuid.UUID, token, content string, ephemeral bool) (*botv1.ExecuteSlashInteractionResponse, error) {
	content = strings.TrimSpace(content)
	if content == "" && !ephemeral {
		return &botv1.ExecuteSlashInteractionResponse{InteractionToken: token}, nil
	}
	if ephemeral {
		// Ephemeral: no persisted message; client renders locally for invoker only.
		return &botv1.ExecuteSlashInteractionResponse{
			InteractionToken: token,
			Content:          &content,
			IsEphemeral:      true,
		}, nil
	}
	msg, err := s.postMessage(ctx, botRow, chat, content)
	if err != nil {
		return nil, err
	}
	return &botv1.ExecuteSlashInteractionResponse{
		InteractionToken: token,
		Content:          &content,
		Message:          msg,
	}, nil
}

func chatTypePtr(t chatv1.ChatType) *chatv1.ChatType {
	v := t
	return &v
}

func (s *BotGRPC) postMessage(ctx context.Context, botRow *store.BotRow, chat *chatv1.ChatRef, content string) (*messagingv1.Message, error) {
	if s.Messaging == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging client not configured")
	}
	ctx = metadata.AppendToOutgoingContext(ctx,
		authctx.HeaderProfileID, botRow.ActorProfileID.String(),
		authctx.HeaderUserID, botRow.OwnerAccountID.String(),
	)
	resp, err := s.Messaging.SendMessage(ctx, &messagingv1.SendMessageRequest{
		Chat:    chat,
		Content: content,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetMessage(), nil
}
