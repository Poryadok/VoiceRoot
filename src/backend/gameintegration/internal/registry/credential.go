package registry

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInvalidCredentialRequest = errors.New("invalid credential request")
	ErrCredentialRevealExpired  = errors.New("credential retry reveal expired")
	ErrInvalidServiceCredential = errors.New("invalid service credential")
)

var allowedServiceScopes = []string{
	"game.commands.read", "game.events.write", "game.roster.write", "game.sessions.write", "game.sessions.manage",
}

type IssueCredentialInput struct {
	OwnerAccountID uuid.UUID
	ApplicationID  uuid.UUID
	EnvironmentID  uuid.UUID
	Scopes         []string
	IdempotencyKey string
	SecretKey      []byte
}

type Credential struct {
	ID             uuid.UUID
	ApplicationID  uuid.UUID
	OwnerAccountID uuid.UUID
	EnvironmentID  uuid.UUID
	Scopes         []string
	Generation     int64
	Secret         string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

func canonicalCredentialInput(in IssueCredentialInput) (IssueCredentialInput, [32]byte, error) {
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.OwnerAccountID == uuid.Nil || in.ApplicationID == uuid.Nil || in.EnvironmentID == uuid.Nil ||
		in.IdempotencyKey == "" || len(in.IdempotencyKey) > 128 || len(in.SecretKey) != 32 ||
		len(in.Scopes) == 0 || len(in.Scopes) > len(allowedServiceScopes) {
		return IssueCredentialInput{}, [32]byte{}, ErrInvalidCredentialRequest
	}
	in.Scopes = append([]string(nil), in.Scopes...)
	slices.Sort(in.Scopes)
	for i, scope := range in.Scopes {
		if !slices.Contains(allowedServiceScopes, scope) || i > 0 && in.Scopes[i-1] == scope {
			return IssueCredentialInput{}, [32]byte{}, ErrInvalidCredentialRequest
		}
	}
	encoded, err := json.Marshal(struct {
		ApplicationID string   `json:"application_id"`
		EnvironmentID string   `json:"environment_id"`
		Scopes        []string `json:"scopes"`
	}{in.ApplicationID.String(), in.EnvironmentID.String(), in.Scopes})
	if err != nil {
		return IssueCredentialInput{}, [32]byte{}, fmt.Errorf("canonical credential request: %w", err)
	}
	return in, sha256.Sum256(encoded), nil
}

func deriveCredentialSecret(key []byte, id uuid.UUID) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("voice-game-service-v1:" + id.String()))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func credentialDigest(key []byte, secret string, credential Credential) []byte {
	mac := hmac.New(sha256.New, key)
	scopes := append([]string(nil), credential.Scopes...)
	slices.Sort(scopes)
	_, _ = mac.Write([]byte("credential-authority-v2\n"))
	_, _ = mac.Write([]byte(secret + "\n" + credential.ID.String() + "\n" + credential.ApplicationID.String() + "\n" +
		credential.OwnerAccountID.String() + "\n" + credential.EnvironmentID.String() + "\n" +
		fmt.Sprint(credential.Generation) + "\n" + credential.CreatedAt.UTC().Format(time.RFC3339Nano) + "\n" +
		credential.ExpiresAt.UTC().Format(time.RFC3339Nano) + "\n" + strings.Join(scopes, "\n")))
	return mac.Sum(nil)
}

