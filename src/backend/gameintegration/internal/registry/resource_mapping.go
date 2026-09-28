package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidResourceMapping  = errors.New("invalid resource mapping")
	ErrResourceMappingConflict = errors.New("resource mapping conflict")
)

type ResourceMappingInput struct {
	ApplicationID   uuid.UUID
	EnvironmentID   uuid.UUID
	OperationID     uuid.UUID
	ResourceKind    string
	ExternalKey     string
	ResourceID      uuid.UUID
	ChatID          uuid.UUID
	ChatOperationID uuid.UUID
	ChatRequestHash string
}

type ResourceMappingReceipt struct {
	OperationID     uuid.UUID `json:"operation_id"`
	MappingID       uuid.UUID `json:"mapping_id"`
	ApplicationID   uuid.UUID `json:"application_id"`
	EnvironmentID   uuid.UUID `json:"environment_id"`
	ExternalKey     string    `json:"external_key"`
	ResourceKind    string    `json:"resource_kind"`
	ResourceID      uuid.UUID `json:"resource_id"`
	ChatID          uuid.UUID `json:"chat_id,omitempty"`
	ChatOperationID uuid.UUID `json:"chat_operation_id,omitempty"`
	ChatRequestHash string    `json:"chat_request_hash,omitempty"`
	MappingRevision int64     `json:"mapping_revision"`
	Status          string    `json:"status"`
}

type resourceMappingRequest struct {
	ApplicationID   uuid.UUID `json:"application_id"`
	EnvironmentID   uuid.UUID `json:"environment_id"`
	OperationID     uuid.UUID `json:"operation_id"`
	ResourceKind    string    `json:"resource_kind"`
	ExternalKey     string    `json:"external_key"`
	ResourceID      uuid.UUID `json:"resource_id"`
	ChatID          uuid.UUID `json:"chat_id,omitempty"`
	ChatOperationID uuid.UUID `json:"chat_operation_id,omitempty"`
	ChatRequestHash string    `json:"chat_request_hash,omitempty"`
}

