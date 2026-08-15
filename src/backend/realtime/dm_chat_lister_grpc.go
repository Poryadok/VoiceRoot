package main

import (
	"context"
	"fmt"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// gRPC metadata keys aligned with Chat Service / Gateway downstream (see chat internal authctx).
const (
	grpcMDVoiceUserID    = "x-voice-user-id"
	grpcMDVoiceProfileID = "x-voice-profile-id"
)

type grpcDMChatLister struct {
	client chatv1.ChatServiceClient
}

func newGRPCDMChatLister(cc *grpc.ClientConn) *grpcDMChatLister {
	if cc == nil {
		return nil
	}
	return &grpcDMChatLister{client: chatv1.NewChatServiceClient(cc)}
}

// ListChatIDs pages Chat.ListChats and collects all chat IDs visible to the profile.
func (g *grpcDMChatLister) ListChatIDs(ctx context.Context, accountID, profileID string) ([]string, error) {
	if g == nil || g.client == nil {
		return nil, fmt.Errorf("chat client not configured")
	}
	ctx = metadata.AppendToOutgoingContext(ctx, grpcMDVoiceUserID, accountID, grpcMDVoiceProfileID, profileID)

	const pageSize int32 = 100
	var out []string
	cursor := ""
	for {
		resp, err := g.client.ListChats(ctx, &chatv1.ListChatsRequest{
			Page: &commonv1.CursorPageRequest{
				Cursor:   cursor,
				PageSize: pageSize,
			},
		})
		if err != nil {
			return out, err
		}
		cl := resp.GetChatList()
		for _, item := range cl.GetItems() {
			ch := item.GetChat()
			if ch == nil {
				continue
			}
			if ch.GetId() != "" {
				out = append(out, ch.GetId())
			}
		}
		cursor = cl.GetNextCursor()
		if cursor == "" {
			break
		}
	}
	return out, nil
}
