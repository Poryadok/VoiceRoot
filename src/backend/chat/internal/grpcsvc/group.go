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
	if req.GetType() != chatv1.ChatType_CHAT_TYPE_GROUP {
		return nil, status.Error(codes.InvalidArgument, "only group chats are supported")
	}
	if req.SpaceId != nil && strings.TrimSpace(req.GetSpaceId()) != "" {
		return nil, status.Error(codes.Unimplemented, "space groups are not supported yet")
	}
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	row, err := s.DM.CreateGroupChat(ctx, caller, name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatCreated(ctx, row.ID.String(), "group"); err != nil {
			log.Printf("chat: publish chat.created: %v", err)
		}
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, row.ID.String(), caller.String(), "joined"); err != nil {
			log.Printf("chat: publish chat.member_changed: %v", err)
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
	if role != "owner" {
		return nil, status.Error(codes.PermissionDenied, "only the group owner can update the chat")
	}

	var name, avatar *string
	if req.Name != nil {
		n := strings.TrimSpace(req.GetName())
		name = &n
	}
	if req.AvatarUrl != nil {
		a := strings.TrimSpace(req.GetAvatarUrl())
		avatar = &a
	}
	updated, err := s.DM.UpdateGroupChat(ctx, chatID, name, avatar)
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
