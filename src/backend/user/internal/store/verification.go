package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

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
	_, err := s.pool.Exec(ctx, `
		UPDATE organization_verification_requests SET status = 'verified', verified_at = now()
		WHERE profile_id = $1 AND status = 'pending'`, profileID)
	if err != nil {
		return nil, err
	}
	return s.SetProfileVerification(ctx, profileID, "organization", "dns")
}

func randomHexToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
