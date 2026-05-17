package grpcsvc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/social/internal/authctx"
	"voice/backend/social/internal/store"

	socialv1 "voice.app/voice/social/v1"
)

const (
	friendsDefaultPage = 20
	friendsMaxPage     = 50
)

// SocialGRPC implements voice.social.v1.SocialService (friend subset + embedded defaults).
type SocialGRPC struct {
	socialv1.UnimplementedSocialServiceServer
	Friends *store.FriendshipStore
	Blocks  *store.BlockStore
}

type friendsCursorPayload struct {
	UpdatedAtUnixNano int64  `json:"ts"`
	ID                string `json:"id"`
}

func encodeFriendsCursor(updatedAt time.Time, id uuid.UUID) (string, error) {
	p := friendsCursorPayload{UpdatedAtUnixNano: updatedAt.UnixNano(), ID: id.String()}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return "f1." + base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeFriendsCursor(s string) (*store.FriendsListCursor, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if !strings.HasPrefix(s, "f1.") {
		return nil, errors.New("invalid friends cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(s, "f1."))
	if err != nil {
		return nil, errors.New("invalid friends cursor")
	}
	var p friendsCursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errors.New("invalid friends cursor")
	}
	id, err := uuid.Parse(p.ID)
	if err != nil {
		return nil, errors.New("invalid friends cursor")
	}
	ts := time.Unix(0, p.UpdatedAtUnixNano).UTC()
	return &store.FriendsListCursor{UpdatedAt: ts, ID: id}, nil
}

func parseUUIDField(name, value string) (uuid.UUID, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid %s", name)
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid %s", name)
	}
	return id, nil
}

// SendFriendInvitation implements voice.social.v1.SocialService.
func (s *SocialGRPC) SendFriendInvitation(ctx context.Context, req *socialv1.SendFriendInvitationRequest) (*socialv1.SendFriendInvitationResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	target, err := parseUUIDField("target_profile_id", req.GetTargetProfileId())
	if err != nil {
		return nil, err
	}
	if s.Friends == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	err = s.Friends.SendInvitation(ctx, caller, target)
	switch {
	case err == nil:
		return &socialv1.SendFriendInvitationResponse{}, nil
	case errors.Is(err, store.ErrSelfInvitation):
		return nil, status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, store.ErrAlreadyFriends):
		return nil, status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, store.ErrIncomingPendingExists):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	default:
		return nil, status.Error(codes.Internal, err.Error())
	}
}

// AcceptFriendInvitation implements voice.social.v1.SocialService.
func (s *SocialGRPC) AcceptFriendInvitation(ctx context.Context, req *socialv1.AcceptFriendInvitationRequest) (*socialv1.AcceptFriendInvitationResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	requester, err := parseUUIDField("requester_profile_id", req.GetRequesterProfileId())
	if err != nil {
		return nil, err
	}
	if s.Friends == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	err = s.Friends.AcceptInvitation(ctx, caller, requester)
	if err != nil {
		if errors.Is(err, store.ErrFriendshipNotFound) {
			return nil, status.Error(codes.NotFound, "pending friend request not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &socialv1.AcceptFriendInvitationResponse{}, nil
}

// DeclineFriendInvitation implements voice.social.v1.SocialService.
func (s *SocialGRPC) DeclineFriendInvitation(ctx context.Context, req *socialv1.DeclineFriendInvitationRequest) (*socialv1.DeclineFriendInvitationResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	requester, err := parseUUIDField("requester_profile_id", req.GetRequesterProfileId())
	if err != nil {
		return nil, err
	}
	if s.Friends == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	err = s.Friends.DeclineInvitation(ctx, caller, requester)
	if err != nil {
		if errors.Is(err, store.ErrFriendshipNotFound) {
			return nil, status.Error(codes.NotFound, "pending friend request not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &socialv1.DeclineFriendInvitationResponse{}, nil
}

// RemoveFriend implements voice.social.v1.SocialService.
func (s *SocialGRPC) RemoveFriend(ctx context.Context, req *socialv1.RemoveFriendRequest) (*socialv1.RemoveFriendResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	friend, err := parseUUIDField("friend_profile_id", req.GetFriendProfileId())
	if err != nil {
		return nil, err
	}
	if s.Friends == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	err = s.Friends.RemoveFriend(ctx, caller, friend)
	if err != nil {
		if errors.Is(err, store.ErrFriendshipNotFound) {
			return nil, status.Error(codes.NotFound, "friendship not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &socialv1.RemoveFriendResponse{}, nil
}

// ListFriends implements voice.social.v1.SocialService.
func (s *SocialGRPC) ListFriends(ctx context.Context, req *socialv1.ListFriendsRequest) (*socialv1.ListFriendsResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	if s.Friends == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	page := req.GetPage()
	pageSize := friendsDefaultPage
	if page != nil && page.GetPageSize() > 0 {
		pageSize = int(page.GetPageSize())
	}
	if pageSize > friendsMaxPage {
		pageSize = friendsMaxPage
	}
	cursorIn := ""
	if page != nil {
		cursorIn = page.GetCursor()
	}
	after, err := decodeFriendsCursor(cursorIn)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	rows, err := s.Friends.ListFriends(ctx, caller, after, pageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}
	edges := make([]*socialv1.FriendEdge, 0, len(rows))
	var next string
	for i, r := range rows {
		edges = append(edges, &socialv1.FriendEdge{
			ProfileId:    r.OtherProfileID.String(),
			FriendsSince: timestamppb.New(r.FriendsSince.UTC()),
		})
		if hasMore && i == len(rows)-1 {
			next, err = encodeFriendsCursor(r.FriendsSince, r.FriendshipID)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}
	return &socialv1.ListFriendsResponse{
		FriendList: &socialv1.FriendList{
			Friends:    edges,
			NextCursor: next,
		},
	}, nil
}

// ListFriendRequests implements voice.social.v1.SocialService.
func (s *SocialGRPC) ListFriendRequests(ctx context.Context, _ *socialv1.ListFriendRequestsRequest) (*socialv1.ListFriendRequestsResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	if s.Friends == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	incoming, outgoing, err := s.Friends.ListFriendRequests(ctx, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	inProto := make([]*socialv1.PendingFriendRequest, 0, len(incoming))
	for _, r := range incoming {
		inProto = append(inProto, &socialv1.PendingFriendRequest{
			ProfileId: r.RequesterProfileID.String(),
			CreatedAt: timestamppb.New(r.CreatedAt.UTC()),
		})
	}
	outProto := make([]*socialv1.PendingFriendRequest, 0, len(outgoing))
	for _, r := range outgoing {
		outProto = append(outProto, &socialv1.PendingFriendRequest{
			ProfileId: r.TargetProfileID.String(),
			CreatedAt: timestamppb.New(r.CreatedAt.UTC()),
		})
	}
	return &socialv1.ListFriendRequestsResponse{
		FriendRequestList: &socialv1.FriendRequestList{
			Incoming: inProto,
			Outgoing: outProto,
		},
	}, nil
}
