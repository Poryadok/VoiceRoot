package privacy

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"voice/backend/pkg/grpcclient"

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
	resp, err := f.client.AreFriendsOfFriends(ctx, &socialv1.AreFriendsRequest{
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

// ListFeedAuthorIDs returns profile ids whose stories may appear in viewer's feed.
func (f *FriendChecker) ListFeedAuthorIDs(ctx context.Context, viewerProfileID uuid.UUID) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{viewerProfileID: {}}
	out := []uuid.UUID{viewerProfileID}
	if f == nil || f.client == nil {
		return out, nil
	}
	resp, err := f.client.ListFriends(
		metadata.AppendToOutgoingContext(ctx, "x-voice-profile-id", viewerProfileID.String()),
		&socialv1.ListFriendsRequest{},
	)
	if err != nil {
		return out, err
	}
	for _, edge := range resp.GetFriendList().GetFriends() {
		parsed, err := uuid.Parse(strings.TrimSpace(edge.GetProfileId()))
		if err != nil {
			continue
		}
		if _, ok := seen[parsed]; ok {
			continue
		}
		seen[parsed] = struct{}{}
		out = append(out, parsed)
	}
	return out, nil
}
