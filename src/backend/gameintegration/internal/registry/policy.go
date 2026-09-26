package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrPolicyUnavailable = errors.New("authorization policy unavailable")
	ErrPolicyConflict    = errors.New("authorization policy revision conflict")
	ErrInvalidPolicy     = errors.New("invalid authorization policy")
)

var allowedPlayerScopes = []string{
	"game.chat.read", "game.chat.send", "game.identity.read", "game.invites.create",
	"game.presence.write", "game.voice.join",
}

type UpdateSandboxPolicyInput struct {
	OwnerAccountID   uuid.UUID
	ApplicationID    uuid.UUID
	EnvironmentID    uuid.UUID
	ExpectedRevision int64
	RedirectURIs     []string
	AllowedOrigins   []string
	Providers        []string
	PlayerScopes     []string
	IdempotencyKey   string
}

type AuthorizationPolicy struct {
	ApplicationID  uuid.UUID `json:"application_id"`
	EnvironmentID  uuid.UUID `json:"environment_id"`
	Revision       int64     `json:"revision"`
	DisplayName    string    `json:"display_name"`
	RedirectURIs   []string  `json:"redirect_uris"`
	AllowedOrigins []string  `json:"allowed_origins"`
	Providers      []string  `json:"providers"`
	PlayerScopes   []string  `json:"player_scopes"`
}

func normalizePolicy(in UpdateSandboxPolicyInput) (UpdateSandboxPolicyInput, [32]byte, error) {
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.OwnerAccountID == uuid.Nil || in.ApplicationID == uuid.Nil || in.EnvironmentID == uuid.Nil ||
		in.ExpectedRevision <= 0 || in.IdempotencyKey == "" || len(in.IdempotencyKey) > 128 ||
		len(in.RedirectURIs) == 0 || len(in.RedirectURIs) > 10 || len(in.AllowedOrigins) > 10 ||
		len(in.Providers) != 1 || len(in.PlayerScopes) == 0 || len(in.PlayerScopes) > len(allowedPlayerScopes) {
		return UpdateSandboxPolicyInput{}, [32]byte{}, ErrInvalidPolicy
	}
	if in.Providers[0] != "google" {
		return UpdateSandboxPolicyInput{}, [32]byte{}, ErrInvalidPolicy
	}
	in.RedirectURIs = append([]string(nil), in.RedirectURIs...)
	in.AllowedOrigins = append([]string(nil), in.AllowedOrigins...)
	in.Providers = append([]string(nil), in.Providers...)
	in.PlayerScopes = append([]string(nil), in.PlayerScopes...)
	for _, redirect := range in.RedirectURIs {
		if !validRedirect(redirect) {
			return UpdateSandboxPolicyInput{}, [32]byte{}, ErrInvalidPolicy
		}
	}
	for _, origin := range in.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
			parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" ||
			strings.ContainsAny(origin, "\r\n") {
			return UpdateSandboxPolicyInput{}, [32]byte{}, ErrInvalidPolicy
		}
	}
	for _, scope := range in.PlayerScopes {
		if !slices.Contains(allowedPlayerScopes, scope) {
			return UpdateSandboxPolicyInput{}, [32]byte{}, ErrInvalidPolicy
		}
	}
	for _, values := range [][]string{in.RedirectURIs, in.AllowedOrigins, in.PlayerScopes} {
		slices.Sort(values)
		for i := 1; i < len(values); i++ {
			if values[i] == values[i-1] {
				return UpdateSandboxPolicyInput{}, [32]byte{}, ErrInvalidPolicy
			}
		}
	}
	encoded, err := json.Marshal(struct {
		ApplicationID    uuid.UUID `json:"application_id"`
		EnvironmentID    uuid.UUID `json:"environment_id"`
		ExpectedRevision int64     `json:"expected_revision"`
		RedirectURIs     []string  `json:"redirect_uris"`
		AllowedOrigins   []string  `json:"allowed_origins"`
		Providers        []string  `json:"providers"`
		PlayerScopes     []string  `json:"player_scopes"`
	}{in.ApplicationID, in.EnvironmentID, in.ExpectedRevision, in.RedirectURIs,
		in.AllowedOrigins, in.Providers, in.PlayerScopes})
	if err != nil {
		return UpdateSandboxPolicyInput{}, [32]byte{}, fmt.Errorf("canonical policy request: %w", err)
	}
	return in, sha256.Sum256(encoded), nil
}

