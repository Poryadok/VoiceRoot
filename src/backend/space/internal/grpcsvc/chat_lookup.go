package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	chatv1 "voice.app/voice/chat/v1"
)

// ChatInfo holds display metadata for a linked chat node.
type ChatInfo struct {
	Name     string
	ChatType chatv1.ChatType
}

// ChatLookup resolves chat display names and types for space tree enrichment.
type ChatLookup interface {
	GetChatNames(ctx context.Context, chatIDs []uuid.UUID) (map[uuid.UUID]ChatInfo, error)
}

type mapChatLookup struct {
	chats map[uuid.UUID]ChatInfo
}

func (m *mapChatLookup) GetChatNames(_ context.Context, chatIDs []uuid.UUID) (map[uuid.UUID]ChatInfo, error) {
	out := make(map[uuid.UUID]ChatInfo, len(chatIDs))
	for _, id := range chatIDs {
		if info, ok := m.chats[id]; ok {
			out[id] = info
		}
	}
	return out, nil
}

type grpcChatLookup struct {
	client chatv1.ChatServiceClient
}

// NewGRPCChatLookup resolves chat metadata via Chat Service GetChat (S2S).
func NewGRPCChatLookup(cc grpc.ClientConnInterface) ChatLookup {
	return &grpcChatLookup{client: chatv1.NewChatServiceClient(cc)}
}

func (g *grpcChatLookup) GetChatNames(ctx context.Context, chatIDs []uuid.UUID) (map[uuid.UUID]ChatInfo, error) {
	if g == nil || g.client == nil || len(chatIDs) == 0 {
		return nil, nil
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "space"))
	out := make(map[uuid.UUID]ChatInfo, len(chatIDs))
	lookupFailed := false
	for _, id := range chatIDs {
		resp, err := g.client.GetChat(ctx, &chatv1.GetChatRequest{ChatId: id.String()})
		if err != nil {
			lookupFailed = true
			continue
		}
		chat := resp.GetChat()
		if chat == nil {
			continue
		}
		info := ChatInfo{ChatType: chat.GetType()}
		if chat.Name != nil {
			info.Name = chat.GetName()
		}
		out[id] = info
	}
	if lookupFailed {
		return out, errors.New("chat lookup failed")
	}
	return out, nil
}
