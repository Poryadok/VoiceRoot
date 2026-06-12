package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProfileSpaceSearchStore queries profile and space projection tables.
type ProfileSpaceSearchStore struct {
	Pool *pgxpool.Pool
}

func NewProfileSpaceSearchStore(pool *pgxpool.Pool) *ProfileSpaceSearchStore {
	return &ProfileSpaceSearchStore{Pool: pool}
}

func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func (s *ProfileSpaceSearchStore) UpsertProfile(ctx context.Context, doc ProfileDocument) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("profile search store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO profile_search_documents (profile_id, account_id, username, discriminator, display_name, username_lower, updated_at)
		VALUES ($1, $2, $3, $4, $5, lower($3), now())
		ON CONFLICT (profile_id) DO UPDATE SET
			account_id = EXCLUDED.account_id,
			username = EXCLUDED.username,
			discriminator = EXCLUDED.discriminator,
			display_name = EXCLUDED.display_name,
			username_lower = EXCLUDED.username_lower,
			updated_at = now()`,
		doc.ProfileID, doc.AccountID, doc.Username, doc.Discriminator, doc.DisplayName,
	)
	return err
}

func (s *ProfileSpaceSearchStore) DeleteProfile(ctx context.Context, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("profile search store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM profile_search_documents WHERE profile_id = $1`, profileID)
	return err
}

func (s *ProfileSpaceSearchStore) SearchProfiles(ctx context.Context, _ uuid.UUID, query string, excludeAccounts []uuid.UUID, limit int) ([]ProfileHit, error) {
	if s == nil || s.Pool == nil {
		return nil, fmt.Errorf("profile search store unavailable")
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	pat := "%" + escapeLikePattern(query) + "%"
	args := []any{pat}
	excludeSQL := ""
	if len(excludeAccounts) > 0 {
		args = append(args, excludeAccounts)
		excludeSQL = fmt.Sprintf(` AND account_id <> ALL($%d)`, len(args))
	}
	args = append(args, limit)
	sql := fmt.Sprintf(`
		SELECT profile_id
		FROM profile_search_documents
		WHERE (username ILIKE $1 ESCAPE '\' OR display_name ILIKE $1 ESCAPE '\')
		%s
		ORDER BY username_lower ASC, discriminator ASC, profile_id ASC
		LIMIT $%d`, excludeSQL, len(args))

	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProfileHit, 0, limit)
	for rows.Next() {
		var hit ProfileHit
		if err := rows.Scan(&hit.ProfileID); err != nil {
			return nil, err
		}
		out = append(out, hit)
	}
	return out, rows.Err()
}

func (s *ProfileSpaceSearchStore) UpsertSpace(ctx context.Context, doc SpaceDocument) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("space search store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO space_search_documents (space_id, name, description, visibility, member_count, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (space_id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			visibility = EXCLUDED.visibility,
			member_count = EXCLUDED.member_count,
			updated_at = now()`,
		doc.SpaceID, doc.Name, doc.Description, doc.Visibility, doc.MemberCount,
	)
	return err
}

func (s *ProfileSpaceSearchStore) DeleteSpace(ctx context.Context, spaceID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("space search store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM space_search_documents WHERE space_id = $1`, spaceID)
	return err
}

type spaceCursor struct {
	Name    string    `json:"n"`
	SpaceID uuid.UUID `json:"s"`
}

func encodeSpaceCursor(c spaceCursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeSpaceCursor(raw string) (*spaceCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	var c spaceCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	if c.SpaceID == uuid.Nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	return &c, nil
}

func (s *ProfileSpaceSearchStore) SearchSpaces(ctx context.Context, query string, cursor *string, limit int) ([]SpaceHit, string, error) {
	if s == nil || s.Pool == nil {
		return nil, "", fmt.Errorf("space search store unavailable")
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	pat := "%" + escapeLikePattern(query) + "%"

	var after *spaceCursor
	if cursor != nil && strings.TrimSpace(*cursor) != "" {
		c, err := decodeSpaceCursor(*cursor)
		if err != nil {
			return nil, "", err
		}
		after = c
	}

	args := []any{pat}
	where := `visibility IN ('public', 'invite_only') AND (name ILIKE $1 ESCAPE '\' OR description ILIKE $1 ESCAPE '\')`
	if after != nil {
		args = append(args, after.Name, after.SpaceID)
		where += fmt.Sprintf(` AND (name, space_id) > ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, limit+1)
	sql := fmt.Sprintf(`
		SELECT space_id, name
		FROM space_search_documents
		WHERE %s
		ORDER BY name ASC, space_id ASC
		LIMIT $%d`, where, len(args))

	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	hits := make([]SpaceHit, 0, limit+1)
	names := make([]string, 0, limit+1)
	for rows.Next() {
		var hit SpaceHit
		var name string
		if err := rows.Scan(&hit.SpaceID, &name); err != nil {
			return nil, "", err
		}
		hits = append(hits, hit)
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var next string
	if len(hits) > limit {
		last := hits[limit-1]
		c, err := encodeSpaceCursor(spaceCursor{Name: names[limit-1], SpaceID: last.SpaceID})
		if err != nil {
			return nil, "", err
		}
		next = c
		hits = hits[:limit]
	}
	return hits, next, nil
}

func (s *ProfileSpaceSearchStore) UpsertChat(ctx context.Context, chatID uuid.UUID, title string) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("chat search store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO chat_search_documents (chat_id, title, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (chat_id) DO UPDATE SET title = EXCLUDED.title, updated_at = now()`,
		chatID, title,
	)
	return err
}

func (s *ProfileSpaceSearchStore) SearchChats(ctx context.Context, query string, limit int) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return nil, fmt.Errorf("chat search store unavailable")
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	pat := "%" + escapeLikePattern(query) + "%"
	rows, err := s.Pool.Query(ctx, `
		SELECT chat_id
		FROM chat_search_documents
		WHERE title ILIKE $1 ESCAPE '\'
		ORDER BY title ASC, chat_id ASC
		LIMIT $2`, pat, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
