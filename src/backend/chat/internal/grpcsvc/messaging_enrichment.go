package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

const defaultMessagingEnrichmentTimeout = 3 * time.Second

type MessagingListEnricher struct {
	Client  messagingv1.MessagingServiceClient
	Timeout time.Duration
}

func NewMessagingListEnricher(client messagingv1.MessagingServiceClient) *MessagingListEnricher {
	return &MessagingListEnricher{
		Client:  client,
		Timeout: defaultMessagingEnrichmentTimeout,
	}
}

func (e *MessagingListEnricher) EnrichListChats(ctx context.Context, _ uuid.UUID, chatIDs []uuid.UUID) (map[uuid.UUID]ListChatExtra, error) {
	if e == nil || e.Client == nil {
		return nil, errors.New("messaging list enricher: client not configured")
	}
	if len(chatIDs) == 0 {
		return map[uuid.UUID]ListChatExtra{}, nil
	}

	callCtx := ctx
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		callCtx = metadata.NewOutgoingContext(ctx, md.Copy())
	}
	if e.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(callCtx, e.Timeout)
		defer cancel()
	}

	refs := make([]*chatv1.ChatRef, 0, len(chatIDs))
	for _, id := range chatIDs {
		refs = append(refs, &chatv1.ChatRef{Id: id.String()})
	}
	resp, err := e.Client.GetChatListMetadata(callCtx, &messagingv1.GetChatListMetadataRequest{Chats: refs})
	if err != nil {
		return nil, err
	}

	out := make(map[uuid.UUID]ListChatExtra, len(resp.GetByChatId()))
	for rawID, item := range resp.GetByChatId() {
		id, err := uuid.Parse(rawID)
		if err != nil && item.GetChat().GetId() != "" {
			id, err = uuid.Parse(item.GetChat().GetId())
		}
		if err != nil {
			continue
		}
		out[id] = ListChatExtra{
			LastMessagePreview: item.GetLastMessagePreview(),
			UnreadCount:        item.GetUnreadCount(),
		}
	}
	return out, nil
}
