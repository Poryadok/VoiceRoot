package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"voice/backend/role/permissions"
)

var (
	ErrOwnershipTransferConflict = errors.New("ownership transfer operation conflicts with recorded request")
	ErrOwnershipTransferState    = errors.New("ownership transfer owner state does not match expected owner")
	ErrOwnershipTransferMissing  = errors.New("ownership transfer apply receipt not found")
)

type OwnershipTransferInput struct {
	SpaceID, OldOwnerProfileID, NewOwnerProfileID, OperationID uuid.UUID
	RequestHash                                                string
}

func (s *RoleStore) ApplyOwnershipTransfer(ctx context.Context, in OwnershipTransferInput) (uuid.UUID, error) {
	return s.transitionOwnerRole(ctx, "apply", in)
}

func (s *RoleStore) CompensateOwnershipTransfer(ctx context.Context, in OwnershipTransferInput) (uuid.UUID, error) {
	return s.transitionOwnerRole(ctx, "compensate", in)
}

func (s *RoleStore) transitionOwnerRole(ctx context.Context, action string, in OwnershipTransferInput) (uuid.UUID, error) {
	if s == nil || s.Pool == nil {
		return uuid.Nil, errors.New("role store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, in.SpaceID.String()); err != nil {
		return uuid.Nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, in.OperationID.String()); err != nil {
		return uuid.Nil, err
	}
	var retired bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL)`, in.SpaceID).Scan(&retired); err != nil {
		return uuid.Nil, err
	}
	if retired {
		return uuid.Nil, ErrSpaceRetired
	}

	// A durable compensation is terminal, including when it fenced an Apply
	// that had not reached this lock yet. Check it before replaying Apply success.
	var terminalSpace, terminalOld, terminalNew uuid.UUID
	var terminalHash string
	err = tx.QueryRow(ctx, `SELECT space_id, old_owner_profile_id, new_owner_profile_id, request_hash FROM ownership_transfer_role_receipts WHERE operation_id=$1 AND action='compensate'`, in.OperationID).Scan(&terminalSpace, &terminalOld, &terminalNew, &terminalHash)
	if err == nil {
		if terminalSpace != in.SpaceID || terminalOld != in.OldOwnerProfileID || terminalNew != in.NewOwnerProfileID || terminalHash != in.RequestHash {
			return uuid.Nil, ErrOwnershipTransferConflict
		}
		if action == "apply" {
			return uuid.Nil, ErrOwnershipTransferState
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	var gotSpace, gotOld, gotNew uuid.UUID
	var gotHash string
	err = tx.QueryRow(ctx, `SELECT space_id, old_owner_profile_id, new_owner_profile_id, request_hash FROM ownership_transfer_role_receipts WHERE operation_id = $1 AND action = $2`, in.OperationID, action).Scan(&gotSpace, &gotOld, &gotNew, &gotHash)
	if err == nil {
		if gotSpace != in.SpaceID || gotOld != in.OldOwnerProfileID || gotNew != in.NewOwnerProfileID || gotHash != in.RequestHash {
			return uuid.Nil, ErrOwnershipTransferConflict
		}
		var current uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT current_owner_profile_id FROM ownership_transfer_role_receipts WHERE operation_id = $1 AND action = $2`, in.OperationID, action).Scan(&current); err != nil {
			return uuid.Nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return uuid.Nil, err
		}
		return current, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	var otherSpace, otherOld, otherNew uuid.UUID
	var otherHash string
	err = tx.QueryRow(ctx, `SELECT space_id, old_owner_profile_id, new_owner_profile_id, request_hash FROM ownership_transfer_role_receipts WHERE operation_id = $1 LIMIT 1`, in.OperationID).Scan(&otherSpace, &otherOld, &otherNew, &otherHash)
	if err == nil && (otherSpace != in.SpaceID || otherOld != in.OldOwnerProfileID || otherNew != in.NewOwnerProfileID || otherHash != in.RequestHash) {
		return uuid.Nil, ErrOwnershipTransferConflict
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	abortWithoutApply := action == "compensate" && errors.Is(err, pgx.ErrNoRows)

	var ownerRoleID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM roles WHERE space_id=$1 AND name=$2 FOR UPDATE`, in.SpaceID, permissions.RoleOwner).Scan(&ownerRoleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, errRoleNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	from, to := in.OldOwnerProfileID, in.NewOwnerProfileID
	if action == "compensate" {
		from, to = to, from
	}
	if abortWithoutApply {
		from = in.OldOwnerProfileID
	}
	var hasFrom bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM member_roles WHERE space_id=$1 AND profile_id=$2 AND role_id=$3)`, in.SpaceID, from, ownerRoleID).Scan(&hasFrom); err != nil {
		return uuid.Nil, err
	}
	if !hasFrom {
		return uuid.Nil, ErrOwnershipTransferState
	}
	if abortWithoutApply {
		var ownerCount int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM member_roles WHERE space_id=$1 AND role_id=$2`, in.SpaceID, ownerRoleID).Scan(&ownerCount); err != nil {
			return uuid.Nil, err
		}
		if ownerCount != 1 {
			return uuid.Nil, ErrOwnershipTransferState
		}
	} else {
		if _, err := tx.Exec(ctx, `INSERT INTO member_roles (space_id, profile_id, role_id, assigned_by) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`, in.SpaceID, to, ownerRoleID, from); err != nil {
			return uuid.Nil, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM member_roles WHERE space_id=$1 AND profile_id=$2 AND role_id=$3`, in.SpaceID, from, ownerRoleID); err != nil {
			return uuid.Nil, err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ownership_transfer_role_receipts (operation_id,action,space_id,old_owner_profile_id,new_owner_profile_id,request_hash,current_owner_profile_id) VALUES ($1,$2,$3,$4,$5,$6,$7)`, in.OperationID, action, in.SpaceID, in.OldOwnerProfileID, in.NewOwnerProfileID, in.RequestHash, to); err != nil {
		return uuid.Nil, fmt.Errorf("record ownership transfer receipt: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return to, nil
}
