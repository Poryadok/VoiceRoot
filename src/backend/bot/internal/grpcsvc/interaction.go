package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/authctx"
	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/installprincipal"
	"voice/backend/bot/internal/s2s"
	"voice/backend/bot/internal/store"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
)

func chatRefIsDM(ref *chatv1.ChatRef) bool {
	return ref != nil && ref.Type != nil && *ref.Type == chatv1.ChatType_CHAT_TYPE_DM
}

func (s *BotGRPC) InstallBotInSpace(ctx context.Context, req *botv1.InstallBotInSpaceRequest) (*botv1.InstallBotInSpaceResponse, error) {
	admin, err := installprincipal.Require(ctx, botv1.BotService_InstallBotInSpace_FullMethodName, req)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "verified administrator required")
	}
	if s.MembershipCoordinator == nil {
		return nil, status.Error(codes.Unavailable, "protected installation coordinator unavailable")
	}
	return s.MembershipCoordinator.Install(ctx, admin, req)
}

func (s *BotGRPC) UninstallBotFromSpace(ctx context.Context, req *botv1.UninstallBotFromSpaceRequest) (*botv1.UninstallBotFromSpaceResponse, error) {
	admin, err := installprincipal.Require(ctx, botv1.BotService_UninstallBotFromSpace_FullMethodName, req)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "verified administrator required")
	}
	if s.MembershipCoordinator == nil {
		return nil, status.Error(codes.Unavailable, "protected installation coordinator unavailable")
	}
	return s.MembershipCoordinator.Uninstall(ctx, admin, req)
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
	type installation struct {
		id  uuid.UUID
		bot store.BotRow
	}
	var installations []installation
	for rows.Next() {
		var installID uuid.UUID
		var row store.BotRow
		if err := rows.Scan(
			&installID, &row.ID, &row.OwnerAccountID, &row.Name, &row.Description, &row.AvatarURL, &row.WebhookURL,
			&row.IsPollingMode, &row.ScopesJSON, &row.Status, &row.ActorProfileID, &row.Slug, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		installations = append(installations, installation{id: installID, bot: row})
	}
	if err := rows.Err(); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	rows.Close()
	var out []*botv1.InstalledBot
	chatRefs := make(map[string]*chatv1.ChatRef)
	for _, installed := range installations {
		row := installed.bot
		whitelist, err := s.Store.Pool.Query(ctx, `SELECT chat_id::text FROM bot_chat_whitelist WHERE bot_id = $1 AND space_id = $2 AND enabled = true`, row.ID, spaceID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		var ids []string
		for whitelist.Next() {
			var cid string
			if err := whitelist.Scan(&cid); err != nil {
				whitelist.Close()
				return nil, status.Error(codes.Internal, err.Error())
			}
			ids = append(ids, cid)
		}
		if err := whitelist.Err(); err != nil {
			whitelist.Close()
			return nil, status.Error(codes.Internal, err.Error())
		}
		whitelist.Close()
		var refs []*chatv1.ChatRef
		for _, cid := range ids {
			ref, ok := chatRefs[cid]
			if !ok {
				ref = s.installedChatRef(ctx, spaceID.String(), cid)
				chatRefs[cid] = ref
			}
			refs = append(refs, ref)
		}
		out = append(out, &botv1.InstalledBot{
			Bot:            botToProto(row),
			InstallationId: installed.id.String(),
			AllowedChats:   refs,
			Online:         s.isBotOnline(ctx, row.ID),
		})
	}
	return &botv1.ListInstalledBotsResponse{InstalledBots: out}, nil
}

// Chat owns its current type. Unavailable or mismatched hydration leaves the
// optional type absent rather than turning every whitelist entry into a channel.
func (s *BotGRPC) installedChatRef(ctx context.Context, spaceID, chatID string) *chatv1.ChatRef {
	ref := &chatv1.ChatRef{Id: chatID}
	if s.Chat == nil {
		return ref
	}
	resp, err := s.Chat.GetChat(s2s.ForwardIncomingMetadata(ctx), &chatv1.GetChatRequest{ChatId: chatID})
	if err != nil {
		return ref
	}
	chat := resp.GetChat()
	if chat.GetId() != chatID || chat.GetSpaceId() != spaceID {
		return ref
	}
	switch chat.GetType() {
	case chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL:
		ref.Type = chatTypePtr(chat.GetType())
	}
	return ref
}

func (s *BotGRPC) checkManageBots(ctx context.Context, spaceID uuid.UUID) (uuid.UUID, error) {
	profile, ok := authctx.ProfileID(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Role != nil {
		resp, err := s.Role.CheckPermission(ctx, &rolev1.CheckPermissionRequest{
			SpaceId:        spaceID.String(),
			ProfileId:      profile.String(),
			PermissionName: "SPACE_MANAGE_BOTS",
		})
		if err != nil {
			return uuid.Nil, status.Error(codes.Internal, err.Error())
		}
		if !resp.GetAllowed() {
			return uuid.Nil, status.Error(codes.PermissionDenied, "SPACE_MANAGE_BOTS required")
		}
	}
	return profile, nil
}

func (s *BotGRPC) ListBotsInChat(ctx context.Context, req *botv1.ListBotsInChatRequest) (*botv1.ListBotsInChatResponse, error) {
	chatID, err := parseUUID("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if _, err := s.checkManageBots(ctx, spaceID); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListBotsInChat(ctx, chatID, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*botv1.ChatBotEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, &botv1.ChatBotEntry{
			Bot:         botToProto(row.BotRow),
			Enabled:     row.Enabled,
			Whitelisted: row.Whitelisted,
		})
	}
	return &botv1.ListBotsInChatResponse{Bots: out}, nil
}

