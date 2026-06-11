package squad

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"

	"voice/backend/matchmaking/internal/authctx"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

// GRPCChatClient provisions ephemeral match squad group chats via ChatService.
type GRPCChatClient struct {
	Client chatv1.ChatServiceClient
}

// GRPCVoiceClient provisions ephemeral match squad voice via VoiceService group_voice.
type GRPCVoiceClient struct {
	Client callsv1.VoiceServiceClient
}

func withCreatorProfile(ctx context.Context, profileID uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, profileID.String())
}

// CreateMatchChat creates a group chat and adds all participants.
func (c *GRPCChatClient) CreateMatchChat(ctx context.Context, matchID uuid.UUID, profileIDs []uuid.UUID) (string, error) {
	if c == nil || c.Client == nil {
		return "", fmt.Errorf("chat client unavailable")
	}
	if len(profileIDs) == 0 {
		return "", fmt.Errorf("no participants")
	}
	creator := profileIDs[0]
	ctx = withCreatorProfile(ctx, creator)
	name := fmt.Sprintf("Match %s", matchID.String()[:8])
	resp, err := c.Client.CreateChat(ctx, &chatv1.CreateChatRequest{
		Type: chatv1.ChatType_CHAT_TYPE_GROUP,
		Name: &name,
	})
	if err != nil {
		return "", err
	}
	chatID := strings.TrimSpace(resp.GetChat().GetId())
	if chatID == "" {
		return "", fmt.Errorf("empty chat id")
	}
	if len(profileIDs) > 1 {
		others := make([]string, 0, len(profileIDs)-1)
		for _, id := range profileIDs[1:] {
			others = append(others, id.String())
		}
		_, err = c.Client.AddMembers(ctx, &chatv1.AddMembersRequest{
			ChatId:     chatID,
			ProfileIds: others,
		})
		if err != nil {
			return "", err
		}
	}
	return chatID, nil
}

// CreateMatchRoom starts a group voice session linked to the match chat.
func (c *GRPCVoiceClient) CreateMatchRoom(ctx context.Context, matchID uuid.UUID, profileIDs []uuid.UUID, chatID string) (string, error) {
	if c == nil || c.Client == nil {
		return "", fmt.Errorf("voice client unavailable")
	}
	if len(profileIDs) == 0 {
		return "", fmt.Errorf("no participants")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return "", fmt.Errorf("chat id required")
	}
	creator := profileIDs[0]
	ctx = withCreatorProfile(ctx, creator)
	groupType := chatv1.ChatType_CHAT_TYPE_GROUP
	resp, err := c.Client.StartCall(ctx, &callsv1.StartCallRequest{
		RoomType:     "group_voice",
		RoomTypeEnum: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE.Enum(),
		LinkedChat: &chatv1.ChatRef{
			Id:   chatID,
			Type: &groupType,
		},
		MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO.Enum(),
	})
	if err != nil {
		return "", err
	}
	roomID := strings.TrimSpace(resp.GetCallSession().GetRoomId())
	if roomID == "" {
		return "", fmt.Errorf("empty voice room id")
	}
	_ = matchID
	return roomID, nil
}
