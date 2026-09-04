package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	chatv1 "voice.app/voice/chat/v1"
)

// chatSubscriptionChecker asks Chat whether this active profile may currently
// view a chat. Any error is deliberately fail-closed at the WebSocket boundary.
type chatSubscriptionChecker interface {
	AuthorizeChat(ctx context.Context, accountID, profileID, chatID string) error
}

type grpcChatSubscriptionChecker struct {
	client chatv1.ChatServiceClient
}

var chatSubscriptionCheckTimeout = 2 * time.Second

func newGRPCChatSubscriptionChecker(cc *grpc.ClientConn) chatSubscriptionChecker {
	if cc == nil {
		return &grpcChatSubscriptionChecker{}
	}
	return &grpcChatSubscriptionChecker{client: chatv1.NewChatServiceClient(cc)}
}

func (g *grpcChatSubscriptionChecker) AuthorizeChat(ctx context.Context, accountID, profileID, chatID string) error {
	if g == nil || g.client == nil {
		return fmt.Errorf("chat subscription checker is not configured")
	}
	accountID = strings.TrimSpace(accountID)
	profileID = strings.TrimSpace(profileID)
	chatID = strings.TrimSpace(chatID)
	if accountID == "" || profileID == "" || chatID == "" {
		return fmt.Errorf("chat subscription checker requires account_id, profile_id and chat_id")
	}
	ctx, cancel := context.WithTimeout(ctx, chatSubscriptionCheckTimeout)
	defer cancel()
	// GetChat validates caller membership from normal user/profile metadata. Do
	// not mark this query internal: that would bypass the caller ACL in Chat.
	if outgoing, ok := metadata.FromOutgoingContext(ctx); ok {
		outgoing = outgoing.Copy()
		outgoing.Delete(grpcMDVoiceInternalCaller)
		ctx = metadata.NewOutgoingContext(ctx, outgoing)
	}
	ctx = metadata.AppendToOutgoingContext(ctx,
		grpcMDVoiceUserID, accountID,
		grpcMDVoiceProfileID, profileID,
	)
	resp, err := g.client.GetChat(ctx, &chatv1.GetChatRequest{ChatId: chatID})
	if err != nil {
		return err
	}
	if resp.GetChat() == nil || strings.TrimSpace(resp.GetChat().GetId()) != chatID {
		return fmt.Errorf("chat subscription checker received unexpected chat")
	}
	return nil
}
