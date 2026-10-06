package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrChatDeletedProjectionNotReady = errors.New("chat deletion projection is waiting for the permanent purge fence")
	ErrChatDeletedProjectionConflict = errors.New("chat deletion event conflicts with the permanent purge fence")
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
		INSERT INTO search_user_profile_generation_documents (generation, profile_id, account_id, username, discriminator, display_name, username_lower, verification_type, updated_at)
		SELECT active_generation, $1, $2, $3, $4, $5, lower($3), $6, now()
		FROM search_user_profile_generation_route WHERE singleton=true
		ON CONFLICT (generation, profile_id) DO UPDATE SET
			account_id = EXCLUDED.account_id,
			username = EXCLUDED.username,
			discriminator = EXCLUDED.discriminator,
			display_name = EXCLUDED.display_name,
			username_lower = EXCLUDED.username_lower,
			verification_type = EXCLUDED.verification_type,
			updated_at = now()`,
		doc.ProfileID, doc.AccountID, doc.Username, doc.Discriminator, doc.DisplayName, doc.VerificationType,
	)
	return err
}

func (s *ProfileSpaceSearchStore) DeleteProfile(ctx context.Context, profileID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("profile search store unavailable")
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM search_user_profile_generation_documents WHERE profile_id = $1
		AND generation=(SELECT active_generation FROM search_user_profile_generation_route WHERE singleton=true)`, profileID)
	return err
}

func (s *ProfileSpaceSearchStore) SearchProfiles(ctx context.Context, viewerAccount uuid.UUID, query string, excludeAccounts []uuid.UUID, limit int) ([]ProfileHit, error) {
	if s == nil || s.Pool == nil {
		return nil, fmt.Errorf("profile search store unavailable")
	}
	var generation int64
	if err := s.Pool.QueryRow(ctx, `SELECT active_generation FROM search_user_profile_generation_route WHERE singleton=true`).Scan(&generation); err != nil || generation <= 0 {
		return nil, fmt.Errorf("active profile generation route unavailable")
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	pat := "%" + escapeLikePattern(query) + "%"
	args := []any{pat, viewerAccount}
	excludeSQL := ""
	if len(excludeAccounts) > 0 {
		args = append(args, excludeAccounts)
		excludeSQL = fmt.Sprintf(` AND account_id <> ALL($%d)`, len(args))
	}
	args = append(args, limit)
	sql := fmt.Sprintf(`
		SELECT profile_id, account_id
		FROM search_user_profile_generation_documents
		WHERE generation=(SELECT active_generation FROM search_user_profile_generation_route WHERE singleton=true)
		AND tombstoned_at IS NULL
		AND (username ILIKE $1 ESCAPE '\' OR display_name ILIKE $1 ESCAPE '\')
		AND account_id <> $2
		%s
		ORDER BY (CASE WHEN verification_type <> 'none' AND verification_type <> '' THEN 0 ELSE 1 END),
		         username_lower ASC, discriminator ASC, profile_id ASC
		LIMIT $%d`, excludeSQL, len(args))

	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProfileHit, 0, limit)
	for rows.Next() {
		var hit ProfileHit
		if err := rows.Scan(&hit.ProfileID, &hit.AccountID); err != nil {
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
	tx, err := beginGovernedTx(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lifecycleGateSpace(ctx, tx, doc.SpaceID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
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
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *ProfileSpaceSearchStore) DeleteSpace(ctx context.Context, spaceID uuid.UUID) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("space search store unavailable")
	}
	tx, err := beginGovernedTx(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lifecycleGateSpace(ctx, tx, spaceID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM space_search_documents WHERE space_id = $1`, spaceID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
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
	tx, err := beginGovernedTx(ctx, s.Pool)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lifecycleGateAnySpaceSearch(ctx, tx, query); err != nil {
		return nil, "", err
	}

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

	rows, err := tx.Query(ctx, sql, args...)
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
	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	return hits, next, nil
}

func (s *ProfileSpaceSearchStore) UpsertChat(ctx context.Context, chatID uuid.UUID, title string) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("chat search store unavailable")
	}
	tx, err := beginGovernedTx(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := lifecycleGateChat(ctx, tx, chatID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO chat_search_documents (chat_id, title, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (chat_id) DO UPDATE SET title = EXCLUDED.title, updated_at = now()`,
		chatID, title,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteChat applies only a terminal Chat event bound to the exact completed Search P3 purge.
func (s *ProfileSpaceSearchStore) DeleteChat(ctx context.Context, chatID, spaceID, operationID uuid.UUID, generation uint64, manifestID uuid.UUID, manifestHash []byte) error {
	if s == nil || s.Pool == nil {
		return fmt.Errorf("chat search store unavailable")
	}
	if chatID == uuid.Nil || spaceID == uuid.Nil || operationID == uuid.Nil || manifestID == uuid.Nil || generation < 2 || generation > math.MaxInt64 || len(manifestHash) != 32 {
		return ErrChatDeletedProjectionConflict
	}
	tx, err := beginGovernedTx(ctx, s.Pool)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := AcquireLifecycleSpaceLock(ctx, tx, spaceID); err != nil {
		return err
	}
	var state string
	var currentOperation uuid.UUID
	var currentGeneration int64
	err = tx.QueryRow(ctx, `SELECT state,deletion_operation_id,generation FROM search_space_lifecycle_fences WHERE space_id=$1`, spaceID).Scan(&state, &currentOperation, &currentGeneration)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrChatDeletedProjectionNotReady
	}
	if err != nil {
		return err
	}
	if state != "PURGED" {
		return ErrChatDeletedProjectionNotReady
	}
	if currentOperation != operationID || currentGeneration != int64(generation) {
		return ErrChatDeletedProjectionConflict
	}
	var bound bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM search_space_chat_manifest_items i
		JOIN search_space_chat_manifests m USING(space_id,deletion_operation_id)
		WHERE i.space_id=$1 AND i.deletion_operation_id=$2 AND i.generation=$3-1 AND i.chat_id=$4
		  AND m.generation=$3-1 AND m.manifest_id=$5 AND m.manifest_sha256=$6
		UNION ALL
		SELECT 1 FROM search_space_purged_chat_fences q
		WHERE q.space_id=$1 AND q.chat_id=$4)`, spaceID, operationID, generation, chatID, manifestID.String(), manifestHash).Scan(&bound); err != nil {
		return err
	}
	if !bound {
		return ErrChatDeletedProjectionConflict
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_search_documents WHERE chat_id=$1`, chatID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *ProfileSpaceSearchStore) SearchChats(ctx context.Context, query string, limit int) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return nil, fmt.Errorf("chat search store unavailable")
	}
	tx, err := beginGovernedTx(ctx, s.Pool)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var blocked bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_search_documents c JOIN search_space_chat_manifest_items p ON p.chat_id=c.chat_id JOIN search_space_lifecycle_fences f ON f.space_id=p.space_id AND f.deletion_operation_id=p.deletion_operation_id WHERE f.state IN ('FROZEN','PURGE_DECIDED','PURGED') AND c.title ILIKE '%' || $1 || '%')`, query).Scan(&blocked); err != nil {
		return nil, err
	}
	if blocked {
		return nil, fmt.Errorf("chat projection frozen")
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	pat := "%" + escapeLikePattern(query) + "%"
	rows, err := tx.Query(ctx, `
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
