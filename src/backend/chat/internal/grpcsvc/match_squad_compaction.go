package grpcsvc

import (
	"context"
	"crypto/sha256"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func (s *MatchSquadChatGRPC) CompactMatchSquadChat(ctx context.Context, req *chatv1.CompactMatchSquadChatRequest) (*chatv1.CompactMatchSquadChatResponse, error) {
	if _, ok := principal.FromContext(ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "verified Matchmaking principal required")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "MatchSquad Chat store unavailable")
	}
	if req.GetProtocolVersion() != 1 {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad compaction request")
	}
	parseID := func(value string) (uuid.UUID, error) {
		id, err := canonicalUUID(value)
		if err != nil || id == uuid.Nil {
			return uuid.Nil, status.Error(codes.InvalidArgument, "invalid MatchSquad compaction identifier")
		}
		return id, nil
	}
	operationID, err := parseID(req.GetOperationId())
	if err != nil {
		return nil, err
	}
	aggregateID, err := parseID(req.GetTeardownAggregateId())
	if err != nil {
		return nil, err
	}
	matchID, err := parseID(req.GetMatchId())
	if err != nil {
		return nil, err
	}
	chatID, err := parseID(req.GetChatId())
	if err != nil {
		return nil, err
	}
	creationReceiptID, err := parseID(req.GetCreationReceiptId())
	if err != nil {
		return nil, err
	}
	teardownOperationID, err := parseID(req.GetTeardownOperationId())
	if err != nil {
		return nil, err
	}
	teardownReceiptID, err := parseID(req.GetTeardownReceiptId())
	if err != nil {
		return nil, err
	}
	if len(req.GetCreationRequestSha256()) != sha256.Size || len(req.GetTeardownReceiptSha256()) != sha256.Size || len(req.GetParticipantManifestSha256()) != sha256.Size {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad compaction digest")
	}
	if req.GetAggregateCompletedAt() == nil || req.GetCompactionAuthorizedAt() == nil || req.GetAggregateCompletedAt().CheckValid() != nil || req.GetCompactionAuthorizedAt().CheckValid() != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad aggregate timestamps")
	}
	aggregateCompletedAt := req.GetAggregateCompletedAt().AsTime().UTC()
	compactionAuthorizedAt := req.GetCompactionAuthorizedAt().AsTime().UTC()
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad compaction request")
	}
	receiptBytes, err := s.Store.Compact(ctx, MatchSquadCompaction{
		OperationID: operationID, AggregateID: aggregateID, MatchID: matchID, ChatID: chatID,
		CreationReceiptID: creationReceiptID, CreationRequestSHA256: req.GetCreationRequestSha256(),
		TeardownOperationID: teardownOperationID, TeardownReceiptID: teardownReceiptID,
		TeardownReceiptSHA256: req.GetTeardownReceiptSha256(), ParticipantManifestSHA256: req.GetParticipantManifestSha256(),
		AggregateCompletedAt: aggregateCompletedAt, CompactionAuthorizedAt: compactionAuthorizedAt, RequestBytes: requestBytes,
	})
	if err != nil {
		return nil, matchSquadStoreStatus(err)
	}
	receipt := new(chatv1.MatchSquadChatCompactionReceipt)
	if err := proto.Unmarshal(receiptBytes, receipt); err != nil {
		return nil, status.Error(codes.Internal, "stored MatchSquad compaction receipt is invalid")
	}
	return &chatv1.CompactMatchSquadChatResponse{Receipt: receipt}, nil
}
