package grpcsvc

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/bot/internal/authctx"
	"voice/backend/bot/internal/store"

	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

func presenceTTL() time.Duration {
	if v := strings.TrimSpace(os.Getenv("BOT_PRESENCE_TTL")); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return store.DefaultPresenceTTL
}

func (s *BotGRPC) requireScope(botRow *store.BotRow, scope string) error {
	if botRow == nil || !store.ScopeAllows(botRow.ScopesJSON, scope) {
		return status.Errorf(codes.PermissionDenied, "bot lacks %s", scope)
	}
	return nil
}

func (s *BotGRPC) botActorCtx(ctx context.Context, botRow *store.BotRow) context.Context {
	return metadata.AppendToOutgoingContext(ctx,
		authctx.HeaderProfileID, botRow.ActorProfileID.String(),
		authctx.HeaderUserID, botRow.OwnerAccountID.String(),
	)
}

func (s *BotGRPC) touchPresence(ctx context.Context, botID uuid.UUID) {
	_ = s.Store.TouchPresence(ctx, botID)
}

func (s *BotGRPC) isBotOnline(ctx context.Context, botID uuid.UUID) bool {
	online, _ := s.Store.IsBotOnline(ctx, botID, presenceTTL())
	return online
}

func (s *BotGRPC) TouchPresence(ctx context.Context, req *botv1.TouchPresenceRequest) (*botv1.TouchPresenceResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetBotId() != "" && req.GetBotId() != botRow.ID.String() {
		return nil, status.Error(codes.PermissionDenied, "bot_id mismatch")
	}
	s.touchPresence(ctx, botRow.ID)
	return &botv1.TouchPresenceResponse{}, nil
}

func (s *BotGRPC) AssignBotRole(ctx context.Context, req *botv1.AssignBotRoleRequest) (*botv1.AssignBotRoleResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireScope(botRow, "MEMBER_ASSIGN_ROLES"); err != nil {
		return nil, err
	}
	if s.Role == nil {
		return nil, status.Error(codes.FailedPrecondition, "role client not configured")
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	roleCtx := s.botActorCtx(ctx, botRow)
	_, err = s.Role.AssignRole(roleCtx, &rolev1.AssignRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: strings.TrimSpace(req.GetProfileId()),
		RoleId:    strings.TrimSpace(req.GetRoleId()),
	})
	if err != nil {
		return nil, err
	}
	return &botv1.AssignBotRoleResponse{}, nil
}

