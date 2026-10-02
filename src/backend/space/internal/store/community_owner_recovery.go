package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrCommunityOwnerRecoveryConflict = errors.New("community owner recovery conflicts")

type CommunityOwnerRecoveryInput struct {
	OperationID            uuid.UUID
	ApplicationID          uuid.UUID
	EnvironmentID          uuid.UUID
	CorporationKey         string
	SpaceID                uuid.UUID
	PreviousOwnerAccountID uuid.UUID
	PreviousOwnerProfileID uuid.UUID
	ReplacementAccountID   uuid.UUID
	ReplacementProfileID   uuid.UUID
	ExpectedGeneration     int64
	ReasonCode             string
	EvidenceSHA256         []byte
}

type CommunityOwnerRecoveryOperation struct {
	Input            CommunityOwnerRecoveryInput
	Status           string
	RoleReceipt      []byte
	ResultGeneration int64
}

func communityOwnerRecoveryHash(in CommunityOwnerRecoveryInput) ([32]byte, error) {
	canonical, err := json.Marshal(struct {
		OperationID            uuid.UUID `json:"operation_id"`
		ApplicationID          uuid.UUID `json:"application_id"`
		EnvironmentID          uuid.UUID `json:"environment_id"`
		CorporationKey         string    `json:"corporation_key"`
		SpaceID                uuid.UUID `json:"space_id"`
		PreviousOwnerAccountID uuid.UUID `json:"previous_owner_account_id"`
		PreviousOwnerProfileID uuid.UUID `json:"previous_owner_profile_id"`
		ReplacementAccountID   uuid.UUID `json:"replacement_account_id"`
		ReplacementProfileID   uuid.UUID `json:"replacement_profile_id"`
		ExpectedGeneration     int64     `json:"expected_generation"`
		ReasonCode             string    `json:"reason_code"`
		EvidenceSHA256         []byte    `json:"evidence_sha256"`
	}{in.OperationID, in.ApplicationID, in.EnvironmentID, in.CorporationKey, in.SpaceID, in.PreviousOwnerAccountID, in.PreviousOwnerProfileID, in.ReplacementAccountID, in.ReplacementProfileID, in.ExpectedGeneration, in.ReasonCode, in.EvidenceSHA256})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(canonical), nil
}

func validateCommunityOwnerRecovery(in CommunityOwnerRecoveryInput) error {
	if in.OperationID == uuid.Nil || in.ApplicationID == uuid.Nil || in.EnvironmentID == uuid.Nil || in.SpaceID == uuid.Nil || in.PreviousOwnerAccountID == uuid.Nil || in.PreviousOwnerProfileID == uuid.Nil || in.ReplacementAccountID == uuid.Nil || in.ReplacementProfileID == uuid.Nil || in.PreviousOwnerAccountID == in.ReplacementAccountID || in.PreviousOwnerProfileID == in.ReplacementProfileID || in.ExpectedGeneration <= 0 || in.CorporationKey == "" || len(in.CorporationKey) > 256 || len(in.EvidenceSHA256) != sha256.Size || (in.ReasonCode != "owner_lost" && in.ReasonCode != "corporation_dissolved") {
		return ErrCommunityOwnerRecoveryConflict
	}
	return nil
}

