package grpcsvc

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

func (s *ChatGRPC) CreateChat(ctx context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	chatType := req.GetType()
	if chatType != chatv1.ChatType_CHAT_TYPE_GROUP && chatType != chatv1.ChatType_CHAT_TYPE_CHANNEL {
		return nil, status.Error(codes.InvalidArgument, "only group and channel chats are supported")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	var row *store.ChatRow
	var err error
	spaceIDRaw := strings.TrimSpace(req.GetSpaceId())
	if chatType == chatv1.ChatType_CHAT_TYPE_CHANNEL {
		if spaceIDRaw == "" {
			return nil, status.Error(codes.InvalidArgument, "space_id is required for channels")
		}
		spaceID, parseErr := parseUUIDField("space_id", spaceIDRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		row, err = s.DM.CreateSpaceChannelChat(ctx, caller, spaceID, name)
	} else if spaceIDRaw != "" {
		spaceID, parseErr := parseUUIDField("space_id", spaceIDRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		row, err = s.DM.CreateSpaceGroupChat(ctx, caller, spaceID, name)
	} else {
		row, err = s.DM.CreateGroupChat(ctx, caller, name)
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		eventType := "group"
		if row.Type == "channel" {
			eventType = "channel"
		}
		if err := s.ChatEvents.PublishChatCreated(ctx, row.ID.String(), eventType); err != nil {
			log.Printf("chat: publish chat.created: %v", err)
		}
		if row.Type == "group" && row.SpaceID == nil {
			if err := s.ChatEvents.PublishChatMemberChanged(ctx, row.ID.String(), caller.String(), "joined"); err != nil {
				log.Printf("chat: publish chat.member_changed: %v", err)
			}
		}
	}
	return &chatv1.CreateChatResponse{Chat: chatRowToProto(row)}, nil
}

func (s *ChatGRPC) UpdateChat(ctx context.Context, req *chatv1.UpdateChatRequest) (*chatv1.UpdateChatResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if row.Type != "group" {
		return nil, status.Error(codes.InvalidArgument, "only group chats can be updated")
	}
	role, err := s.DM.GetMemberRole(ctx, chatID, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var name, avatar *string
	var slowMode *int32
	if req.Name != nil {
		n := strings.TrimSpace(req.GetName())
		name = &n
	}
	if req.AvatarUrl != nil {
		a := strings.TrimSpace(req.GetAvatarUrl())
		avatar = &a
	}
	if req.SlowModeSeconds != nil {
		slow := req.GetSlowModeSeconds()
		slowMode = &slow
	}

	slowModeOnly := slowMode != nil && name == nil && avatar == nil
	if row.SpaceID != nil && slowModeOnly && s.Roles != nil {
		if err := canSetSpaceChatSlowMode(ctx, s.Roles, *row.SpaceID, caller); err != nil {
			return nil, err
		}
	} else if role != "owner" {
		return nil, status.Error(codes.PermissionDenied, "only the group owner can update the chat")
	}

	updated, err := s.DM.UpdateGroupChat(ctx, chatID, name, avatar, slowMode)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if updated == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	return &chatv1.UpdateChatResponse{Chat: chatRowToProto(updated)}, nil
}

func (s *ChatGRPC) AddMembers(ctx context.Context, req *chatv1.AddMembersRequest) (*chatv1.AddMembersResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if row.Type != "group" {
		return nil, status.Error(codes.InvalidArgument, "add members only supported for groups")
	}
	member, err := s.DM.IsChatMember(ctx, chatID, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !member {
		return nil, status.Error(codes.PermissionDenied, "not a chat member")
	}

	ids := make([]uuid.UUID, 0, len(req.GetProfileIds()))
	for _, raw := range req.GetProfileIds() {
		pid, perr := parseUUIDField("profile_id", raw)
		if perr != nil {
			return nil, perr
		}
		if s.Profiles != nil {
			if _, perr := s.Profiles.AccountIDByProfileID(ctx, pid); perr != nil {
				return nil, perr
			}
		}
		if err := s.ensureInvitePrivacy(ctx, caller, pid); err != nil {
			return nil, err
		}
		ids = append(ids, pid)
	}

	added, err := s.DM.AddGroupMembers(ctx, chatID, ids)
	if errors.Is(err, store.ErrGroupTooFewMembers) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if errors.Is(err, store.ErrGroupMemberLimit) {
		return nil, status.Error(codes.ResourceExhausted, err.Error())
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		for _, pid := range added {
			if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), pid.String(), "joined"); err != nil {
				log.Printf("chat: publish chat.member_changed: %v", err)
			}
		}
	}
	return &chatv1.AddMembersResponse{}, nil
}

func (s *ChatGRPC) RemoveMember(ctx context.Context, req *chatv1.RemoveMemberRequest) (*chatv1.RemoveMemberResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	targetID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if row.Type != "group" {
		return nil, status.Error(codes.InvalidArgument, "remove member only supported for groups")
	}
	role, err := s.DM.GetMemberRole(ctx, chatID, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if role != "owner" {
		return nil, status.Error(codes.PermissionDenied, "only the group owner can remove members")
	}
	if err := s.DM.RemoveGroupMember(ctx, chatID, targetID); err != nil {
		if errors.Is(err, store.ErrCannotRemoveOwner) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "member not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), targetID.String(), "removed"); err != nil {
			log.Printf("chat: publish chat.member_changed: %v", err)
		}
	}
	return &chatv1.RemoveMemberResponse{}, nil
}

func (s *ChatGRPC) LeaveChat(ctx context.Context, req *chatv1.LeaveChatRequest) (*chatv1.LeaveChatResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if row.Type != "group" {
		return nil, status.Error(codes.InvalidArgument, "leave only supported for groups")
	}
	if err := s.DM.LeaveGroupChat(ctx, chatID, caller); err != nil {
		if errors.Is(err, store.ErrOwnerMustTransfer) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "not a group member")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), caller.String(), "left"); err != nil {
			log.Printf("chat: publish chat.member_changed: %v", err)
		}
	}
	return &chatv1.LeaveChatResponse{}, nil
}

func (s *ChatGRPC) TransferGroupOwnership(ctx context.Context, req *chatv1.TransferGroupOwnershipRequest) (*chatv1.TransferGroupOwnershipResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	newOwner, err := parseUUIDField("new_owner_profile_id", req.GetNewOwnerProfileId())
	if err != nil {
		return nil, err
	}
	row, err := s.DM.FindChatByID(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "chat not found")
	}
	if row.Type != "group" {
		return nil, status.Error(codes.InvalidArgument, "transfer only supported for groups")
	}
	if err := s.DM.TransferGroupOwnership(ctx, chatID, caller, newOwner); err != nil {
		if errors.Is(err, store.ErrNotGroupOwner) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "new owner is not a group member")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, chatID.String(), newOwner.String(), "owner_transferred"); err != nil {
			log.Printf("chat: publish chat.member_changed: %v", err)
		}
	}
	return &chatv1.TransferGroupOwnershipResponse{}, nil
}
