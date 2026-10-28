package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const communityBootstrapApprovalTTL = 15 * time.Minute

type CommunityBootstrapOperation struct {
	OperationID     uuid.UUID
	ApplicationID   uuid.UUID
	EnvironmentID   uuid.UUID
	OwnerAccountID  uuid.UUID
	OwnerProfileID  uuid.UUID
	OwnerGeneration int64
	CorporationKey  string
	TemplateID      string
	Status          string
	OperatorID      *uuid.UUID
	ApprovedAt      *time.Time
	ExpiresAt       *time.Time
	SpaceID         *uuid.UUID
	CreatedAt       time.Time
}

type OwnerAuthority struct {
	OwnerAccountID uuid.UUID
	OwnerProfileID uuid.UUID
	Generation     int64
	Status         string
	SpaceID        pgtype.UUID
}

type OwnerRecoveryOperation struct {
	// TargetSpaceID is resolved from current locked authority for dispatch;
	// SpaceID remains the nullable durable operation result.
	TargetSpaceID          uuid.UUID
	OperationID            uuid.UUID
	ApplicationID          uuid.UUID
	EnvironmentID          uuid.UUID
	CorporationKey         string
	PreviousOwnerAccountID uuid.UUID
	PreviousOwnerProfileID uuid.UUID
	ReplacementAccountID   uuid.UUID
	ReplacementProfileID   uuid.UUID
	ExpectedGeneration     int64
	ReasonCode             string
	EvidenceSHA256         []byte
	Status                 string
	OperatorID             *uuid.UUID
	ApprovedAt             *time.Time
	ExpiresAt              *time.Time
	CancelledBy            *uuid.UUID
	CancelledAt            *time.Time
	SpaceID                *uuid.UUID
	ResultGeneration       *int64
	CreatedAt              time.Time
}

type OpenOwnerRecoveryInput struct {
	OperationID          uuid.UUID
	ApplicationID        uuid.UUID
	EnvironmentID        uuid.UUID
	CorporationKey       string
	ReplacementAccountID uuid.UUID
	ReplacementProfileID uuid.UUID
	ExpectedGeneration   int64
	ReasonCode           string
	EvidenceSHA256       []byte
}

func normalizeOpenOwnerRecovery(in OpenOwnerRecoveryInput) (OpenOwnerRecoveryInput, [32]byte, error) {
	if in.OperationID == uuid.Nil || in.ApplicationID == uuid.Nil || in.EnvironmentID == uuid.Nil ||
		in.CorporationKey == "" || len(in.CorporationKey) > 256 || in.ReplacementAccountID == uuid.Nil ||
		in.ReplacementProfileID == uuid.Nil || in.ExpectedGeneration <= 0 ||
		(in.ReasonCode != "owner_lost" && in.ReasonCode != "corporation_dissolved") || len(in.EvidenceSHA256) != sha256.Size {
		return OpenOwnerRecoveryInput{}, [32]byte{}, ErrInvalidApplication
	}
	canonical, err := json.Marshal(struct {
		ApplicationID        uuid.UUID `json:"application_id"`
		EnvironmentID        uuid.UUID `json:"environment_id"`
		CorporationKey       string    `json:"corporation_key"`
		ReplacementAccountID uuid.UUID `json:"replacement_account_id"`
		ReplacementProfileID uuid.UUID `json:"replacement_profile_id"`
		ExpectedGeneration   int64     `json:"expected_generation"`
		ReasonCode           string    `json:"reason_code"`
		EvidenceSHA256       []byte    `json:"evidence_sha256"`
	}{in.ApplicationID, in.EnvironmentID, in.CorporationKey, in.ReplacementAccountID, in.ReplacementProfileID, in.ExpectedGeneration, in.ReasonCode, in.EvidenceSHA256})
	if err != nil {
		return OpenOwnerRecoveryInput{}, [32]byte{}, err
	}
	return in, sha256.Sum256(canonical), nil
}

