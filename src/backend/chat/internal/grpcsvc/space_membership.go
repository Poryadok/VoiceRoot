package grpcsvc

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
)

const permTextChatView = "TEXT_CHAT_VIEW"

func (s *ChatGRPC) isEffectiveChatMember(ctx context.Context, row *store.ChatRow, profileID uuid.UUID) (bool, error) {
	if s == nil || row == nil {
		return false, nil
	}
	var member bool
	var err error
	if s.SpaceMembers != nil {
		member, err = s.SpaceMembers.IsEffectiveChatMember(ctx, s.dmStore(), row, profileID)
	} else if s.DM != nil {
		member, err = s.DM.IsChatMember(ctx, row.ID, profileID)
	} else {
		return false, nil
	}
	if err != nil || !member {
		return member, err
	}
	if row.SpaceID == nil || s.Roles == nil {
		return true, nil
	}
	return s.hasChatViewPermission(ctx, *row.SpaceID, profileID, row)
}

func (s *ChatGRPC) listEffectiveChatMembers(ctx context.Context, row *store.ChatRow) ([]store.ChatMemberRow, error) {
	if s == nil || row == nil {
		return nil, nil
	}
	var members []store.ChatMemberRow
	var err error
	if s.SpaceMembers != nil {
		members, err = s.SpaceMembers.ListEffectiveChatMembers(ctx, s.dmStore(), row)
	} else if s.DM != nil {
		members, err = s.DM.ListChatMembers(ctx, row.ID)
	} else {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.SpaceID == nil || s.Roles == nil {
		return members, nil
	}
	filtered := make([]store.ChatMemberRow, 0, len(members))
	for _, m := range members {
		ok, perr := s.hasChatViewPermission(ctx, *row.SpaceID, m.ProfileID, row)
		if perr != nil {
			return nil, perr
		}
		if ok {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

func (s *ChatGRPC) hasChatViewPermission(ctx context.Context, spaceID, profileID uuid.UUID, row *store.ChatRow) (bool, error) {
	if s == nil || s.Roles == nil {
		return true, nil
	}
	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	if row.Type == "channel" {
		chatType = chatv1.ChatType_CHAT_TYPE_CHANNEL
	}
	resp, err := s.Roles.CheckPermission(ctx, &rolev1.CheckPermissionRequest{
		SpaceId:        spaceID.String(),
		ProfileId:      profileID.String(),
		PermissionName: permTextChatView,
		Chat:           &chatv1.ChatRef{Id: row.ID.String(), Type: &chatType},
	})
	if err != nil {
		return false, err
	}
	return resp.GetAllowed(), nil
}

func (s *ChatGRPC) dmStore() *store.DMStore {
	if s == nil {
		return nil
	}
	if dm, ok := s.DM.(*store.DMStore); ok {
		return dm
	}
	return nil
}