// IssueCredential records a scoped game-service secret. The secret is derived
// from a random credential ID and a deployment key, so an identical request
// can recover a lost response during the bounded ten-minute reveal window.
func (s *Store) IssueCredential(ctx context.Context, input IssueCredentialInput) (Credential, error) {
	in, hash, err := canonicalCredentialInput(input)
	if err != nil {
		return Credential{}, err
	}
	if s == nil || s.Pool == nil {
		return Credential{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Credential{}, fmt.Errorf("begin credential issue: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var ownerID uuid.UUID
	var appStatus, envStatus, environmentKind string
	err = tx.QueryRow(ctx, `SELECT a.owner_account_id, a.status, e.status, e.kind FROM environments e
		JOIN applications a ON a.id=e.application_id
		WHERE e.id=$1 AND a.id=$2 FOR UPDATE OF e,a`, in.EnvironmentID, in.ApplicationID).
		Scan(&ownerID, &appStatus, &envStatus, &environmentKind)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credential{}, ErrAdmissionConflict
	}
	if err != nil {
		return Credential{}, fmt.Errorf("lock credential environment: %w", err)
	}
	if ownerID != in.OwnerAccountID || envStatus != "active" || environmentKind != "sandbox" ||
		(appStatus != "sandbox" && appStatus != "active" && appStatus != "suspended") {
		return Credential{}, ErrAdmissionConflict
	}
	if appStatus == "suspended" {
		return Credential{}, ErrApplicationSuspended
	}
	const route = "credentials.issue"
	command, err := tx.Exec(ctx, `INSERT INTO registry_operations
		(actor_kind, actor_id, route, idempotency_key, request_hash, status)
		VALUES ('account', $1, $2, $3, $4, 'pending') ON CONFLICT DO NOTHING`,
		in.OwnerAccountID, route, in.IdempotencyKey, hash[:])
	if err != nil {
		return Credential{}, fmt.Errorf("insert credential operation: %w", err)
	}
	if command.RowsAffected() == 0 {
		var savedHash []byte
		var resultID pgtype.UUID
		var status string
		err = tx.QueryRow(ctx, `SELECT request_hash, result_id, status FROM registry_operations
			WHERE actor_kind='account' AND actor_id=$1 AND route=$2 AND idempotency_key=$3`,
			in.OwnerAccountID, route, in.IdempotencyKey).Scan(&savedHash, &resultID, &status)
		if err != nil {
			return Credential{}, fmt.Errorf("read credential operation: %w", err)
		}
		if !hmac.Equal(savedHash, hash[:]) {
			return Credential{}, ErrIdempotencyConflict
		}
		if status != "succeeded" || !resultID.Valid {
			return Credential{}, ErrRegistryUnavailable
		}
		credential, err := getCredential(ctx, tx, uuid.UUID(resultID.Bytes))
		if err != nil {
			return Credential{}, err
		}
		var savedDigest []byte
		if err := tx.QueryRow(ctx, `SELECT secret_digest FROM service_credentials WHERE id=$1`, credential.ID).Scan(&savedDigest); err != nil {
			return Credential{}, fmt.Errorf("read credential retry authority: %w", err)
		}
		secret := deriveCredentialSecret(in.SecretKey, credential.ID)
		if !hmac.Equal(savedDigest, credentialDigest(in.SecretKey, secret, credential)) {
			return Credential{}, ErrInvalidServiceCredential
		}
		if !s.now().Before(credential.CreatedAt.Add(10 * time.Minute)) {
			return Credential{}, ErrCredentialRevealExpired
		}
		credential.Secret = secret
		if err := tx.Commit(ctx); err != nil {
			return Credential{}, fmt.Errorf("commit credential retry: %w", err)
		}
		return credential, nil
	}
	var generation int64
	err = tx.QueryRow(ctx, `SELECT coalesce(max(generation),0)+1 FROM service_credentials WHERE environment_id=$1`, in.EnvironmentID).Scan(&generation)
	if err != nil {
		return Credential{}, fmt.Errorf("select credential generation: %w", err)
	}
	// PostgreSQL stores timestamps at microsecond precision. Return and persist
	// the same creation/expiry values so an idempotent retry has an identical
	// response after the transaction is read back from the database.
	now := s.now().Truncate(time.Microsecond)
	credential := Credential{ID: uuid.New(), ApplicationID: in.ApplicationID, OwnerAccountID: in.OwnerAccountID,
		EnvironmentID: in.EnvironmentID, Scopes: in.Scopes,
		Generation: generation, CreatedAt: now, ExpiresAt: now.Add(90 * 24 * time.Hour)}
	credential.Secret = deriveCredentialSecret(in.SecretKey, credential.ID)
	// Bound every active predecessor and reseal its SQL authority fields. The
	// bearer remains the same through the overlap, but SQL-only scope/expiry/env
	// edits no longer retain a valid keyed digest.
	cutoff := now.Add(10 * time.Minute)
	rows, err := tx.Query(ctx, `SELECT c.id,e.application_id,a.owner_account_id,c.environment_id,c.scopes,
		c.generation,c.created_at,c.expires_at FROM service_credentials c
		JOIN environments e ON e.id=c.environment_id JOIN applications a ON a.id=e.application_id
		WHERE c.environment_id=$1 AND c.revoked_at IS NULL AND c.expires_at>$2 ORDER BY c.id FOR UPDATE OF c`,
		in.EnvironmentID, cutoff)
	if err != nil {
		return Credential{}, fmt.Errorf("bound prior credential overlap: %w", err)
	}
	var predecessors []Credential
	for rows.Next() {
		var prior Credential
		if err := rows.Scan(&prior.ID, &prior.ApplicationID, &prior.OwnerAccountID, &prior.EnvironmentID,
			&prior.Scopes, &prior.Generation, &prior.CreatedAt, &prior.ExpiresAt); err != nil {
			rows.Close()
			return Credential{}, fmt.Errorf("read prior credential overlap: %w", err)
		}
		prior.CreatedAt = prior.CreatedAt.UTC()
		prior.ExpiresAt = prior.ExpiresAt.UTC()
		predecessors = append(predecessors, prior)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Credential{}, fmt.Errorf("read prior credential overlap: %w", err)
	}
	rows.Close()
	for _, prior := range predecessors {
		if prior.ExpiresAt.After(cutoff) {
			prior.ExpiresAt = cutoff
			digest := credentialDigest(in.SecretKey, deriveCredentialSecret(in.SecretKey, prior.ID), prior)
			if _, err := tx.Exec(ctx, `UPDATE service_credentials SET expires_at=$2,secret_digest=$3 WHERE id=$1`,
				prior.ID, cutoff, digest); err != nil {
				return Credential{}, fmt.Errorf("update prior credential overlap: %w", err)
			}
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO service_credentials
		(id,environment_id,secret_digest,scopes,generation,created_at,expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, credential.ID, in.EnvironmentID,
		credentialDigest(in.SecretKey, credential.Secret, credential), in.Scopes, generation, now, credential.ExpiresAt)
	if err != nil {
		return Credential{}, fmt.Errorf("insert service credential: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE registry_operations SET result_id=$1, status='succeeded', updated_at=now()
		WHERE actor_kind='account' AND actor_id=$2 AND route=$3 AND idempotency_key=$4`,
		credential.ID, in.OwnerAccountID, route, in.IdempotencyKey)
	if err != nil {
		return Credential{}, fmt.Errorf("complete credential operation: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO registry_audit
		(id, actor_kind, actor_id, application_id, environment_id, action, new_status, operation_key,
		 source, result, reason_code)
		VALUES ($1,'account',$2,$3,$4,'issue_credential','active',$5,'authenticated_account','success','credential_issued')`,
		uuid.New(), in.OwnerAccountID, in.ApplicationID, in.EnvironmentID, in.IdempotencyKey)
	if err != nil {
		return Credential{}, fmt.Errorf("audit credential issue: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Credential{}, fmt.Errorf("commit credential issue: %w", err)
	}
	return credential, nil
}

func getCredential(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Credential, error) {
	var credential Credential
	err := tx.QueryRow(ctx, `SELECT c.id, e.application_id, a.owner_account_id, c.environment_id, c.scopes,
		c.generation, c.created_at, c.expires_at FROM service_credentials c
		JOIN environments e ON e.id=c.environment_id JOIN applications a ON a.id=e.application_id WHERE c.id=$1`, id).
		Scan(&credential.ID, &credential.ApplicationID, &credential.OwnerAccountID, &credential.EnvironmentID,
			&credential.Scopes, &credential.Generation, &credential.CreatedAt, &credential.ExpiresAt)
	if err != nil {
		return Credential{}, fmt.Errorf("read service credential: %w", err)
	}
	credential.CreatedAt = credential.CreatedAt.UTC()
	credential.ExpiresAt = credential.ExpiresAt.UTC()
	return credential, nil
}
