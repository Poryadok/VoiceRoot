package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrInvalidAuditCursor = errors.New("invalid audit cursor")

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

type auditCursorPayload struct {
	T string `json:"t"`
	I string `json:"i"`
}

func encodeAuditCursor(createdAt time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(auditCursorPayload{
		T: createdAt.UTC().Format(time.RFC3339Nano),
		I: id.String(),
	})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeAuditCursor(raw string) (time.Time, uuid.UUID, error) {
	if raw == "" {
		return time.Time{}, uuid.Nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidAuditCursor
	}
	var payload auditCursorPayload
	if err := json.Unmarshal(b, &payload); err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidAuditCursor
	}
	createdAt, err := time.Parse(time.RFC3339Nano, payload.T)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidAuditCursor
	}
	id, err := uuid.Parse(payload.I)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidAuditCursor
	}
	if createdAt.IsZero() || id == uuid.Nil {
		return time.Time{}, uuid.Nil, ErrInvalidAuditCursor
	}
	return createdAt.UTC(), id, nil
}

// ListAuditLogPage returns a space-scoped, newest-first stable keyset page.
func (s *SpaceStore) ListAuditLogPage(ctx context.Context, spaceID uuid.UUID, cursor string, limit int) (*AuditLogPage, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("space store: pool not configured")
	}
	if limit < 1 {
		limit = 1
	}
	createdAt, id, err := decodeAuditCursor(cursor)
	if err != nil {
		return nil, err
	}

	var rows pgx.Rows
	if createdAt.IsZero() {
		rows, err = s.Pool.Query(ctx, `
SELECT id, space_id, actor_profile_id, action, target_type, target_id, details::text, created_at
FROM audit_log
WHERE space_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2
`, spaceID, limit+1)
	} else {
		rows, err = s.Pool.Query(ctx, `
SELECT id, space_id, actor_profile_id, action, target_type, target_id, details::text, created_at
FROM audit_log
WHERE space_id = $1 AND (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $4
`, spaceID, createdAt, id, limit+1)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*AuditLogRow, 0, limit)
	for rows.Next() {
		row := new(AuditLogRow)
		if err := rows.Scan(&row.ID, &row.SpaceID, &row.ActorProfileID, &row.Action, &row.TargetType, &row.TargetID, &row.DetailsJSON, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.CreatedAt = row.CreatedAt.UTC()
		if len(out) == limit {
			last := out[len(out)-1]
			return &AuditLogPage{Rows: out, NextCursor: encodeAuditCursor(last.CreatedAt, last.ID)}, nil
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &AuditLogPage{Rows: out}, nil
}
