package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const verificationSourceResolutionSQL = `
	SELECT verification_type, badge
	FROM profile_verification_sources
	WHERE profile_id = $1 AND verified = true
	ORDER BY CASE verification_type WHEN 'organization' THEN 0 ELSE 1 END,
	         CASE source WHEN 'twitch' THEN 0 WHEN 'youtube' THEN 1 ELSE 2 END,
	         revision DESC
	LIMIT 1`

// ApplyVerificationSourceState atomically applies a per-source monotonic revision and
// refreshes the compatibility summary on profiles. Older or conflicting equal revisions
// are ignored, while an exact retry is idempotent.
func (s *ProfileStore) ApplyVerificationSourceState(
	ctx context.Context,
	profileID uuid.UUID,
	source string,
	revision int64,
	verified bool,
	verificationType string,
	badge string,
) (*ProfileRow, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row, applied, err := applyVerificationSourceStateTx(
		ctx, tx, profileID, source, revision, verified, verificationType, badge,
	)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return row, applied, nil
}

func applyVerificationSourceStateTx(
	ctx context.Context,
	tx pgx.Tx,
	profileID uuid.UUID,
	source string,
	revision int64,
	verified bool,
	verificationType string,
	badge string,
) (*ProfileRow, bool, error) {
	var lockedProfileID uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id FROM profiles
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE`, profileID).Scan(&lockedProfileID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var currentRevision int64
	var currentVerified bool
	var currentType, currentBadge string
	err = tx.QueryRow(ctx, `
		SELECT revision, verified, verification_type, badge
		FROM profile_verification_sources
		WHERE profile_id = $1 AND source = $2
		FOR UPDATE`, profileID, source).Scan(&currentRevision, &currentVerified, &currentType, &currentBadge)
	applied := false
	resolveSummary := false
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_, err = tx.Exec(ctx, `
			INSERT INTO profile_verification_sources
				(profile_id, source, verification_type, badge, verified, revision)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			profileID, source, verificationType, badge, verified, revision)
		applied = err == nil
		resolveSummary = err == nil
	case err != nil:
		return nil, false, err
	case revision > currentRevision:
		_, err = tx.Exec(ctx, `
			UPDATE profile_verification_sources
			SET verification_type = $3, badge = $4, verified = $5, revision = $6, updated_at = now()
			WHERE profile_id = $1 AND source = $2`,
			profileID, source, verificationType, badge, verified, revision)
		applied = err == nil
		resolveSummary = err == nil
	case revision == currentRevision && currentVerified == verified && currentType == verificationType && currentBadge == badge:
		// Exact retry; the effective summary is still repaired below.
		resolveSummary = true
	default:
		// Stale or conflicting equal revision must not mutate even compatibility timestamps.
	}
	if err != nil {
		return nil, false, err
	}
	if !resolveSummary {
		profile, err := scanProfile(tx.QueryRow(ctx, `
			SELECT `+profileSelectCols+`
			FROM profiles
			WHERE id = $1 AND deleted_at IS NULL`, profileID))
		return profile, false, err
	}

	effectiveType := "none"
	var effectiveBadge *string
	var resolvedType, resolvedBadge string
	err = tx.QueryRow(ctx, verificationSourceResolutionSQL, profileID).Scan(&resolvedType, &resolvedBadge)
	if err == nil {
		effectiveType = resolvedType
		effectiveBadge = &resolvedBadge
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	row := tx.QueryRow(ctx, `
		UPDATE profiles
		SET verification_type = $2, verification_badge = $3, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+profileSelectCols, profileID, effectiveType, effectiveBadge)
	profile, err := scanProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return profile, applied, nil
}

// SetProfileVerification updates verification_type and badge on a profile.
func (s *ProfileStore) SetProfileVerification(ctx context.Context, profileID uuid.UUID, verificationType, badge string) (*ProfileRow, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE profiles SET verification_type = $2, verification_badge = $3, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+profileSelectCols,
		profileID, verificationType, badge,
	)
	p, err := scanProfile(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// ClearProfileVerification resets verification fields.
func (s *ProfileStore) ClearProfileVerification(ctx context.Context, profileID uuid.UUID) (*ProfileRow, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE profiles SET verification_type = 'none', verification_badge = NULL, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+profileSelectCols, profileID)
	p, err := scanProfile(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// HasVerifiedUsernameConflict returns true if normalized username matches a verified profile.
func (s *ProfileStore) HasVerifiedUsernameConflict(ctx context.Context, normalizedKey string, excludeProfileID uuid.UUID) (bool, error) {
	if normalizedKey == "" {
		return false, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT username FROM profiles
		WHERE verification_type <> 'none' AND deleted_at IS NULL AND id <> $1`,
		excludeProfileID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return false, err
		}
		if NormalizeUsernameKey(username) == normalizedKey {
			return true, nil
		}
	}
	return false, rows.Err()
}