func (s *BotGRPC) SetBotChatEnabled(ctx context.Context, req *botv1.SetBotChatEnabledRequest) (*botv1.SetBotChatEnabledResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	chatID, err := parseUUID("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profile, err := s.checkManageBots(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	if err := s.Store.SetChatEnabled(ctx, botID, chatID, spaceID, profile, req.GetEnabled()); err != nil {
		return nil, mapStoreErr(err)
	}
	return &botv1.SetBotChatEnabledResponse{}, nil
}

func (s *BotGRPC) ListSlashCommandsForChat(ctx context.Context, req *botv1.ListSlashCommandsForChatRequest) (*botv1.ListSlashCommandsForChatResponse, error) {
	invoker, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUID("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	if err := s.requireInvokerMembership(ctx, chatID, invoker); err != nil {
		return nil, err
	}
	rows, err := s.Store.Pool.Query(ctx, `
SELECT c.bot_id, c.name, c.description, c.parameters::text, b.name
FROM bot_commands c
JOIN bots b ON b.id = c.bot_id
JOIN bot_chat_whitelist w ON w.bot_id = c.bot_id
WHERE w.chat_id = $1 AND w.enabled = true AND b.status = 'live'
  AND b.scopes @> '["TEXT_CHAT_SEND_MESSAGES"]'::jsonb
  AND (NOT $2::bool OR b.scopes @> '["DM_SEND"]'::jsonb)
ORDER BY b.name, c.name`, chatID, chatRefIsDM(req.GetChat()))
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
			Online:      s.isBotOnline(ctx, botID),
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
	if err := s.requireInvokerMembership(ctx, chatID, invoker); err != nil {
		return nil, err
	}
	botRow, err := s.Store.GetBotByID(ctx, botID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	if botRow.IsPollingMode && !store.DevPollingEnabled() {
		return nil, status.Error(codes.FailedPrecondition, "development polling is disabled")
	}
	if !botRow.IsPollingMode && (botRow.WebhookURL == nil || strings.TrimSpace(*botRow.WebhookURL) == "") {
		return nil, status.Error(codes.FailedPrecondition, "webhook is not configured")
	}
	if !store.ScopeAllows(botRow.ScopesJSON, "TEXT_CHAT_SEND_MESSAGES") {
		return nil, status.Error(codes.PermissionDenied, "bot lacks TEXT_CHAT_SEND_MESSAGES")
	}
	if chatRefIsDM(req.GetChat()) {
		if err := s.requireScope(botRow, "DM_SEND"); err != nil {
			return nil, err
		}
	}
	if !s.isBotOnline(ctx, botID) {
		code := "bot_unavailable"
		msg := "Bot is unavailable."
		return &botv1.ExecuteSlashInteractionResponse{
			ErrorCode:    &code,
			ErrorMessage: &msg,
		}, nil
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

	token := uuid.NewString()
	var options map[string]any
	_ = json.Unmarshal([]byte(req.GetOptionsJson()), &options)
	ch := s.Hub.Register(token, botRow.Session())

	eventPayload := map[string]any{
		"interaction_token":  token,
		"command_name":       cmdName,
		"options":            options,
		"chat_id":            chatID.String(),
		"chat_type":          req.GetChat().GetType().String(),
		"invoker_profile_id": invoker.String(),
	}
	eventID, err := s.Store.EnqueueEvent(ctx, botRow.Session(), "interaction", eventPayload, token)
	if err != nil {
		s.Hub.Cancel(token)
		return nil, status.Error(codes.Unavailable, "interaction could not be accepted")
	}
	if s.Events != nil {
		_ = s.Events.PublishCommandExecuted(ctx, botID.String(), cmdName, chatID.String())
	}

	if botRow.IsPollingMode || botRow.WebhookURL == nil || strings.TrimSpace(*botRow.WebhookURL) == "" {
		// polling delivery via event queue
	} else {
		go func() {
			_, _ = s.processSlashInteraction(context.Background(), eventID)
		}()
	}

	reply, ok := s.Hub.Wait(ch, dispatch.DefaultTimeout())
	if !ok || reply.Err == dispatch.ErrTimeout {
		s.Hub.Cancel(token)
		// A remote rotation does not notify this process's Hub. Revalidate the
		// durable capability before reporting that the command is still queued.
		if err := s.withInteractionSession(ctx, botRow, token, req.GetChat(), func(context.Context, pgx.Tx, *store.BotRow, *interactionAuthority) error { return nil }); err != nil {
			return nil, err
		}
		code := "bot_timeout"
		msg := "Bot did not respond in time. The accepted command is still being delivered."
		return &botv1.ExecuteSlashInteractionResponse{
			InteractionToken: token,
			ErrorCode:        &code,
			ErrorMessage:     &msg,
		}, nil
	}
	if reply.Err != nil {
		s.Hub.Cancel(token)
		if errors.Is(reply.Err, store.ErrSessionRevoked) {
			return nil, status.Error(codes.NotFound, "interaction not found")
		}
		return nil, status.Error(codes.Unavailable, reply.Err.Error())
	}
	if reply.Deferred {
		if err := s.withInteractionSession(ctx, botRow, token, req.GetChat(), func(call context.Context, tx pgx.Tx, _ *store.BotRow, _ *interactionAuthority) error {
			_, err := tx.Exec(call, `UPDATE bot_event_log SET delivery_status='deferred' WHERE bot_id=$1 AND interaction_token=$2 AND delivery_status IN ('pending','delivered')`, botID, token)
			return err
		}); err != nil {
			return nil, err
		}
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
	err = s.withInteractionSession(ctx, botRow, token, nil, func(call context.Context, tx pgx.Tx, current *store.BotRow, authority *interactionAuthority) error {
		if authority.state == "deferred" {
			if reply.Deferred {
				return nil
			}
			if reply.Ephemeral || reply.Content == "" {
				return status.Error(codes.FailedPrecondition, "deferred interaction requires a public content reply")
			}
			if _, err := s.postInteractionMessage(call, current, authority.chat, token, reply.Content, ""); err != nil {
				return err
			}
			s.Hub.FinishDeferred(token)
		} else if !s.Hub.Complete(token, current.Session(), reply) {
			return status.Error(codes.NotFound, "unknown interaction token")
		}
		state := "delivered"
		if reply.Deferred {
			state = "deferred"
		}
		_, err := tx.Exec(call, `UPDATE bot_event_log SET delivery_status=$3,delivered_at=clock_timestamp(),interaction_completed_at=CASE WHEN $4 THEN clock_timestamp() ELSE interaction_completed_at END WHERE bot_id=$1 AND interaction_token=$2`, botRow.ID, token, state, authority.state == "deferred" && !reply.Deferred)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &botv1.CompleteInteractionResponse{}, nil
}

type interactionAuthority struct {
	state   string
	chat    *chatv1.ChatRef
	invoker uuid.UUID
	space   *uuid.UUID
}

func (s *BotGRPC) authorizeInteraction(ctx context.Context, botRow *store.BotRow, token string, requested *chatv1.ChatRef) (*interactionAuthority, error) {
	if token == "" || botRow == nil || botRow.Status != "live" {
		return nil, status.Error(codes.NotFound, "interaction not found")
	}
	var payload string
	var state string
	var space *uuid.UUID
	err := s.Store.Pool.QueryRow(ctx, `
SELECT payload::text, delivery_status,space_id FROM bot_event_log
WHERE bot_id = $1 AND interaction_token = $2 AND event_type = 'interaction'
	  AND delivery_status IN ('pending', 'deferred', 'delivered') AND session_epoch=$3 AND interaction_completed_at IS NULL
ORDER BY created_at DESC LIMIT 1`,
		botRow.ID, token, botRow.SessionEpoch).Scan(&payload, &state, &space)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "interaction not found")
		}
		return nil, status.Error(codes.Unavailable, "interaction authority unavailable")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return nil, status.Error(codes.NotFound, "interaction not found")
	}
	chatID, chatErr := uuid.Parse(stringValue(m["chat_id"]))
	invoker, invokerErr := uuid.Parse(stringValue(m["invoker_profile_id"]))
	if chatErr != nil || invokerErr != nil {
		return nil, status.Error(codes.NotFound, "interaction not found")
	}
	chatType := persistedInteractionChatType(m["chat_type"])
	ref := &chatv1.ChatRef{Id: chatID.String(), Type: chatTypePtr(chatType)}
	if requested != nil && (requested.GetId() != ref.GetId() || requested.GetType() != ref.GetType()) {
		return nil, status.Error(codes.PermissionDenied, "interaction token is bound to a different chat")
	}
	if err := s.requireScope(botRow, "TEXT_CHAT_SEND_MESSAGES"); err != nil {
		return nil, err
	}
	if chatRefIsDM(ref) {
		if err := s.requireScope(botRow, "DM_SEND"); err != nil {
			return nil, err
		}
	}
	allowed, err := s.Store.IsChatWhitelisted(ctx, botRow.ID, chatID)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "chat whitelist unavailable")
	}
	if !allowed {
		return nil, status.Error(codes.PermissionDenied, "bot not enabled in chat")
	}
	if err := s.requireInvokerMembership(ctx, chatID, invoker); err != nil {
		return nil, err
	}
	return &interactionAuthority{state: state, chat: ref, invoker: invoker, space: space}, nil
}

