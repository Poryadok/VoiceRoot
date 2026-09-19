package main

import (
	"context"
	"fmt"
	"strings"
	"time"

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
// delivery. Realtime requests pages of 100 members; notification and archive
// behaviour applies to all members, not just the first page.
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
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, grpcMDVoiceInternalCaller, "realtime")
	out := make(map[string]chatMemberDeliveryState)
	cursor := ""
	seenCursors := make(map[string]struct{})
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resp, err := g.client.ListMembers(ctx, &chatv1.ListMembersRequest{
			ChatId: chatID,
			Page:   &commonv1.CursorPageRequest{Cursor: cursor, PageSize: 100},
		})
		if err != nil {
			return nil, err
		}
		list := resp.GetMemberList()
		if list == nil {
			return nil, fmt.Errorf("chat membership response is missing its member list")
		}
		for _, m := range list.GetMembers() {
			if m == nil {
				return nil, fmt.Errorf("chat membership contains a missing member")
			}
			pid := strings.TrimSpace(m.GetProfileId())
			if pid == "" {
				return nil, fmt.Errorf("chat membership contains a blank profile ID")
			}
			if _, exists := out[pid]; exists {
				return nil, fmt.Errorf("chat membership contains a duplicate profile ID")
			}
			out[pid] = chatMemberDeliveryState{
				InboxBucket: strings.TrimSpace(m.GetInboxBucket()),
				IsArchived:  m.GetIsArchived(),
			}
		}
		next := strings.TrimSpace(list.GetNextCursor())
		if next == "" {
			return out, nil
		}
		if _, seen := seenCursors[next]; seen {
			return nil, fmt.Errorf("chat membership pagination cursor repeated")
		}
		seenCursors[next] = struct{}{}
		cursor = next
	}
}
