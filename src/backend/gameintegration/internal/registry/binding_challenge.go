package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrBindingChallengeNotFound = errors.New("binding challenge not found")
	ErrInvalidBindingChallenge  = errors.New("invalid binding challenge")
)

type BindingChallenge struct {
	ChallengeID         uuid.UUID `json:"challenge_id"`
	Nonce               string    `json:"nonce"`
	ApplicationID       uuid.UUID `json:"application_id"`
	EnvironmentID       uuid.UUID `json:"environment_id"`
	Provider            string    `json:"provider"`
	RedirectURIHash     string    `json:"redirect_uri_sha256"`
	PKCEChallenge       string    `json:"pkce_challenge"`
	DeviceKeyID         uuid.UUID `json:"device_key_id"`
	DeviceKeyThumbprint string    `json:"device_key_thumbprint"`
	SourceAccountID     uuid.UUID `json:"source_account_id"`
	SourceActorID       uuid.UUID `json:"source_actor_id"`
	SourceDeviceID      uuid.UUID `json:"source_device_id"`
	SourceGeneration    int64     `json:"source_generation"`
	TargetAccountID     uuid.UUID `json:"target_account_id"`
	TargetProfileID     uuid.UUID `json:"target_profile_id"`
	ProfileRevision     int64     `json:"profile_revision"`
	ConsentRevision     int64     `json:"consent_revision"`
	PolicyRevision      int64     `json:"policy_revision"`
	Scopes              []string  `json:"scopes"`
	OperationID         uuid.UUID `json:"operation_id"`
	ExpiresAt           time.Time `json:"expires_at"`
	Status              string    `json:"status"`
}

// BindingChallengeCreate is Auth's authenticated, post-proof challenge intent.
// GIS allocates the challenge ID and nonce; operation ID is the replay key.
type BindingChallengeCreate struct {
	ApplicationID       uuid.UUID `json:"application_id"`
	EnvironmentID       uuid.UUID `json:"environment_id"`
	Provider            string    `json:"provider"`
	RedirectURIHash     string    `json:"redirect_uri_sha256"`
	PKCEChallenge       string    `json:"pkce_challenge"`
	DeviceKeyID         uuid.UUID `json:"device_key_id"`
	DeviceKeyThumbprint string    `json:"device_key_thumbprint"`
	OperationID         uuid.UUID `json:"operation_id"`
	ExpiresAt           time.Time `json:"expires_at"`
	SourceAccountID     uuid.UUID `json:"source_account_id"`
	SourceActorID       uuid.UUID `json:"source_actor_id"`
	SourceDeviceID      uuid.UUID `json:"source_device_id"`
	SourceGeneration    int64     `json:"source_generation"`
	TargetAccountID     uuid.UUID `json:"target_account_id"`
	TargetProfileID     uuid.UUID `json:"target_profile_id"`
	ProfileRevision     int64     `json:"profile_revision"`
	ConsentRevision     int64     `json:"consent_revision"`
	PolicyRevision      int64     `json:"policy_revision"`
	Scopes              []string  `json:"scopes"`
}

