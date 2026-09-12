package store

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidAuditCursor = errors.New("invalid audit cursor")

const (
	auditCursorVersion = 1
	auditCursorTTL     = 15 * time.Minute
)

// AuditLogRow is one administrative action recorded for a space.
type AuditLogRow struct {
	ID             uuid.UUID
	SpaceID        uuid.UUID
	ActorProfileID uuid.UUID
	Action         string
	TargetType     string
	TargetID       uuid.UUID
	DetailsJSON    string
	CreatedAt      time.Time
}

// AuditLogPage holds a stable keyset page ordered newest first.
type AuditLogPage struct {
	Rows       []*AuditLogRow
	NextCursor string
}

// AuditLogQuery is the complete filter and disclosure scope bound into a
// signed audit cursor. To is exclusive; From is inclusive.
type AuditLogQuery struct {
	SpaceID        uuid.UUID
	ActorProfileID *uuid.UUID
	Action         *string
	From           *time.Time
	To             *time.Time
	PageSize       int
	Cursor         string
	AccessScope    string
}

type signedAuditCursorPayload struct {
	Version     int    `json:"v"`
	BindingHash string `json:"b"`
	CeilingTime string `json:"ct"`
	CeilingID   string `json:"ci"`
	LastTime    string `json:"lt"`
	LastID      string `json:"li"`
	ExpiresAt   int64  `json:"e"`
}

