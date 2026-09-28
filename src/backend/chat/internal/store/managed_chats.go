package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrManagedOperationConflict     = errors.New("managed chat operation conflicts with saved request")
	ErrManagedResourceConflict      = errors.New("managed chat key conflicts with saved resource")
	ErrManagedChatNotFound          = errors.New("managed chat not found")
	ErrManagedChatPrincipalRequired = errors.New("managed chat mutation requires the GIS principal")
)

var managedRequestHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type ManagedChatCreate struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	OperationID   uuid.UUID
	ExternalKey   string
	RequestHash   string
	Name          string
	Topic         *string
}

type ManagedChatCreateResult struct {
	Chat     *ChatRow
	Replayed bool
}

type ManagedChatMemberSync struct {
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	OperationID   uuid.UUID
	ChatID        uuid.UUID
	RequestHash   string
	ProfileIDs    []uuid.UUID
}

type ManagedChatMemberSyncResult struct {
	ProfileIDs []uuid.UUID
	Replayed   bool
}

type managedChatReceipt struct {
	ChatID     string   `json:"chat_id"`
	ProfileIDs []string `json:"profile_ids,omitempty"`
}

func (s *DMStore) ProvisionManagedChat(ctx context.Context, request ManagedChatCreate) (ManagedChatCreateResult, error) {
	if s == nil || s.Pool == nil {
		return ManagedChatCreateResult{}, errors.New("dm store: pool not configured")
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.ApplicationID == uuid.Nil || request.EnvironmentID == uuid.Nil || request.OperationID == uuid.Nil ||
		request.ExternalKey == "" || len(request.ExternalKey) > 512 || request.Name == "" ||
		!managedRequestHashPattern.MatchString(request.RequestHash) {
		return ManagedChatCreateResult{}, errors.New("invalid managed chat request")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ManagedChatCreateResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockManagedOperation(ctx, tx, request.ApplicationID, request.EnvironmentID, request.OperationID); err != nil {
		return ManagedChatCreateResult{}, err
	}
	if saved, exists, err := readManagedOperation(ctx, tx, request.ApplicationID, request.EnvironmentID, request.OperationID, "create", request.RequestHash); err != nil {
		return ManagedChatCreateResult{}, err
	} else if exists {
		chat, err := managedChatByID(ctx, tx, saved.ChatID)
		if err != nil {
			return ManagedChatCreateResult{}, err
		}
		if chat == nil || chat.ManagedByApplicationID == nil || *chat.ManagedByApplicationID != request.ApplicationID || chat.ManagedEnvironmentID == nil || *chat.ManagedEnvironmentID != request.EnvironmentID {
			return ManagedChatCreateResult{}, ErrManagedOperationConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return ManagedChatCreateResult{}, err
		}
		return ManagedChatCreateResult{Chat: chat, Replayed: true}, nil
	}
	resourceLock := request.ApplicationID.String() + "/" + request.EnvironmentID.String() + "/" + request.ExternalKey
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 31421))`, resourceLock); err != nil {
		return ManagedChatCreateResult{}, err
	}
	chat, err := managedChatByExternalKey(ctx, tx, request.ApplicationID, request.EnvironmentID, request.ExternalKey)
	if err != nil {
		return ManagedChatCreateResult{}, err
	}
	if chat != nil {
		if managedString(chat.Name) != request.Name || managedString(chat.Topic) != managedString(request.Topic) {
			return ManagedChatCreateResult{}, ErrManagedResourceConflict
		}
	} else {
		chat = &ChatRow{}
		var createdAt, updatedAt time.Time
		err = tx.QueryRow(ctx, `
INSERT INTO chats (type, name, topic, creator_profile_id, managed_by_application_id, managed_environment_id, external_chat_key,
                   slow_mode_seconds, threads_enabled, allow_user_main_feed, allow_guests)
VALUES ('group', $1, $2, NULL, $3, $4, $5, 0, false, false, false)
RETURNING id, created_at, updated_at
`, request.Name, optionalTopicArg(request.Topic), request.ApplicationID, request.EnvironmentID, request.ExternalKey).
			Scan(&chat.ID, &createdAt, &updatedAt)
		if err != nil {
			return ManagedChatCreateResult{}, err
		}
		name := request.Name
		chat.Type, chat.Name, chat.Topic = "group", &name, optionalTopicPtr(request.Topic)
		chat.ManagedByApplicationID, chat.ManagedEnvironmentID = &request.ApplicationID, &request.EnvironmentID
		chat.CreatedAt, chat.UpdatedAt = createdAt.UTC(), updatedAt.UTC()
	}
	receipt, err := json.Marshal(managedChatReceipt{ChatID: chat.ID.String()})
	if err != nil {
		return ManagedChatCreateResult{}, err
	}
	if err := saveManagedOperation(ctx, tx, request.ApplicationID, request.EnvironmentID, request.OperationID, "create", request.RequestHash, chat.ID, receipt); err != nil {
		return ManagedChatCreateResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ManagedChatCreateResult{}, err
	}
	return ManagedChatCreateResult{Chat: chat}, nil
}

func managedString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *DMStore) SyncManagedChatMembers(ctx context.Context, request ManagedChatMemberSync) (ManagedChatMemberSyncResult, error) {
	if s == nil || s.Pool == nil {
		return ManagedChatMemberSyncResult{}, errors.New("dm store: pool not configured")
	}
	if request.ApplicationID == uuid.Nil || request.EnvironmentID == uuid.Nil || request.OperationID == uuid.Nil || request.ChatID == uuid.Nil ||
		!managedRequestHashPattern.MatchString(request.RequestHash) || len(request.ProfileIDs) > GroupMemberLimit {
		return ManagedChatMemberSyncResult{}, errors.New("invalid managed chat member request")
	}
	seen := make(map[uuid.UUID]struct{}, len(request.ProfileIDs))
	for _, profileID := range request.ProfileIDs {
		if profileID == uuid.Nil {
			return ManagedChatMemberSyncResult{}, errors.New("invalid managed chat member request")
		}
		if _, duplicate := seen[profileID]; duplicate {
			return ManagedChatMemberSyncResult{}, errors.New("duplicate managed chat member")
		}
		seen[profileID] = struct{}{}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ManagedChatMemberSyncResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockManagedOperation(ctx, tx, request.ApplicationID, request.EnvironmentID, request.OperationID); err != nil {
		return ManagedChatMemberSyncResult{}, err
	}
	if saved, exists, err := readManagedOperation(ctx, tx, request.ApplicationID, request.EnvironmentID, request.OperationID, "sync_members", request.RequestHash); err != nil {
		return ManagedChatMemberSyncResult{}, err
	} else if exists {
		var receipt managedChatReceipt
		if err := json.Unmarshal(saved.Bytes, &receipt); err != nil {
			return ManagedChatMemberSyncResult{}, ErrManagedOperationConflict
		}
		profiles, err := parseManagedProfileIDs(receipt.ProfileIDs)
		if err != nil || receipt.ChatID != request.ChatID.String() {
			return ManagedChatMemberSyncResult{}, ErrManagedOperationConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return ManagedChatMemberSyncResult{}, err
		}
		return ManagedChatMemberSyncResult{ProfileIDs: profiles, Replayed: true}, nil
	}
	var managedApplication, managedEnvironment uuid.UUID
	err = tx.QueryRow(ctx, `SELECT managed_by_application_id, managed_environment_id FROM chats WHERE id = $1 FOR UPDATE`, request.ChatID).
		Scan(&managedApplication, &managedEnvironment)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (managedApplication != request.ApplicationID || managedEnvironment != request.EnvironmentID) {
		return ManagedChatMemberSyncResult{}, ErrManagedChatNotFound
	}
	if err != nil {
		return ManagedChatMemberSyncResult{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_members WHERE chat_id = $1`, request.ChatID); err != nil {
		return ManagedChatMemberSyncResult{}, err
	}
	for _, profileID := range request.ProfileIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket) VALUES ($1, $2, 'member', 'main')`, request.ChatID, profileID); err != nil {
			return ManagedChatMemberSyncResult{}, err
		}
	}
	profiles := append([]uuid.UUID(nil), request.ProfileIDs...)
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].String() < profiles[j].String() })
	profileStrings := make([]string, len(profiles))
	for i, profileID := range profiles {
		profileStrings[i] = profileID.String()
	}
	receipt, err := json.Marshal(managedChatReceipt{ChatID: request.ChatID.String(), ProfileIDs: profileStrings})
	if err != nil {
		return ManagedChatMemberSyncResult{}, err
	}
	if err := saveManagedOperation(ctx, tx, request.ApplicationID, request.EnvironmentID, request.OperationID, "sync_members", request.RequestHash, request.ChatID, receipt); err != nil {
		return ManagedChatMemberSyncResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ManagedChatMemberSyncResult{}, err
	}
	return ManagedChatMemberSyncResult{ProfileIDs: profiles}, nil
}

type savedManagedOperation struct {
	ChatID uuid.UUID
	Bytes  []byte
}

func lockManagedOperation(ctx context.Context, tx pgx.Tx, applicationID, environmentID, operationID uuid.UUID) error {
	key := applicationID.String() + "/" + environmentID.String() + "/" + operationID.String()
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 31420))`, key)
	return err
}

