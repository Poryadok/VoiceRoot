package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
)

type MatchSquadChatGRPC struct {
	chatv1.UnimplementedMatchSquadChatServiceServer
	Store *store.MatchSquadStore
}

func (s *MatchSquadChatGRPC) CreateMatchSquadChat(ctx context.Context, req *chatv1.CreateMatchSquadChatRequest) (*chatv1.CreateMatchSquadChatResponse, error) {
	if _, ok := principal.FromContext(ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "verified Matchmaking principal required")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "MatchSquad Chat store unavailable")
	}
	operationID, err := canonicalUUID(req.GetOperationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid operation id")
	}
	matchID, err := canonicalUUID(req.GetMatchId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid match id")
	}
	if req.GetProtocolVersion() != 1 || len(req.GetParticipants()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad Chat request")
	}
	participants := make([]uuid.UUID, 0, len(req.GetParticipants()))
	for _, participant := range req.GetParticipants() {
		if participant == nil {
			return nil, status.Error(codes.InvalidArgument, "invalid participant manifest")
		}
		profileID, parseErr := canonicalUUID(participant.GetProfileId())
		if parseErr != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid participant manifest")
		}
		participants = append(participants, profileID)
	}
	for i := 1; i < len(participants); i++ {
		if bytes.Compare(participants[i-1][:], participants[i][:]) >= 0 {
			return nil, status.Error(codes.InvalidArgument, "participant manifest must be sorted and unique")
		}
	}
	manifest := participantManifestHash(participants)
	if !bytes.Equal(manifest, req.GetParticipantManifestSha256()) {
		return nil, status.Error(codes.InvalidArgument, "participant manifest hash mismatch")
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad Chat request")
	}
	receiptBytes, err := s.Store.Create(ctx, operationID, matchID, participants, manifest, requestBytes)
	if err != nil {
		return nil, matchSquadStoreStatus(err)
	}
	receipt := new(chatv1.MatchSquadChatReceipt)
	if err := proto.Unmarshal(receiptBytes, receipt); err != nil {
		return nil, status.Error(codes.Internal, "stored MatchSquad receipt is invalid")
	}
	return &chatv1.CreateMatchSquadChatResponse{Receipt: receipt}, nil
}

func (s *MatchSquadChatGRPC) TeardownMatchSquadChat(ctx context.Context, req *chatv1.TeardownMatchSquadChatRequest) (*chatv1.TeardownMatchSquadChatResponse, error) {
	if _, ok := principal.FromContext(ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "verified Matchmaking principal required")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "MatchSquad Chat store unavailable")
	}
	operationID, err := canonicalUUID(req.GetTeardownOperationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid teardown operation id")
	}
	matchID, err := canonicalUUID(req.GetMatchId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid match id")
	}
	chatID, err := canonicalUUID(req.GetChatId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid chat id")
	}
	receiptID, err := canonicalUUID(req.GetCreationReceiptId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid creation receipt id")
	}
	if req.GetProtocolVersion() != 1 || len(req.GetParticipantManifestSha256()) != sha256.Size || len(req.GetCreationRequestSha256()) != sha256.Size {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad teardown request")
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid MatchSquad teardown request")
	}
	receiptBytes, err := s.Store.Teardown(ctx, operationID, matchID, chatID, receiptID, req.GetParticipantManifestSha256(), req.GetCreationRequestSha256(), requestBytes)
	if err != nil {
		return nil, matchSquadStoreStatus(err)
	}
	receipt := new(chatv1.MatchSquadChatTeardownReceipt)
	if err := proto.Unmarshal(receiptBytes, receipt); err != nil {
		return nil, status.Error(codes.Internal, "stored MatchSquad teardown receipt is invalid")
	}
	return &chatv1.TeardownMatchSquadChatResponse{Receipt: receipt}, nil
}

func matchSquadStoreStatus(err error) error {
	switch {
	case errors.Is(err, store.ErrMatchSquadNotFound):
		return status.Error(codes.NotFound, "MatchSquad Chat not found")
	case errors.Is(err, store.ErrMatchSquadConflict):
		return status.Error(codes.FailedPrecondition, "MatchSquad Chat binding mismatch")
	case errors.Is(err, store.ErrMatchSquadTerminal):
		return status.Error(codes.FailedPrecondition, "MatchSquad Chat is terminal")
	default:
		return status.Error(codes.Unavailable, "MatchSquad Chat persistence unavailable")
	}
}

func canonicalUUID(value string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed.String() != value || strings.TrimSpace(value) != value {
		return uuid.Nil, errors.New("UUID must be canonical lower-case")
	}
	return parsed, nil
}

func participantManifestHash(participants []uuid.UUID) []byte {
	ordered := append([]uuid.UUID(nil), participants...)
	sort.Slice(ordered, func(i, j int) bool { return bytes.Compare(ordered[i][:], ordered[j][:]) < 0 })
	hash := sha256.New()
	for _, participant := range ordered {
		_, _ = hash.Write(participant[:])
	}
	return hash.Sum(nil)
}

func requestSHA256(raw []byte) []byte { sum := sha256.Sum256(raw); return sum[:] }
func requestHashClaim(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
