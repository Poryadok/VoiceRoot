package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/space/internal/entrypolicy"
)

var (
	ErrAdmissionRequired      = errors.New("entry verification required")
	ErrEntryPolicyUnavailable = errors.New("entry policy unavailable")
	ErrEntryPolicyConflict    = errors.New("entry policy operation conflict")
	ErrEntryPolicyVersion     = errors.New("entry policy revision mismatch")
	ErrLegacyEntryPolicyWrite = errors.New("legacy entry policy write rejected")
)

type EntryPolicyRow struct {
	Version int64
	Policy  entrypolicy.Policy
}
type SetEntryPolicyInput struct {
	SpaceID, ActorProfileID, OperationID uuid.UUID
	ExpectedVersion                      int64
	Policy                               entrypolicy.Policy
}

func entryPolicyTable(ctx context.Context, db spaceStoreDB) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT to_regclass('space_entry_policies') IS NOT NULL`).Scan(&exists)
	return exists, err
}

func (s *SpaceStore) GetEntryPolicy(ctx context.Context, spaceID uuid.UUID) (*EntryPolicyRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrEntryPolicyUnavailable
	}
	if s.tx == nil {
		return withOwnershipScopeValue(s, ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) (*EntryPolicyRow, error) { return scoped.GetEntryPolicy(ctx, spaceID) })
	}
	exists, err := entryPolicyTable(ctx, s.db())
	if err != nil {
		return nil, err
	}
	if exists {
		var version int64
		var raw []byte
		err := s.db().QueryRow(ctx, `SELECT version,policy FROM space_entry_policies WHERE space_id=$1`, spaceID).Scan(&version, &raw)
		if err == nil {
			policy, parseErr := entrypolicy.Parse(raw)
			if parseErr != nil {
				return nil, ErrEntryPolicyUnavailable
			}
			return &EntryPolicyRow{version, policy}, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	// Before migration and for newly created Spaces, derive the restrictive policy
	// under the same database fence. Missing migration never skips verification.
	var requirement string
	var questions *string
	if err := s.db().QueryRow(ctx, `SELECT entry_requirement,entry_questions::text FROM spaces WHERE id=$1`, spaceID).Scan(&requirement, &questions); err != nil {
		return nil, err
	}
	var raw []byte
	if questions != nil {
		raw = []byte(*questions)
	}
	policy, err := entrypolicy.FromLegacy(requirement, raw)
	if err != nil {
		return nil, ErrEntryPolicyUnavailable
	}
	if exists {
		encoded, err := json.Marshal(policy)
		if err != nil {
			return nil, err
		}
		if _, err := s.db().Exec(ctx, `INSERT INTO space_entry_policies(space_id,policy) VALUES($1,$2::jsonb)`, spaceID, encoded); err != nil {
			return nil, err
		}
	}
	return &EntryPolicyRow{1, policy}, nil
}

func (s *SpaceStore) SetEntryPolicy(ctx context.Context, in SetEntryPolicyInput) (*EntryPolicyRow, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrEntryPolicyUnavailable
	}
	if in.SpaceID == uuid.Nil || in.ActorProfileID == uuid.Nil || in.OperationID == uuid.Nil || in.ExpectedVersion < 1 {
		return nil, entrypolicy.ErrInvalid
	}
	raw, err := json.Marshal(in.Policy)
	if err != nil {
		return nil, entrypolicy.ErrInvalid
	}
	policy, err := entrypolicy.Parse(raw)
	if err != nil {
		return nil, err
	}
	raw, err = json.Marshal(policy)
	if err != nil {
		return nil, entrypolicy.ErrInvalid
	}
	if s.tx == nil {
		return withOwnershipScopeValue(s, ctx, []uuid.UUID{in.SpaceID}, func(scoped *SpaceStore) (*EntryPolicyRow, error) { return scoped.SetEntryPolicy(ctx, in) })
	}
	exists, err := entryPolicyTable(ctx, s.db())
	if err != nil || !exists {
		return nil, ErrEntryPolicyUnavailable
	}
	// Actor-scoped operation keys may target different Spaces. Serialize the key
	// in addition to the ordinary Space fence, before reading a durable outcome.
	key := sha256.Sum256(append(append([]byte("entry-policy-operation:"), in.ActorProfileID[:]...), in.OperationID[:]...))
	if _, err := s.db().Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(binary.BigEndian.Uint64(key[:8]))); err != nil {
		return nil, err
	}
	request, _ := json.Marshal(struct {
		SpaceID         uuid.UUID
		ExpectedVersion int64
		Policy          entrypolicy.Policy
	}{in.SpaceID, in.ExpectedVersion, policy})
	digest := sha256.Sum256(request)
	var storedDigest, storedPolicy []byte
	var storedVersion int64
	err = s.db().QueryRow(ctx, `SELECT request_digest,outcome_version,outcome_policy FROM space_entry_policy_operations WHERE actor_profile_id=$1 AND operation_id=$2`, in.ActorProfileID, in.OperationID).Scan(&storedDigest, &storedVersion, &storedPolicy)
	if err == nil {
		if !bytes.Equal(digest[:], storedDigest) {
			return nil, ErrEntryPolicyConflict
		}
		outcome, err := entrypolicy.Parse(storedPolicy)
		if err != nil {
			return nil, ErrEntryPolicyUnavailable
		}
		return &EntryPolicyRow{storedVersion, outcome}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	current, err := s.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		// Invalid legacy settings can be repaired by a valid CAS, never admitted.
		if !errors.Is(err, ErrEntryPolicyUnavailable) {
			return nil, err
		}
		var version int64
		if err := s.db().QueryRow(ctx, `SELECT version FROM space_entry_policies WHERE space_id=$1`, in.SpaceID).Scan(&version); err != nil {
			return nil, err
		}
		current = &EntryPolicyRow{Version: version}
	}
	if current.Version != in.ExpectedVersion {
		return nil, ErrEntryPolicyVersion
	}
	prior, _ := json.Marshal(current.Policy)
	version := current.Version
	if !bytes.Equal(prior, raw) {
		if version == math.MaxInt64 {
			return nil, ErrEntryPolicyUnavailable
		}
		version++
		if _, err := s.db().Exec(ctx, `UPDATE space_entry_policies SET version=$2,policy=$3::jsonb,updated_at=clock_timestamp() WHERE space_id=$1`, in.SpaceID, version, raw); err != nil {
			return nil, err
		}
	}
	if _, err := s.db().Exec(ctx, `INSERT INTO space_entry_policy_operations(actor_profile_id,operation_id,space_id,request_digest,outcome_version,outcome_policy) VALUES($1,$2,$3,$4,$5,$6::jsonb)`, in.ActorProfileID, in.OperationID, in.SpaceID, digest[:], version, raw); err != nil {
		return nil, err
	}
	return &EntryPolicyRow{version, policy}, nil
}

func (s *SpaceStore) requireNonInteractiveAdmission(ctx context.Context, spaceID uuid.UUID) error {
	row, err := s.GetEntryPolicy(ctx, spaceID)
	if err != nil {
		return err
	}
	p := row.Policy
	if p.RequirePhone || p.RequireCaptcha || p.RequireManualApproval || len(p.Questions) > 0 {
		return ErrAdmissionRequired
	}
	return nil
}