// OpenCommunityOwnerRecovery freezes GIS-authorized community writes while a
// separate allowlisted operator reviews a replacement human Owner.
func (s *Store) OpenCommunityOwnerRecovery(ctx context.Context, in OpenOwnerRecoveryInput) (OwnerRecoveryOperation, error) {
	in, requestHash, err := normalizeOpenOwnerRecovery(in)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	if s == nil || s.Pool == nil {
		return OwnerRecoveryOperation{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("begin owner recovery: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var envApp uuid.UUID
	var envKind, envStatus string
	if err := tx.QueryRow(ctx, `SELECT application_id,kind,status FROM environments WHERE id=$1 FOR SHARE`, in.EnvironmentID).Scan(&envApp, &envKind, &envStatus); err != nil || envApp != in.ApplicationID || envKind != "sandbox" || envStatus != "active" {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return OwnerRecoveryOperation{}, fmt.Errorf("read recovery environment: %w", err)
		}
		return OwnerRecoveryOperation{}, ErrInvalidApplication
	}
	authority, err := loadOwnerAuthorityForUpdate(ctx, tx, in.ApplicationID, in.EnvironmentID, in.CorporationKey)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	if authority.Generation != in.ExpectedGeneration || authority.OwnerAccountID == in.ReplacementAccountID {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	if authority.Status == "recovery_pending" {
		op, loadErr := loadOwnerRecoveryForUpdate(ctx, tx, in.OperationID)
		if loadErr != nil {
			return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
		}
		savedHash, hashErr := ownerRecoveryRequestHash(ctx, tx, in.OperationID)
		if hashErr != nil {
			return OwnerRecoveryOperation{}, hashErr
		}
		if !bytesEqual(savedHash, requestHash[:]) || op.ReplacementAccountID != in.ReplacementAccountID || op.ReplacementProfileID != in.ReplacementProfileID || op.PreviousOwnerAccountID != authority.OwnerAccountID || op.PreviousOwnerProfileID != authority.OwnerProfileID {
			return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
		}
		if op.Status == "approved" {
			if !authority.SpaceID.Valid || uuid.UUID(authority.SpaceID.Bytes) == uuid.Nil {
				return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
			}
			op.TargetSpaceID = uuid.UUID(authority.SpaceID.Bytes)
		}
		if err := tx.Commit(ctx); err != nil {
			return OwnerRecoveryOperation{}, fmt.Errorf("commit owner recovery replay: %w", err)
		}
		return op, nil
	}
	if authority.Status != "active" {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_community_owner_recovery_operations
		(operation_id,application_id,environment_id,corporation_key,previous_owner_account_id,previous_owner_profile_id,replacement_account_id,replacement_profile_id,expected_generation,reason_code,evidence_sha256,request_hash,status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'pending') ON CONFLICT(operation_id) DO NOTHING`,
		in.OperationID, in.ApplicationID, in.EnvironmentID, in.CorporationKey, authority.OwnerAccountID, authority.OwnerProfileID, in.ReplacementAccountID, in.ReplacementProfileID, in.ExpectedGeneration, in.ReasonCode, in.EvidenceSHA256, requestHash[:])
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
		}
		return OwnerRecoveryOperation{}, fmt.Errorf("insert owner recovery: %w", err)
	}
	op, err := loadOwnerRecoveryForUpdate(ctx, tx, in.OperationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	savedHash, hashErr := ownerRecoveryRequestHash(ctx, tx, in.OperationID)
	if hashErr != nil {
		return OwnerRecoveryOperation{}, hashErr
	}
	if !bytesEqual(savedHash, requestHash[:]) {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	if op.PreviousOwnerAccountID != authority.OwnerAccountID || op.PreviousOwnerProfileID != authority.OwnerProfileID || op.ExpectedGeneration != authority.Generation || op.ReplacementAccountID != in.ReplacementAccountID || op.ReplacementProfileID != in.ReplacementProfileID {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	if op.Status == "cancelled" && authority.Status == "active" {
		if err := tx.Commit(ctx); err != nil {
			return OwnerRecoveryOperation{}, fmt.Errorf("commit cancelled recovery replay: %w", err)
		}
		return op, nil
	}
	if op.Status == "pending" && authority.Status == "active" {
		if _, err := tx.Exec(ctx, `UPDATE gis_community_owner_authority SET status='recovery_pending',updated_at=clock_timestamp() WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND owner_generation=$4 AND status='active'`, in.ApplicationID, in.EnvironmentID, in.CorporationKey, in.ExpectedGeneration); err != nil {
			return OwnerRecoveryOperation{}, fmt.Errorf("freeze community owner writes: %w", err)
		}
	} else if authority.Status != "recovery_pending" {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	op, err = loadOwnerRecovery(ctx, tx, in.OperationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("commit owner recovery: %w", err)
	}
	return op, nil
}

// CancelCommunityOwnerRecovery restores the exact prior Owner generation when
// an allowlisted operator rejects an unfinished request. Reopening requires a
// new operation ID; the cancelled receipt remains durable for replay/audit.
func (s *Store) CancelCommunityOwnerRecovery(ctx context.Context, applicationID, operationID, operatorID uuid.UUID) (OwnerRecoveryOperation, error) {
	if s == nil || s.Pool == nil {
		return OwnerRecoveryOperation{}, ErrRegistryUnavailable
	}
	if applicationID == uuid.Nil || operationID == uuid.Nil || operatorID == uuid.Nil {
		return OwnerRecoveryOperation{}, ErrInvalidApplication
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("begin owner recovery cancellation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	peek, err := loadOwnerRecovery(ctx, tx, operationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	authority, err := loadOwnerAuthorityForUpdate(ctx, tx, peek.ApplicationID, peek.EnvironmentID, peek.CorporationKey)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	op, err := loadOwnerRecoveryForUpdate(ctx, tx, operationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	if op.ApplicationID != applicationID || operatorID == op.PreviousOwnerAccountID || operatorID == op.ReplacementAccountID {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	if op.Status == "cancelled" {
		if op.CancelledBy == nil || *op.CancelledBy != operatorID {
			return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return OwnerRecoveryOperation{}, err
		}
		return op, nil
	}
	if op.Status != "pending" || authority.Status != "recovery_pending" || authority.Generation != op.ExpectedGeneration || authority.OwnerAccountID != op.PreviousOwnerAccountID || authority.OwnerProfileID != op.PreviousOwnerProfileID {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	now := s.communityBootstrapNow()
	if _, err := tx.Exec(ctx, `UPDATE gis_community_owner_recovery_operations SET status='cancelled',cancelled_by_account_id=$2,cancelled_at=$3,updated_at=$3 WHERE operation_id=$1 AND status IN ('pending','approved')`, operationID, operatorID, now); err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("cancel owner recovery: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE gis_community_owner_authority SET status='active',updated_at=$4 WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND owner_generation=$5 AND owner_account_id=$6 AND owner_profile_id=$7 AND status='recovery_pending'`, op.ApplicationID, op.EnvironmentID, op.CorporationKey, now, op.ExpectedGeneration, op.PreviousOwnerAccountID, op.PreviousOwnerProfileID); err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("reopen prior community owner: %w", err)
	}
	op, err = loadOwnerRecovery(ctx, tx, operationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("commit owner recovery cancellation: %w", err)
	}
	return op, nil
}

func (s *Store) ApproveCommunityOwnerRecovery(ctx context.Context, applicationID, operationID, operatorID uuid.UUID) (OwnerRecoveryOperation, error) {
	if s == nil || s.Pool == nil {
		return OwnerRecoveryOperation{}, ErrRegistryUnavailable
	}
	if applicationID == uuid.Nil || operationID == uuid.Nil || operatorID == uuid.Nil {
		return OwnerRecoveryOperation{}, ErrInvalidApplication
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("begin owner recovery approval: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	peek, err := loadOwnerRecovery(ctx, tx, operationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	authority, err := loadOwnerAuthorityForUpdate(ctx, tx, peek.ApplicationID, peek.EnvironmentID, peek.CorporationKey)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	op, err := loadOwnerRecoveryForUpdate(ctx, tx, operationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	if op.ApplicationID != applicationID || authority.Status != "recovery_pending" || authority.Generation != op.ExpectedGeneration || authority.OwnerAccountID != op.PreviousOwnerAccountID || authority.OwnerProfileID != op.PreviousOwnerProfileID || operatorID == op.PreviousOwnerAccountID || operatorID == op.ReplacementAccountID {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	now := s.communityBootstrapNow()
	if op.Status == "succeeded" || (op.Status == "approved" && op.ExpiresAt != nil && now.Before(*op.ExpiresAt)) {
		if op.OperatorID == nil || *op.OperatorID != operatorID {
			return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return OwnerRecoveryOperation{}, err
		}
		return op, nil
	}
	if op.Status != "pending" && op.Status != "approved" {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	expires := now.Add(communityBootstrapApprovalTTL)
	if _, err := tx.Exec(ctx, `UPDATE gis_community_owner_recovery_operations SET status='approved',operator_account_id=$2,approved_at=$3,approval_expires_at=$4,updated_at=$3 WHERE operation_id=$1 AND expected_generation=$5`, operationID, operatorID, now, expires, op.ExpectedGeneration); err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("save owner recovery approval: %w", err)
	}
	op, err = loadOwnerRecovery(ctx, tx, operationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("commit owner recovery approval: %w", err)
	}
	return op, nil
}

// CompleteCommunityOwnerRecovery advances the GIS authority only after the
// Space/Role recovery receipt matches the frozen generation and exact operation.
func (s *Store) CompleteCommunityOwnerRecovery(ctx context.Context, operationID, replacementAccountID, replacementProfileID, spaceID uuid.UUID, resultGeneration int64) (OwnerRecoveryOperation, error) {
	if s == nil || s.Pool == nil {
		return OwnerRecoveryOperation{}, ErrRegistryUnavailable
	}
	if operationID == uuid.Nil || replacementAccountID == uuid.Nil || replacementProfileID == uuid.Nil || spaceID == uuid.Nil || resultGeneration <= 0 {
		return OwnerRecoveryOperation{}, ErrInvalidApplication
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("begin owner recovery receipt: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	peek, err := loadOwnerRecovery(ctx, tx, operationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	authority, err := loadOwnerAuthorityForUpdate(ctx, tx, peek.ApplicationID, peek.EnvironmentID, peek.CorporationKey)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	op, err := loadOwnerRecoveryForUpdate(ctx, tx, operationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	if op.ReplacementAccountID != replacementAccountID || op.ReplacementProfileID != replacementProfileID {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	if op.Status == "succeeded" {
		if op.SpaceID == nil || *op.SpaceID != spaceID || op.ResultGeneration == nil || *op.ResultGeneration != resultGeneration {
			return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return OwnerRecoveryOperation{}, err
		}
		return op, nil
	}
	if authority.Status != "recovery_pending" || authority.Generation != op.ExpectedGeneration || authority.OwnerAccountID != op.PreviousOwnerAccountID || authority.OwnerProfileID != op.PreviousOwnerProfileID || !authority.SpaceID.Valid || uuid.UUID(authority.SpaceID.Bytes) != spaceID {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryConflict
	}
	if op.Status != "approved" || op.OperatorID == nil || *op.OperatorID == replacementAccountID || op.ExpiresAt == nil || !s.communityBootstrapNow().Before(*op.ExpiresAt) || resultGeneration != op.ExpectedGeneration+1 {
		return OwnerRecoveryOperation{}, ErrOwnerRecoveryExpired
	}
	if _, err := tx.Exec(ctx, `UPDATE gis_community_grants SET revoked_at=clock_timestamp(),revocation_reason='owner_generation_changed',updated_at=clock_timestamp()
		WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND owner_generation=$4 AND revoked_at IS NULL`,
		op.ApplicationID, op.EnvironmentID, op.CorporationKey, op.ExpectedGeneration); err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("revoke old-generation community grants: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE gis_community_owner_authority SET owner_account_id=$4,owner_profile_id=$5,owner_generation=$6,status='active',roster_source_revision=0,roster_sha256=NULL,roster_lease_expires_at=NULL,updated_at=clock_timestamp() WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND owner_generation=$7 AND status='recovery_pending'`, op.ApplicationID, op.EnvironmentID, op.CorporationKey, replacementAccountID, replacementProfileID, resultGeneration, op.ExpectedGeneration); err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("advance community owner authority: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE gis_community_owner_recovery_operations SET status='succeeded',space_id=$2,result_generation=$3,updated_at=clock_timestamp() WHERE operation_id=$1 AND expected_generation=$4 AND status='approved'`, operationID, spaceID, resultGeneration, op.ExpectedGeneration); err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("save owner recovery receipt: %w", err)
	}
	op, err = loadOwnerRecovery(ctx, tx, operationID)
	if err != nil {
		return OwnerRecoveryOperation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("commit owner recovery receipt: %w", err)
	}
	return op, nil
}

func ownerRecoveryRequestHash(ctx context.Context, tx pgx.Tx, id uuid.UUID) ([]byte, error) {
	var value []byte
	err := tx.QueryRow(ctx, `SELECT request_hash FROM gis_community_owner_recovery_operations WHERE operation_id=$1`, id).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidApplication
	}
	if err != nil {
		return nil, fmt.Errorf("read owner recovery request hash: %w", err)
	}
	return value, nil
}
func bytesEqual(a, b []byte) bool {
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

const ownerRecoverySelect = `SELECT operation_id,application_id,environment_id,corporation_key,previous_owner_account_id,previous_owner_profile_id,replacement_account_id,replacement_profile_id,expected_generation,reason_code,evidence_sha256,status,operator_account_id,approved_at,approval_expires_at,cancelled_by_account_id,cancelled_at,space_id,result_generation,created_at FROM gis_community_owner_recovery_operations`

func scanOwnerRecovery(row pgx.Row) (OwnerRecoveryOperation, error) {
	var op OwnerRecoveryOperation
	err := row.Scan(&op.OperationID, &op.ApplicationID, &op.EnvironmentID, &op.CorporationKey, &op.PreviousOwnerAccountID, &op.PreviousOwnerProfileID, &op.ReplacementAccountID, &op.ReplacementProfileID, &op.ExpectedGeneration, &op.ReasonCode, &op.EvidenceSHA256, &op.Status, &op.OperatorID, &op.ApprovedAt, &op.ExpiresAt, &op.CancelledBy, &op.CancelledAt, &op.SpaceID, &op.ResultGeneration, &op.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return OwnerRecoveryOperation{}, ErrInvalidApplication
	}
	if err != nil {
		return OwnerRecoveryOperation{}, fmt.Errorf("scan owner recovery: %w", err)
	}
	return op, nil
}
func loadOwnerRecovery(ctx context.Context, tx pgx.Tx, id uuid.UUID) (OwnerRecoveryOperation, error) {
	return scanOwnerRecovery(tx.QueryRow(ctx, ownerRecoverySelect+` WHERE operation_id=$1`, id))
}
func loadOwnerRecoveryForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (OwnerRecoveryOperation, error) {
	return scanOwnerRecovery(tx.QueryRow(ctx, ownerRecoverySelect+` WHERE operation_id=$1 FOR UPDATE`, id))
}

type OpenCommunityBootstrapInput struct {
	OperationID    uuid.UUID
	ApplicationID  uuid.UUID
	EnvironmentID  uuid.UUID
	OwnerAccountID uuid.UUID
	OwnerProfileID uuid.UUID
	CorporationKey string
	TemplateID     string
}

func normalizeOpenCommunityBootstrap(in OpenCommunityBootstrapInput) (OpenCommunityBootstrapInput, [32]byte, error) {
	if in.OperationID == uuid.Nil || in.ApplicationID == uuid.Nil || in.EnvironmentID == uuid.Nil ||
		in.OwnerAccountID == uuid.Nil || in.OwnerProfileID == uuid.Nil || in.CorporationKey == "" ||
		len(in.CorporationKey) > 256 || in.TemplateID == "" || len(in.TemplateID) > 128 {
		return OpenCommunityBootstrapInput{}, [32]byte{}, ErrInvalidApplication
	}
	canonical, err := json.Marshal(struct {
		ApplicationID  uuid.UUID `json:"application_id"`
		EnvironmentID  uuid.UUID `json:"environment_id"`
		OwnerAccountID uuid.UUID `json:"owner_account_id"`
		OwnerProfileID uuid.UUID `json:"owner_profile_id"`
		CorporationKey string    `json:"corporation_key"`
		TemplateID     string    `json:"template_id"`
	}{in.ApplicationID, in.EnvironmentID, in.OwnerAccountID, in.OwnerProfileID, in.CorporationKey, in.TemplateID})
	if err != nil {
		return OpenCommunityBootstrapInput{}, [32]byte{}, err
	}
	return in, sha256.Sum256(canonical), nil
}

// OpenCommunityBootstrap durably opens an Owner-authenticated, exact request.
// It does not contact Space or create any product resources.
func (s *Store) OpenCommunityBootstrap(ctx context.Context, input OpenCommunityBootstrapInput) (CommunityBootstrapOperation, error) {
	in, requestHash, err := normalizeOpenCommunityBootstrap(input)
	if err != nil {
		return CommunityBootstrapOperation{}, err
	}
	if s == nil || s.Pool == nil {
		return CommunityBootstrapOperation{}, ErrRegistryUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("begin community bootstrap: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var owner uuid.UUID
	var appStatus string
	if err := tx.QueryRow(ctx, `SELECT owner_account_id,status FROM applications WHERE id=$1 FOR SHARE`, in.ApplicationID).Scan(&owner, &appStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CommunityBootstrapOperation{}, ErrInvalidApplication
		}
		return CommunityBootstrapOperation{}, fmt.Errorf("read community application: %w", err)
	}
	if owner != in.OwnerAccountID || (appStatus != "sandbox" && appStatus != "active") {
		return CommunityBootstrapOperation{}, ErrInvalidApplication
	}
	var envApp uuid.UUID
	var envKind, envStatus string
	if err := tx.QueryRow(ctx, `SELECT application_id,kind,status FROM environments WHERE id=$1 FOR SHARE`, in.EnvironmentID).Scan(&envApp, &envKind, &envStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CommunityBootstrapOperation{}, ErrInvalidApplication
		}
		return CommunityBootstrapOperation{}, fmt.Errorf("read community environment: %w", err)
	}
	if envApp != in.ApplicationID || envKind != "sandbox" || envStatus != "active" {
		return CommunityBootstrapOperation{}, ErrInvalidApplication
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_community_owner_authority
		(application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status)
		VALUES($1,$2,$3,$4,$5,1,'pending') ON CONFLICT DO NOTHING`,
		in.ApplicationID, in.EnvironmentID, in.CorporationKey, in.OwnerAccountID, in.OwnerProfileID)
	if err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("open community owner authority: %w", err)
	}
	var authority OwnerAuthority
	if err := tx.QueryRow(ctx, `SELECT owner_account_id,owner_profile_id,owner_generation,status,space_id FROM gis_community_owner_authority WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 FOR UPDATE`, in.ApplicationID, in.EnvironmentID, in.CorporationKey).Scan(&authority.OwnerAccountID, &authority.OwnerProfileID, &authority.Generation, &authority.Status, &authority.SpaceID); err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("read community owner authority: %w", err)
	}
	if authority.OwnerAccountID != in.OwnerAccountID || authority.OwnerProfileID != in.OwnerProfileID || authority.Status == "recovery_pending" {
		return CommunityBootstrapOperation{}, ErrInvalidApplication
	}
	_, err = tx.Exec(ctx, `INSERT INTO gis_community_bootstrap_operations
		(operation_id,application_id,environment_id,owner_account_id,owner_profile_id,corporation_key,template_id,owner_generation,request_hash,status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending') ON CONFLICT(operation_id) DO NOTHING`,
		in.OperationID, in.ApplicationID, in.EnvironmentID, in.OwnerAccountID, in.OwnerProfileID, in.CorporationKey, in.TemplateID, authority.Generation, requestHash[:])
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return CommunityBootstrapOperation{}, ErrCommunityBootstrapConflict
		}
		return CommunityBootstrapOperation{}, fmt.Errorf("insert community bootstrap operation: %w", err)
	}
	var savedHash []byte
	if err := tx.QueryRow(ctx, `SELECT request_hash FROM gis_community_bootstrap_operations WHERE operation_id=$1 FOR UPDATE`, in.OperationID).Scan(&savedHash); err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("read community bootstrap operation: %w", err)
	}
	if string(savedHash) != string(requestHash[:]) {
		return CommunityBootstrapOperation{}, ErrCommunityBootstrapConflict
	}
	op, err := loadCommunityBootstrap(ctx, tx, in.OperationID)
	if err != nil {
		return CommunityBootstrapOperation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("commit community bootstrap operation: %w", err)
	}
	return op, nil
}

