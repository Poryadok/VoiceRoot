package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"voice/backend/bot/internal/manifest"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidToken  = errors.New("invalid bot token")
	ErrNotWhitelisted = errors.New("chat not whitelisted")
)

// BotStore persists bot platform data.
type BotStore struct {
	Pool *pgxpool.Pool
}

type BotRow struct {
	ID              uuid.UUID
	OwnerAccountID  uuid.UUID
	Name            string
	Description     string
	AvatarURL       *string
	TokenHash       string
	WebhookURL      *string
	WebhookSecret   string
	IsPollingMode   bool
	ScopesJSON      string
	Status          string
	ActorProfileID  uuid.UUID
	Slug            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CommandRow struct {
	ID          uuid.UUID
	BotID       uuid.UUID
	Name        string
	Description string
	Parameters  string
}

type PendingInteraction struct {
	Token           string
	BotID           uuid.UUID
	ChatID          uuid.UUID
	ChatType        string
	InvokerProfile  uuid.UUID
	CommandName     string
	OptionsJSON     string
	ResponseCh      chan InteractionReply
}

type InteractionReply struct {
	Content    string
	Ephemeral  bool
	Deferred   bool
	Err        error
}

func NewToken() (plain string, hash string, err error) {
	var buf [32]byte
	if _, err = rand.Read(buf[:]); err != nil {
		return "", "", err
	}
	plain = "vb_" + hex.EncodeToString(buf[:])
	sum := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(sum[:])
	return plain, hash, nil
}

func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "bot"
	}
	return out
}