// SoftDeleteProfile archives a non-primary owned profile.
func (s *ProfileStore) SoftDeleteProfile(ctx context.Context, accountID, profileID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE profiles SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND account_id = $2 AND is_primary = false AND deleted_at IS NULL`,
		profileID, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ApplyDowngradeProfileSelection unfreezes kept profiles and freezes others for free tier.
func (s *ProfileStore) ApplyDowngradeProfileSelection(ctx context.Context, accountID uuid.UUID, kept []uuid.UUID) error {
	keptSet := make(map[uuid.UUID]struct{}, len(kept))
	for _, id := range kept {
		keptSet[id] = struct{}{}
	}
	rows, err := s.ListByAccountID(ctx, accountID)
	if err != nil {
		return err
	}
	for _, p := range rows {
		_, keep := keptSet[p.ID]
		if keep {
			_, err = s.pool.Exec(ctx, `UPDATE profiles SET frozen_at = NULL, updated_at = now()
				WHERE id = $1 AND account_id = $2`, p.ID, accountID)
		} else {
			_, err = s.pool.Exec(ctx, `UPDATE profiles SET frozen_at = now(), updated_at = now()
				WHERE id = $1 AND account_id = $2 AND frozen_at IS NULL`, p.ID, accountID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// StartOrgVerification creates a pending DNS TXT verification request.
func (s *ProfileStore) StartOrgVerification(ctx context.Context, profileID uuid.UUID, domain string) (txtRecord string, err error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return "", fmt.Errorf("domain required")
	}
	token, err := randomHexToken(16)
	if err != nil {
		return "", err
	}
	txt := "voice-verify=" + token
	_, err = s.pool.Exec(ctx, `
		INSERT INTO organization_verification_requests (profile_id, domain, txt_token, status)
		VALUES ($1, $2, $3, 'pending')`,
		profileID, domain, token)
	if err != nil {
		return "", err
	}
	return txt, nil
}

// LatestOrgVerification returns domain and txt token for pending org verification.
func (s *ProfileStore) LatestOrgVerification(ctx context.Context, profileID uuid.UUID) (domain, token string, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT domain, txt_token FROM organization_verification_requests
		WHERE profile_id = $1 AND status = 'pending'
		ORDER BY created_at DESC LIMIT 1`, profileID).Scan(&domain, &token)
	return domain, token, err
}

// MarkOrgVerificationVerified marks request verified and updates profile.
func (s *ProfileStore) MarkOrgVerificationVerified(ctx context.Context, profileID uuid.UUID) (*ProfileRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		UPDATE organization_verification_requests SET status = 'verified', verified_at = now()
		WHERE profile_id = $1 AND status = 'pending'`, profileID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	var revision int64
	if err := tx.QueryRow(ctx, `SELECT nextval('profile_verification_revision_seq')`).Scan(&revision); err != nil {
		return nil, err
	}
	row, _, err := applyVerificationSourceStateTx(
		ctx, tx, profileID, "organization_dns", revision, true, "organization", "dns",
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return row, nil
}

func randomHexToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
