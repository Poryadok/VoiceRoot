package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	messagingv1 "voice.app/voice/messaging/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const deliveryAckMessageLookupTimeout = 3 * time.Second

type deliveryAckMessage struct {
	ID              string
	ChatID          string
	SenderProfileID string
}

type deliveryAckMessageReader interface {
	GetMessage(ctx context.Context, messageID, viewerProfileID string) (deliveryAckMessage, error)
}

type grpcDeliveryAckMessageReader struct {
	client messagingv1.MessagingServiceClient
}

func newGRPCDeliveryAckMessageReader(cc grpc.ClientConnInterface) *grpcDeliveryAckMessageReader {
	if cc == nil {
		return nil
	}
	return &grpcDeliveryAckMessageReader{client: messagingv1.NewMessagingServiceClient(cc)}
}

func (r *grpcDeliveryAckMessageReader) GetMessage(ctx context.Context, messageID, viewerProfileID string) (deliveryAckMessage, error) {
	if r == nil || r.client == nil {
		return deliveryAckMessage{}, fmt.Errorf("messaging client not configured")
	}
	messageID = strings.TrimSpace(messageID)
	viewerProfileID = strings.TrimSpace(viewerProfileID)
	if messageID == "" || viewerProfileID == "" {
		return deliveryAckMessage{}, fmt.Errorf("delivery ack lookup requires message and verified viewer IDs")
	}
	ctx, cancel := context.WithTimeout(ctx, deliveryAckMessageLookupTimeout)
	defer cancel()
	// Replace outgoing metadata so no ambient/internal-caller marker can select
	// Messaging's trusted-service branch. The viewer comes only from WS claims.
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(grpcMDVoiceProfileID, viewerProfileID))
	resp, err := r.client.GetMessage(ctx, &messagingv1.GetMessageRequest{MessageId: messageID})
	if err != nil {
		return deliveryAckMessage{}, err
	}
	if resp == nil || resp.GetMessage() == nil || resp.GetMessage().GetChat() == nil {
		return deliveryAckMessage{}, fmt.Errorf("messaging returned no authoritative message binding")
	}
	message := resp.GetMessage()
	return deliveryAckMessage{
		ID:              message.GetId(),
		ChatID:          message.GetChat().GetId(),
		SenderProfileID: message.GetSenderProfileId(),
	}, nil
}
