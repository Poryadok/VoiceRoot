package privacy

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	socialv1 "voice.app/voice/social/v1"
	spacev1 "voice.app/voice/space/v1"
)

type feedSocialClient struct {
	socialv1.SocialServiceClient
	friends []string
}

func (s feedSocialClient) ListFriends(context.Context, *socialv1.ListFriendsRequest, ...grpc.CallOption) (*socialv1.ListFriendsResponse, error) {
	edges := make([]*socialv1.FriendEdge, 0, len(s.friends))
	for _, id := range s.friends {
		edges = append(edges, &socialv1.FriendEdge{ProfileId: id})
	}
	return &socialv1.ListFriendsResponse{FriendList: &socialv1.FriendList{Friends: edges}}, nil
}

type feedSpaceClient struct {
	spacev1.SpaceServiceClient
	spaces  func(context.Context, *spacev1.ListMySpacesRequest) (*spacev1.ListMySpacesResponse, error)
	members func(context.Context, *spacev1.ListMembersRequest) (*spacev1.ListMembersResponse, error)
}

func (s feedSpaceClient) ListMySpaces(ctx context.Context, req *spacev1.ListMySpacesRequest, _ ...grpc.CallOption) (*spacev1.ListMySpacesResponse, error) {
	return s.spaces(ctx, req)
}

func (s feedSpaceClient) ListMembers(ctx context.Context, req *spacev1.ListMembersRequest, _ ...grpc.CallOption) (*spacev1.ListMembersResponse, error) {
	return s.members(ctx, req)
}

func feedSpaces(cursor string, ids ...string) *spacev1.ListMySpacesResponse {
	rows := make([]*spacev1.Space, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, &spacev1.Space{Id: id})
	}
	return &spacev1.ListMySpacesResponse{SpaceList: &spacev1.SpaceList{Spaces: rows, NextCursor: cursor}}
}

func feedMembers(cursor string, ids ...string) *spacev1.ListMembersResponse {
	rows := make([]*spacev1.SpaceMembership, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, &spacev1.SpaceMembership{ProfileId: id})
	}
	return &spacev1.ListMembersResponse{SpaceMemberList: &spacev1.SpaceMemberList{Members: rows, NextCursor: cursor}}
}

func TestListFeedAuthorIDsIncludesAllViewerSpaceMemberPages(t *testing.T) {
	viewer, friend, first, second := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	spaceA, spaceB := uuid.NewString(), uuid.NewString()
	var spaceCursors, memberPages []string
	checkViewer := func(ctx context.Context) {
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Equal(t, []string{viewer.String()}, md.Get("x-voice-profile-id"))
	}
	f := &FriendChecker{
		client: feedSocialClient{friends: []string{friend.String(), viewer.String()}},
		spaceClient: feedSpaceClient{
			spaces: func(ctx context.Context, req *spacev1.ListMySpacesRequest) (*spacev1.ListMySpacesResponse, error) {
				checkViewer(ctx)
				require.EqualValues(t, 100, req.GetPage().GetPageSize())
				spaceCursors = append(spaceCursors, req.GetPage().GetCursor())
				if req.GetPage().GetCursor() == "" {
					return feedSpaces("next-space", spaceA), nil
				}
				require.Equal(t, "next-space", req.GetPage().GetCursor())
				return feedSpaces("", spaceA, spaceB), nil
			},
			members: func(ctx context.Context, req *spacev1.ListMembersRequest) (*spacev1.ListMembersResponse, error) {
				checkViewer(ctx)
				require.EqualValues(t, 100, req.GetPage().GetPageSize())
				memberPages = append(memberPages, req.GetSpaceId()+":"+req.GetPage().GetCursor())
				if req.GetSpaceId() == spaceA {
					if req.GetPage().GetCursor() == "" {
						return feedMembers("next-member", viewer.String(), friend.String(), first.String()), nil
					}
					require.Equal(t, "next-member", req.GetPage().GetCursor())
					return feedMembers("", second.String(), "invalid", uuid.Nil.String()), nil
				}
				require.Equal(t, spaceB, req.GetSpaceId())
				return feedMembers("", first.String()), nil
			},
		},
	}
	got, err := f.ListFeedAuthorIDs(context.Background(), viewer)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{viewer, friend, first, second}, got)
	require.Equal(t, []string{"", "next-space"}, spaceCursors)
	require.Equal(t, []string{spaceA + ":", spaceA + ":next-member", spaceB + ":"}, memberPages)
}

func TestListFeedAuthorIDsSpaceLookupFailsWithoutPartialAuthors(t *testing.T) {
	for _, failure := range []string{"spaces", "later-spaces", "members", "later-members", "space-cycle", "member-cycle", "invalid-space", "nil-spaces", "nil-members"} {
		t.Run(failure, func(t *testing.T) {
			viewer, peer := uuid.New(), uuid.NewString()
			spaceID := uuid.NewString()
			dependencyErr := errors.New("space dependency failed")
			f := &FriendChecker{
				client: feedSocialClient{friends: []string{uuid.NewString()}},
				spaceClient: feedSpaceClient{
					spaces: func(_ context.Context, req *spacev1.ListMySpacesRequest) (*spacev1.ListMySpacesResponse, error) {
						if failure == "spaces" || (failure == "later-spaces" && req.GetPage().GetCursor() != "") {
							return feedSpaces("", spaceID), dependencyErr
						}
						if failure == "nil-spaces" {
							return nil, nil
						}
						if failure == "invalid-space" {
							return feedSpaces("", "invalid"), nil
						}
						if failure == "later-spaces" || failure == "space-cycle" {
							return feedSpaces("repeat", spaceID), nil
						}
						return feedSpaces("", spaceID), nil
					},
					members: func(_ context.Context, req *spacev1.ListMembersRequest) (*spacev1.ListMembersResponse, error) {
						if failure == "members" || (failure == "later-members" && req.GetPage().GetCursor() != "") {
							return feedMembers("", peer), dependencyErr
						}
						if failure == "nil-members" {
							return nil, nil
						}
						if failure == "later-members" || failure == "member-cycle" {
							return feedMembers("repeat", peer), nil
						}
						return feedMembers("", peer), nil
					},
				},
			}
			got, err := f.ListFeedAuthorIDs(context.Background(), viewer)
			require.Error(t, err)
			require.Nil(t, got)
		})
	}
}

func TestListFeedAuthorIDsSpaceMembersWithoutSocialClient(t *testing.T) {
	viewer, peer := uuid.New(), uuid.New()
	spaceID := uuid.NewString()
	f := &FriendChecker{spaceClient: feedSpaceClient{
		spaces: func(context.Context, *spacev1.ListMySpacesRequest) (*spacev1.ListMySpacesResponse, error) {
			return feedSpaces("", spaceID), nil
		},
		members: func(context.Context, *spacev1.ListMembersRequest) (*spacev1.ListMembersResponse, error) {
			return feedMembers("", peer.String()), nil
		},
	}}
	got, err := f.ListFeedAuthorIDs(context.Background(), viewer)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{viewer, peer}, got)
}