func (s *Store) ApproveCommunityBootstrap(ctx context.Context, applicationID, operationID, operatorID uuid.UUID) (CommunityBootstrapOperation, error) {
	if s == nil || s.Pool == nil {
		return CommunityBootstrapOperation{}, ErrRegistryUnavailable
	}
	if applicationID == uuid.Nil || operationID == uuid.Nil || operatorID == uuid.Nil {
		return CommunityBootstrapOperation{}, ErrInvalidApplication
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("begin community bootstrap approval: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	op, err := loadCommunityBootstrapForUpdate(ctx, tx, operationID)
	if err != nil {
		return CommunityBootstrapOperation{}, err
	}
	if op.ApplicationID != applicationID || op.OwnerAccountID == operatorID {
		return CommunityBootstrapOperation{}, ErrInvalidApplication
	}
	if op.Status != "succeeded" {
		if err := requireCommunityBootstrapParent(ctx, tx, op.ApplicationID, op.EnvironmentID); err != nil {
			return CommunityBootstrapOperation{}, err
		}
	}
	authority, err := loadOwnerAuthorityForUpdate(ctx, tx, op.ApplicationID, op.EnvironmentID, op.CorporationKey)
	if err != nil {
		return CommunityBootstrapOperation{}, err
	}
	if authority.OwnerAccountID != op.OwnerAccountID || authority.OwnerProfileID != op.OwnerProfileID ||
		authority.Generation != op.OwnerGeneration || authority.Status == "recovery_pending" {
		return CommunityBootstrapOperation{}, ErrCommunityBootstrapConflict
	}
	if op.Status == "succeeded" || (op.Status == "approved" && op.ExpiresAt != nil && s.communityBootstrapNow().Before(*op.ExpiresAt)) {
		if op.OperatorID == nil || *op.OperatorID != operatorID {
			return CommunityBootstrapOperation{}, ErrCommunityBootstrapConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return CommunityBootstrapOperation{}, fmt.Errorf("commit repeated community approval: %w", err)
		}
		return op, nil
	}
	now := s.communityBootstrapNow()
	expires := now.Add(communityBootstrapApprovalTTL)
	_, err = tx.Exec(ctx, `UPDATE gis_community_bootstrap_operations SET status='approved',operator_account_id=$2,approved_at=$3,approval_expires_at=$4,updated_at=$3 WHERE operation_id=$1`, operationID, operatorID, now, expires)
	if err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("save community bootstrap approval: %w", err)
	}
	op, err = loadCommunityBootstrap(ctx, tx, operationID)
	if err != nil {
		return CommunityBootstrapOperation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("commit community bootstrap approval: %w", err)
	}
	return op, nil
}

// CompleteCommunityBootstrap persists the Space-owned receipt after GIS calls
// the idempotent Space endpoint. A lost GIS response is recovered by replaying
// the same Space operation and then repeating this exact receipt write.
func (s *Store) CompleteCommunityBootstrap(ctx context.Context, operationID, ownerAccountID, spaceID uuid.UUID) (CommunityBootstrapOperation, error) {
	if s == nil || s.Pool == nil {
		return CommunityBootstrapOperation{}, ErrRegistryUnavailable
	}
	if operationID == uuid.Nil || ownerAccountID == uuid.Nil || spaceID == uuid.Nil {
		return CommunityBootstrapOperation{}, ErrInvalidApplication
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("begin community bootstrap receipt: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	op, err := loadCommunityBootstrapForUpdate(ctx, tx, operationID)
	if err != nil {
		return CommunityBootstrapOperation{}, err
	}
	if op.OwnerAccountID != ownerAccountID {
		return CommunityBootstrapOperation{}, ErrInvalidApplication
	}
	if op.Status != "succeeded" {
		if err := requireCommunityBootstrapParent(ctx, tx, op.ApplicationID, op.EnvironmentID); err != nil {
			return CommunityBootstrapOperation{}, err
		}
	}
	authority, err := loadOwnerAuthorityForUpdate(ctx, tx, op.ApplicationID, op.EnvironmentID, op.CorporationKey)
	if err != nil {
		return CommunityBootstrapOperation{}, err
	}
	if authority.OwnerAccountID != op.OwnerAccountID || authority.OwnerProfileID != op.OwnerProfileID || authority.Generation != op.OwnerGeneration {
		return CommunityBootstrapOperation{}, ErrCommunityBootstrapConflict
	}
	if op.Status == "succeeded" {
		if op.SpaceID == nil || *op.SpaceID != spaceID || authority.Status != "active" || !authority.SpaceID.Valid || uuid.UUID(authority.SpaceID.Bytes) != spaceID {
			return CommunityBootstrapOperation{}, ErrCommunityBootstrapConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return CommunityBootstrapOperation{}, err
		}
		return op, nil
	}
	if op.Status != "approved" || op.ExpiresAt == nil || !s.communityBootstrapNow().Before(*op.ExpiresAt) {
		return CommunityBootstrapOperation{}, ErrCommunityBootstrapExpired
	}
	if authority.Status != "pending" {
		return CommunityBootstrapOperation{}, ErrCommunityBootstrapConflict
	}
	_, err = tx.Exec(ctx, `UPDATE gis_community_bootstrap_operations SET status='succeeded',space_id=$2,updated_at=clock_timestamp() WHERE operation_id=$1 AND owner_generation=$3`, operationID, spaceID, op.OwnerGeneration)
	if err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("save community bootstrap receipt: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE gis_community_owner_authority SET status='active',space_id=$4,updated_at=clock_timestamp()
		WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND owner_account_id=$5 AND owner_profile_id=$6 AND owner_generation=$7 AND status='pending'`,
		op.ApplicationID, op.EnvironmentID, op.CorporationKey, spaceID, op.OwnerAccountID, op.OwnerProfileID, op.OwnerGeneration)
	if err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("activate community owner authority: %w", err)
	}
	op, err = loadCommunityBootstrap(ctx, tx, operationID)
	if err != nil {
		return CommunityBootstrapOperation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("commit community bootstrap receipt: %w", err)
	}
	return op, nil
}

func (s *Store) communityBootstrapNow() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

const communityBootstrapSelect = `SELECT operation_id,application_id,environment_id,owner_account_id,owner_profile_id,owner_generation,corporation_key,template_id,status,operator_account_id,approved_at,approval_expires_at,space_id,created_at FROM gis_community_bootstrap_operations`

func scanCommunityBootstrap(row pgx.Row) (CommunityBootstrapOperation, error) {
	var op CommunityBootstrapOperation
	var operator, space *uuid.UUID
	var approved, expires *time.Time
	err := row.Scan(&op.OperationID, &op.ApplicationID, &op.EnvironmentID, &op.OwnerAccountID, &op.OwnerProfileID, &op.OwnerGeneration, &op.CorporationKey, &op.TemplateID, &op.Status, &operator, &approved, &expires, &space, &op.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CommunityBootstrapOperation{}, ErrInvalidApplication
	}
	if err != nil {
		return CommunityBootstrapOperation{}, fmt.Errorf("scan community bootstrap operation: %w", err)
	}
	op.OperatorID, op.ApprovedAt, op.ExpiresAt, op.SpaceID = operator, approved, expires, space
	return op, nil
}

func loadOwnerAuthorityForUpdate(ctx context.Context, tx pgx.Tx, applicationID, environmentID uuid.UUID, corporationKey string) (OwnerAuthority, error) {
	var authority OwnerAuthority
	err := tx.QueryRow(ctx, `SELECT owner_account_id,owner_profile_id,owner_generation,status,space_id
		FROM gis_community_owner_authority WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 FOR UPDATE`,
		applicationID, environmentID, corporationKey).Scan(&authority.OwnerAccountID, &authority.OwnerProfileID, &authority.Generation, &authority.Status, &authority.SpaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return OwnerAuthority{}, ErrInvalidApplication
	}
	if err != nil {
		return OwnerAuthority{}, fmt.Errorf("read community owner authority: %w", err)
	}
	return authority, nil
}

func loadCommunityBootstrap(ctx context.Context, tx pgx.Tx, id uuid.UUID) (CommunityBootstrapOperation, error) {
	return scanCommunityBootstrap(tx.QueryRow(ctx, communityBootstrapSelect+` WHERE operation_id=$1`, id))
}

func loadCommunityBootstrapForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (CommunityBootstrapOperation, error) {
	return scanCommunityBootstrap(tx.QueryRow(ctx, communityBootstrapSelect+` WHERE operation_id=$1 FOR UPDATE`, id))
}

// requireCommunityBootstrapParent admits normal provisioning only while its
// parent remains active. Security recovery/cancellation deliberately use their
// own frozen-authority rules rather than this normal-provisioning gate.
func requireCommunityBootstrapParent(ctx context.Context, tx pgx.Tx, applicationID, environmentID uuid.UUID) error {
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM applications WHERE id=$1 FOR SHARE`, applicationID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidApplication
		}
		return fmt.Errorf("read community bootstrap parent: %w", err)
	}
	if status != "sandbox" && status != "active" {
		return ErrInvalidApplication
	}
	var envApp uuid.UUID
	var kind, envStatus string
	if err := tx.QueryRow(ctx, `SELECT application_id,kind,status FROM environments WHERE id=$1 FOR SHARE`, environmentID).Scan(&envApp, &kind, &envStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidApplication
		}
		return fmt.Errorf("read community bootstrap environment: %w", err)
	}
	if envApp != applicationID || kind != "sandbox" || envStatus != "active" {
		return ErrInvalidApplication
	}
	return nil
}
