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
	err = tx.QueryRow(ctx, `SELECT space_id, old_owner_profile_id, new_owner_profile_id FROM ownership_transfer_role_receipts WHERE operation_id = $1 LIMIT 1`, in.OperationID).Scan(&otherSpace, &otherOld, &otherNew)
	if err == nil && (otherSpace != in.SpaceID || otherOld != in.OldOwnerProfileID || otherNew != in.NewOwnerProfileID) {
		return uuid.Nil, ErrOwnershipTransferConflict
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	if action == "compensate" {
		var exists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ownership_transfer_role_receipts WHERE operation_id=$1 AND action='apply' AND space_id=$2 AND old_owner_profile_id=$3 AND new_owner_profile_id=$4)`, in.OperationID, in.SpaceID, in.OldOwnerProfileID, in.NewOwnerProfileID).Scan(&exists)
		if err != nil {
			return uuid.Nil, err
		}
		if !exists {
			return uuid.Nil, ErrOwnershipTransferMissing
		}
	}
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
	var hasFrom bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM member_roles WHERE space_id=$1 AND profile_id=$2 AND role_id=$3)`, in.SpaceID, from, ownerRoleID).Scan(&hasFrom); err != nil {
		return uuid.Nil, err
	}
	if !hasFrom {
		return uuid.Nil, ErrOwnershipTransferState
	}
	if _, err := tx.Exec(ctx, `INSERT INTO member_roles (space_id, profile_id, role_id, assigned_by) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`, in.SpaceID, to, ownerRoleID, from); err != nil {
		return uuid.Nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM member_roles WHERE space_id=$1 AND profile_id=$2 AND role_id=$3`, in.SpaceID, from, ownerRoleID); err != nil {
		return uuid.Nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO ownership_transfer_role_receipts (operation_id,action,space_id,old_owner_profile_id,new_owner_profile_id,request_hash,current_owner_profile_id) VALUES ($1,$2,$3,$4,$5,$6,$7)`, in.OperationID, action, in.SpaceID, in.OldOwnerProfileID, in.NewOwnerProfileID, in.RequestHash, to); err != nil {
		return uuid.Nil, fmt.Errorf("record ownership transfer receipt: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return to, nil
}
