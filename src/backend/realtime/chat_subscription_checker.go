package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

// chatSubscriptionChecker asks Chat whether this active profile may currently
// view a chat. Any error is deliberately fail-closed at the WebSocket boundary.
type chatSubscriptionChecker interface {
	AuthorizeChat(ctx context.Context, accountID, profileID, chatID string) error
}

// chatSideEffectChecker revalidates the authoritative Social block state for
// an already-open DM subscription. Non-DM subscriptions need no Social read.
type chatSideEffectChecker interface {
	AuthorizeSideEffect(ctx context.Context, accountID, chatID string) error
}

type chatSubscriptionRegistrar interface {
	AuthorizeAndAddChat(ctx context.Context, accountID, profileID, chatID string, reg *connReg) error
}

type grpcChatSubscriptionChecker struct {
	chat   chatSubscriptionChatClient
	user   chatSubscriptionUserClient
	social chatSubscriptionSocialClient
	hub    *wsHub
}

type chatSubscriptionChatClient interface {
	GetChat(context.Context, *chatv1.GetChatRequest, ...grpc.CallOption) (*chatv1.GetChatResponse, error)
	ListMembers(context.Context, *chatv1.ListMembersRequest, ...grpc.CallOption) (*chatv1.ListMembersResponse, error)
}

type chatSubscriptionUserClient interface {
	GetProfile(context.Context, *userv1.GetProfileRequest, ...grpc.CallOption) (*userv1.GetProfileResponse, error)
}

type chatSubscriptionSocialClient interface {
	IsBlocked(context.Context, *socialv1.IsBlockedRequest, ...grpc.CallOption) (*socialv1.IsBlockedResponse, error)
}

var chatSubscriptionCheckTimeout = 2 * time.Second

func newGRPCChatSubscriptionChecker(cc *grpc.ClientConn) chatSubscriptionChecker {
	if cc == nil {
		return &grpcChatSubscriptionChecker{}
	}
	return &grpcChatSubscriptionChecker{chat: chatv1.NewChatServiceClient(cc)}
}

func newChatSubscriptionPolicy(chat chatSubscriptionChatClient, user chatSubscriptionUserClient, social chatSubscriptionSocialClient, hub *wsHub) chatSubscriptionChecker {
	return &grpcChatSubscriptionChecker{chat: chat, user: user, social: social, hub: hub}
}

func newGRPCChatSubscriptionPolicy(chatCC, userCC, socialCC grpc.ClientConnInterface, hub *wsHub) chatSubscriptionChecker {
	checker := &grpcChatSubscriptionChecker{hub: hub}
	if chatCC != nil {
		checker.chat = chatv1.NewChatServiceClient(chatCC)
	}
	if userCC != nil {
		checker.user = userv1.NewUserServiceClient(userCC)
	}
	if socialCC != nil {
		checker.social = socialv1.NewSocialServiceClient(socialCC)
	}
	return checker
}

func (g *grpcChatSubscriptionChecker) AuthorizeChat(ctx context.Context, accountID, profileID, chatID string) error {
	return g.authorizeChat(ctx, accountID, profileID, chatID, nil)
}

func (g *grpcChatSubscriptionChecker) AuthorizeAndAddChat(ctx context.Context, accountID, profileID, chatID string, reg *connReg) error {
	if reg == nil {
		return fmt.Errorf("chat subscription policy requires a connection")
	}
	return g.authorizeChat(ctx, accountID, profileID, chatID, reg)
}