// ReserveCommunityOwnerRecovery freezes the Space authority at the exact
// generation before Role V2 calls. Exact retries resume the durable operation.
func (s *SpaceStore) ReserveCommunityOwnerRecovery(ctx context.Context, in CommunityOwnerRecoveryInput) (CommunityOwnerRecoveryOperation, bool, error) {
	if s == nil || s.Pool == nil {
		return CommunityOwnerRecoveryOperation{}, false, errors.New("space store: pool not configured")
	}
	if err := validateCommunityOwnerRecovery(in); err != nil {
		return CommunityOwnerRecoveryOperation{}, false, err
	}
	hash, err := communityOwnerRecoveryHash(in)
	if err != nil {
		return CommunityOwnerRecoveryOperation{}, false, err
	}
	var existing CommunityOwnerRecoveryOperation
	var savedHash []byte
	var existingGeneration sql.NullInt64
	err = s.Pool.QueryRow(ctx, `SELECT operation_id,application_id,environment_id,corporation_key,space_id,previous_owner_account_id,previous_owner_profile_id,replacement_account_id,replacement_profile_id,expected_generation,reason_code,evidence_sha256,request_hash,status,role_receipt,result_generation FROM community_owner_recovery_operations WHERE operation_id=$1`, in.OperationID).Scan(
		&existing.Input.OperationID, &existing.Input.ApplicationID, &existing.Input.EnvironmentID, &existing.Input.CorporationKey, &existing.Input.SpaceID, &existing.Input.PreviousOwnerAccountID, &existing.Input.PreviousOwnerProfileID, &existing.Input.ReplacementAccountID, &existing.Input.ReplacementProfileID, &existing.Input.ExpectedGeneration, &existing.Input.ReasonCode, &existing.Input.EvidenceSHA256, &savedHash, &existing.Status, &existing.RoleReceipt, &existingGeneration)
	if existingGeneration.Valid {
		existing.ResultGeneration = existingGeneration.Int64
	}
	if err == nil {
		if !equalBytes(savedHash, hash[:]) {
			return CommunityOwnerRecoveryOperation{}, false, ErrCommunityOwnerRecoveryConflict
		}
		if existing.Status == "succeeded" {
			return existing, false, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return CommunityOwnerRecoveryOperation{}, false, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return CommunityOwnerRecoveryOperation{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var accountID, profileID uuid.UUID
	var generation int64
	var status string
	err = tx.QueryRow(ctx, `SELECT owner_account_id,owner_profile_id,owner_generation,status FROM community_owner_authority WHERE space_id=$1 AND application_id=$2 AND environment_id=$3 AND corporation_key=$4 FOR UPDATE`, in.SpaceID, in.ApplicationID, in.EnvironmentID, in.CorporationKey).Scan(&accountID, &profileID, &generation, &status)
	if err != nil {
		return CommunityOwnerRecoveryOperation{}, false, err
	}
	if status == "recovery_pending" {
		// A concurrent exact retry may have reserved this operation while this
		// transaction waited for the authority row lock.
		row, e := loadCommunityOwnerRecovery(ctx, tx, in.OperationID, true)
		if e == nil && equalBytes(rowHash(row.Input), hash[:]) {
			if e = tx.Commit(ctx); e != nil {
				return CommunityOwnerRecoveryOperation{}, false, e
			}
			return row, false, nil
		}
		return CommunityOwnerRecoveryOperation{}, false, ErrCommunityOwnerRecoveryConflict
	}
	if status != "active" || generation != in.ExpectedGeneration || accountID != in.PreviousOwnerAccountID || profileID != in.PreviousOwnerProfileID {
		return CommunityOwnerRecoveryOperation{}, false, ErrCommunityOwnerRecoveryConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO community_owner_recovery_operations(operation_id,space_id,application_id,environment_id,corporation_key,previous_owner_account_id,previous_owner_profile_id,replacement_account_id,replacement_profile_id,expected_generation,reason_code,evidence_sha256,request_hash,status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'pending') ON CONFLICT(operation_id) DO NOTHING`, in.OperationID, in.SpaceID, in.ApplicationID, in.EnvironmentID, in.CorporationKey, in.PreviousOwnerAccountID, in.PreviousOwnerProfileID, in.ReplacementAccountID, in.ReplacementProfileID, in.ExpectedGeneration, in.ReasonCode, in.EvidenceSHA256, hash[:])
	if err != nil {
		if isPGUniqueViolation(err) {
			return CommunityOwnerRecoveryOperation{}, false, ErrCommunityOwnerRecoveryConflict
		}
		return CommunityOwnerRecoveryOperation{}, false, err
	}
	row, err := loadCommunityOwnerRecovery(ctx, tx, in.OperationID, true)
	if err != nil {
		return CommunityOwnerRecoveryOperation{}, false, err
	}
	if !equalBytes(rowHash(row.Input), hash[:]) {
		return CommunityOwnerRecoveryOperation{}, false, ErrCommunityOwnerRecoveryConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE community_owner_authority SET status='recovery_pending',updated_at=clock_timestamp() WHERE space_id=$1 AND owner_generation=$2 AND status='active'`, in.SpaceID, in.ExpectedGeneration); err != nil {
		return CommunityOwnerRecoveryOperation{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CommunityOwnerRecoveryOperation{}, false, err
	}
	return row, true, nil
}

func (s *SpaceStore) CompleteCommunityOwnerRecovery(ctx context.Context, in CommunityOwnerRecoveryInput, roleReceipt []byte) (CommunityOwnerRecoveryOperation, error) {
	if s == nil || s.Pool == nil {
		return CommunityOwnerRecoveryOperation{}, errors.New("space store: pool not configured")
	}
	if err := validateCommunityOwnerRecovery(in); err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	if len(roleReceipt) == 0 {
		return CommunityOwnerRecoveryOperation{}, ErrCommunityOwnerRecoveryConflict
	}
	hash, err := communityOwnerRecoveryHash(in)
	if err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var accountID, profileID uuid.UUID
	var generation int64
	var status string
	err = tx.QueryRow(ctx, `SELECT owner_account_id,owner_profile_id,owner_generation,status FROM community_owner_authority WHERE space_id=$1 AND application_id=$2 AND environment_id=$3 AND corporation_key=$4 FOR UPDATE`, in.SpaceID, in.ApplicationID, in.EnvironmentID, in.CorporationKey).Scan(&accountID, &profileID, &generation, &status)
	if err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	op, err := loadCommunityOwnerRecovery(ctx, tx, in.OperationID, true)
	if err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	if !equalBytes(rowHash(op.Input), hash[:]) {
		return CommunityOwnerRecoveryOperation{}, ErrCommunityOwnerRecoveryConflict
	}
	if op.Status == "succeeded" {
		if !equalBytes(op.RoleReceipt, roleReceipt) {
			return CommunityOwnerRecoveryOperation{}, ErrCommunityOwnerRecoveryConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return CommunityOwnerRecoveryOperation{}, err
		}
		return op, nil
	}
	if op.Status != "pending" || status != "recovery_pending" || generation != in.ExpectedGeneration || accountID != in.PreviousOwnerAccountID || profileID != in.PreviousOwnerProfileID {
		return CommunityOwnerRecoveryOperation{}, ErrCommunityOwnerRecoveryConflict
	}
	member, err := tx.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, in.SpaceID, in.ReplacementProfileID)
	if err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	if member.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx, `UPDATE spaces SET member_count=member_count+1 WHERE id=$1`, in.SpaceID); err != nil {
			return CommunityOwnerRecoveryOperation{}, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE spaces SET owner_profile_id=$2,updated_at=clock_timestamp() WHERE id=$1 AND owner_profile_id=$3`, in.SpaceID, in.ReplacementProfileID, in.PreviousOwnerProfileID); err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE community_roster_members SET revoked_at=clock_timestamp() WHERE space_id=$1 AND owner_generation=$2 AND revoked_at IS NULL`, in.SpaceID, in.ExpectedGeneration); err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE community_owner_authority SET owner_account_id=$2,owner_profile_id=$3,owner_generation=$4,status='active',roster_source_revision=0,roster_sha256=NULL,roster_lease_expires_at=NULL,updated_at=clock_timestamp() WHERE space_id=$1 AND owner_generation=$5 AND status='recovery_pending'`, in.SpaceID, in.ReplacementAccountID, in.ReplacementProfileID, in.ExpectedGeneration+1, in.ExpectedGeneration); err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE community_owner_recovery_operations SET status='succeeded',role_receipt=$2,result_generation=$3,updated_at=clock_timestamp() WHERE operation_id=$1 AND status='pending'`, in.OperationID, roleReceipt, in.ExpectedGeneration+1); err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	op, err = loadCommunityOwnerRecovery(ctx, tx, in.OperationID, false)
	if err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CommunityOwnerRecoveryOperation{}, err
	}
	return op, nil
}