func (s *BotStore) CreateBot(ctx context.Context, owner uuid.UUID, name, description, scopesJSON string, actorProfileID uuid.UUID) (BotRow, string, error) {
	plain, hash, err := NewToken()
	if err != nil {
		return BotRow{}, "", err
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return BotRow{}, "", err
	}
	secret := hex.EncodeToString(secretBytes)
	id := uuid.New()
	actor := actorProfileID
	if actor == uuid.Nil {
		actor = uuid.New()
	}
	slug := slugify(name) + "-" + id.String()[:8]
	now := time.Now().UTC()
	row := BotRow{
		ID:             id,
		OwnerAccountID: owner,
		Name:           strings.TrimSpace(name),
		Description:    strings.TrimSpace(description),
		TokenHash:      hash,
		WebhookSecret:  secret,
		IsPollingMode:  false,
		ScopesJSON:     scopesJSON,
		Status:         "live",
		ActorProfileID: actor,
		Slug:           slug,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_, err = s.Pool.Exec(ctx, `
INSERT INTO bots (
	id, owner_account_id, name, description, token_hash, webhook_secret,
	is_polling_mode, scopes, status, actor_profile_id, slug
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11)`,
		row.ID, row.OwnerAccountID, row.Name, row.Description, row.TokenHash, row.WebhookSecret,
		row.IsPollingMode, row.ScopesJSON, row.Status, row.ActorProfileID, row.Slug)
	if err != nil {
		return BotRow{}, "", err
	}
	return row, plain, nil
}

func (s *BotStore) GetBotByID(ctx context.Context, id uuid.UUID) (*BotRow, error) {
	row := s.Pool.QueryRow(ctx, `
SELECT id, owner_account_id, name, description, avatar_url, token_hash, webhook_url, webhook_secret,
	is_polling_mode, scopes::text, status, actor_profile_id, slug, created_at, updated_at
FROM bots WHERE id = $1`, id)
	return scanBot(row)
}

func (s *BotStore) GetBotByTokenHash(ctx context.Context, hash string) (*BotRow, error) {
	row := s.Pool.QueryRow(ctx, `
SELECT id, owner_account_id, name, description, avatar_url, token_hash, webhook_url, webhook_secret,
	is_polling_mode, scopes::text, status, actor_profile_id, slug, created_at, updated_at
FROM bots WHERE token_hash = $1 AND status = 'live'`, hash)
	return scanBot(row)
}

func (s *BotStore) ListBotsByOwner(ctx context.Context, owner uuid.UUID) ([]BotRow, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id, owner_account_id, name, description, avatar_url, token_hash, webhook_url, webhook_secret,
	is_polling_mode, scopes::text, status, actor_profile_id, slug, created_at, updated_at
FROM bots WHERE owner_account_id = $1 ORDER BY created_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BotRow
	for rows.Next() {
		b, err := scanBot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

func (s *BotStore) RegenerateToken(ctx context.Context, botID uuid.UUID) (string, error) {
	plain, hash, err := NewToken()
	if err != nil {
		return "", err
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE bots SET token_hash = $2, updated_at = now() WHERE id = $1`, botID, hash)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "", ErrNotFound
	}
	return plain, nil
}

func (s *BotStore) ApplyManifest(ctx context.Context, botID uuid.UUID, doc manifest.Document) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	webhook := strings.TrimSpace(doc.WebhookURL)
	polling := webhook == ""
	_, err = tx.Exec(ctx, `
UPDATE bots SET name = $2, description = $3, avatar_url = NULLIF($4, ''), webhook_url = NULLIF($5, ''),
	is_polling_mode = $6, scopes = $7::jsonb, updated_at = now()
WHERE id = $1`,
		botID, doc.Name, doc.Description, doc.IconURL, webhook, polling, manifest.ScopesJSON(doc.Scopes))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM bot_commands WHERE bot_id = $1`, botID)
	if err != nil {
		return err
	}
	for _, cmd := range manifest.FlattenCommands(doc.Commands) {
		params, _ := json.Marshal(cmd.Options)
		storeName := cmd.Name
		if cmd.GroupName != "" {
			storeName = cmd.GroupName + " " + cmd.Name
		}
		_, err = tx.Exec(ctx, `
INSERT INTO bot_commands (id, bot_id, name, description, parameters)
VALUES ($1, $2, $3, $4, $5::jsonb)`,
			uuid.New(), botID, strings.TrimSpace(storeName), strings.TrimSpace(cmd.Description), string(params))
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *BotStore) ListCommands(ctx context.Context, botID uuid.UUID) ([]CommandRow, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id, bot_id, name, description, parameters::text
FROM bot_commands WHERE bot_id = $1 ORDER BY name`, botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CommandRow
	for rows.Next() {
		var c CommandRow
		if err := rows.Scan(&c.ID, &c.BotID, &c.Name, &c.Description, &c.Parameters); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *BotStore) InstallInSpace(ctx context.Context, botID, spaceID, installer uuid.UUID, chats []uuid.UUID) (uuid.UUID, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	installID := uuid.New()
	_, err = tx.Exec(ctx, `
INSERT INTO bot_space_installations (id, bot_id, space_id, installed_by_profile_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (bot_id, space_id) DO UPDATE SET installed_by_profile_id = EXCLUDED.installed_by_profile_id`,
		installID, botID, spaceID, installer)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = tx.Exec(ctx, `DELETE FROM bot_chat_whitelist WHERE bot_id = $1 AND space_id = $2`, botID, spaceID)
	if err != nil {
		return uuid.Nil, err
	}
	for _, chatID := range chats {
		_, err = tx.Exec(ctx, `
INSERT INTO bot_chat_whitelist (bot_id, chat_id, space_id, enabled, added_by_profile_id)
VALUES ($1, $2, $3, true, $4)`,
			botID, chatID, spaceID, installer)
		if err != nil {
			return uuid.Nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return installID, nil
}

func (s *BotStore) UninstallFromSpace(ctx context.Context, botID, spaceID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM bot_space_installations WHERE bot_id = $1 AND space_id = $2`, botID, spaceID)
	return err
}

// PrimaryInstalledSpace returns the space_id when the bot has exactly one installation.
func (s *BotStore) PrimaryInstalledSpace(ctx context.Context, botID uuid.UUID) (uuid.UUID, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT space_id FROM bot_space_installations WHERE bot_id = $1`, botID)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	var spaces []uuid.UUID
	for rows.Next() {
		var sid uuid.UUID
		if err := rows.Scan(&sid); err != nil {
			return uuid.Nil, err
		}
		spaces = append(spaces, sid)
	}
	if err := rows.Err(); err != nil {
		return uuid.Nil, err
	}
	switch len(spaces) {
	case 0:
		return uuid.Nil, ErrNotFound
	case 1:
		return spaces[0], nil
	default:
		return uuid.Nil, fmt.Errorf("bot installed in multiple spaces; use InstallBotInSpace")
	}
}

func (s *BotStore) IsChatWhitelisted(ctx context.Context, botID, chatID uuid.UUID) (bool, error) {
	var enabled bool
	err := s.Pool.QueryRow(ctx, `
SELECT enabled FROM bot_chat_whitelist WHERE bot_id = $1 AND chat_id = $2`, botID, chatID).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return enabled, nil
}

func (s *BotStore) ListSlashCommandsForChat(ctx context.Context, chatID uuid.UUID) ([]CommandRow, string, uuid.UUID, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT c.id, c.bot_id, c.name, c.description, c.parameters::text, b.name
FROM bot_commands c
JOIN bots b ON b.id = c.bot_id
JOIN bot_chat_whitelist w ON w.bot_id = c.bot_id
WHERE w.chat_id = $1 AND w.enabled = true AND b.status = 'live'
ORDER BY b.name, c.name`, chatID)
	if err != nil {
		return nil, "", uuid.Nil, err
	}
	defer rows.Close()
	var out []CommandRow
	var botName string
	var botID uuid.UUID
	for rows.Next() {
		var c CommandRow
		var name string
		if err := rows.Scan(&c.ID, &c.BotID, &c.Name, &c.Description, &c.Parameters, &name); err != nil {
			return nil, "", uuid.Nil, err
		}
		out = append(out, c)
		botName = name
		botID = c.BotID
	}
	return out, botName, botID, rows.Err()
}

func (s *BotStore) EnqueueEvent(ctx context.Context, botID uuid.UUID, eventType string, payload map[string]any, token string) (uuid.UUID, error) {
	id := uuid.New()
	b, _ := json.Marshal(payload)
	_, err := s.Pool.Exec(ctx, `
INSERT INTO bot_event_log (id, bot_id, event_type, payload, delivery_status, interaction_token)
VALUES ($1, $2, $3, $4::jsonb, 'pending', $5)`,
		id, botID, eventType, string(b), token)
	return id, err
}

func (s *BotStore) ListPendingEvents(ctx context.Context, botID uuid.UUID, limit int) ([]uuid.UUID, []string, []string, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, event_type, payload::text
FROM bot_event_log
WHERE bot_id = $1 AND delivery_status = 'pending'
ORDER BY created_at ASC
LIMIT $2`, botID, limit)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	var types []string
	var payloads []string
	for rows.Next() {
		var id uuid.UUID
		var et, payload string
		if err := rows.Scan(&id, &et, &payload); err != nil {
			return nil, nil, nil, err
		}
		ids = append(ids, id)
		types = append(types, et)
		payloads = append(payloads, payload)
	}
	return ids, types, payloads, rows.Err()
}

func (s *BotStore) MarkEventDelivered(ctx context.Context, eventID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = 'delivered', delivered_at = now(), attempts = attempts + 1
WHERE id = $1`, eventID)
	return err
}

func (s *BotStore) MarkInteractionDelivered(ctx context.Context, botID uuid.UUID, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = 'delivered', delivered_at = now(), attempts = attempts + 1
WHERE bot_id = $1 AND interaction_token = $2 AND delivery_status = 'pending'`, botID, token)
	return err
}

func (s *BotStore) MarkEventFailed(ctx context.Context, botID uuid.UUID, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = 'failed', attempts = attempts + 1
WHERE bot_id = $1 AND interaction_token = $2 AND delivery_status = 'pending'`, botID, token)
	return err
}

func (s *BotStore) MarkEventTimeout(ctx context.Context, botID uuid.UUID, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = 'timeout', attempts = attempts + 1
WHERE bot_id = $1 AND interaction_token = $2 AND delivery_status = 'pending'`, botID, token)
	return err
}

func (s *BotStore) IncrementEventAttempts(ctx context.Context, botID uuid.UUID, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET attempts = attempts + 1
WHERE bot_id = $1 AND interaction_token = $2 AND delivery_status = 'pending'`, botID, token)
	return err
}

func scanBot(row pgx.Row) (*BotRow, error) {
	var b BotRow
	err := row.Scan(
		&b.ID, &b.OwnerAccountID, &b.Name, &b.Description, &b.AvatarURL, &b.TokenHash, &b.WebhookURL,
		&b.WebhookSecret, &b.IsPollingMode, &b.ScopesJSON, &b.Status, &b.ActorProfileID, &b.Slug,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func ScopeAllows(scopesJSON, scope string) bool {
	var scopes []string
	_ = json.Unmarshal([]byte(scopesJSON), &scopes)
	for _, s := range scopes {
		if strings.TrimSpace(s) == scope {
			return true
		}
	}
	return false
}

func SlugFromName(name string) string {
	return slugify(name)
}

func FormatCommandListJSON(commands []CommandRow) string {
	type cmd struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Options     json.RawMessage `json:"options"`
	}
	out := make([]cmd, 0, len(commands))
	for _, c := range commands {
		out = append(out, cmd{Name: c.Name, Description: c.Description, Options: json.RawMessage(c.Parameters)})
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func ChatRefKey(chatID uuid.UUID, chatType string) string {
	return fmt.Sprintf("%s:%s", chatType, chatID.String())
}
