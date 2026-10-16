package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrChatCreateRequestConflict = errors.New("chat create request id already used with different request")

// ChatCreateRequest is a normal user-owned chat create bound to a stable request ID.
type ChatCreateRequest struct {
	CreatorProfileID uuid.UUID
	RequestID        uuid.UUID
	Type             string
	SpaceID          *uuid.UUID
	Name             string
	Topic            *string
}

// CreateChatWithRequestID atomically records a request key and the chat it creates.
// A concurrent or later identical request returns the original chat without a
// second insert. The caller scopes the hash to the canonical create fields.
func (s *DMStore) CreateChatWithRequestID(ctx context.Context, request ChatCreateRequest) (*ChatRow, bool, error) {
	if s == nil || s.Pool == nil {
		return nil, false, errors.New("dm store: pool not configured")
	}
	if request.CreatorProfileID == uuid.Nil || request.RequestID == uuid.Nil {
		return nil, false, errors.New("invalid chat create request identity")
	}
	if request.Type != "group" && request.Type != "channel" {
		return nil, false, errors.New("unsupported chat create type")
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return nil, false, errors.New("chat name is required")
	}
	topic := optionalTopicArg(request.Topic)
	spaceID := ""
	if request.SpaceID != nil {
		spaceID = request.SpaceID.String()
	}
	canonicalTopic := request.Topic
	if canonicalTopic != nil {
		trimmed := strings.TrimSpace(*canonicalTopic)
		if trimmed == "" {
			canonicalTopic = nil
		} else {
			canonicalTopic = &trimmed
		}
	}
	canonical := struct {
		Type    string  `json:"type"`
		SpaceID string  `json:"space_id"`
		Name    string  `json:"name"`
		Topic   *string `json:"topic"`
	}{Type: request.Type, SpaceID: spaceID, Name: name, Topic: canonicalTopic}
	canonicalBytes, err := json.Marshal(canonical)
	if err != nil {
		return nil, false, fmt.Errorf("encode canonical chat create request: %w", err)
	}
	digest := sha256.Sum256(canonicalBytes)
	requestHash := "sha256:" + hex.EncodeToString(digest[:])

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var inserted uuid.UUID
	err = tx.QueryRow(ctx, `
INSERT INTO chat_create_requests (creator_profile_id, request_id, request_hash)
VALUES ($1, $2, $3)
ON CONFLICT (creator_profile_id, request_id) DO NOTHING
RETURNING request_id
`, request.CreatorProfileID, request.RequestID, requestHash).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		var priorHash string
		var chatID *uuid.UUID
		var resultBytes []byte
		if err := tx.QueryRow(ctx, `
SELECT request_hash, chat_id, result_bytes FROM chat_create_requests
WHERE creator_profile_id=$1 AND request_id=$2
`, request.CreatorProfileID, request.RequestID).Scan(&priorHash, &chatID, &resultBytes); err != nil {
			return nil, false, err
		}
		if priorHash != requestHash {
			return nil, false, ErrChatCreateRequestConflict
		}
		if chatID == nil || *chatID == uuid.Nil {
			return nil, false, errors.New("chat create request has no committed result")
		}
		var row ChatRow
		if err := json.Unmarshal(resultBytes, &row); err != nil {
			return nil, false, fmt.Errorf("decode saved chat create result: %w", err)
		}
		if row.ID != *chatID || row.CreatorProfileID != request.CreatorProfileID {
			return nil, false, errors.New("saved chat create result does not match its owner and key")
		}
		return &row, true, nil
	}
	if err != nil {
		return nil, false, err
	}

	if request.SpaceID != nil {
		if err := lockSpaceLifecycleMutation(ctx, tx, *request.SpaceID); err != nil {
			return nil, false, err
		}
	}
	var chatID uuid.UUID
	var createdAt, updatedAt time.Time
	var query string
	var args []any
	switch {
	case request.SpaceID != nil && request.Type == "channel":
		query = `INSERT INTO chats (type, space_id, name, creator_profile_id, slow_mode_seconds, threads_enabled, allow_user_main_feed, topic)
VALUES ('channel', $1, $2, $3, 0, true, false, $4) RETURNING id, created_at, updated_at`
		args = []any{*request.SpaceID, name, request.CreatorProfileID, topic}
	case request.SpaceID != nil:
		query = `INSERT INTO chats (type, space_id, name, creator_profile_id, slow_mode_seconds, topic)
VALUES ('group', $1, $2, $3, 0, $4) RETURNING id, created_at, updated_at`
		args = []any{*request.SpaceID, name, request.CreatorProfileID, topic}
	case request.Type == "channel":
		query = `INSERT INTO chats (type, name, creator_profile_id, slow_mode_seconds, threads_enabled, allow_user_main_feed, topic)
VALUES ('channel', $1, $2, 0, true, false, $3) RETURNING id, created_at, updated_at`
		args = []any{name, request.CreatorProfileID, topic}
	default:
		query = `INSERT INTO chats (type, name, creator_profile_id, slow_mode_seconds, topic)
VALUES ('group', $1, $2, 0, $3) RETURNING id, created_at, updated_at`
		args = []any{name, request.CreatorProfileID, topic}
	}
	if err := tx.QueryRow(ctx, query, args...).Scan(&chatID, &createdAt, &updatedAt); err != nil {
		return nil, false, err
	}
	if request.SpaceID == nil {
		if _, err := tx.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket) VALUES ($1,$2,'owner','main')`, chatID, request.CreatorProfileID); err != nil {
			return nil, false, err
		}
	}
	row, err := scanChatRow(tx.QueryRow(ctx, `
SELECT id, type, space_id, name, avatar_url, topic, creator_profile_id, managed_by_application_id, managed_environment_id, slow_mode_seconds,
       last_message_at, created_at, updated_at, threads_enabled, allow_user_main_feed, e2e_enabled, allow_guests
FROM chats WHERE id=$1
`, chatID))
	if err != nil {
		return nil, false, err
	}
	row.CreatedAt, row.UpdatedAt = createdAt.UTC(), updatedAt.UTC()
	if request.SpaceID == nil {
		row.InboxBucket = "main"
	}
	resultBytes, err := json.Marshal(row)
	if err != nil {
		return nil, false, fmt.Errorf("encode saved chat create result: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE chat_create_requests SET chat_id=$3, result_bytes=$4 WHERE creator_profile_id=$1 AND request_id=$2`, request.CreatorProfileID, request.RequestID, chatID, resultBytes); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, fmt.Errorf("commit chat create request: %w", err)
	}
	return row, false, nil
}
