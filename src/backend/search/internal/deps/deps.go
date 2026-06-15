package deps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"voice/backend/pkg/grpcclient"
	"voice/backend/search/internal/authctx"
	"voice/backend/search/internal/s2s"

	commonv1 "voice.app/voice/common/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	socialv1 "voice.app/voice/social/v1"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

// MessagingFetcher loads message bodies for indexing.
type MessagingFetcher struct {
	Client messagingv1.MessagingServiceClient
}

func (m *MessagingFetcher) GetMessageBody(ctx context.Context, chatID, messageID uuid.UUID) (string, time.Time, error) {
	if m == nil || m.Client == nil {
		return "", time.Time{}, fmt.Errorf("messaging client unavailable")
	}
	ctx = s2s.ForwardIncomingMetadata(ctx)
	resp, err := m.Client.GetMessage(ctx, &messagingv1.GetMessageRequest{
		MessageId: messageID.String(),
	})
	if err != nil {
		return "", time.Time{}, err
	}
	msg := resp.GetMessage()
	if msg == nil {
		return "", time.Time{}, fmt.Errorf("message not found")
	}
	created := msg.GetCreatedAt().AsTime()
	return msg.GetContent(), created, nil
}

// MessageRow is a message body for bulk reindex.
type MessageRow struct {
	ID              uuid.UUID
	SenderProfileID uuid.UUID
	Body            string
	CreatedAt       time.Time
}