// persistedInteractionChatType restores the persisted interaction destination.
// Missing or invalid values are from legacy/malformed rows and retain the
// historical channel fallback.
func persistedInteractionChatType(value any) chatv1.ChatType {
	if name, ok := value.(string); ok {
		if raw, found := chatv1.ChatType_value[name]; found {
			chatType := chatv1.ChatType(raw)
			switch chatType {
			case chatv1.ChatType_CHAT_TYPE_DM, chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL:
				return chatType
			}
		}
	}
	return chatv1.ChatType_CHAT_TYPE_CHANNEL
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
		_, hasBotToken := authctx.BotToken(ctx)
		if hasBotToken || strings.TrimSpace(req.GetInteractionToken()) != "" {
			return nil, err
		}
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
	if requestedBotID := strings.TrimSpace(req.GetBotId()); requestedBotID != "" && requestedBotID != botRow.ID.String() {
		return nil, status.Error(codes.PermissionDenied, "bot_id does not match credential")
	}
	if err := s.requireScope(botRow, "TEXT_CHAT_SEND_MESSAGES"); err != nil {
		return nil, err
	}
	if req.GetInteractionToken() != "" {
		token := strings.TrimSpace(req.GetInteractionToken())
		authority, err := s.authorizeInteraction(ctx, botRow, token, req.GetChat())
		if err != nil {
			return nil, err
		}
		if authority.state == "deferred" {
			return s.sendDeferredInteractionMessage(ctx, botRow, token, req.GetChat(), req.GetContent(), req.GetThreadParentId())
		}
		reply := store.InteractionReply{Content: req.GetContent()}
		var completed bool
		err = s.withInteractionSession(ctx, botRow, token, req.GetChat(), func(_ context.Context, _ pgx.Tx, current *store.BotRow, _ *interactionAuthority) error {
			completed = s.Hub.Complete(token, current.Session(), reply)
			return nil
		})
		if err != nil {
			return nil, err
		}
		if completed {
			return &botv1.SendBotMessageResponse{}, nil
		}
		return s.sendDeferredInteractionMessage(ctx, botRow, token, req.GetChat(), req.GetContent(), req.GetThreadParentId())
	}
	chatID, err := parseUUID("chat.id", req.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	if chatRefIsDM(req.GetChat()) {
		if err := s.requireScope(botRow, "DM_SEND"); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.GetInteractionToken()) == "" {
			return nil, status.Error(codes.PermissionDenied, "DM_SEND requires interaction context")
		}
	}
	allowed, err := s.Store.IsChatWhitelisted(ctx, botRow.ID, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !allowed {
		return nil, status.Error(codes.PermissionDenied, "bot not enabled in chat")
	}
	msg, err := s.postMessage(ctx, botRow, req.GetChat(), req.GetContent(), req.GetThreadParentId())
	if err != nil {
		return nil, err
	}
	return &botv1.SendBotMessageResponse{Message: msg}, nil
}

func (s *BotGRPC) sendDeferredInteractionMessage(ctx context.Context, botRow *store.BotRow, token string, requested *chatv1.ChatRef, content, threadParentID string) (*botv1.SendBotMessageResponse, error) {
	msg, err := s.completeDeferredInteraction(ctx, botRow, token, requested, content, threadParentID)
	if err != nil {
		return nil, err
	}
	s.Hub.FinishDeferred(token)
	return &botv1.SendBotMessageResponse{Message: msg}, nil
}

func (s *BotGRPC) completeDeferredInteraction(ctx context.Context, botRow *store.BotRow, token string, requested *chatv1.ChatRef, content, threadParentID string) (*messagingv1.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, status.Error(codes.InvalidArgument, "content required")
	}
	var message *messagingv1.Message
	err := s.withInteractionSession(ctx, botRow, token, requested, func(call context.Context, tx pgx.Tx, current *store.BotRow, authority *interactionAuthority) error {
		if authority.state != "deferred" {
			return status.Error(codes.NotFound, "interaction not deferred")
		}
		var err error
		message, err = s.postInteractionMessage(call, current, authority.chat, token, content, threadParentID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(call, `UPDATE bot_event_log SET delivery_status='delivered',delivered_at=clock_timestamp(),interaction_completed_at=clock_timestamp(),attempts=attempts+1 WHERE bot_id=$1 AND interaction_token=$2 AND delivery_status='deferred'`, botRow.ID, token)
		return err
	})
	if err != nil {
		return nil, err
	}
	return message, nil
}

func (s *BotGRPC) SendEphemeral(ctx context.Context, req *botv1.SendEphemeralRequest) (*botv1.SendEphemeralResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireScope(botRow, "TEXT_CHAT_SEND_MESSAGES"); err != nil {
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
	if err := s.requireInvokerMembership(ctx, chatID, target); err != nil {
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
	return runtimeCall(ctx, s, botRow, func(call context.Context, current *store.BotRow) (*botv1.EditBotMessageResponse, error) {
		return s.editBotMessageRuntime(call, current, req)
	})
}

func (s *BotGRPC) editBotMessageRuntime(ctx context.Context, botRow *store.BotRow, req *botv1.EditBotMessageRequest) (*botv1.EditBotMessageResponse, error) {
	if err := s.requireScope(botRow, "TEXT_CHAT_SEND_MESSAGES"); err != nil {
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
	var result *botv1.ExecuteSlashInteractionResponse
	effect := func(call context.Context, tx pgx.Tx, current *store.BotRow) error {
		if err := s.requireScope(current, "TEXT_CHAT_SEND_MESSAGES"); err != nil {
			return err
		}
		if chatRefIsDM(chat) {
			if err := s.requireScope(current, "DM_SEND"); err != nil {
				return err
			}
		}
		var err error
		result, err = s.finishInteractionResult(call, current, chat, invoker, token, content, ephemeral)
		if err != nil {
			return err
		}
		if token != "" {
			_, err = tx.Exec(call, `UPDATE bot_event_log SET delivery_status='delivered',delivered_at=clock_timestamp(),interaction_completed_at=clock_timestamp() WHERE bot_id=$1 AND interaction_token=$2`, botRow.ID, token)
		}
		return err
	}
	var err error
	if token != "" {
		err = s.withInteractionSession(ctx, botRow, token, chat, func(call context.Context, tx pgx.Tx, current *store.BotRow, _ *interactionAuthority) error {
			return effect(call, tx, current)
		})
	} else {
		id, e := parseUUID("chat.id", chat.GetId())
		if e != nil {
			return nil, e
		}
		err = s.withBotSession(ctx, botRow, id, effect)
	}
	return result, err
}

func (s *BotGRPC) finishInteractionResult(ctx context.Context, botRow *store.BotRow, chat *chatv1.ChatRef, invoker uuid.UUID, token, content string, ephemeral bool) (*botv1.ExecuteSlashInteractionResponse, error) {
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
	msg, err := s.postMessage(ctx, botRow, chat, content, "")
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

func (s *BotGRPC) postMessage(ctx context.Context, botRow *store.BotRow, chat *chatv1.ChatRef, content, threadParentID string) (*messagingv1.Message, error) {
	if !chatRefIsDM(chat) {
		req := &messagingv1.SendMessageRequest{Chat: chat, Content: content}
		if threadParentID != "" {
			req.ThreadParentId = &threadParentID
		}
		return s.postSpaceBotMessage(ctx, botRow, req)
	}
	if s.Messaging == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging client not configured")
	}
	ctx = metadata.AppendToOutgoingContext(ctx,
		authctx.HeaderProfileID, botRow.ActorProfileID.String(),
		authctx.HeaderUserID, botRow.OwnerAccountID.String(),
	)
	req := &messagingv1.SendMessageRequest{Chat: chat, Content: content}
	if threadParentID != "" {
		req.ThreadParentId = &threadParentID
	}
	resp, err := s.Messaging.SendMessage(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetMessage(), nil
}

func (s *BotGRPC) postInteractionMessage(ctx context.Context, botRow *store.BotRow, chat *chatv1.ChatRef, token, content, threadParentID string) (*messagingv1.Message, error) {
	clientMessageID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("bot-interaction:"+botRow.ID.String()+":"+token)).String()
	if !chatRefIsDM(chat) {
		req := &messagingv1.SendMessageRequest{Chat: chat, Content: content, ClientMessageId: &clientMessageID}
		if threadParentID != "" {
			req.ThreadParentId = &threadParentID
		}
		return s.postSpaceBotMessage(ctx, botRow, req)
	}
	if s.Messaging == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging client not configured")
	}
	ctx = metadata.AppendToOutgoingContext(ctx,
		authctx.HeaderProfileID, botRow.ActorProfileID.String(),
		authctx.HeaderUserID, botRow.OwnerAccountID.String(),
	)
	req := &messagingv1.SendMessageRequest{
		Chat:            chat,
		Content:         content,
		ClientMessageId: &clientMessageID,
	}
	if threadParentID != "" {
		req.ThreadParentId = &threadParentID
	}
	resp, err := s.Messaging.SendMessage(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.GetMessage(), nil
}
