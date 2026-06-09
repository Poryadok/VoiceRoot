package grpcsvc

import (
	"context"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

func (s *ChatGRPC) CreateDM(ctx context.Context, req *chatv1.CreateDMRequest) (*chatv1.CreateDMResponse, error) {
	c, err := s.ensureDM(ctx, req.GetOtherProfileId())
	if err != nil {
		return nil, err
	}
	return &chatv1.CreateDMResponse{Chat: chatRowToProto(c)}, nil
}

func (s *ChatGRPC) GetDM(ctx context.Context, req *chatv1.GetDMRequest) (*chatv1.GetDMResponse, error) {
	c, err := s.ensureDM(ctx, req.GetOtherProfileId())
	if err != nil {
		return nil, err
	}
	return &chatv1.GetDMResponse{Chat: chatRowToProto(c)}, nil
}

// ensureDM applies PLAN Phase 1: DM without friendship; blocks via Social (both directions).
func (s *ChatGRPC) ensureDM(ctx context.Context, otherProfileRaw string) (*store.ChatRow, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	if s.Profiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "user profile lookup not configured")
	}
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	callerProfile, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	otherProfile, err := parseUUIDField("other_profile_id", otherProfileRaw)
	if err != nil {
		return nil, err
	}
	if otherProfile == callerProfile {
		return nil, status.Error(codes.InvalidArgument, "cannot open dm with self")
	}

	otherAccount, err := s.Profiles.AccountIDByProfileID(ctx, otherProfile)
	if err != nil {
		return nil, err
	}

	if s.Blocks != nil {
		blocked, err := s.Blocks.AccountPairBlocked(ctx, accountID, otherAccount)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if blocked {
			return nil, status.Error(codes.PermissionDenied, "dm not allowed between blocked accounts")
		}
	}

	row, created, err := s.DM.EnsureDM(ctx, callerProfile, otherProfile)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if created && s.ChatEvents != nil {
		if err := s.ChatEvents.PublishChatCreated(ctx, row.ID.String(), "dm"); err != nil {
			log.Printf("chat: publish chat.created: %v", err)
		}
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, row.ID.String(), callerProfile.String(), "joined"); err != nil {
			log.Printf("chat: publish chat.member_changed: %v", err)
		}
		if err := s.ChatEvents.PublishChatMemberChanged(ctx, row.ID.String(), otherProfile.String(), "joined"); err != nil {
			log.Printf("chat: publish chat.member_changed: %v", err)
		}
	}
	return row, nil
}

func chatRowToProto(r *store.ChatRow) *chatv1.Chat {
	if r == nil {
		return nil
	}
	chatType := chatv1.ChatType_CHAT_TYPE_DM
	switch r.Type {
	case "group":
		chatType = chatv1.ChatType_CHAT_TYPE_GROUP
	case "channel":
		chatType = chatv1.ChatType_CHAT_TYPE_CHANNEL
	}
	out := &chatv1.Chat{
		Id:               r.ID.String(),
		Type:             chatType,
		CreatorProfileId: r.CreatorProfileID.String(),
		SlowModeSeconds:  r.SlowModeSeconds,
		CreatedAt:        timestamppb.New(r.CreatedAt),
		UpdatedAt:        timestamppb.New(r.UpdatedAt),
	}
	if r.SpaceID != nil {
		sid := r.SpaceID.String()
		out.SpaceId = &sid
	}
	if r.Name != nil {
		out.Name = r.Name
	}
	if r.AvatarURL != nil {
		out.AvatarUrl = r.AvatarURL
	}
	if r.Topic != nil {
		out.Topic = r.Topic
	}
	if r.LastMessageAt != nil {
		out.LastMessageAt = timestamppb.New(*r.LastMessageAt)
	}
	return out
}
