package grpcsvc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/bot/internal/authctx"
	"voice/backend/bot/internal/dispatch"
	"voice/backend/bot/internal/manifest"
	"voice/backend/bot/internal/store"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	userv1 "voice.app/voice/user/v1"
	spacev1 "voice.app/voice/space/v1"
)

// BotGRPC implements voice.bot.v1.BotService.
type BotGRPC struct {
	botv1.UnimplementedBotServiceServer
	Store      *store.BotStore
	Hub        *dispatch.Hub
	HTTPClient *http.Client
	Chat       chatv1.ChatServiceClient
	Messaging  messagingv1.MessagingServiceClient
	Role       rolev1.RoleServiceClient
	User       userv1.UserServiceClient
	Space      spacev1.SpaceServiceClient
	Events     BotEventPublisher
}

// BotEventPublisher publishes bot lifecycle events (optional NATS).
type BotEventPublisher interface {
	PublishBotRegistered(ctx context.Context, botID, ownerID string) error
	PublishCommandExecuted(ctx context.Context, botID, command, chatID string) error
	PublishWebhookDelivered(ctx context.Context, botID, deliveryID string, success bool) error
	PublishWebhookFailed(ctx context.Context, botID, eventType, errMsg string) error
}

func NewBotGRPC(st *store.BotStore, hub *dispatch.Hub) *BotGRPC {
	return &BotGRPC{
		Store: st,
		Hub:   hub,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (s *BotGRPC) RegisterBot(ctx context.Context, req *botv1.RegisterBotRequest) (*botv1.RegisterBotResponse, error) {
	owner, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing account")
	}
	scopes := strings.TrimSpace(req.GetScopesJson())
	if scopes == "" {
		scopes = `["TEXT_CHAT_SEND_MESSAGES"]`
	}
	actorID, actorErr := s.provisionActorProfile(ctx, req.GetName())
	if actorErr != nil {
		return nil, actorErr
	}
	row, plain, err := s.Store.CreateBot(ctx, owner, req.GetName(), req.GetDescription(), scopes, actorID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &botv1.RegisterBotResponse{
		Bot: botToProto(row),
		TokenResponse: &botv1.TokenResponse{Token: plain},
	}
	if s.Events != nil {
		_ = s.Events.PublishBotRegistered(ctx, row.ID.String(), owner.String())
	}
	return resp, nil
}

func (s *BotGRPC) UpdateBot(ctx context.Context, req *botv1.UpdateBotRequest) (*botv1.UpdateBotResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwner(ctx, botID); err != nil {
		return nil, err
	}
	// minimal update: name/description only via SQL inline
	row, err := s.Store.GetBotByID(ctx, botID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	if req.Name != nil {
		row.Name = strings.TrimSpace(req.GetName())
	}
	if req.Description != nil {
		row.Description = strings.TrimSpace(req.GetDescription())
	}
	_, err = s.Store.Pool.Exec(ctx, `UPDATE bots SET name = $2, description = $3, updated_at = now() WHERE id = $1`,
		botID, row.Name, row.Description)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	updated, err := s.Store.GetBotByID(ctx, botID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return &botv1.UpdateBotResponse{Bot: botToProto(*updated)}, nil
}

func (s *BotGRPC) DeleteBot(ctx context.Context, req *botv1.DeleteBotRequest) (*botv1.DeleteBotResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwner(ctx, botID); err != nil {
		return nil, err
	}
	_, err = s.Store.Pool.Exec(ctx, `UPDATE bots SET status = 'disabled', updated_at = now() WHERE id = $1`, botID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &botv1.DeleteBotResponse{}, nil
}

func (s *BotGRPC) GetBot(ctx context.Context, req *botv1.GetBotRequest) (*botv1.GetBotResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwner(ctx, botID); err != nil {
		return nil, err
	}
	row, err := s.Store.GetBotByID(ctx, botID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return &botv1.GetBotResponse{Bot: botToProto(*row)}, nil
}

func (s *BotGRPC) ListBots(ctx context.Context, _ *botv1.ListBotsRequest) (*botv1.ListBotsResponse, error) {
	owner, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing account")
	}
	rows, err := s.Store.ListBotsByOwner(ctx, owner)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	list := make([]*botv1.Bot, 0, len(rows))
	for _, r := range rows {
		b := botToProto(r)
		list = append(list, b)
	}
	return &botv1.ListBotsResponse{BotList: &botv1.BotList{Bots: list}}, nil
}

func (s *BotGRPC) RegenerateToken(ctx context.Context, req *botv1.RegenerateTokenRequest) (*botv1.RegenerateTokenResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwner(ctx, botID); err != nil {
		return nil, err
	}
	plain, err := s.Store.RegenerateToken(ctx, botID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return &botv1.RegenerateTokenResponse{TokenResponse: &botv1.TokenResponse{Token: plain}}, nil
}

func (s *BotGRPC) RegisterCommands(ctx context.Context, req *botv1.RegisterCommandsRequest) (*botv1.RegisterCommandsResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwner(ctx, botID); err != nil {
		return nil, err
	}
	var commands []manifest.Command
	if err := json.Unmarshal([]byte(req.GetCommandsJson()), &commands); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid commands_json")
	}
	if errs := manifest.ValidateCommands(commands); len(errs) > 0 {
		return nil, status.Error(codes.InvalidArgument, strings.Join(errs, "; "))
	}
	_, err = s.Store.Pool.Exec(ctx, `DELETE FROM bot_commands WHERE bot_id = $1`, botID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	for _, cmd := range manifest.FlattenCommands(commands) {
		params, _ := json.Marshal(cmd.Options)
		storeName := cmd.Name
		if cmd.GroupName != "" {
			storeName = cmd.GroupName + " " + cmd.Name
		}
		_, err = s.Store.Pool.Exec(ctx, `
INSERT INTO bot_commands (id, bot_id, name, description, parameters)
VALUES ($1, $2, $3, $4, $5::jsonb)`,
			uuid.New(), botID, storeName, cmd.Description, string(params))
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &botv1.RegisterCommandsResponse{}, nil
}

func (s *BotGRPC) GetCommands(ctx context.Context, req *botv1.GetCommandsRequest) (*botv1.GetCommandsResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	commands, err := s.Store.ListCommands(ctx, botID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &botv1.GetCommandsResponse{CommandList: &botv1.CommandList{CommandsJson: store.FormatCommandListJSON(commands)}}, nil
}

func (s *BotGRPC) ValidateManifest(_ context.Context, req *botv1.ValidateManifestRequest) (*botv1.ValidateManifestResponse, error) {
	doc, errs, err := manifest.ParseYAML(req.GetManifestYaml())
	if err != nil && len(errs) == 0 {
		return &botv1.ValidateManifestResponse{Valid: false, Errors: []string{err.Error()}}, nil
	}
	if len(errs) > 0 {
		return &botv1.ValidateManifestResponse{Valid: false, Errors: errs}, nil
	}
	norm, _ := manifest.ToJSON(doc)
	return &botv1.ValidateManifestResponse{Valid: true, NormalizedManifestJson: &norm}, nil
}

func (s *BotGRPC) ApplyManifest(ctx context.Context, req *botv1.ApplyManifestRequest) (*botv1.ApplyManifestResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwner(ctx, botID); err != nil {
		return nil, err
	}
	doc, errs, err := manifest.ParseYAML(req.GetManifestYaml())
	if err != nil || len(errs) > 0 {
		msg := "manifest invalid"
		if len(errs) > 0 {
			msg = strings.Join(errs, "; ")
		} else if err != nil {
			msg = err.Error()
		}
		return nil, status.Error(codes.InvalidArgument, msg)
	}
	if err := s.Store.ApplyManifest(ctx, botID, doc); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	row, err := s.Store.GetBotByID(ctx, botID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	return &botv1.ApplyManifestResponse{Bot: botToProto(*row)}, nil
}

func (s *BotGRPC) SetWebhookURL(ctx context.Context, req *botv1.SetWebhookURLRequest) (*botv1.SetWebhookURLResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwner(ctx, botID); err != nil {
		return nil, err
	}
	url := strings.TrimSpace(req.GetUrl())
	polling := url == ""
	_, err = s.Store.Pool.Exec(ctx, `UPDATE bots SET webhook_url = NULLIF($2,''), is_polling_mode = $3, updated_at = now() WHERE id = $1`,
		botID, url, polling)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &botv1.SetWebhookURLResponse{}, nil
}

func (s *BotGRPC) GetWebhookURL(ctx context.Context, req *botv1.GetWebhookURLRequest) (*botv1.GetWebhookURLResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwner(ctx, botID); err != nil {
		return nil, err
	}
	row, err := s.Store.GetBotByID(ctx, botID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	resp := &botv1.GetWebhookURLResponse{}
	if row.WebhookURL != nil {
		resp.Url = row.WebhookURL
	}
	return resp, nil
}

func (s *BotGRPC) SetChatWhitelist(ctx context.Context, req *botv1.SetChatWhitelistRequest) (*botv1.SetChatWhitelistResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	if err := s.ensureOwner(ctx, botID); err != nil {
		return nil, err
	}
	profile, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	spaceID, err := s.Store.PrimaryInstalledSpace(ctx, botID)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "install bot in a space before setting chat whitelist")
	}
	chats := make([]uuid.UUID, 0, len(req.GetAllowedChats()))
	for _, ref := range req.GetAllowedChats() {
		cid, err := parseUUID("chat.id", ref.GetId())
		if err != nil {
			return nil, err
		}
		chats = append(chats, cid)
	}
	_, err = s.Store.InstallInSpace(ctx, botID, spaceID, profile, chats)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &botv1.SetChatWhitelistResponse{}, nil
}

func (s *BotGRPC) GetChatWhitelist(ctx context.Context, req *botv1.GetChatWhitelistRequest) (*botv1.GetChatWhitelistResponse, error) {
	botID, err := parseUUID("bot_id", req.GetBotId())
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.Pool.Query(ctx, `
SELECT chat_id::text FROM bot_chat_whitelist WHERE bot_id = $1 AND enabled = true`, botID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer rows.Close()
	var refs []*chatv1.ChatRef
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		refs = append(refs, &chatv1.ChatRef{Id: id, Type: chatTypePtr(chatv1.ChatType_CHAT_TYPE_CHANNEL)})
	}
	return &botv1.GetChatWhitelistResponse{AllowedChats: refs}, nil
}

func (s *BotGRPC) DeferResponse(ctx context.Context, req *botv1.DeferResponseRequest) (*botv1.DeferResponseResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(req.GetInteractionToken())
	s.Hub.Complete(token, store.InteractionReply{Deferred: true})
	_ = s.Store.MarkEventDeferred(ctx, botRow.ID, token)
	return &botv1.DeferResponseResponse{}, nil
}

func (s *BotGRPC) PollEvents(req *botv1.PollEventsRequest, stream botv1.BotService_PollEventsServer) error {
	ctx := stream.Context()
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return err
	}
	s.touchPresence(ctx, botRow.ID)
	ids, types, payloads, err := s.Store.ListPendingEvents(ctx, botRow.ID, 25)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	for i, id := range ids {
		evt := &botv1.BotEvent{
			EventId:     id.String(),
			EventType:   types[i],
			PayloadJson: payloads[i],
			CreatedAt:   timestamppb.Now(),
		}
		if err := stream.Send(&botv1.PollEventsResponse{BotEvent: evt}); err != nil {
			return err
		}
		_ = s.Store.MarkEventDelivered(ctx, id)
	}
	return nil
}

func botToProto(row store.BotRow) *botv1.Bot {
	b := &botv1.Bot{
		Id:              row.ID.String(),
		OwnerAccountId:  row.OwnerAccountID.String(),
		Name:            row.Name,
		Description:     row.Description,
		IsPollingMode:   row.IsPollingMode,
		ScopesJson:      row.ScopesJSON,
		Status:          row.Status,
		CreatedAt:       timestamppb.New(row.CreatedAt),
		StatusEnum:      botv1.BotLifecycleStatus_BOT_LIFECYCLE_STATUS_LIVE.Enum(),
	}
	if row.AvatarURL != nil {
		b.AvatarUrl = row.AvatarURL
	}
	if row.WebhookURL != nil {
		b.WebhookUrl = row.WebhookURL
	}
	if row.ActorProfileID != uuid.Nil {
		id := row.ActorProfileID.String()
		b.ActorProfileId = &id
	}
	return b
}

func (s *BotGRPC) ensureOwner(ctx context.Context, botID uuid.UUID) error {
	owner, ok := authctx.AccountID(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing account")
	}
	row, err := s.Store.GetBotByID(ctx, botID)
	if err != nil {
		return mapStoreErr(err)
	}
	if row.OwnerAccountID != owner {
		return status.Error(codes.PermissionDenied, "not bot owner")
	}
	return nil
}

func (s *BotGRPC) botFromToken(ctx context.Context) (*store.BotRow, error) {
	token, ok := authctx.BotToken(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing bot token")
	}
	row, err := s.Store.GetBotByTokenHash(ctx, store.HashToken(token))
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid bot token")
	}
	return row, nil
}

func parseUUID(field, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, field+" invalid")
	}
	return id, nil
}

func (s *BotGRPC) userCallCtx(ctx context.Context) context.Context {
	account, ok := authctx.AccountID(ctx)
	if !ok {
		return ctx
	}
	pairs := []string{authctx.HeaderUserID, account.String()}
	if profile, ok := authctx.ProfileID(ctx); ok {
		pairs = append(pairs, authctx.HeaderProfileID, profile.String())
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...)
}

func mapStoreErr(err error) error {
	if err == store.ErrNotFound {
		return status.Error(codes.NotFound, "not found")
	}
	return status.Error(codes.Internal, err.Error())
}

func (s *BotGRPC) provisionActorProfile(ctx context.Context, botName string) (uuid.UUID, error) {
	if s.User == nil {
		return uuid.Nil, nil
	}
	name := strings.TrimSpace(botName)
	if name == "" {
		name = "Bot"
	}
	hint := store.SlugFromName(name)
	userCtx := s.userCallCtx(ctx)
	resp, err := s.User.CreateProfile(userCtx, &userv1.CreateProfileRequest{
		DisplayName: name,
		Username:    &hint,
	})
	if err != nil {
		return uuid.Nil, status.Errorf(codes.FailedPrecondition, "create bot actor profile: %v", err)
	}
	if resp.GetProfile() == nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "create bot actor profile: empty response")
	}
	id, err := uuid.Parse(resp.GetProfile().GetId())
	if err != nil {
		return uuid.Nil, status.Errorf(codes.FailedPrecondition, "create bot actor profile: invalid id")
	}
	return id, nil
}
