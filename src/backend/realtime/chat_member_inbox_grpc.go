package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	chatv1 "voice.app/voice/chat/v1"
)

const grpcMDVoiceInternalCaller = "x-voice-internal-caller"

type chatMemberInboxLister interface {
	RecipientInboxBuckets(ctx context.Context, chatID string) (map[string]string, error)
}

type grpcChatMemberInboxLister struct {
	client chatv1.ChatServiceClient
}

func newGRPCChatMemberInboxLister(cc *grpc.ClientConn) chatMemberInboxLister {
	if cc == nil {
		return nil
	}
	return &grpcChatMemberInboxLister{client: chatv1.NewChatServiceClient(cc)}
}

func (g *grpcChatMemberInboxLister) RecipientInboxBuckets(ctx context.Context, chatID string) (map[string]string, error) {
	if g == nil || g.client == nil {
		return nil, fmt.Errorf("chat member inbox lister not configured")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil, fmt.Errorf("chat_id required")
	}
	ctx = metadata.AppendToOutgoingContext(ctx, grpcMDVoiceInternalCaller, "realtime")
	resp, err := g.client.ListMembers(ctx, &chatv1.ListMembersRequest{ChatId: chatID})
	if err != nil {
		return nil, err
	}
	list := resp.GetMemberList()
	if list == nil {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(list.GetMembers()))
	for _, m := range list.GetMembers() {
		if m == nil {
			continue
		}
		pid := strings.TrimSpace(m.GetProfileId())
		if pid == "" {
			continue
		}
		out[pid] = strings.TrimSpace(m.GetInboxBucket())
	}
	return out, nil
}
