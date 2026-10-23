package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *RoleStore) nativeOwnerCatalog(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db().QueryRow(ctx, `SELECT to_regclass('public.role_member_owner_authority') IS NOT NULL`).Scan(&exists)
	return exists, err
}

// Called only by the existing bootstrap writer under its operation/Space fence.
// There is deliberately no migration backfill from raw historical Owner rows.
func (s *RoleStore) recordNativeOwnerBootstrap(ctx context.Context, space, profile, role uuid.UUID) error {
	exists, err := s.nativeOwnerCatalog(ctx)
	if err != nil || !exists {
		return err
	}
	_, err = s.db().Exec(ctx, `INSERT INTO role_member_owner_authority(space_id,profile_id,role_id,origin,generation) VALUES($1,$2,$3,'BOOTSTRAP',1) ON CONFLICT(space_id) DO NOTHING`, space, profile, role)
	if err != nil {
		return err
	}
	var currentProfile, currentRole uuid.UUID
	if err = s.db().QueryRow(ctx, `SELECT profile_id,role_id FROM role_member_owner_authority WHERE space_id=$1`, space).Scan(&currentProfile, &currentRole); err != nil {
		return err
	}
	if currentProfile != profile || currentRole != role {
		return ErrScopeUnavailable
	}
	return nil
}

func (s *RoleStore) recordNativeOwnerTransfer(ctx context.Context, in OwnershipTransferV2Input, role uuid.UUID) error {
	exists, err := s.nativeOwnerCatalog(ctx)
	if err != nil || !exists {
		return err
	}
	var profile uuid.UUID
	err = s.db().QueryRow(ctx, `SELECT profile_id FROM role_member_owner_authority WHERE space_id=$1 FOR UPDATE`, in.SpaceID).Scan(&profile)
	if err == pgx.ErrNoRows {
		_, err = s.db().Exec(ctx, `INSERT INTO role_member_owner_authority(space_id,profile_id,role_id,origin,generation,operation_id) VALUES($1,$2,$3,'TRANSFER',1,$4)`, in.SpaceID, in.NewOwnerProfileID, role, in.OperationID)
		return err
	}
	if err != nil {
		return err
	}
	if profile != in.OldOwnerProfileID {
		return ErrOwnershipTransferState
	}
	_, err = s.db().Exec(ctx, `UPDATE role_member_owner_authority SET profile_id=$2,origin='TRANSFER',generation=generation+1,operation_id=$3 WHERE space_id=$1`, in.SpaceID, in.NewOwnerProfileID, in.OperationID)
	return err
}