// ListChatMessages pages messages in a chat for reindex.
func (m *MessagingFetcher) ListChatMessages(ctx context.Context, chatID uuid.UUID, cursor string, pageSize int32) ([]MessageRow, string, error) {
	if m == nil || m.Client == nil {
		return nil, "", fmt.Errorf("messaging client unavailable")
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	ctx = s2s.ForwardIncomingMetadata(ctx)
	resp, err := m.Client.GetMessages(ctx, &messagingv1.GetMessagesRequest{
		Chat: &chatv1.ChatRef{Id: chatID.String()},
		Page: &commonv1.CursorPageRequest{
			PageSize: pageSize,
			Cursor:   cursor,
		},
	})
	if err != nil {
		return nil, "", err
	}
	list := resp.GetMessageList()
	if list == nil {
		return nil, "", nil
	}
	out := make([]MessageRow, 0, len(list.GetMessages()))
	for _, msg := range list.GetMessages() {
		if msg == nil {
			continue
		}
		id, err := uuid.Parse(strings.TrimSpace(msg.GetId()))
		if err != nil {
			continue
		}
		sender, _ := uuid.Parse(strings.TrimSpace(msg.GetSenderProfileId()))
		out = append(out, MessageRow{
			ID:              id,
			SenderProfileID: sender,
			Body:            msg.GetContent(),
			CreatedAt:       msg.GetCreatedAt().AsTime(),
		})
	}
	return out, strings.TrimSpace(list.GetNextCursor()), nil
}

// RoleAccess checks TEXT_CHAT_VIEW for space channels.
type RoleAccess struct {
	Client rolev1.RoleServiceClient
}

func (r *RoleAccess) CanReadMessages(ctx context.Context, viewer, chatID uuid.UUID) (bool, error) {
	if r == nil || r.Client == nil {
		return true, nil
	}
	ctx = s2s.ForwardIncomingMetadata(ctx)
	resp, err := r.Client.CheckPermission(ctx, &rolev1.CheckPermissionRequest{
		ProfileId:      viewer.String(),
		PermissionName: "TEXT_CHAT_VIEW",
		Chat:           &chatv1.ChatRef{Id: chatID.String()},
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return true, nil
		}
		return false, err
	}
	return resp.GetAllowed(), nil
}

// SocialBlocks lists blocked account IDs for the viewer account.
type SocialBlocks struct {
	Client socialv1.SocialServiceClient
}

func (s *SocialBlocks) BlockedAccountIDs(ctx context.Context) ([]uuid.UUID, error) {
	if s == nil || s.Client == nil {
		return nil, nil
	}
	if _, ok := authctx.AccountID(ctx); !ok {
		return nil, nil
	}
	var out []uuid.UUID
	seen := make(map[uuid.UUID]struct{})
	var cursor string
	for {
		ctx = s2s.ForwardIncomingMetadata(ctx)
		resp, err := s.Client.ListBlocked(ctx, &socialv1.ListBlockedRequest{
			Page: &commonv1.CursorPageRequest{
				PageSize: 100,
				Cursor:   cursor,
			},
		})
		if err != nil {
			return nil, err
		}
		list := resp.GetBlockedList()
		if list == nil {
			break
		}
		for _, row := range list.GetBlocked() {
			id, err := uuid.Parse(strings.TrimSpace(row.GetBlockedAccountId()))
			if err != nil {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
		cursor = strings.TrimSpace(list.GetNextCursor())
		if cursor == "" {
			break
		}
	}
	return out, nil
}

// ChatMembership resolves chats visible to a profile via Chat Service.
type ChatMembership struct {
	Client chatv1.ChatServiceClient
}

func (c *ChatMembership) AccessibleChatIDs(ctx context.Context, viewer uuid.UUID) ([]uuid.UUID, error) {
	if c == nil || c.Client == nil {
		return nil, nil
	}
	ctx = s2s.ForwardIncomingMetadata(ctx)
	resp, err := c.Client.ListChats(ctx, &chatv1.ListChatsRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0)
	for _, item := range resp.GetChatList().GetItems() {
		ch := item.GetChat()
		if ch == nil {
			continue
		}
		id, err := uuid.Parse(ch.GetId())
		if err != nil {
			continue
		}
		out = append(out, id)
		_ = viewer
	}
	return out, nil
}

// ProfileHydrator loads profile fields for search projections.
type ProfileHydrator struct {
	Client userv1.UserServiceClient
}

func (p *ProfileHydrator) LoadProfile(ctx context.Context, profileID uuid.UUID) (uuid.UUID, string, string, string, string, error) {
	if p == nil || p.Client == nil {
		return uuid.Nil, "", "", "", "", fmt.Errorf("user client unavailable")
	}
	ctx = s2s.ForwardIncomingMetadata(ctx)
	resp, err := p.Client.GetProfile(ctx, &userv1.GetProfileRequest{
		By: &userv1.GetProfileRequest_ProfileId{ProfileId: profileID.String()},
	})
	if err != nil {
		return uuid.Nil, "", "", "", "", err
	}
	prof := resp.GetProfile()
	if prof == nil {
		return uuid.Nil, "", "", "", "", fmt.Errorf("profile not found")
	}
	accountID, err := uuid.Parse(prof.GetAccountId())
	if err != nil {
		return uuid.Nil, "", "", "", "", err
	}
	return accountID, prof.GetUsername(), prof.GetDiscriminator(), prof.GetDisplayName(), prof.GetVerificationType(), nil
}

// ChatHydrator loads chat titles for search projections.
type ChatHydrator struct {
	Client chatv1.ChatServiceClient
}

func (c *ChatHydrator) LoadChatTitle(ctx context.Context, chatID uuid.UUID) (string, error) {
	if c == nil || c.Client == nil {
		return "", fmt.Errorf("chat client unavailable")
	}
	ctx = s2s.ForwardIncomingMetadata(ctx)
	resp, err := c.Client.GetChat(ctx, &chatv1.GetChatRequest{ChatId: chatID.String()})
	if err != nil {
		return "", err
	}
	ch := resp.GetChat()
	if ch == nil {
		return "", fmt.Errorf("chat not found")
	}
	if name := strings.TrimSpace(ch.GetName()); name != "" {
		return name, nil
	}
	return chatID.String(), nil
}

// SpaceHydrator loads space catalog fields for search projections.
type SpaceHydrator struct {
	Client spacev1.SpaceServiceClient
}

func (h *SpaceHydrator) LoadSpace(ctx context.Context, spaceID uuid.UUID) (string, string, string, int, error) {
	if h == nil || h.Client == nil {
		return "", "", "", 0, fmt.Errorf("space client unavailable")
	}
	ctx = s2s.ForwardIncomingMetadata(ctx)
	resp, err := h.Client.GetSpace(ctx, &spacev1.GetSpaceRequest{SpaceId: spaceID.String()})
	if err != nil {
		return "", "", "", 0, err
	}
	sp := resp.GetSpace()
	if sp == nil {
		return "", "", "", 0, fmt.Errorf("space not found")
	}
	return sp.GetName(), sp.GetDescription(), sp.GetVisibility(), int(sp.GetMemberCount()), nil
}

// DialGRPC opens an insecure client connection.
func DialGRPC(addr string) (*grpc.ClientConn, error) {
	addr = grpcclient.DialTarget(strings.TrimSpace(addr))
	if addr == "" {
		return nil, fmt.Errorf("empty grpc addr")
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