func (s *BotGRPC) RevokeBotRole(ctx context.Context, req *botv1.RevokeBotRoleRequest) (*botv1.RevokeBotRoleResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireScope(botRow, "MEMBER_ASSIGN_ROLES"); err != nil {
		return nil, err
	}
	if s.Role == nil {
		return nil, status.Error(codes.FailedPrecondition, "role client not configured")
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	roleCtx := s.botActorCtx(ctx, botRow)
	_, err = s.Role.RevokeRole(roleCtx, &rolev1.RevokeRoleRequest{
		SpaceId:   spaceID.String(),
		ProfileId: strings.TrimSpace(req.GetProfileId()),
		RoleId:    strings.TrimSpace(req.GetRoleId()),
	})
	if err != nil {
		return nil, err
	}
	return &botv1.RevokeBotRoleResponse{}, nil
}

func (s *BotGRPC) ListSpaceMembersForBot(ctx context.Context, req *botv1.ListSpaceMembersForBotRequest) (*botv1.ListSpaceMembersForBotResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireScope(botRow, "SPACE_VIEW_MEMBER_LIST"); err != nil {
		return nil, err
	}
	if s.Space == nil {
		return nil, status.Error(codes.FailedPrecondition, "space client not configured")
	}
	spaceID := strings.TrimSpace(req.GetSpaceId())
	spaceCtx := s.botActorCtx(ctx, botRow)
	listReq := &spacev1.ListMembersRequest{SpaceId: spaceID}
	if c := strings.TrimSpace(req.GetCursor()); c != "" {
		listReq.Page = &commonv1.CursorPageRequest{Cursor: c}
	}
	resp, err := s.Space.ListMembers(spaceCtx, listReq)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range resp.GetSpaceMemberList().GetMembers() {
		if m.GetProfileId() != "" {
			ids = append(ids, m.GetProfileId())
		}
	}
	out := &botv1.ListSpaceMembersForBotResponse{ProfileIds: ids}
	if next := resp.GetSpaceMemberList().GetNextCursor(); next != "" {
		out.NextCursor = &next
	}
	return out, nil
}

func (s *BotGRPC) CreateBotChat(ctx context.Context, req *botv1.CreateBotChatRequest) (*botv1.CreateBotChatResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireScope(botRow, "TEXT_CHAT_CREATE_IN_SPACE"); err != nil {
		return nil, err
	}
	if s.Chat == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat client not configured")
	}
	count, err := s.Store.IncrementDailyChatCreates(ctx, botRow.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if count > 10 {
		return nil, status.Error(codes.ResourceExhausted, "daily chat create limit exceeded")
	}
	spaceID := strings.TrimSpace(req.GetSpaceId())
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	if strings.EqualFold(strings.TrimSpace(req.GetChatType()), "channel") {
		chatType = chatv1.ChatType_CHAT_TYPE_CHANNEL
	}
	chatCtx := s.botActorCtx(ctx, botRow)
	createResp, err := s.Chat.CreateChat(chatCtx, &chatv1.CreateChatRequest{
		Type:    chatType,
		SpaceId: &spaceID,
		Name:    &name,
	})
	if err != nil {
		return nil, err
	}
	chat := createResp.GetChat()
	if chat == nil {
		return nil, status.Error(codes.Internal, "empty chat response")
	}
	ref := &chatv1.ChatRef{Id: chat.GetId(), Type: &chatType}
	return &botv1.CreateBotChatResponse{Chat: ref}, nil
}

func (s *BotGRPC) GetChatMessagesForBot(ctx context.Context, req *botv1.GetChatMessagesForBotRequest) (*botv1.GetChatMessagesForBotResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireScope(botRow, store.PrivilegedScopeReadHistory); err != nil {
		return nil, err
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
	if s.Messaging == nil {
		return nil, status.Error(codes.Unimplemented, "history fetch not implemented in v1")
	}
	msgReq := &messagingv1.GetMessagesRequest{Chat: req.GetChat()}
	if c := strings.TrimSpace(req.GetCursor()); c != "" {
		msgReq.Page = &commonv1.CursorPageRequest{Cursor: c}
	}
	msgCtx := s.botActorCtx(ctx, botRow)
	resp, err := s.Messaging.GetMessages(msgCtx, msgReq)
	if err != nil {
		return nil, err
	}
	msgs := resp.GetMessageList().GetMessages()
	var ids []string
	for _, m := range msgs {
		if m.GetId() != "" {
			ids = append(ids, m.GetId())
		}
	}
	out := &botv1.GetChatMessagesForBotResponse{
		MessageIds: ids,
		Messages:   msgs,
	}
	if next := resp.GetMessageList().GetNextCursor(); next != "" {
		out.NextCursor = &next
	}
	return out, nil
}

func (s *BotGRPC) CreateBotRole(ctx context.Context, req *botv1.CreateBotRoleRequest) (*botv1.CreateBotRoleResponse, error) {
	botRow, err := s.botFromToken(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireScope(botRow, store.PrivilegedScopeManageRoles); err != nil {
		return nil, err
	}
	if s.Role == nil {
		return nil, status.Error(codes.FailedPrecondition, "role client not configured")
	}
	spaceID, err := parseUUID("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	roleCtx := s.botActorCtx(ctx, botRow)
	createResp, err := s.Role.CreateRole(roleCtx, &rolev1.CreateRoleRequest{
		SpaceId:         spaceID.String(),
		Name:            name,
		PermissionsMask: req.GetPermissionsMask(),
		Position:        req.GetPosition(),
	})
	if err != nil {
		return nil, err
	}
	return &botv1.CreateBotRoleResponse{Role: createResp.GetRole()}, nil
}

func deferredTTL() time.Duration {
	if v := strings.TrimSpace(os.Getenv("BOT_DEFERRED_TTL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 24 * time.Hour
}

// RunDeferredTTLSweeper abandons deferred interaction rows older than BOT_DEFERRED_TTL.
func (s *BotGRPC) RunDeferredTTLSweeper(ctx context.Context) error {
	if s == nil || s.Store == nil {
		return nil
	}
	return s.Store.AbandonStaleDeferred(ctx, deferredTTL())
}

// RehydrateDeferred restores deferred interaction tokens into the hub after restart.
func (s *BotGRPC) RehydrateDeferred(ctx context.Context) {
	if s == nil || s.Hub == nil || s.Store == nil {
		return
	}
	tokens, err := s.Store.ListDeferredTokens(ctx)
	if err != nil {
		return
	}
	for _, token := range tokens {
		s.Hub.RegisterDeferred(token)
	}
}
