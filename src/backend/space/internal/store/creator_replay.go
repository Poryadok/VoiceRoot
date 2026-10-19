package store

import (
	"context"
	"encoding/base64"
	"time"
)

// ClaimCreatorPrincipalReplay is a committed, actor-free anti-replay reservation
// before creator session checks or domain work. Ambiguous DB outcomes never
// authorize the writer. Every Space instance shares the same permanent key.
func (s *SpaceStore) ClaimCreatorPrincipalReplay(ctx context.Context, issuer, jti string, expiry time.Time) error {
	if issuer != "gateway" || !validCreatorJWTID(jti) || expiry.IsZero() {
		return ErrMemberAdmissionConflict
	}
	if s == nil || s.Pool == nil || s.tx != nil {
		return ErrMemberAuthorityUnavailable
	}
	tag, err := s.Pool.Exec(ctx, `INSERT INTO space_creator_principal_replays(issuer,jti,expires_at)
	 SELECT $1,$2,$3 WHERE $3>clock_timestamp() AND $3<=clock_timestamp()+interval '35 seconds'
	 ON CONFLICT(issuer,jti) DO NOTHING`, issuer, jti, expiry.UTC())
	if err != nil {
		return ErrMemberAuthorityUnavailable
	}
	if tag.RowsAffected() != 1 {
		return ErrMemberAdmissionConflict
	}
	return nil
}

func validCreatorJWTID(jti string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(jti)
	return err == nil && len(raw) == 20 && base64.RawURLEncoding.EncodeToString(raw) == jti
}

func (s *SpaceStore) CleanupCreatorPrincipalReplays(ctx context.Context, limit int) (int64, error) {
	if s == nil || s.Pool == nil || s.tx != nil || limit < 1 || limit > 100 {
		return 0, ErrMemberAdmissionConflict
	}
	tag, err := s.Pool.Exec(ctx, `WITH expired AS (
	 SELECT issuer,jti FROM space_creator_principal_replays WHERE expires_at<clock_timestamp()-interval '5 seconds'
	 ORDER BY expires_at,jti LIMIT $1 FOR UPDATE SKIP LOCKED)
	 DELETE FROM space_creator_principal_replays p USING expired e WHERE p.issuer=e.issuer AND p.jti=e.jti`, limit)
	if err != nil {
		return 0, ErrMemberAuthorityUnavailable
	}
	return tag.RowsAffected(), nil
}
