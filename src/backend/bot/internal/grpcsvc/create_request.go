package grpcsvc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func parseBotChatRequestID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return nil, status.Error(codes.InvalidArgument, "request_id must be a canonical UUID")
	}
	return &id, nil
}

func botChatCreateRequestHash(actorProfileID uuid.UUID, spaceID, name, chatType string) (string, error) {
	canonical := struct {
		ActorProfileID string `json:"actor_profile_id"`
		SpaceID string `json:"space_id"`
		Name    string `json:"name"`
		Type    string `json:"type"`
	}{ActorProfileID: actorProfileID.String(), SpaceID: spaceID, Name: name, Type: chatType}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("encode canonical bot chat create request: %w", err)
	}
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
