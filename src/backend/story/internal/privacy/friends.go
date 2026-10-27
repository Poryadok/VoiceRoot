package privacy

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"voice/backend/pkg/grpcclient"

	commonv1 "voice.app/voice/common/v1"
	socialv1 "voice.app/voice/social/v1"
	spacev1 "voice.app/voice/space/v1"
)

// FriendChecker uses Social Service for friendship and feed author resolution.
type FriendChecker struct {
	client      socialv1.SocialServiceClient
	spaceClient spacev1.SpaceServiceClient
	logger      *slog.Logger
}

// NewFriendChecker dials Social when SOCIAL_GRPC_ADDR is set.
func NewFriendChecker(logger *slog.Logger) *FriendChecker {
	addr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR"))
	var client socialv1.SocialServiceClient
	if addr != "" {
		conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			if logger != nil {
				logger.Error("story social dial failed", slog.String("error", err.Error()))
			}
		} else {
			client = socialv1.NewSocialServiceClient(conn)
		}
	}
	var spaceClient spacev1.SpaceServiceClient
	if spaceAddr := strings.TrimSpace(os.Getenv("SPACE_GRPC_ADDR")); spaceAddr != "" {
		conn, err := grpc.NewClient(grpcclient.DialTarget(spaceAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			if logger != nil {
				logger.Error("story space dial failed", slog.String("error", err.Error()))
			}
		} else {
			spaceClient = spacev1.NewSpaceServiceClient(conn)
		}
	}
	return &FriendChecker{client: client, spaceClient: spaceClient, logger: logger}
}

// IsFriend returns whether viewer and author are friends.
func (f *FriendChecker) IsFriend(ctx context.Context, viewerProfileID, authorProfileID uuid.UUID) (bool, error) {
	ok, err := f.AreFriends(ctx, viewerProfileID, authorProfileID)
	return ok, err
}

// AreFriends implements privacy.SocialGraph.
func (f *FriendChecker) AreFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	if f == nil || f.client == nil {
		return false, nil
	}
	resp, err := f.client.AreFriends(ctx, &socialv1.AreFriendsRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetFriends(), nil
}

// AreFriendsOfFriends implements privacy.SocialGraph.
func (f *FriendChecker) AreFriendsOfFriends(ctx context.Context, profileA, profileB uuid.UUID) (bool, error) {
	if f == nil || f.client == nil {
		return false, nil
	}
	resp, err := f.client.AreFriendsOfFriends(ctx, &socialv1.AreFriendsOfFriendsRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetFriends(), nil
}

// AreCoMembers implements privacy.SpaceCoMembership.
func (f *FriendChecker) AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []string) (bool, error) {
	if f == nil || f.spaceClient == nil {
		return false, nil
	}
	resp, err := f.spaceClient.AreCoMembers(ctx, &spacev1.AreCoMembersRequest{
		ProfileIdA: profileA.String(),
		ProfileIdB: profileB.String(),
		SpaceIds:   spaceIDs,
	})
	if err != nil {
		return false, err
	}
	return resp.GetCoMembers(), nil
}

// ListFeedAuthorIDs returns candidates; per-Story privacy still decides visibility.
func (f *FriendChecker) ListFeedAuthorIDs(ctx context.Context, viewerProfileID uuid.UUID) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{viewerProfileID: {}}
	out := []uuid.UUID{viewerProfileID}
	if f == nil {
		return out, nil
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set("x-voice-profile-id", viewerProfileID.String())
	ctx = metadata.NewOutgoingContext(ctx, md)
	appendAuthor := func(raw string) {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil || id == uuid.Nil {
			return
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	if f.client != nil {
		resp, err := f.client.ListFriends(ctx, &socialv1.ListFriendsRequest{})
		if err != nil {
			return nil, err
		}
		for _, edge := range resp.GetFriendList().GetFriends() {
			appendAuthor(edge.GetProfileId())
		}
	}
	if f.spaceClient == nil {
		return out, nil
	}
	// Read only the viewer's live Spaces and their current-member rosters.
	spacesSeen := make(map[uuid.UUID]struct{})
	spaceCursors := map[string]struct{}{"": {}}
	for cursor := ""; ; {
		resp, err := f.spaceClient.ListMySpaces(ctx, &spacev1.ListMySpacesRequest{
			Page: &commonv1.CursorPageRequest{PageSize: 100, Cursor: cursor},
		})
		if err != nil {
			return nil, err
		}
		if resp.GetSpaceList() == nil {
			return nil, errors.New("space feed author list unavailable")
		}
		for _, space := range resp.GetSpaceList().GetSpaces() {
			spaceID, err := uuid.Parse(space.GetId())
			if err != nil || spaceID == uuid.Nil {
				return nil, errors.New("invalid feed space id")
			}
			if _, exists := spacesSeen[spaceID]; exists {
				continue
			}
			spacesSeen[spaceID] = struct{}{}
			memberCursors := map[string]struct{}{"": {}}
			for memberCursor := ""; ; {
				members, err := f.spaceClient.ListMembers(ctx, &spacev1.ListMembersRequest{
					SpaceId: spaceID.String(),
					Page:    &commonv1.CursorPageRequest{PageSize: 100, Cursor: memberCursor},
				})
				if err != nil {
					return nil, err
				}
				if members.GetSpaceMemberList() == nil {
					return nil, errors.New("space feed roster unavailable")
				}
				for _, member := range members.GetSpaceMemberList().GetMembers() {
					appendAuthor(member.GetProfileId())
				}
				memberCursor = members.GetSpaceMemberList().GetNextCursor()
				if memberCursor == "" {
					break
				}
				if _, exists := memberCursors[memberCursor]; exists {
					return nil, errors.New("space feed roster cursor repeated")
				}
				memberCursors[memberCursor] = struct{}{}
			}
		}
		cursor = resp.GetSpaceList().GetNextCursor()
		if cursor == "" {
			break
		}
		if _, exists := spaceCursors[cursor]; exists {
			return nil, errors.New("space feed list cursor repeated")
		}
		spaceCursors[cursor] = struct{}{}
	}
	return out, nil
}
