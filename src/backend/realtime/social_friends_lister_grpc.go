package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	commonv1 "voice.app/voice/common/v1"
	socialv1 "voice.app/voice/social/v1"
)

// friendLister resolves accepted friend profile IDs for presence fan-out.
type friendLister interface {
	ListFriendProfileIDs(ctx context.Context, profileID string) ([]string, error)
}

type grpcFriendLister struct {
	client socialv1.SocialServiceClient
}

func newGRPCFriendLister(cc *grpc.ClientConn) *grpcFriendLister {
	if cc == nil {
		return nil
	}
	return &grpcFriendLister{client: socialv1.NewSocialServiceClient(cc)}
}

func (g *grpcFriendLister) ListFriendProfileIDs(ctx context.Context, profileID string) ([]string, error) {
	if g == nil || g.client == nil {
		return nil, fmt.Errorf("social client not configured")
	}
	ctx = metadata.AppendToOutgoingContext(ctx, grpcMDVoiceProfileID, profileID)

	const pageSize int32 = 100
	var out []string
	cursor := ""
	for {
		resp, err := g.client.ListFriends(ctx, &socialv1.ListFriendsRequest{
			Page: &commonv1.CursorPageRequest{
				Cursor:   cursor,
				PageSize: pageSize,
			},
		})
		if err != nil {
			return out, err
		}
		list := resp.GetFriendList()
		if list == nil {
			break
		}
		for _, edge := range list.GetFriends() {
			if id := edge.GetProfileId(); id != "" {
				out = append(out, id)
			}
		}
		next := list.GetNextCursor()
		if next == "" {
			break
		}
		cursor = next
	}
	return out, nil
}
