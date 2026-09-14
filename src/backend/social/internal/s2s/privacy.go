package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"

	"voice/backend/pkg/privacy"

	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
)

type GRPCUserPrivacy struct {
	Client userv1.UserServiceClient
	Issuer *principal.Issuer
}

// Each call owns its request ID and credential; caller metadata is never authority.
func privacyS2SContext(ctx context.Context, issuer *principal.Issuer, audience, rpc string, request proto.Message) (context.Context, error) {
	if issuer == nil {
		return nil, status.Error(codes.Unauthenticated, "Social principal signer unavailable")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Social principal request binding failed")
	}
	requestID := uuid.NewString()
	token, err := issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Social principal signing failed")
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}

func (u *GRPCUserPrivacy) AllowFriendRequestsAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.Audience{}, status.Error(codes.Unavailable, "User privacy client unavailable")
	}
	req := &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	}
	ctx, err := privacyS2SContext(ctx, u.Issuer, "user", userv1.UserService_GetPrivacySettings_FullMethodName, req)
	if err != nil {
		return privacy.Audience{}, err
	}
	resp, err := u.Client.GetPrivacySettings(ctx, req)
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetAllowFriendRequests()), nil
}

func (u *GRPCUserPrivacy) AllowPhoneSearchAudience(ctx context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if u == nil || u.Client == nil {
		return privacy.Audience{}, status.Error(codes.Unavailable, "User privacy client unavailable")
	}
	req := &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	}
	ctx, err := privacyS2SContext(ctx, u.Issuer, "user", userv1.UserService_GetPrivacySettings_FullMethodName, req)
	if err != nil {
		return privacy.Audience{}, err
	}
	resp, err := u.Client.GetPrivacySettings(ctx, req)
	if err != nil {
		return privacy.Audience{}, err
	}
	return privacy.FromProto(resp.GetPrivacySettings().GetAllowPhoneSearch()), nil
}

type GRPCSpaceCoMembership struct {
	Client spacev1.SpaceServiceClient
	Issuer *principal.Issuer
}

func NewGRPCSpaceCoMembership(cc grpc.ClientConnInterface) *GRPCSpaceCoMembership {
	if cc == nil {
		return nil
	}
	return &GRPCSpaceCoMembership{Client: spacev1.NewSpaceServiceClient(cc)}
}

func (s *GRPCSpaceCoMembership) AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error) {
	if s == nil || s.Client == nil {
		return false, status.Error(codes.Unavailable, "Space co-membership client unavailable")
	}
	req := &spacev1.AreCoMembersRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
		SpaceIds:   spaceIDs,
	}
	ctx, err := privacyS2SContext(ctx, s.Issuer, "space", spacev1.SpaceService_AreCoMembers_FullMethodName, req)
	if err != nil {
		return false, err
	}
	resp, err := s.Client.AreCoMembers(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.GetCoMembers(), nil
}

type storeSocialGraph struct {
	friends interface {
		AreFriendsAccepted(context.Context, uuid.UUID, uuid.UUID) (bool, error)
		AreFriendsOfFriendsAccepted(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	}
}

func NewStoreSocialGraph(friends interface {
	AreFriendsAccepted(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	AreFriendsOfFriendsAccepted(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}) *storeSocialGraph {
	if friends == nil {
		return nil
	}
	return &storeSocialGraph{friends: friends}
}

func (g *storeSocialGraph) AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	return g.friends.AreFriendsAccepted(ctx, profileA, profileB)
}

func (g *storeSocialGraph) AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	return g.friends.AreFriendsOfFriendsAccepted(ctx, profileA, profileB)
}