func (g *grpcChatSubscriptionChecker) authorizeChat(ctx context.Context, accountID, profileID, chatID string, reg *connReg) error {
	if g == nil || g.chat == nil {
		return fmt.Errorf("chat subscription checker is not configured")
	}
	accountID = strings.TrimSpace(accountID)
	profileID = strings.TrimSpace(profileID)
	chatID = strings.TrimSpace(chatID)
	if accountID == "" || profileID == "" || chatID == "" {
		return fmt.Errorf("chat subscription checker requires account_id, profile_id and chat_id")
	}
	requestedChatID, err := uuid.Parse(chatID)
	if err != nil {
		return fmt.Errorf("chat subscription checker requires a valid chat_id")
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
	resp, err := g.chat.GetChat(ctx, &chatv1.GetChatRequest{ChatId: chatID})
	if err != nil {
		return err
	}
	if resp.GetChat() == nil {
		return fmt.Errorf("chat subscription checker received unexpected chat")
	}
	returnedChatID, err := uuid.Parse(strings.TrimSpace(resp.GetChat().GetId()))
	if err != nil || returnedChatID != requestedChatID {
		return fmt.Errorf("chat subscription checker received unexpected chat")
	}
	if resp.GetChat().GetType() != chatv1.ChatType_CHAT_TYPE_DM {
		if reg != nil && !g.hub.addChat(reg, chatID) {
			return status.Error(codes.PermissionDenied, "chat subscription denied")
		}
		return nil
	}
	if g.user == nil || g.social == nil || g.hub == nil {
		return status.Error(codes.Unavailable, "DM subscription policy is not configured")
	}
	membersResp, err := g.chat.ListMembers(ctx, &chatv1.ListMembersRequest{
		ChatId: chatID,
		Page:   &commonv1.CursorPageRequest{PageSize: 3},
	})
	if err != nil {
		return err
	}
	members := membersResp.GetMemberList().GetMembers()
	if len(members) != 2 || membersResp.GetMemberList().GetNextCursor() != "" {
		return fmt.Errorf("DM subscription policy received unexpected members")
	}
	canonicalProfileID := canonicalUUID(profileID)
	peerProfileID := ""
	for _, member := range members {
		memberID, parseErr := uuid.Parse(strings.TrimSpace(member.GetProfileId()))
		if parseErr != nil {
			return fmt.Errorf("DM subscription policy received invalid member")
		}
		if memberID.String() == canonicalProfileID {
			continue
		}
		if peerProfileID != "" {
			return fmt.Errorf("DM subscription policy received unexpected members")
		}
		peerProfileID = memberID.String()
	}
	if peerProfileID == "" {
		return fmt.Errorf("DM subscription policy did not find peer")
	}
	// Resolve ownership without end-user viewer metadata. User's public profile
	// visibility path intentionally hides a blocked peer before Realtime can make
	// its own authoritative, two-direction Social decision.
	policyCtx := metadata.NewOutgoingContext(ctx, metadata.MD{})
	profileResp, err := g.user.GetProfile(policyCtx, &userv1.GetProfileRequest{
		By: &userv1.GetProfileRequest_ProfileId{ProfileId: peerProfileID},
	})
	if err != nil {
		return err
	}
	if profileResp == nil || profileResp.GetProfile() == nil {
		return fmt.Errorf("DM subscription policy received missing peer profile")
	}
	peerAccountID, err := uuid.Parse(strings.TrimSpace(profileResp.GetProfile().GetAccountId()))
	if err != nil || peerAccountID == uuid.Nil {
		return fmt.Errorf("DM subscription policy received invalid peer account")
	}
	callerAccountID := canonicalUUID(accountID)
	pair, ok := canonicalAccountPair(callerAccountID, peerAccountID.String())
	if !ok {
		return fmt.Errorf("DM subscription policy received invalid account pair")
	}
	version := g.hub.beginDMPairCheck(pair)
	finished := false
	defer func() {
		if !finished {
			g.hub.finishDMPairCheck(pair, version, nil, chatID, false)
		}
	}()
	for _, pair := range [][2]string{{callerAccountID, peerAccountID.String()}, {peerAccountID.String(), callerAccountID}} {
		blocked, blockErr := g.social.IsBlocked(policyCtx, &socialv1.IsBlockedRequest{AccountIdA: pair[0], AccountIdB: pair[1]})
		if blockErr != nil {
			return blockErr
		}
		if blocked == nil {
			return fmt.Errorf("DM subscription policy received missing Social response")
		}
		if blocked.GetBlocked() {
			return status.Error(codes.PermissionDenied, "DM subscription denied")
		}
	}
	accepted := g.hub.finishDMPairCheck(pair, version, reg, chatID, true)
	finished = true
	if !accepted {
		return status.Error(codes.PermissionDenied, "DM subscription policy changed")
	}
	return nil
}

func (g *grpcChatSubscriptionChecker) AuthorizeSideEffect(ctx context.Context, accountID, chatID string) error {
	if g == nil || g.hub == nil {
		return status.Error(codes.Unavailable, "DM side-effect policy is not configured")
	}
	pair, isDM := g.hub.dmAccountPairForLocalChat(accountID, chatID)
	if !isDM {
		return nil
	}
	if g.social == nil {
		return status.Error(codes.Unavailable, "DM side-effect policy is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, chatSubscriptionCheckTimeout)
	defer cancel()
	policyCtx := metadata.NewOutgoingContext(ctx, metadata.MD{})
	for _, direction := range [][2]string{{pair.first, pair.second}, {pair.second, pair.first}} {
		blocked, err := g.social.IsBlocked(policyCtx, &socialv1.IsBlockedRequest{
			AccountIdA: direction[0],
			AccountIdB: direction[1],
		})
		if err != nil {
			return err
		}
		if blocked == nil {
			return fmt.Errorf("DM side-effect policy received missing Social response")
		}
		if blocked.GetBlocked() {
			// The event is only an eager local revoke. The authoritative read is
			// also allowed to repair this instance when publication/delivery failed.
			g.hub.revokeAccountPairDMChats(pair.first, pair.second)
			return status.Error(codes.PermissionDenied, "DM side effect denied")
		}
	}
	return nil
}
