package chatmembers

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	commonv1 "voice.app/voice/common/v1"

	chatv1 "voice.app/voice/chat/v1"

	"voice/backend/notification/internal/s2s"
)

const memberPageSize = 500

// Member is one chat participant with inbox routing metadata.
type Member struct {
	ProfileID   string
	InboxBucket string
	IsArchived  bool
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
	return listAllMemberPages(ctx, chatID, func(ctx context.Context, req *chatv1.ListMembersRequest) (*chatv1.ListMembersResponse, error) {
		return l.client.ListMembers(s2s.Context(ctx), req)
	})
}

type memberPageFetcher func(context.Context, *chatv1.ListMembersRequest) (*chatv1.ListMembersResponse, error)

// listAllMemberPages resolves complete, valid recipient metadata before the
// caller can route an event. A partial or malformed listing is never safe to
// treat as an empty recipient set.
func listAllMemberPages(ctx context.Context, chatID string, fetch memberPageFetcher) ([]Member, error) {
	if fetch == nil {
		return nil, fmt.Errorf("chat members: page fetcher unavailable")
	}
	var out []Member
	cursor := ""
	seenCursors := make(map[string]struct{})
	seenProfiles := make(map[string]struct{})
	for {
		resp, err := fetch(ctx, &chatv1.ListMembersRequest{
			ChatId: chatID,
			Page:   &commonv1.CursorPageRequest{Cursor: cursor, PageSize: memberPageSize},
		})
		if err != nil {
			return nil, err
		}
		list := resp.GetMemberList()
		if list == nil {
			return nil, fmt.Errorf("chat members: missing member list")
		}
		for _, member := range list.GetMembers() {
			if member == nil || strings.TrimSpace(member.GetProfileId()) == "" {
				return nil, fmt.Errorf("chat members: invalid member metadata")
			}
			profileID := strings.TrimSpace(member.GetProfileId())
			if _, duplicate := seenProfiles[profileID]; duplicate {
				return nil, fmt.Errorf("chat members: duplicate member metadata")
			}
			seenProfiles[profileID] = struct{}{}
		}
		next := strings.TrimSpace(list.GetNextCursor())
		if len(list.GetMembers()) == 0 && cursor != "" {
			return nil, fmt.Errorf("chat members: empty continuation page")
		}
		if len(list.GetMembers()) == 0 && next != "" {
			return nil, fmt.Errorf("chat members: empty initial page with continuation")
		}
		out = append(out, membersFromList(list)...)
		if next == "" {
			return out, nil
		}
		if _, duplicate := seenCursors[next]; duplicate || next == cursor {
			return nil, fmt.Errorf("chat members: non-progressing member cursor")
		}
		seenCursors[next] = struct{}{}
		cursor = next
	}
}

func membersFromList(list *chatv1.MemberList) []Member {
	if list == nil {
		return nil
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
				IsArchived:  m.GetIsArchived(),
			})
		}
	}
	return out
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

// NoopLister fails closed so MessageSent retries while Chat metadata is unavailable.
type NoopLister struct{}

func (NoopLister) ListMemberProfileIDs(context.Context, string) ([]string, error) {
	return nil, fmt.Errorf("chat members lister unavailable")
}

func (NoopLister) ListMembers(context.Context, string) ([]Member, error) {
	return nil, fmt.Errorf("chat members lister unavailable")
}
