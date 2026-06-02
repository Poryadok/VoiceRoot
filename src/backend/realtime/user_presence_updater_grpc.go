package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	userv1 "voice.app/voice/user/v1"
)

type grpcPresenceUpdater struct {
	client userv1.UserServiceClient
}

func newGRPCPresenceUpdater(cc *grpc.ClientConn) *grpcPresenceUpdater {
	if cc == nil {
		return nil
	}
	return &grpcPresenceUpdater{client: userv1.NewUserServiceClient(cc)}
}

func (g *grpcPresenceUpdater) UpdatePresence(ctx context.Context, accountID, profileID, status, customStatus string) error {
	if g == nil || g.client == nil {
		return fmt.Errorf("user client not configured")
	}
	ctx = metadata.AppendToOutgoingContext(ctx, grpcMDVoiceUserID, accountID, grpcMDVoiceProfileID, profileID)
	req := &userv1.UpdatePresenceRequest{Status: status}
	if customStatus != "" {
		req.CustomStatus = proto.String(customStatus)
	}
	_, err := g.client.UpdatePresence(ctx, req)
	return err
}
