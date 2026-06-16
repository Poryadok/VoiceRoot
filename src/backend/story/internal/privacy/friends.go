package privacy

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/pkg/grpcclient"

	socialv1 "voice.app/voice/social/v1"
)

// FriendChecker uses Social Service to resolve friendship.
type FriendChecker struct {
	client socialv1.SocialServiceClient
	logger *slog.Logger
}

// NewFriendChecker dials Social when SOCIAL_GRPC_ADDR is set.
func NewFriendChecker(logger *slog.Logger) *FriendChecker {
	addr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR"))
	if addr == "" {
		return &FriendChecker{logger: logger}
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(addr), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		if logger != nil {
			logger.Error("story social dial failed", slog.String("error", err.Error()))
		}
		return &FriendChecker{logger: logger}
	}
	return &FriendChecker{client: socialv1.NewSocialServiceClient(conn), logger: logger}
}

// IsFriend returns whether viewer and author are friends.
func (f *FriendChecker) IsFriend(ctx context.Context, viewerProfileID, authorProfileID uuid.UUID) (bool, error) {
	if f == nil || f.client == nil {
		return false, nil
	}
	resp, err := f.client.AreFriends(ctx, &socialv1.AreFriendsRequest{
		ProfileIdA: viewerProfileID.String(),
		ProfileIdB: authorProfileID.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetFriends(), nil
}
