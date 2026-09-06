package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

const grpcMDVoiceInternalCaller = "x-voice-internal-caller"

type chatMemberInboxLister interface {
	RecipientDeliveryStates(ctx context.Context, chatID string) (map[string]chatMemberDeliveryState, error)
}

// chatMemberDeliveryState is the per-recipient state Chat owns for event
// delivery. Realtime must page through every member: Chat caps a ListMembers
// response at 100, while notification and archive behaviour applies to all
// members, not just the first page.
type chatMemberDeliveryState struct {
	InboxBucket string
	IsArchived  bool
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

func (g *grpcChatMemberInboxLister) RecipientDeliveryStates(ctx context.Context, chatID string) (map[string]chatMemberDeliveryState, error) {
	if g == nil || g.client == nil {
		return nil, fmt.Errorf("chat member inbox lister not configured")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil, fmt.Errorf("chat_id required")
	}
	ctx = metadata.AppendToOutgoingContext(ctx, grpcMDVoiceInternalCaller, "realtime")
	out := make(map[string]chatMemberDeliveryState)
	cursor := ""
	for {
		resp, err := g.client.ListMembers(ctx, &chatv1.ListMembersRequest{
			ChatId: chatID,
			Page:   &commonv1.CursorPageRequest{Cursor: cursor, PageSize: 100},
		})
		if err != nil {
			return nil, err
		}
		list := resp.GetMemberList()
		if list == nil {
			return out, nil
		}
		for _, m := range list.GetMembers() {
			if m == nil {
				continue
			}
			pid := strings.TrimSpace(m.GetProfileId())
			if pid == "" {
				continue
			}
			out[pid] = chatMemberDeliveryState{
				InboxBucket: strings.TrimSpace(m.GetInboxBucket()),
				IsArchived:  m.GetIsArchived(),
			}
		}
		next := strings.TrimSpace(list.GetNextCursor())
		if next == "" || next == cursor {
			return out, nil
		}
		cursor = next
	}
}