// CreateResourceMapping stores the mapping and immutable operation receipt in
// one GIS transaction. Callers must pass the persisted Chat RPC receipt.
func (s *Store) CreateResourceMapping(ctx context.Context, in ResourceMappingInput) (ResourceMappingReceipt, error) {
	if in.ApplicationID == uuid.Nil || in.EnvironmentID == uuid.Nil || in.OperationID == uuid.Nil ||
		(in.ResourceKind != "chat" && in.ResourceKind != "voice" && in.ResourceKind != "space") ||
		strings.TrimSpace(in.ExternalKey) == "" || !utf8.ValidString(in.ExternalKey) || utf8.RuneCountInString(in.ExternalKey) > 512 || in.ResourceID == uuid.Nil {
		return ResourceMappingReceipt{}, ErrInvalidResourceMapping
	}
	if in.ResourceKind == "space" {
		if in.ChatID != uuid.Nil || in.ChatOperationID != uuid.Nil || in.ChatRequestHash != "" {
			return ResourceMappingReceipt{}, ErrInvalidResourceMapping
		}
	} else if in.ChatID == uuid.Nil || in.ChatOperationID == uuid.Nil || !canonicalChatRequestHash(in.ChatRequestHash) ||
		(in.ResourceKind == "chat" && in.ResourceID != in.ChatID) {
		return ResourceMappingReceipt{}, ErrInvalidResourceMapping
	}
	if s == nil || s.Pool == nil {
		return ResourceMappingReceipt{}, ErrRegistryUnavailable
	}
	canonical, err := json.Marshal(resourceMappingRequest(in))
	if err != nil {
		return ResourceMappingReceipt{}, fmt.Errorf("encode resource mapping request: %w", err)
	}
	hash := sha256.Sum256(canonical)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ResourceMappingReceipt{}, fmt.Errorf("begin resource mapping: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var savedHash []byte
	var savedReceipt []byte
	err = tx.QueryRow(ctx, `SELECT request_hash, receipt FROM game_resource_operations
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 FOR UPDATE`,
		in.ApplicationID, in.EnvironmentID, in.OperationID).Scan(&savedHash, &savedReceipt)
	if err == nil {
		if string(savedHash) != string(hash[:]) {
			return ResourceMappingReceipt{}, ErrIdempotencyConflict
		}
		var receipt ResourceMappingReceipt
		if err := json.Unmarshal(savedReceipt, &receipt); err != nil || receipt.OperationID != in.OperationID || receipt.MappingID == uuid.Nil {
			return ResourceMappingReceipt{}, ErrRegistryUnavailable
		}
		if err := tx.Commit(ctx); err != nil {
			return ResourceMappingReceipt{}, fmt.Errorf("commit resource mapping replay: %w", err)
		}
		return receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ResourceMappingReceipt{}, fmt.Errorf("read resource mapping operation: %w", err)
	}

	_, err = tx.Exec(ctx, `INSERT INTO game_resource_operations
		(application_id, environment_id, operation_id, request_hash, receipt)
		VALUES ($1,$2,$3,$4,'{}'::jsonb)`, in.ApplicationID, in.EnvironmentID, in.OperationID, hash[:])
	if err != nil {
		return ResourceMappingReceipt{}, fmt.Errorf("reserve resource mapping operation: %w", err)
	}
	var appStatus, envStatus string
	err = tx.QueryRow(ctx, `SELECT a.status,e.status FROM applications a
		JOIN environments e ON e.application_id=a.id WHERE a.id=$1 AND e.id=$2 FOR SHARE OF a,e`,
		in.ApplicationID, in.EnvironmentID).Scan(&appStatus, &envStatus)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (appStatus != "sandbox" && appStatus != "active" || envStatus != "active") {
		return ResourceMappingReceipt{}, ErrInvalidResourceMapping
	}
	if err != nil {
		return ResourceMappingReceipt{}, fmt.Errorf("read resource mapping scope: %w", err)
	}
	if in.ResourceKind == "voice" {
		var chatKind, chatStatus string
		var chatResourceID uuid.UUID
		var chatOpID uuid.UUID
		var chatHash string
		err = tx.QueryRow(ctx, `SELECT resource_kind,status,resource_id,chat_operation_id,chat_request_hash
			FROM game_resource_mappings WHERE application_id=$1 AND environment_id=$2 AND chat_id=$3
			FOR SHARE`, in.ApplicationID, in.EnvironmentID, in.ChatID).Scan(&chatKind, &chatStatus,
			&chatResourceID, &chatOpID, &chatHash)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && (chatKind != "chat" || chatStatus != "active" ||
			chatResourceID != in.ChatID || chatOpID != in.ChatOperationID || chatHash != in.ChatRequestHash) {
			return ResourceMappingReceipt{}, ErrInvalidResourceMapping
		}
		if err != nil {
			return ResourceMappingReceipt{}, fmt.Errorf("read associated Chat mapping receipt: %w", err)
		}
	}

	var receipt ResourceMappingReceipt
	var chatID, chatOperationID pgtype.UUID
	var chatRequestHash *string
	err = tx.QueryRow(ctx, `SELECT mapping_id,resource_kind,resource_id,chat_id,chat_operation_id,chat_request_hash,
		status,mapping_revision FROM game_resource_mappings
		WHERE application_id=$1 AND environment_id=$2 AND external_key=$3 FOR UPDATE`,
		in.ApplicationID, in.EnvironmentID, in.ExternalKey).Scan(&receipt.MappingID, &receipt.ResourceKind,
		&receipt.ResourceID, &chatID, &chatOperationID, &chatRequestHash,
		&receipt.Status, &receipt.MappingRevision)
	if chatID.Valid {
		receipt.ChatID = uuid.UUID(chatID.Bytes)
	}
	if chatOperationID.Valid {
		receipt.ChatOperationID = uuid.UUID(chatOperationID.Bytes)
	}
	if chatRequestHash != nil {
		receipt.ChatRequestHash = *chatRequestHash
	}
	if errors.Is(err, pgx.ErrNoRows) {
		receipt = ResourceMappingReceipt{MappingID: uuid.New(), ResourceKind: in.ResourceKind, ResourceID: in.ResourceID,
			ChatID: in.ChatID, ChatOperationID: in.ChatOperationID, ChatRequestHash: in.ChatRequestHash,
			Status: "active", MappingRevision: 1}
		_, err = tx.Exec(ctx, `INSERT INTO game_resource_mappings
			(mapping_id,application_id,environment_id,external_key,resource_kind,resource_id,chat_id,
			 chat_operation_id,chat_request_hash,status,mapping_revision)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'active',1)`, receipt.MappingID, in.ApplicationID,
			in.EnvironmentID, in.ExternalKey, in.ResourceKind, in.ResourceID, nullableUUID(in.ChatID),
			nullableUUID(in.ChatOperationID), nullableString(in.ChatRequestHash))
		if err != nil {
			return ResourceMappingReceipt{}, fmt.Errorf("insert resource mapping: %w", err)
		}
	} else if err != nil {
		return ResourceMappingReceipt{}, fmt.Errorf("read scoped resource mapping: %w", err)
	} else if receipt.Status != "active" || receipt.ResourceKind != in.ResourceKind || receipt.ResourceID != in.ResourceID ||
		receipt.ChatID != in.ChatID || receipt.ChatOperationID != in.ChatOperationID || receipt.ChatRequestHash != in.ChatRequestHash {
		return ResourceMappingReceipt{}, ErrResourceMappingConflict
	}
	receipt.OperationID, receipt.ApplicationID, receipt.EnvironmentID = in.OperationID, in.ApplicationID, in.EnvironmentID
	receipt.ExternalKey = in.ExternalKey
	receiptBytes, err := json.Marshal(receipt)
	if err != nil {
		return ResourceMappingReceipt{}, fmt.Errorf("encode resource mapping receipt: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE game_resource_operations SET receipt=$4
		WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`,
		in.ApplicationID, in.EnvironmentID, in.OperationID, receiptBytes)
	if err != nil {
		return ResourceMappingReceipt{}, fmt.Errorf("persist resource mapping receipt: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ResourceMappingReceipt{}, fmt.Errorf("commit resource mapping: %w", err)
	}
	return receipt, nil
}

func canonicalChatRequestHash(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[7:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func nullableUUID(value uuid.UUID) any {
	if value == uuid.Nil {
		return nil
	}
	return value
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// AuthorizeAppBindingChat fails closed unless the complete GIS-owned tuple,
// binding, active mapping, and unexpired T31 roster relation all agree.
func (s *Store) AuthorizeAppBindingChat(ctx context.Context, appID, envID, bindingID, chatID uuid.UUID) (bool, int64, error) {
	if appID == uuid.Nil || envID == uuid.Nil || bindingID == uuid.Nil || chatID == uuid.Nil {
		return false, 0, nil
	}
	if s == nil || s.Pool == nil {
		return false, 0, ErrRegistryUnavailable
	}
	var revision int64
	err := s.Pool.QueryRow(ctx, `SELECT m.mapping_revision
		FROM game_resource_binding_chats r
		JOIN applications a ON a.id=r.application_id AND a.status IN ('sandbox','active')
		JOIN environments e ON e.id=r.environment_id AND e.application_id=r.application_id AND e.status='active'
		JOIN player_bindings b ON b.application_id=r.application_id AND b.environment_id=r.environment_id
			AND b.binding_id=r.binding_id AND b.status='active'
		JOIN game_resource_mappings m ON m.application_id=r.application_id AND m.environment_id=r.environment_id
			AND m.resource_kind='chat' AND m.chat_id=r.chat_id AND m.status='active'
		WHERE r.application_id=$1 AND r.environment_id=$2 AND r.binding_id=$3 AND r.chat_id=$4
		AND r.status='active' AND r.lease_expires_at > clock_timestamp()`, appID, envID, bindingID, chatID).Scan(&revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, fmt.Errorf("authorize app binding chat tuple: %w", err)
	}
	if revision <= 0 {
		return false, 0, ErrRegistryUnavailable
	}
	return true, revision, nil
}
