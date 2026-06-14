package s2s

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	moderationv1 "voice.app/voice/moderation/v1"
)

// GRPCPlatformModeration calls Moderation Service for shadow ban and spam checks.
type GRPCPlatformModeration struct {
	Client moderationv1.ModerationServiceClient
}

func withModerationInternal(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-voice-internal", "true")
}

func (m *GRPCPlatformModeration) IsShadowBanned(ctx context.Context, accountID uuid.UUID) (bool, error) {
	if m == nil || m.Client == nil {
		return false, status.Error(codes.FailedPrecondition, "moderation service not configured")
	}
	resp, err := m.Client.IsShadowBanned(withModerationInternal(ctx), &moderationv1.IsShadowBannedRequest{
		AccountId: accountID.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetShadowBanned(), nil
}

func (m *GRPCPlatformModeration) CheckMessageAllowed(ctx context.Context, profileID, chatID uuid.UUID, content string) error {
	if m == nil || m.Client == nil {
		return status.Error(codes.FailedPrecondition, "moderation service not configured")
	}
	resp, err := m.Client.CheckMessage(withModerationInternal(ctx), &moderationv1.CheckMessageRequest{
		Chat:            &chatv1.ChatRef{Id: chatID.String()},
		Content:          content,
		SenderProfileId:  profileID.String(),
	})
	if err != nil {
		return err
	}
	result := resp.GetCheckResult()
	if result == nil || result.GetAllowed() {
		return nil
	}
	reason := strings.TrimSpace(result.GetBlockReason())
	if reason == "" {
		reason = "message_blocked"
	}
	return fmt.Errorf("%s", reason)
}