func validRedirect(value string) bool {
	if value == "" || len(value) > 2048 || strings.ContainsAny(value, "\r\n") {
		return false
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" || u.Path == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return u.Hostname() != ""
	case "http":
		host := u.Hostname()
		return host == "localhost" || net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
	case "voicegame":
		return u.Host == "auth"
	default:
		return false
	}
}

// UpdateSandboxPolicy permits an approved sandbox owner to configure exact
// browser origins, callbacks, provider and player scopes with CAS revision.
func (s *Store) UpdateSandboxPolicy(ctx context.Context, input UpdateSandboxPolicyInput) (Environment, error) {
	in, hash, err := normalizePolicy(input)
	if err != nil {
		return Environment{}, err
	}
	if s == nil || s.Pool == nil {
		return Environment{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Environment{}, fmt.Errorf("begin policy update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var ownerID uuid.UUID
	var appStatus, envStatus, envKind string
	var revision int64
	err = tx.QueryRow(ctx, `SELECT a.owner_account_id,a.status,e.status,e.kind,e.revision
		FROM environments e JOIN applications a ON a.id=e.application_id
		WHERE a.id=$1 AND e.id=$2 FOR UPDATE OF e,a`, in.ApplicationID, in.EnvironmentID).
		Scan(&ownerID, &appStatus, &envStatus, &envKind, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return Environment{}, ErrAdmissionConflict
	}
	if err != nil {
		return Environment{}, fmt.Errorf("lock policy environment: %w", err)
	}
	if ownerID != in.OwnerAccountID || appStatus != "sandbox" || envStatus != "active" || envKind != "sandbox" {
		return Environment{}, ErrAdmissionConflict
	}
	const route = "environments.update_sandbox_policy"
	command, err := tx.Exec(ctx, `INSERT INTO registry_operations
		(actor_kind,actor_id,route,idempotency_key,request_hash,status)
		VALUES ('account',$1,$2,$3,$4,'pending') ON CONFLICT DO NOTHING`,
		in.OwnerAccountID, route, in.IdempotencyKey, hash[:])
	if err != nil {
		return Environment{}, fmt.Errorf("insert policy operation: %w", err)
	}
	if command.RowsAffected() == 0 {
		var savedHash []byte
		var resultID pgtype.UUID
		var resultRevision pgtype.Int8
		var status string
		err = tx.QueryRow(ctx, `SELECT request_hash,result_id,result_revision,status FROM registry_operations
			WHERE actor_kind='account' AND actor_id=$1 AND route=$2 AND idempotency_key=$3`,
			in.OwnerAccountID, route, in.IdempotencyKey).Scan(&savedHash, &resultID, &resultRevision, &status)
		if err != nil {
			return Environment{}, fmt.Errorf("read policy operation: %w", err)
		}
		if string(savedHash) != string(hash[:]) {
			return Environment{}, ErrIdempotencyConflict
		}
		if status != "succeeded" || !resultID.Valid || !resultRevision.Valid {
			return Environment{}, ErrRegistryUnavailable
		}
		env, err := getEnvironment(ctx, tx, uuid.UUID(resultID.Bytes))
		if err != nil {
			return Environment{}, err
		}
		env.Revision = resultRevision.Int64
		if err := tx.Commit(ctx); err != nil {
			return Environment{}, fmt.Errorf("commit policy retry: %w", err)
		}
		return env, nil
	}
	if revision != in.ExpectedRevision {
		return Environment{}, ErrPolicyConflict
	}
	providerPolicy, err := json.Marshal(struct {
		Providers    []string `json:"providers"`
		PlayerScopes []string `json:"player_scopes"`
	}{in.Providers, in.PlayerScopes})
	if err != nil {
		return Environment{}, fmt.Errorf("encode provider policy: %w", err)
	}
	redirects, _ := json.Marshal(in.RedirectURIs)
	origins, _ := json.Marshal(in.AllowedOrigins)
	_, err = tx.Exec(ctx, `UPDATE environments SET provider_policy=$1::jsonb,redirect_uris=$2::jsonb,
		allowed_origins=$3::jsonb,revision=revision+1,updated_at=now() WHERE id=$4`,
		string(providerPolicy), string(redirects), string(origins), in.EnvironmentID)
	if err != nil {
		return Environment{}, fmt.Errorf("update sandbox policy: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE registry_operations SET result_id=$1,result_revision=$5,status='succeeded',updated_at=now()
		WHERE actor_kind='account' AND actor_id=$2 AND route=$3 AND idempotency_key=$4`,
		in.EnvironmentID, in.OwnerAccountID, route, in.IdempotencyKey, revision+1)
	if err != nil {
		return Environment{}, fmt.Errorf("complete policy operation: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO registry_audit
		(id,actor_kind,actor_id,application_id,environment_id,action,new_status,operation_key)
		VALUES ($1,'account',$2,$3,$4,'update_sandbox_policy','active',$5)`,
		uuid.New(), in.OwnerAccountID, in.ApplicationID, in.EnvironmentID, in.IdempotencyKey)
	if err != nil {
		return Environment{}, fmt.Errorf("audit policy update: %w", err)
	}
	env, err := getEnvironment(ctx, tx, in.EnvironmentID)
	if err != nil {
		return Environment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Environment{}, fmt.Errorf("commit policy update: %w", err)
	}
	return env, nil
}

// LoadAuthorizationPolicy is the fail-closed Auth policy projection. The
// caller must independently authenticate Auth workload identity.
func (s *Store) LoadAuthorizationPolicy(ctx context.Context, environmentID uuid.UUID) (AuthorizationPolicy, error) {
	if environmentID == uuid.Nil || s == nil || s.Pool == nil {
		return AuthorizationPolicy{}, ErrPolicyUnavailable
	}
	var policy AuthorizationPolicy
	var providerRaw, redirectsRaw, originsRaw []byte
	var appStatus, envStatus string
	err := s.Pool.QueryRow(ctx, `SELECT a.id,e.id,e.revision,a.name,e.provider_policy,e.redirect_uris,
		e.allowed_origins,a.status,e.status FROM environments e
		JOIN applications a ON a.id=e.application_id WHERE e.id=$1`, environmentID).
		Scan(&policy.ApplicationID, &policy.EnvironmentID, &policy.Revision, &policy.DisplayName,
			&providerRaw, &redirectsRaw, &originsRaw, &appStatus, &envStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return AuthorizationPolicy{}, ErrPolicyUnavailable
	}
	if err != nil {
		return AuthorizationPolicy{}, fmt.Errorf("read authorization policy: %w", err)
	}
	if envStatus != "active" || appStatus != "sandbox" && appStatus != "active" {
		return AuthorizationPolicy{}, ErrPolicyUnavailable
	}
	var providers struct {
		Providers    []string `json:"providers"`
		PlayerScopes []string `json:"player_scopes"`
	}
	if json.Unmarshal(providerRaw, &providers) != nil || json.Unmarshal(redirectsRaw, &policy.RedirectURIs) != nil ||
		json.Unmarshal(originsRaw, &policy.AllowedOrigins) != nil || len(providers.Providers) != 1 ||
		providers.Providers[0] != "google" || len(providers.PlayerScopes) == 0 || len(policy.RedirectURIs) == 0 {
		return AuthorizationPolicy{}, ErrPolicyUnavailable
	}
	policy.Providers = providers.Providers
	policy.PlayerScopes = providers.PlayerScopes
	return policy, nil
}
