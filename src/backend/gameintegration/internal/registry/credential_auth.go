package registry

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ServicePrincipal struct {
	CredentialID  uuid.UUID
	ApplicationID uuid.UUID
	EnvironmentID uuid.UUID
	Scopes        []string
}

// VerifyCredential checks current database state on every request. A Voice
// player token cannot authenticate as a game service credential.
func (s *Store) VerifyCredential(ctx context.Context, bearer, scope string, key []byte) (ServicePrincipal, error) {
	parts := strings.Split(bearer, "_")
	if len(parts) != 3 || parts[0] != "vgi1" || len(key) != 32 || !slices.Contains(allowedServiceScopes, scope) {
		return ServicePrincipal{}, ErrInvalidServiceCredential
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil {
		return ServicePrincipal{}, ErrInvalidServiceCredential
	}
	secretBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(secretBytes) != 32 || base64.RawURLEncoding.EncodeToString(secretBytes) != parts[2] {
		return ServicePrincipal{}, ErrInvalidServiceCredential
	}
	if s == nil || s.Pool == nil {
		return ServicePrincipal{}, ErrRegistryUnavailable
	}
	var principal ServicePrincipal
	var digest []byte
	var appStatus, envStatus string
	var expiresAt time.Time
	var revokedAt pgtype.Timestamptz
	err = s.Pool.QueryRow(ctx, `SELECT c.id, a.id, e.id, c.secret_digest, c.scopes,
		c.expires_at, c.revoked_at, a.status, e.status
		FROM service_credentials c JOIN environments e ON e.id=c.environment_id
		JOIN applications a ON a.id=e.application_id WHERE c.id=$1`, id).
		Scan(&principal.CredentialID, &principal.ApplicationID, &principal.EnvironmentID,
			&digest, &principal.Scopes, &expiresAt, &revokedAt, &appStatus, &envStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return ServicePrincipal{}, ErrInvalidServiceCredential
	}
	if err != nil {
		return ServicePrincipal{}, fmt.Errorf("read service credential: %w", err)
	}
	if !hmac.Equal(digest, credentialDigest(key, parts[2])) || !slices.Contains(principal.Scopes, scope) ||
		!time.Now().Before(expiresAt) || revokedAt.Valid || envStatus != "active" ||
		(appStatus != "sandbox" && appStatus != "active") {
		return ServicePrincipal{}, ErrInvalidServiceCredential
	}
	return principal, nil
}

// RevokeCredential makes admission fail on the next verification. Repeating
// the same owner request is a no-op with no additional audit entry.
func (s *Store) RevokeCredential(ctx context.Context, ownerID, appID, envID, credentialID uuid.UUID) error {
	if ownerID == uuid.Nil || appID == uuid.Nil || envID == uuid.Nil || credentialID == uuid.Nil {
		return ErrInvalidCredentialRequest
	}
	if s == nil || s.Pool == nil {
		return ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin credential revoke: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var storedOwner uuid.UUID
	var revokedAt pgtype.Timestamptz
	err = tx.QueryRow(ctx, `SELECT a.owner_account_id, c.revoked_at FROM service_credentials c
		JOIN environments e ON e.id=c.environment_id JOIN applications a ON a.id=e.application_id
		WHERE c.id=$1 AND e.id=$2 AND a.id=$3 FOR UPDATE OF c`, credentialID, envID, appID).
		Scan(&storedOwner, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAdmissionConflict
	}
	if err != nil {
		return fmt.Errorf("lock credential revoke: %w", err)
	}
	if storedOwner != ownerID {
		return ErrAdmissionConflict
	}
	if !revokedAt.Valid {
		_, err = tx.Exec(ctx, `UPDATE service_credentials SET revoked_at=now() WHERE id=$1`, credentialID)
		if err != nil {
			return fmt.Errorf("revoke credential: %w", err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO registry_audit
			(id,actor_kind,actor_id,application_id,environment_id,action,new_status,operation_key)
			VALUES ($1,'account',$2,$3,$4,'revoke_credential','revoked',$5)`,
			uuid.New(), ownerID, appID, envID, credentialID.String())
		if err != nil {
			return fmt.Errorf("audit credential revoke: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit credential revoke: %w", err)
	}
	return nil
}