func loadCommunityOwnerRecovery(ctx context.Context, tx pgx.Tx, id uuid.UUID, forUpdate bool) (CommunityOwnerRecoveryOperation, error) {
	q := `SELECT operation_id,application_id,environment_id,corporation_key,space_id,previous_owner_account_id,previous_owner_profile_id,replacement_account_id,replacement_profile_id,expected_generation,reason_code,evidence_sha256,status,role_receipt,result_generation FROM community_owner_recovery_operations WHERE operation_id=$1`
	if forUpdate {
		q += ` FOR UPDATE`
	}
	var op CommunityOwnerRecoveryOperation
	var resultGeneration sql.NullInt64
	err := tx.QueryRow(ctx, q, id).Scan(&op.Input.OperationID, &op.Input.ApplicationID, &op.Input.EnvironmentID, &op.Input.CorporationKey, &op.Input.SpaceID, &op.Input.PreviousOwnerAccountID, &op.Input.PreviousOwnerProfileID, &op.Input.ReplacementAccountID, &op.Input.ReplacementProfileID, &op.Input.ExpectedGeneration, &op.Input.ReasonCode, &op.Input.EvidenceSHA256, &op.Status, &op.RoleReceipt, &resultGeneration)
	if resultGeneration.Valid {
		op.ResultGeneration = resultGeneration.Int64
	}
	return op, err
}
func rowHash(in CommunityOwnerRecoveryInput) []byte {
	hash, _ := communityOwnerRecoveryHash(in)
	return hash[:]
}
func equalBytes(a, b []byte) bool {
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
