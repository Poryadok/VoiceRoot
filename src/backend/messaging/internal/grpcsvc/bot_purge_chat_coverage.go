package grpcsvc

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/botpinprincipal"
	"voice/backend/messaging/internal/store"
)

type BotPurgeCoverageStore interface {
	ObserveBotPurgeChatCoverage(context.Context, store.BotPurgeChatCoverageInput) (*store.BotPurgeChatCoverage, error)
}

func (s *BotPinCleanupGRPC) GetBotPurgeChatCoverage(ctx context.Context, request *messagingv1.GetBotPurgeChatCoverageRequest) (*messagingv1.GetBotPurgeChatCoverageResponse, error) {
	v, err := botpinprincipal.RequireCoverage(ctx, request)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "verified Bot purge coverage required")
	}
	q, err := v.Request()
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid Bot purge coverage proof")
	}
	if s == nil || s.Coverage == nil {
		return nil, status.Error(codes.Unavailable, "Bot purge coverage store unavailable")
	}
	input := store.BotPurgeChatCoverageInput{SpaceID: uuid.MustParse(q.SpaceId), DeletionOperationID: uuid.MustParse(q.DeletionOperationId), PurgeGeneration: q.Generation, Manifest: proto.Clone(q.Manifest).(*commonv1.ManifestBinding)}
	for _, id := range q.ChatIds {
		input.Chats = append(input.Chats, uuid.MustParse(id))
	}
	observation, err := s.Coverage.ObserveBotPurgeChatCoverage(ctx, input)
	if v.Validate(q) != nil {
		return nil, status.Error(codes.Unauthenticated, "Bot purge coverage proof expired or changed")
	}
	if errors.Is(err, store.ErrSpaceManifestBinding) || errors.Is(err, store.ErrSpaceManifestNotSealed) {
		return nil, status.Error(codes.FailedPrecondition, "Bot purge coverage source unavailable")
	}
	if err != nil || observation == nil || observation.SourceScheduleGeneration == 0 || len(observation.Covered) != len(q.ChatIds) {
		return nil, status.Error(codes.Unavailable, "Bot purge coverage observation unavailable")
	}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil {
		return nil, status.Error(codes.Internal, "Bot purge coverage encoding failed")
	}
	hash := sha256.Sum256(raw)
	response := &messagingv1.GetBotPurgeChatCoverageResponse{ProtocolVersion: 1, RequestId: q.RequestId, SpaceId: q.SpaceId, DeletionOperationId: q.DeletionOperationId, Generation: q.Generation, Manifest: proto.Clone(q.Manifest).(*commonv1.ManifestBinding), SourceScheduleGeneration: observation.SourceScheduleGeneration, RequestSha256: hash[:]}
	for i, id := range q.ChatIds {
		response.Coverage = append(response.Coverage, &messagingv1.BotPurgeChatCoverage{ChatId: id, Covered: observation.Covered[i]})
	}
	return response, nil
}
