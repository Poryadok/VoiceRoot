package s2s

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/messaging/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

// GRPCChatGuard validates membership via ChatService.ListMembers (S2S; forwards caller metadata).
type GRPCChatGuard struct {
	Client chatv1.ChatServiceClient
}

func NewGRPCChatGuard(c chatv1.ChatServiceClient) *GRPCChatGuard {
	return &GRPCChatGuard{Client: c}
}

func (g *GRPCChatGuard) dmMembers(ctx context.Context, chatID uuid.UUID) ([]*chatv1.ChatMember, error) {
	if g == nil || g.Client == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat service not configured")
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "messaging"))
	resp, err := g.Client.ListMembers(ctx, &chatv1.ListMembersRequest{
		ChatId: chatID.String(),
		Page:   &commonv1.CursorPageRequest{PageSize: 100},
	})
	if err != nil {
		return nil, err
	}
	return resp.GetMemberList().GetMembers(), nil
}

// DMReceiptVisibilityTargets lists every durable DM for a profile. It uses
// Chat's dedicated Messaging-only RPC, never the caller-scoped ListChats API.
func (g *GRPCChatGuard) DMReceiptVisibilityTargets(ctx context.Context, profileID uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	if g == nil || g.Client == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat service not configured")
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "messaging"))
	out := map[uuid.UUID]uuid.UUID{}
	cursor := ""
	for {
		resp, err := g.Client.ListDMReceiptVisibilityTargets(ctx, &chatv1.ListDMReceiptVisibilityTargetsRequest{ProfileId: profileID.String(), Page: &commonv1.CursorPageRequest{Cursor: cursor, PageSize: 500}})
		if err != nil {
			return nil, err
		}
		for _, target := range resp.GetTargets() {
			chatID, err := uuid.Parse(target.GetChatId())
			if err != nil {
				return nil, status.Error(codes.Internal, "invalid chat receipt target")
			}
			peerID, err := uuid.Parse(target.GetPeerProfileId())
			if err != nil {
				return nil, status.Error(codes.Internal, "invalid peer receipt target")
			}
			out[chatID] = peerID
		}
		cursor = resp.GetNextCursor()
		if cursor == "" {
			return out, nil
		}
	}
}

func (g *GRPCChatGuard) EnsureMember(ctx context.Context, chatID, profileID uuid.UUID) error {
	members, err := g.dmMembers(ctx, chatID)
	if err != nil {
		return grpcMemberErr(err)
	}
	for _, m := range members {
		if strings.EqualFold(m.GetProfileId(), profileID.String()) {
			return nil
		}
	}
	return store.ErrNotChatMember
}

// MessageReadEntitled asks Chat to evaluate the immutable creation time against
// the managed membership interval ledger. The caller identity is set locally;
// user supplied metadata cannot select the internal service principal.
func (g *GRPCChatGuard) MessageReadEntitled(ctx context.Context, chatID, profileID uuid.UUID, createdAt time.Time) (bool, error) {
	if g == nil || g.Client == nil {
		return false, status.Error(codes.FailedPrecondition, "chat service not configured")
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-internal-caller", "messaging"))
	resp, err := g.Client.CheckMessageReadEntitlement(ctx, &chatv1.CheckMessageReadEntitlementRequest{
		ChatId: chatID.String(), ProfileId: profileID.String(), MessageCreatedAt: timestamppb.New(createdAt),
	})
	if err != nil {
		return false, grpcMemberErr(err)
	}
	return resp.GetEntitled(), nil
}

func (g *GRPCChatGuard) DMOtherProfileID(ctx context.Context, chatID, profileID uuid.UUID) (uuid.UUID, error) {
	members, err := g.dmMembers(ctx, chatID)
	if err != nil {
		return uuid.Nil, grpcMemberErr(err)
	}
	var peer uuid.UUID
	seenSelf := false
	for _, m := range members {
		pid, perr := uuid.Parse(strings.TrimSpace(m.GetProfileId()))
		if perr != nil {
			return uuid.Nil, status.Error(codes.Internal, "invalid profile_id on chat member")
		}
		if pid == profileID {
			seenSelf = true
			continue
		}
		if peer != uuid.Nil {
			return uuid.Nil, status.Error(codes.FailedPrecondition, "dm must have exactly two members")
		}
		peer = pid
	}
	if !seenSelf {
		return uuid.Nil, store.ErrNotChatMember
	}
	if peer == uuid.Nil {
		return uuid.Nil, status.Error(codes.FailedPrecondition, "dm must have exactly two members")
	}
	return peer, nil
}

func (g *GRPCChatGuard) OtherMemberProfileIDs(ctx context.Context, chatID, profileID uuid.UUID) ([]uuid.UUID, error) {
	members, err := g.dmMembers(ctx, chatID)
	if err != nil {
		return nil, grpcMemberErr(err)
	}
	var out []uuid.UUID
	seenSelf := false
	for _, m := range members {
		pid, perr := uuid.Parse(strings.TrimSpace(m.GetProfileId()))
		if perr != nil {
			return nil, status.Error(codes.Internal, "invalid profile_id on chat member")
		}
		if pid == profileID {
			seenSelf = true
			continue
		}
		out = append(out, pid)
	}
	if !seenSelf {
		return nil, store.ErrNotChatMember
	}
	return out, nil
}

func (g *GRPCChatGuard) MemberRole(ctx context.Context, chatID, profileID uuid.UUID) (string, error) {
	members, err := g.dmMembers(ctx, chatID)
	if err != nil {
		return "", grpcMemberErr(err)
	}
	for _, m := range members {
		if strings.EqualFold(m.GetProfileId(), profileID.String()) {
			return strings.TrimSpace(m.GetRole()), nil
		}
	}
	return "", store.ErrNotChatMember
}

func grpcMemberErr(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	switch st.Code() {
	case codes.PermissionDenied, codes.NotFound:
		return store.ErrNotChatMember
	default:
		return err
	}
}
