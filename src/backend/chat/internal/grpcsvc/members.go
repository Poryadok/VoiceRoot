package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/chat/internal/authctx"

	chatv1 "voice.app/voice/chat/v1"
)

func (s *ChatGRPC) ListMembers(ctx context.Context, req *chatv1.ListMembersRequest) (*chatv1.ListMembersResponse, error) {
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
	member, err := s.DM.IsChatMember(ctx, chatID, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !member {
		return nil, status.Error(codes.PermissionDenied, "not a chat member")
	}
	members, err := s.DM.ListChatMembers(ctx, chatID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	pageSize := int(req.GetPage().GetPageSize())
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 500 {
		pageSize = 500
	}
	start := 0
	if cur := req.GetPage().GetCursor(); cur != "" {
		// Cursor is the profile_id of the last member returned on the previous page (exclusive start for this page).
		afterID, perr := uuid.Parse(cur)
		if perr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page cursor")
		}
		found := false
		for i := range members {
			if members[i].ProfileID == afterID {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			return nil, status.Error(codes.InvalidArgument, "invalid page cursor")
		}
	}
	if start > len(members) {
		return &chatv1.ListMembersResponse{
			MemberList: &chatv1.MemberList{
				Members:    nil,
				NextCursor: "",
			},
		}, nil
	}
	slice := members[start:]
	var next string
	if len(slice) > pageSize {
		next = slice[pageSize-1].ProfileID.String()
		slice = slice[:pageSize]
	}
	out := make([]*chatv1.ChatMember, 0, len(slice))
	for i := range slice {
		m := slice[i]
		cm := &chatv1.ChatMember{
			ProfileId:  m.ProfileID.String(),
			Role:       m.Role,
			JoinedAt:   timestamppb.New(m.JoinedAt.UTC()),
			IsArchived: m.IsArchived,
		}
		if m.MutedUntil.Valid {
			cm.MutedUntil = timestamppb.New(m.MutedUntil.Time.UTC())
		}
		out = append(out, cm)
	}
	return &chatv1.ListMembersResponse{
		MemberList: &chatv1.MemberList{
			Members:    out,
			NextCursor: next,
		},
	}, nil
}

func (s *ChatGRPC) GetChat(ctx context.Context, req *chatv1.GetChatRequest) (*chatv1.GetChatResponse, error) {
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
	member, err := s.DM.IsChatMember(ctx, chatID, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !member {
		return nil, status.Error(codes.PermissionDenied, "not a chat member")
	}
	return &chatv1.GetChatResponse{Chat: chatRowToProto(row)}, nil
}