var (
	challengeNoncePattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	challengeProviderPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	challengeDigestPattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	challengeCodePattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

// CreateBindingChallenge is the GIS-owned persistence seam for the verified
// T13 challenge writer. It does not accept or store provider tokens/subjects.
func (s *Store) CreateBindingChallenge(ctx context.Context, challenge BindingChallenge) error {
	if s == nil || s.Pool == nil {
		return ErrRegistryUnavailable
	}
	if challenge.ChallengeID == uuid.Nil || challenge.ApplicationID == uuid.Nil || challenge.EnvironmentID == uuid.Nil ||
		challenge.DeviceKeyID == uuid.Nil || challenge.OperationID == uuid.Nil ||
		!challengeNoncePattern.MatchString(challenge.Nonce) || !challengeProviderPattern.MatchString(challenge.Provider) ||
		!challengeDigestPattern.MatchString(challenge.RedirectURIHash) || !challengeCodePattern.MatchString(challenge.PKCEChallenge) ||
		!challengeCodePattern.MatchString(challenge.DeviceKeyThumbprint) || challenge.ExpiresAt.IsZero() ||
		challenge.Status != "pending" {
		return ErrInvalidBindingChallenge
	}
	requestHash := bindingChallengeRequestHash(challenge)
	_, err := s.Pool.Exec(ctx, `INSERT INTO player_binding_challenges
		(challenge_id,nonce,application_id,environment_id,provider,redirect_uri_sha256,pkce_challenge,
		 device_key_id,device_key_thumbprint,operation_id,expires_at,status,request_sha256)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'pending',$12)`, challenge.ChallengeID, challenge.Nonce,
		challenge.ApplicationID, challenge.EnvironmentID, challenge.Provider, challenge.RedirectURIHash,
		challenge.PKCEChallenge, challenge.DeviceKeyID, challenge.DeviceKeyThumbprint, challenge.OperationID,
		challenge.ExpiresAt, requestHash)
	if err != nil {
		return fmt.Errorf("persist GIS binding challenge: %w", err)
	}
	return nil
}

// CreateBindingChallengeForAuth persists an Auth-verified T14 intent. A retry
// with the same operation and bytes returns the original GIS-generated facts.
func (s *Store) CreateBindingChallengeForAuth(ctx context.Context, request BindingChallengeCreate,
	requestHash string, id uuid.UUID, nonce string) (BindingChallenge, error) {
	if s == nil || s.Pool == nil {
		return BindingChallenge{}, ErrRegistryUnavailable
	}
	if request.ApplicationID == uuid.Nil || request.EnvironmentID == uuid.Nil || request.DeviceKeyID == uuid.Nil ||
		request.OperationID == uuid.Nil || !challengeProviderPattern.MatchString(request.Provider) ||
		!challengeDigestPattern.MatchString(request.RedirectURIHash) || !challengeCodePattern.MatchString(request.PKCEChallenge) ||
		!challengeCodePattern.MatchString(request.DeviceKeyThumbprint) || !challengeNoncePattern.MatchString(nonce) ||
		id == uuid.Nil || !validChallengeExpiry(s.now(), request.ExpiresAt) || !challengeDigestPattern.MatchString(requestHash) ||
		request.SourceAccountID == uuid.Nil || request.SourceActorID == uuid.Nil || request.SourceDeviceID != request.DeviceKeyID ||
		request.SourceGeneration <= 0 || request.TargetAccountID == uuid.Nil || request.TargetProfileID == uuid.Nil ||
		request.ProfileRevision <= 0 || request.ConsentRevision <= 0 || request.PolicyRevision <= 0 || !validScopes(request.Scopes) {
		return BindingChallenge{}, ErrInvalidBindingChallenge
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return BindingChallenge{}, fmt.Errorf("begin GIS binding challenge: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO player_binding_challenges
		(challenge_id,nonce,application_id,environment_id,provider,redirect_uri_sha256,pkce_challenge,
		 device_key_id,device_key_thumbprint,operation_id,expires_at,status,request_sha256,source_account_id,source_actor_id,
		 source_device_id,source_generation,target_account_id,target_profile_id,profile_revision,consent_revision,policy_revision,scopes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'pending',$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22) ON CONFLICT (operation_id) DO NOTHING`,
		id, nonce, request.ApplicationID, request.EnvironmentID, request.Provider, request.RedirectURIHash,
		request.PKCEChallenge, request.DeviceKeyID, request.DeviceKeyThumbprint, request.OperationID,
		request.ExpiresAt, requestHash, request.SourceAccountID, request.SourceActorID, request.SourceDeviceID,
		request.SourceGeneration, request.TargetAccountID, request.TargetProfileID, request.ProfileRevision,
		request.ConsentRevision, request.PolicyRevision, request.Scopes)
	if err != nil {
		return BindingChallenge{}, fmt.Errorf("persist GIS binding challenge: %w", err)
	}
	var saved BindingChallenge
	err = tx.QueryRow(ctx, `SELECT challenge_id,nonce,application_id,environment_id,provider,redirect_uri_sha256,
		pkce_challenge,device_key_id,device_key_thumbprint,operation_id,expires_at,status,source_account_id,source_actor_id,
		source_device_id,source_generation,target_account_id,target_profile_id,profile_revision,consent_revision,policy_revision,scopes
		FROM player_binding_challenges WHERE operation_id=$1 FOR UPDATE`, request.OperationID).Scan(
		&saved.ChallengeID, &saved.Nonce, &saved.ApplicationID, &saved.EnvironmentID, &saved.Provider,
		&saved.RedirectURIHash, &saved.PKCEChallenge, &saved.DeviceKeyID, &saved.DeviceKeyThumbprint,
		&saved.OperationID, &saved.ExpiresAt, &saved.Status, &saved.SourceAccountID, &saved.SourceActorID,
		&saved.SourceDeviceID, &saved.SourceGeneration, &saved.TargetAccountID, &saved.TargetProfileID,
		&saved.ProfileRevision, &saved.ConsentRevision, &saved.PolicyRevision, &saved.Scopes)
	if err != nil {
		return BindingChallenge{}, fmt.Errorf("read GIS binding challenge replay: %w", err)
	}
	var savedHash string
	if err := tx.QueryRow(ctx, `SELECT request_sha256 FROM player_binding_challenges WHERE operation_id=$1`, request.OperationID).Scan(&savedHash); err != nil {
		return BindingChallenge{}, fmt.Errorf("read GIS binding challenge request hash: %w", err)
	}
	if savedHash != requestHash || saved.ApplicationID != request.ApplicationID || saved.EnvironmentID != request.EnvironmentID ||
		saved.Provider != request.Provider || saved.RedirectURIHash != request.RedirectURIHash || saved.PKCEChallenge != request.PKCEChallenge ||
		saved.DeviceKeyID != request.DeviceKeyID || saved.DeviceKeyThumbprint != request.DeviceKeyThumbprint ||
		saved.SourceAccountID != request.SourceAccountID || saved.SourceActorID != request.SourceActorID ||
		saved.SourceDeviceID != request.SourceDeviceID || saved.SourceGeneration != request.SourceGeneration ||
		saved.TargetAccountID != request.TargetAccountID || saved.TargetProfileID != request.TargetProfileID ||
		saved.ProfileRevision != request.ProfileRevision || saved.ConsentRevision != request.ConsentRevision ||
		saved.PolicyRevision != request.PolicyRevision || !equalStrings(saved.Scopes, request.Scopes) ||
		!saved.ExpiresAt.Equal(request.ExpiresAt) || saved.Status != "pending" || !saved.ExpiresAt.After(s.now()) {
		return BindingChallenge{}, ErrInvalidBindingChallenge
	}
	if err := tx.Commit(ctx); err != nil {
		return BindingChallenge{}, fmt.Errorf("commit GIS binding challenge: %w", err)
	}
	return saved, nil
}

func validScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 16 {
		return false
	}
	seen := make(map[string]struct{}, len(scopes))
	for i, scope := range scopes {
		if scope == "" || (i > 0 && scopes[i-1] >= scope) {
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func validChallengeExpiry(now, expires time.Time) bool {
	return !expires.IsZero() && expires.After(now) && !expires.After(now.Add(5*time.Minute))
}

func bindingChallengeRequestHash(challenge BindingChallenge) string {
	// Legacy test/setup seam; authenticated creation hashes the exact signed body.
	value := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s", challenge.ApplicationID,
		challenge.EnvironmentID, challenge.Provider, challenge.RedirectURIHash, challenge.PKCEChallenge,
		challenge.DeviceKeyID, challenge.DeviceKeyThumbprint, challenge.OperationID, challenge.ExpiresAt.UTC().Format(time.RFC3339Nano), challenge.Nonce)
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// LoadBindingChallenge returns only a live, unconsumed challenge for Auth's
// execution-time consent check. Expired/consumed IDs are deliberately opaque.
func (s *Store) LoadBindingChallenge(ctx context.Context, id uuid.UUID) (BindingChallenge, error) {
	if id == uuid.Nil {
		return BindingChallenge{}, ErrBindingChallengeNotFound
	}
	if s == nil || s.Pool == nil {
		return BindingChallenge{}, ErrRegistryUnavailable
	}
	var challenge BindingChallenge
	err := s.Pool.QueryRow(ctx, `SELECT challenge_id,nonce,application_id,environment_id,provider,
		redirect_uri_sha256,pkce_challenge,device_key_id,device_key_thumbprint,operation_id,expires_at,status,
		COALESCE(source_account_id,'00000000-0000-0000-0000-000000000000'::uuid),
		COALESCE(source_actor_id,'00000000-0000-0000-0000-000000000000'::uuid),
		COALESCE(source_device_id,'00000000-0000-0000-0000-000000000000'::uuid),COALESCE(source_generation,0),
		COALESCE(target_account_id,'00000000-0000-0000-0000-000000000000'::uuid),
		COALESCE(target_profile_id,'00000000-0000-0000-0000-000000000000'::uuid),
		COALESCE(profile_revision,0),COALESCE(consent_revision,0),COALESCE(policy_revision,0),COALESCE(scopes,ARRAY[]::TEXT[])
		FROM player_binding_challenges WHERE challenge_id=$1 AND status='pending' AND expires_at>$2`,
		id, s.now()).Scan(&challenge.ChallengeID, &challenge.Nonce, &challenge.ApplicationID,
		&challenge.EnvironmentID, &challenge.Provider, &challenge.RedirectURIHash, &challenge.PKCEChallenge,
		&challenge.DeviceKeyID, &challenge.DeviceKeyThumbprint, &challenge.OperationID, &challenge.ExpiresAt,
		&challenge.Status, &challenge.SourceAccountID, &challenge.SourceActorID, &challenge.SourceDeviceID,
		&challenge.SourceGeneration, &challenge.TargetAccountID, &challenge.TargetProfileID,
		&challenge.ProfileRevision, &challenge.ConsentRevision, &challenge.PolicyRevision, &challenge.Scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return BindingChallenge{}, ErrBindingChallengeNotFound
	}
	if err != nil {
		return BindingChallenge{}, fmt.Errorf("read GIS binding challenge: %w", err)
	}
	if challenge.ChallengeID != id || challenge.Status != "pending" || !challenge.ExpiresAt.After(s.now()) ||
		!challengeNoncePattern.MatchString(challenge.Nonce) || !challengeProviderPattern.MatchString(challenge.Provider) ||
		!challengeDigestPattern.MatchString(challenge.RedirectURIHash) || !challengeCodePattern.MatchString(challenge.PKCEChallenge) ||
		!challengeCodePattern.MatchString(challenge.DeviceKeyThumbprint) || challenge.OperationID == uuid.Nil ||
		!((challenge.SourceAccountID == uuid.Nil && challenge.SourceActorID == uuid.Nil && challenge.SourceDeviceID == uuid.Nil &&
			challenge.SourceGeneration == 0 && challenge.TargetAccountID == uuid.Nil && challenge.TargetProfileID == uuid.Nil &&
			challenge.ProfileRevision == 0 && challenge.ConsentRevision == 0 && challenge.PolicyRevision == 0 && len(challenge.Scopes) == 0) ||
			(challenge.SourceAccountID != uuid.Nil && challenge.SourceActorID != uuid.Nil && challenge.SourceDeviceID == challenge.DeviceKeyID &&
				challenge.SourceGeneration > 0 && challenge.TargetAccountID != uuid.Nil && challenge.TargetProfileID != uuid.Nil &&
				challenge.ProfileRevision > 0 && challenge.ConsentRevision > 0 && challenge.PolicyRevision > 0 && validScopes(challenge.Scopes))) {
		return BindingChallenge{}, ErrRegistryUnavailable
	}
	return challenge, nil
}
