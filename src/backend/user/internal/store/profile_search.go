package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ProfileSearchCursor is a keyset position in SearchProfiles ordering:
// lower(username) ASC, discriminator ASC, id ASC.
type ProfileSearchCursor struct {
	UsernameLower string    `json:"u"`
	Discriminator string    `json:"d"`
	ID            uuid.UUID `json:"i"`
}

func EncodeSearchCursor(c ProfileSearchCursor) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func DecodeSearchCursor(s string) (*ProfileSearchCursor, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	var c ProfileSearchCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	if c.UsernameLower == "" || len(c.Discriminator) != 4 || c.ID == uuid.Nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	return &c, nil
}

func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// SearchProfilesAfter returns up to limit profiles matching query (ILIKE on username and display_name),
// ordered by lower(username), discriminator, id, excluding excludeAccount's rows.
// after, if non-nil, starts strictly after that key in the same ordering.
func (s *ProfileStore) SearchProfilesAfter(ctx context.Context, excludeAccount uuid.UUID, query string, after *ProfileSearchCursor, limit int) ([]*ProfileRow, error) {
	if limit <= 0 {
		return nil, nil
	}
	pat := "%" + escapeLikePattern(query) + "%"
	args := []any{excludeAccount, pat}
	extra := ""
	if after != nil {
		extra = ` AND (lower(username), discriminator, id) > (lower($3::text), $4::text, $5::uuid)`
		args = append(args, after.UsernameLower, after.Discriminator, after.ID)
	}
	limitArg := len(args) + 1
	q := fmt.Sprintf(`SELECT id, account_id, username, discriminator, display_name, avatar_url, bio,
		locale, theme, is_primary, verification_type, verification_badge, created_at, updated_at
		FROM profiles
		WHERE account_id <> $1
		AND (username ILIKE $2 ESCAPE '\' OR display_name ILIKE $2 ESCAPE '\')
		%s
		ORDER BY lower(username) ASC, discriminator ASC, id ASC
		LIMIT $%d`, extra, limitArg)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*ProfileRow
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
