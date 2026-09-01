package chatmembers

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	chatv1 "voice.app/voice/chat/v1"

	"voice/backend/notification/internal/s2s"
)

// Member is one chat participant with inbox routing metadata.
type Member struct {
	ProfileID   string
	InboxBucket string
}

// Lister resolves chat members for push fan-out.
type Lister interface {
	ListMemberProfileIDs(ctx context.Context, chatID string) ([]string, error)
	ListMembers(ctx context.Context, chatID string) ([]Member, error)
}

// GRPCLister calls Chat Service ListMembers (S2S).
type GRPCLister struct {
	client chatv1.ChatServiceClient
}

func NewGRPCLister(addr string) (*GRPCLister, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("chat members: empty grpc addr")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GRPCLister{client: chatv1.NewChatServiceClient(conn)}, nil
}

func (l *GRPCLister) ListMembers(ctx context.Context, chatID string) ([]Member, error) {
	if l == nil || l.client == nil {
		return nil, fmt.Errorf("chat members lister unavailable")
	}
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil, fmt.Errorf("chat members: chat_id required")
	}
	resp, err := l.client.ListMembers(s2s.Context(ctx), &chatv1.ListMembersRequest{ChatId: chatID})
	if err != nil {
		return nil, err
	}
	list := resp.GetMemberList()
	if list == nil {
		return nil, nil
	}
	out := make([]Member, 0, len(list.GetMembers()))
	for _, m := range list.GetMembers() {
		if m == nil {
			continue
		}
		if pid := strings.TrimSpace(m.GetProfileId()); pid != "" {
			out = append(out, Member{
				ProfileID:   pid,
				InboxBucket: strings.TrimSpace(m.GetInboxBucket()),
			})
		}
	}
	return out, nil
}

func (l *GRPCLister) ListMemberProfileIDs(ctx context.Context, chatID string) ([]string, error) {
	members, err := l.ListMembers(ctx, chatID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, m.ProfileID)
	}
	return out, nil
}

// NoopLister returns empty members (MessageSent push skipped when chat service unavailable).
type NoopLister struct{}

func (NoopLister) ListMemberProfileIDs(context.Context, string) ([]string, error) {
	return nil, nil
}

func (NoopLister) ListMembers(context.Context, string) ([]Member, error) {
	return nil, nil
}
