package s2s

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	botv1 "voice.app/voice/bot/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/chatcreation"
)

type BotInstallationClient interface {
	CheckBotInstallationScope(context.Context, *botv1.CheckBotInstallationScopeRequest, ...grpc.CallOption) (*botv1.CheckBotInstallationScopeResponse, error)
}

type BotInstallationDecision struct {
	Issuer *principal.Issuer
	Client BotInstallationClient
}

func (d *BotInstallationDecision) CheckCurrentBotInstallation(ctx context.Context, b *spacev1.SpaceChatCreationBinding) error {
	if chatcreation.ValidateBinding(b) != nil || b.GetBot() == nil {
		return status.Error(codes.InvalidArgument, "invalid bot composite intent")
	}
	if d == nil || d.Issuer == nil || d.Client == nil {
		return status.Error(codes.Unavailable, "Bot installation authority unavailable")
	}
	bot := b.GetBot()
	req := &botv1.CheckBotInstallationScopeRequest{BotId: bot.BotId, SpaceId: b.SpaceId, ActorProfileId: b.CreatorProfileId, InstallationId: bot.InstallationId, BotScope: bot.BotScope}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid installation decision")
	}
	token, err := d.Issuer.IssueService(principal.ServiceInput{Audience: "bot", RPC: botv1.BotService_CheckBotInstallationScope_FullMethodName, RequestID: b.OperationId, RequestHash: hash})
	if err != nil {
		return status.Error(codes.Unavailable, "installation decision credential unavailable")
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", b.OperationId))
	result, err := d.Client.CheckBotInstallationScope(call, req)
	// Authentication/transport/authority failures never reserve a terminal abort.
	if err != nil {
		return status.Error(codes.Unavailable, "Bot installation authority unavailable")
	}
	if result == nil || chatcreation.HasUnknown(result) || result.BotId != req.BotId || result.SpaceId != req.SpaceId || result.ActorProfileId != req.ActorProfileId || result.InstallationId != req.InstallationId || result.BotScope != req.BotScope {
		return status.Error(codes.Unavailable, "invalid Bot installation decision")
	}
	if !result.Allowed {
		return status.Error(codes.PermissionDenied, "Bot installation scope denied")
	}
	return nil
}
