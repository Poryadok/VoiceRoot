package store

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxDiscriminatorAttempts = 24
	profileCountFree         = 2
	profileCountPremium      = 5
)

const profileSelectCols = `id, account_id, username, discriminator, display_name, avatar_url, banner_url, bio,
		locale, theme, is_primary, verification_type, verification_badge, frozen_at, created_at, updated_at`

// MaxDisplayNameRunes is the maximum length of profile display_name (aligned with Discord).
const MaxDisplayNameRunes = 32

// ProfileRow mirrors user_db.profiles v1 (docs/microservices/user-service.md).
type ProfileRow struct {
	ID                uuid.UUID
	AccountID         uuid.UUID
	Username          string
	Discriminator     string
	DisplayName       string
	AvatarURL         *string
	BannerURL         *string
	Bio               *string
	Locale            string
	Theme             string
	IsPrimary         bool
	VerificationType  string
	VerificationBadge *string
	FrozenAt          *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ProfileStore struct {
	pool *pgxpool.Pool
}

func NewProfileStore(pool *pgxpool.Pool) *ProfileStore {
	return &ProfileStore{pool: pool}
}

func (s *ProfileStore) Pool() *pgxpool.Pool {
	if s == nil {
		return nil
	}
	return s.pool
}

func (s *ProfileStore) GetByID(ctx context.Context, id uuid.UUID) (*ProfileRow, error) {
	return s.scanOne(ctx, `SELECT `+profileSelectCols+` FROM profiles WHERE id = $1`, id)
}

func (s *ProfileStore) GetByUsernameDiscriminator(ctx context.Context, username, discriminator string) (*ProfileRow, error) {
	return s.scanOne(ctx, `SELECT `+profileSelectCols+` FROM profiles WHERE lower(username) = lower($1) AND discriminator = $2`, username, discriminator)
}

func (s *ProfileStore) scanOne(ctx context.Context, sql string, args ...any) (*ProfileRow, error) {
	row := s.pool.QueryRow(ctx, sql, args...)
	p, err := scanProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func scanProfile(row pgx.Row) (*ProfileRow, error) {
	var p ProfileRow
	err := row.Scan(
		&p.ID, &p.AccountID, &p.Username, &p.Discriminator, &p.DisplayName,
		&p.AvatarURL, &p.BannerURL, &p.Bio, &p.Locale, &p.Theme, &p.IsPrimary,
		&p.VerificationType, &p.VerificationBadge, &p.FrozenAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetByIDs returns profiles for the given ids; missing ids are skipped. Order is not guaranteed.
func (s *ProfileStore) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*ProfileRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	q := fmt.Sprintf(`SELECT %s FROM profiles WHERE id IN (%s)`, profileSelectCols, strings.Join(ph, ","))
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

// ListByAccountID returns all profiles owned by the account (primary first, then created_at).
func (s *ProfileStore) ListByAccountID(ctx context.Context, accountID uuid.UUID) ([]*ProfileRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+profileSelectCols+` FROM profiles WHERE account_id = $1 AND deleted_at IS NULL
		ORDER BY is_primary DESC, created_at ASC`, accountID)
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

// GetPrimaryProfileIDForAccount returns the primary profile id for the account, if any.
func (s *ProfileStore) GetPrimaryProfileIDForAccount(ctx context.Context, accountID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM profiles WHERE account_id = $1 AND is_primary = true LIMIT 1`, accountID).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

type UpdateProfileInput struct {
	DisplayName *string
	AvatarURL   *string
	BannerURL   *string
	Bio         *string
	Locale      *string
	Theme       *string
}

func (s *ProfileStore) UpdateOwnedProfile(ctx context.Context, accountID, profileID uuid.UUID, in UpdateProfileInput) (*ProfileRow, error) {
	set := make([]string, 0, 8)
	args := make([]any, 0, 12)
	n := 1

	if in.DisplayName != nil {
		set = append(set, fmt.Sprintf("display_name = $%d", n))
		args = append(args, *in.DisplayName)
		n++
	}
	if in.AvatarURL != nil {
		set = append(set, fmt.Sprintf("avatar_url = $%d", n))
		args = append(args, *in.AvatarURL)
		n++
	}
	if in.BannerURL != nil {
		set = append(set, fmt.Sprintf("banner_url = $%d", n))
		args = append(args, *in.BannerURL)
		n++
	}
	if in.Bio != nil {
		set = append(set, fmt.Sprintf("bio = $%d", n))
		args = append(args, *in.Bio)
		n++
	}
	if in.Locale != nil {
		set = append(set, fmt.Sprintf("locale = $%d", n))
		args = append(args, *in.Locale)
		n++
	}
	if in.Theme != nil {
		set = append(set, fmt.Sprintf("theme = $%d", n))
		args = append(args, *in.Theme)
		n++
	}
	if len(set) == 0 {
		return s.scanOne(ctx, `SELECT `+profileSelectCols+` FROM profiles WHERE id = $1 AND account_id = $2`, profileID, accountID)
	}
	set = append(set, "updated_at = now()")

	w1, w2 := n, n+1
	args = append(args, profileID, accountID)
	sql := fmt.Sprintf(`UPDATE profiles SET %s WHERE id = $%d AND account_id = $%d RETURNING %s`,
		strings.Join(set, ", "), w1, w2, profileSelectCols)

	row := s.pool.QueryRow(ctx, sql, args...)
	p, err := scanProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// CreateSecondaryProfile inserts a non-primary profile for the account (multi-profile row).
func (s *ProfileStore) CreateSecondaryProfile(ctx context.Context, accountID uuid.UUID, displayName string, usernameHint *string) (*ProfileRow, error) {
	dn := truncate(strings.TrimSpace(displayName), MaxDisplayNameRunes)
	if dn == "" {
		return nil, fmt.Errorf("display_name required")
	}

	base := sanitizeUsernameFromHint(usernameHint)
	if base == "" {
		base = sanitizeUsernameFromHint(&dn)
	}

	var lastErr error
	for attempt := 0; attempt < maxDiscriminatorAttempts; attempt++ {
		disc := randomDiscriminator()
		id := uuid.New()
		row := s.pool.QueryRow(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, locale, theme, verification_type)
			VALUES ($1, $2, $3, $4, $5, false, 'ru', 'dark', 'none')
			RETURNING `+profileSelectCols,
			id, accountID, base, disc, dn,
		)
		p, err := scanProfile(row)
		if err == nil {
			return p, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			lastErr = err
			continue
		}
		return nil, err
	}
	if lastErr != nil {
		return nil, fmt.Errorf("username/discriminator exhausted: %w", lastErr)
	}
	return nil, fmt.Errorf("username/discriminator exhausted")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func sanitizeUsernameFromHint(hint *string) string {
	if hint == nil {
		return ""
	}
	raw := strings.TrimSpace(*hint)
	if i := strings.Index(raw, "@"); i >= 0 {
		raw = raw[:i]
	}
	var sb strings.Builder
	for _, r := range strings.ToLower(raw) {
		if sb.Len() >= 32 {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		}
	}
	if sb.Len() == 0 {
		return "user"
	}
	return sb.String()
}

func randomDiscriminator() string {
	var b [2]byte
	_, _ = rand.Read(b[:])
	n := int(binary.BigEndian.Uint16(b[:])) % 10000
	return fmt.Sprintf("%04d", n)
}

// CountByAccountID returns how many profiles the account owns.
func (s *ProfileStore) CountByAccountID(ctx context.Context, accountID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE account_id = $1`, accountID).Scan(&n)
	return n, err
}

// MaxProfilesForTier returns allowed profile count for subscription tier.
func MaxProfilesForTier(tier string) int {
	if tier == "premium" {
		return profileCountPremium
	}
	return profileCountFree
}

// ApplyFreeTierFreeze marks profiles beyond the free cap as frozen (keeps primary + earliest secondaries).
func (s *ProfileStore) ApplyFreeTierFreeze(ctx context.Context, accountID uuid.UUID) error {
	rows, err := s.ListByAccountID(ctx, accountID)
	if err != nil {
		return err
	}
	if len(rows) <= profileCountFree {
		return nil
	}
	for i := profileCountFree; i < len(rows); i++ {
		if rows[i].FrozenAt != nil {
			continue
		}
		_, err := s.pool.Exec(ctx, `UPDATE profiles SET frozen_at = now(), updated_at = now()
			WHERE id = $1 AND account_id = $2 AND frozen_at IS NULL`, rows[i].ID, accountID)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetOwnedProfile returns a profile owned by the account (not soft-deleted).
func (s *ProfileStore) GetOwnedProfile(ctx context.Context, accountID, profileID uuid.UUID) (*ProfileRow, error) {
	return s.scanOne(ctx, `SELECT `+profileSelectCols+` FROM profiles WHERE id = $1 AND account_id = $2 AND deleted_at IS NULL`, profileID, accountID)
}