func readManagedOperation(ctx context.Context, tx pgx.Tx, applicationID, environmentID, operationID uuid.UUID, method, requestHash string) (savedManagedOperation, bool, error) {
	var saved savedManagedOperation
	var savedMethod, savedHash string
	err := tx.QueryRow(ctx, `SELECT method, request_hash, chat_id, receipt FROM managed_chat_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 FOR UPDATE`, applicationID, environmentID, operationID).
		Scan(&savedMethod, &savedHash, &saved.ChatID, &saved.Bytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return savedManagedOperation{}, false, nil
	}
	if err != nil {
		return savedManagedOperation{}, false, err
	}
	if savedMethod != method || savedHash != requestHash {
		return savedManagedOperation{}, false, ErrManagedOperationConflict
	}
	return saved, true, nil
}

func saveManagedOperation(ctx context.Context, tx pgx.Tx, applicationID, environmentID, operationID uuid.UUID, method, requestHash string, chatID uuid.UUID, receipt []byte) error {
	_, err := tx.Exec(ctx, `INSERT INTO managed_chat_operations (application_id, environment_id, operation_id, method, request_hash, chat_id, receipt) VALUES ($1,$2,$3,$4,$5,$6,$7)`, applicationID, environmentID, operationID, method, requestHash, chatID, receipt)
	return err
}

type managedChatQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func managedChatByID(ctx context.Context, q managedChatQuery, chatID uuid.UUID) (*ChatRow, error) {
	return scanChatRow(q.QueryRow(ctx, `SELECT id, type, space_id, name, avatar_url, topic, creator_profile_id, managed_by_application_id, managed_environment_id, slow_mode_seconds,
       last_message_at, created_at, updated_at, threads_enabled, allow_user_main_feed, e2e_enabled, allow_guests FROM chats WHERE id=$1`, chatID))
}

func managedChatByExternalKey(ctx context.Context, q managedChatQuery, applicationID, environmentID uuid.UUID, externalKey string) (*ChatRow, error) {
	row, err := scanChatRow(q.QueryRow(ctx, `SELECT id, type, space_id, name, avatar_url, topic, creator_profile_id, managed_by_application_id, managed_environment_id, slow_mode_seconds,
       last_message_at, created_at, updated_at, threads_enabled, allow_user_main_feed, e2e_enabled, allow_guests FROM chats WHERE managed_by_application_id=$1 AND managed_environment_id=$2 AND external_chat_key=$3 FOR UPDATE`, applicationID, environmentID, externalKey))
	return row, err
}

func parseManagedProfileIDs(values []string) ([]uuid.UUID, error) {
	profiles := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return nil, fmt.Errorf("invalid managed chat receipt")
		}
		profiles = append(profiles, id)
	}
	return profiles, nil
}
