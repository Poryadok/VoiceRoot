package store

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CatalogSpace contains only metadata approved for authenticated nonmembers.
// Keep it separate from SpaceRow so a new private Space field cannot leak here.
type CatalogSpace struct {
	ID          uuid.UUID
	Name        string
	Description string
	IconURL     *string
	MemberCount int32
	IsVerified  bool
}

type CatalogPage struct {
	Spaces     []CatalogSpace
	NextCursor string
}

type catalogCursor struct {
	Version   int       `json:"v"`
	QueryHash string    `json:"q"`
	Verified  bool      `json:"verified"`
	Count     int32     `json:"count"`
	ID        uuid.UUID `json:"id"`
}

func catalogQueryHash(query string) string {
	hash := sha256.Sum256([]byte(query))
	return hex.EncodeToString(hash[:])
}

func decodeCatalogCursor(cursor, query string) (*catalogCursor, error) {
	if cursor == "" {
		return nil, nil
	}
	if len(cursor) > 2048 {
		return nil, ErrInvalidListCursor
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, ErrInvalidListCursor
	}
	var value catalogCursor
	if json.Unmarshal(raw, &value) != nil || value.Version != 1 || value.QueryHash != catalogQueryHash(query) || value.Count < 0 || value.ID == uuid.Nil {
		return nil, ErrInvalidListCursor
	}
	return &value, nil
}

func encodeCatalogCursor(row CatalogSpace, query string) string {
	raw, _ := json.Marshal(catalogCursor{Version: 1, QueryHash: catalogQueryHash(query), Verified: row.IsVerified, Count: row.MemberCount, ID: row.ID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

// SearchPublicSpaces reads current Space authority, never the asynchronous Search
// projection or the member-only GetSpace. Empty query lists the catalog.
func (s *SpaceStore) SearchPublicSpaces(ctx context.Context, query, cursor string, limit int) (*CatalogPage, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrOwnershipScopeUnavailable
	}
	query = strings.TrimSpace(query)
	after, err := decodeCatalogCursor(cursor, query)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	// Candidate discovery precedes locks; re-read and compare after acquiring the
	// same locks as visibility, ownership and deletion mutations. A new candidate
	// is never disclosed under another candidate's lock.
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
		}
		rollback := func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }
		candidates, err := queryCatalog(ctx, tx, query, after, limit+1)
		if err != nil {
			rollback()
			return nil, err
		}
		ids := catalogIDs(candidates)
		if err = lockAndCheckOwnershipScope(ctx, tx, ids); err != nil {
			rollback()
			return nil, err
		}
		current, err := queryCatalog(ctx, tx, query, after, limit+1)
		if err != nil {
			rollback()
			return nil, err
		}
		if !slices.Equal(ids, catalogIDs(current)) {
			rollback()
			continue
		}
		page := &CatalogPage{Spaces: current}
		if len(current) > limit {
			page.Spaces = current[:limit]
			page.NextCursor = encodeCatalogCursor(current[limit-1], query)
		}
		if err = tx.Commit(ctx); err != nil {
			rollback()
			return nil, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
		}
		return page, nil
	}
	return nil, ErrOwnershipScopeUnavailable
}

func catalogIDs(rows []CatalogSpace) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return normalizedOwnershipSpaceIDs(ids)
}

func queryCatalog(ctx context.Context, tx pgx.Tx, query string, after *catalogCursor, limit int) ([]CatalogSpace, error) {
	pattern := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query) + "%"
	args := []any{pattern, limit}
	where := `s.visibility IN ('public','invite_only')
	 AND (s.name ILIKE $1 ESCAPE E'\\' OR s.description ILIKE $1 ESCAPE E'\\')
	 AND NOT EXISTS (SELECT 1 FROM space_lifecycle_aggregates a WHERE a.space_id=s.id AND a.phase<>'LIVE')`
	if after != nil {
		where += ` AND (s.is_verified,s.member_count,s.id)<($3,$4,$5)`
		args = append(args, after.Verified, after.Count, after.ID)
	}
	rows, err := tx.Query(ctx, `SELECT s.id,s.name,s.description,s.icon_url,s.member_count,s.is_verified
	 FROM spaces s WHERE `+where+` ORDER BY s.is_verified DESC,s.member_count DESC,s.id DESC LIMIT $2`, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOwnershipScopeUnavailable, err)
	}
	defer rows.Close()
	result := make([]CatalogSpace, 0)
	for rows.Next() {
		var row CatalogSpace
		if err := rows.Scan(&row.ID, &row.Name, &row.Description, &row.IconURL, &row.MemberCount, &row.IsVerified); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