type auditCursorBinding struct {
	SpaceID        string `json:"space_id"`
	ActorProfileID string `json:"actor_profile_id,omitempty"`
	Action         string `json:"action,omitempty"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	PageSize       int    `json:"page_size"`
	AccessScope    string `json:"access_scope"`
}

func (s *SpaceStore) loadAuditCursorKey(ctx context.Context) ([]byte, error) {
	if s == nil || s.db() == nil {
		return nil, errors.New("space store: pool not configured")
	}
	var key []byte
	if err := s.db().QueryRow(ctx, `SELECT key_bytes FROM audit_cursor_signing_key WHERE singleton`).Scan(&key); err != nil {
		return nil, fmt.Errorf("load audit cursor signing key: %w", err)
	}
	if len(key) != sha256.Size {
		return nil, errors.New("invalid audit cursor signing key")
	}
	return key, nil
}

func auditQueryBindingHash(query AuditLogQuery) (string, error) {
	if query.SpaceID == uuid.Nil || query.PageSize < 1 || query.PageSize > 100 || strings.TrimSpace(query.AccessScope) == "" {
		return "", ErrInvalidAuditCursor
	}
	binding := auditCursorBinding{
		SpaceID:     query.SpaceID.String(),
		PageSize:    query.PageSize,
		AccessScope: query.AccessScope,
	}
	if query.ActorProfileID != nil {
		if *query.ActorProfileID == uuid.Nil {
			return "", ErrInvalidAuditCursor
		}
		binding.ActorProfileID = query.ActorProfileID.String()
	}
	if query.Action != nil {
		if strings.TrimSpace(*query.Action) == "" {
			return "", ErrInvalidAuditCursor
		}
		binding.Action = *query.Action
	}
	if query.From != nil {
		if query.From.IsZero() {
			return "", ErrInvalidAuditCursor
		}
		binding.From = query.From.UTC().Format(time.RFC3339Nano)
	}
	if query.To != nil {
		if query.To.IsZero() {
			return "", ErrInvalidAuditCursor
		}
		binding.To = query.To.UTC().Format(time.RFC3339Nano)
	}
	if query.From != nil && query.To != nil && !query.From.Before(*query.To) {
		return "", ErrInvalidAuditCursor
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return "", ErrInvalidAuditCursor
	}
	digest := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

func encodeSignedAuditCursor(payload signedAuditCursorPayload, key []byte) (string, error) {
	if len(key) < sha256.Size {
		return "", ErrInvalidAuditCursor
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", ErrInvalidAuditCursor
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func decodeSignedAuditCursor(raw string, key []byte, bindingHash string, now time.Time) (signedAuditCursorPayload, error) {
	if len(key) < sha256.Size {
		return signedAuditCursorPayload{}, ErrInvalidAuditCursor
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return signedAuditCursorPayload{}, ErrInvalidAuditCursor
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return signedAuditCursorPayload{}, ErrInvalidAuditCursor
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return signedAuditCursorPayload{}, ErrInvalidAuditCursor
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payloadBytes)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return signedAuditCursorPayload{}, ErrInvalidAuditCursor
	}
	var payload signedAuditCursorPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil || payload.Version != auditCursorVersion || payload.BindingHash != bindingHash {
		return signedAuditCursorPayload{}, ErrInvalidAuditCursor
	}
	if payload.ExpiresAt <= 0 || !now.Before(time.Unix(0, payload.ExpiresAt)) {
		return signedAuditCursorPayload{}, ErrInvalidAuditCursor
	}
	return payload, nil
}

func parseAuditCursorKey(at, id string) (time.Time, uuid.UUID, error) {
	parsedAt, err := time.Parse(time.RFC3339Nano, at)
	if err != nil || parsedAt.IsZero() {
		return time.Time{}, uuid.Nil, ErrInvalidAuditCursor
	}
	parsedID, err := uuid.Parse(id)
	if err != nil || parsedID == uuid.Nil {
		return time.Time{}, uuid.Nil, ErrInvalidAuditCursor
	}
	return parsedAt.UTC(), parsedID, nil
}

// ListAuditLogFilteredPage returns a newest-first page with a signed cursor
// bound to the complete query and disclosure scope. The first page fixes the
// newest visible tuple as the snapshot ceiling for all continuation pages.
func (s *SpaceStore) ListAuditLogFilteredPage(ctx context.Context, query AuditLogQuery, cursorKey []byte, now time.Time) (*AuditLogPage, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if s.tx == nil {
		return withOwnershipScopeValue(s, ctx, []uuid.UUID{query.SpaceID}, func(scoped *SpaceStore) (*AuditLogPage, error) {
			return scoped.ListAuditLogFilteredPage(ctx, query, cursorKey, now)
		})
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	bindingHash, err := auditQueryBindingHash(query)
	if err != nil || len(cursorKey) < sha256.Size {
		return nil, ErrInvalidAuditCursor
	}

	var ceilingAt, lastAt time.Time
	var ceilingID, lastID uuid.UUID
	expiresAt := now.Add(auditCursorTTL)
	if query.Cursor != "" {
		payload, err := decodeSignedAuditCursor(query.Cursor, cursorKey, bindingHash, now)
		if err != nil {
			return nil, err
		}
		ceilingAt, ceilingID, err = parseAuditCursorKey(payload.CeilingTime, payload.CeilingID)
		if err != nil {
			return nil, err
		}
		lastAt, lastID, err = parseAuditCursorKey(payload.LastTime, payload.LastID)
		if err != nil || (lastAt.After(ceilingAt) || (lastAt.Equal(ceilingAt) && lastID.String() > ceilingID.String())) {
			return nil, ErrInvalidAuditCursor
		}
		expiresAt = time.Unix(0, payload.ExpiresAt).UTC()
	}

	args := []any{query.SpaceID}
	where := []string{"space_id = $1"}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if query.ActorProfileID != nil {
		where = append(where, "actor_profile_id = "+addArg(*query.ActorProfileID))
	}
	if query.Action != nil {
		where = append(where, "action = "+addArg(*query.Action))
	}
	if query.From != nil {
		where = append(where, "created_at >= "+addArg(query.From.UTC()))
	}
	if query.To != nil {
		where = append(where, "created_at < "+addArg(query.To.UTC()))
	}
	if !ceilingAt.IsZero() {
		atArg := addArg(ceilingAt)
		idArg := addArg(ceilingID)
		where = append(where, fmt.Sprintf("(created_at, id) <= (%s, %s)", atArg, idArg))
	}
	if !lastAt.IsZero() {
		atArg := addArg(lastAt)
		idArg := addArg(lastID)
		where = append(where, fmt.Sprintf("(created_at, id) < (%s, %s)", atArg, idArg))
	}
	limitArg := addArg(query.PageSize + 1)
	statement := `SELECT id,space_id,actor_profile_id,action,target_type,target_id,details::text,created_at
		FROM audit_log WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC,id DESC LIMIT ` + limitArg
	rows, err := s.db().Query(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*AuditLogRow, 0, query.PageSize+1)
	for rows.Next() {
		row := new(AuditLogRow)
		if err := rows.Scan(&row.ID, &row.SpaceID, &row.ActorProfileID, &row.Action, &row.TargetType, &row.TargetID, &row.DetailsJSON, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.CreatedAt = row.CreatedAt.UTC()
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) <= query.PageSize {
		return &AuditLogPage{Rows: result}, nil
	}
	result = result[:query.PageSize]
	if ceilingAt.IsZero() {
		ceilingAt = result[0].CreatedAt
		ceilingID = result[0].ID
	}
	last := result[len(result)-1]
	next, err := encodeSignedAuditCursor(signedAuditCursorPayload{
		Version:     auditCursorVersion,
		BindingHash: bindingHash,
		CeilingTime: ceilingAt.Format(time.RFC3339Nano),
		CeilingID:   ceilingID.String(),
		LastTime:    last.CreatedAt.Format(time.RFC3339Nano),
		LastID:      last.ID.String(),
		ExpiresAt:   expiresAt.UnixNano(),
	}, cursorKey)
	if err != nil {
		return nil, err
	}
	return &AuditLogPage{Rows: result, NextCursor: next}, nil
}

// ListAuditLogSignedPage is the current unfiltered RPC store surface. When a
// key is not injected by a test, all replicas read the durable database key.
func (s *SpaceStore) ListAuditLogSignedPage(ctx context.Context, spaceID uuid.UUID, cursor string, limit int, accessScope string, cursorKey []byte) (*AuditLogPage, error) {
	if len(cursorKey) == 0 {
		var err error
		cursorKey, err = s.loadAuditCursorKey(ctx)
		if err != nil {
			return nil, err
		}
	}
	return s.ListAuditLogFilteredPage(ctx, AuditLogQuery{
		SpaceID:     spaceID,
		PageSize:    limit,
		Cursor:      cursor,
		AccessScope: accessScope,
	}, cursorKey, time.Now().UTC())
}

// CleanupExpiredAuditLog removes only successfully delivered audit effects
// after the documented 365-day live-Space retention period.
func (s *SpaceStore) CleanupExpiredAuditLog(ctx context.Context, limit int) (int64, error) {
	if s == nil || s.Pool == nil {
		return 0, errors.New("space store: pool not configured")
	}
	if limit < 1 {
		return 0, errors.New("invalid audit cleanup limit")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin audit cleanup: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('voice.audit_delete_mode','retention',true)`); err != nil {
		return 0, fmt.Errorf("enable audit retention cleanup: %w", err)
	}
	tag, err := tx.Exec(ctx, `
WITH candidates AS (
	SELECT audit.id
	FROM audit_log AS audit
	JOIN audit_outbox AS outbox ON outbox.audit_event_id=audit.id
	WHERE audit.created_at < clock_timestamp()-interval '365 days'
	  AND outbox.delivered_at IS NOT NULL
	ORDER BY audit.created_at,audit.id
	FOR UPDATE OF audit SKIP LOCKED
	LIMIT $1
)
DELETE FROM audit_log AS audit
USING candidates
WHERE audit.id=candidates.id`, limit)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired audit log: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit audit cleanup: %w", err)
	}
	return tag.RowsAffected(), nil
}
