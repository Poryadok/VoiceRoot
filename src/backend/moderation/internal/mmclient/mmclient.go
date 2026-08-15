package mmclient

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// PlatformMMBanClient syncs moderation mm_ban sanctions with Matchmaking.
type PlatformMMBanClient interface {
	ApplyPlatformMMBan(ctx context.Context, targetAccountID, bannedByProfileID uuid.UUID, reason string, expiresAt *time.Time) error
	RevokePlatformMMBan(ctx context.Context, targetAccountID uuid.UUID) error
}

type GRPCPlatformMMBan struct {
	Client matchmakingv1.MatchmakingServiceClient
}

func Dial(addr string) (*GRPCPlatformMMBan, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &GRPCPlatformMMBan{Client: matchmakingv1.NewMatchmakingServiceClient(conn)}, nil
}

func (c *GRPCPlatformMMBan) ApplyPlatformMMBan(ctx context.Context, targetAccountID, bannedByProfileID uuid.UUID, reason string, expiresAt *time.Time) error {
	if c == nil || c.Client == nil {
		return nil
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal", "true")
	req := &matchmakingv1.ApplyPlatformMMBanRequest{
		TargetAccountId:   targetAccountID.String(),
		Reason:            strings.TrimSpace(reason),
		BannedByProfileId: bannedByProfileID.String(),
	}
	if expiresAt != nil {
		req.ExpiresAt = timestamppb.New(expiresAt.UTC())
	}
	_, err := c.Client.ApplyPlatformMMBan(ctx, req)
	return err
}

func (c *GRPCPlatformMMBan) RevokePlatformMMBan(ctx context.Context, targetAccountID uuid.UUID) error {
	if c == nil || c.Client == nil {
		return nil
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal", "true")
	_, err := c.Client.RevokePlatformMMBan(ctx, &matchmakingv1.RevokePlatformMMBanRequest{
		TargetAccountId: targetAccountID.String(),
	})
	return err
}
