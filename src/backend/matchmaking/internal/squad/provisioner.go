package squad

import (
	"context"

	"github.com/google/uuid"
)

// ChatClient creates a temporary group chat for a match squad.
type ChatClient interface {
	CreateMatchChat(ctx context.Context, matchID uuid.UUID, profileIDs []uuid.UUID) (chatID string, err error)
}

// VoiceClient creates a group voice room for a match squad.
type VoiceClient interface {
	CreateMatchRoom(ctx context.Context, matchID uuid.UUID, profileIDs []uuid.UUID, chatID string) (voiceRoomID string, err error)
}

// Provisioner wires Chat + Voice for match squad creation.
type Provisioner struct {
	Chat  ChatClient
	Voice VoiceClient
}

// Provision creates chat and voice resources for participants.
func (p *Provisioner) Provision(ctx context.Context, matchID uuid.UUID, profileIDs []uuid.UUID) (voiceRoomID, chatID string, err error) {
	if p == nil {
		return "", "", nil
	}
	if p.Chat != nil {
		chatID, err = p.Chat.CreateMatchChat(ctx, matchID, profileIDs)
		if err != nil {
			return "", "", err
		}
	}
	if p.Voice != nil {
		voiceRoomID, err = p.Voice.CreateMatchRoom(ctx, matchID, profileIDs, chatID)
		if err != nil {
			return "", "", err
		}
	}
	return voiceRoomID, chatID, nil
}
