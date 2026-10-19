package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"

	"github.com/google/uuid"
	"voice/backend/pkg/memberadmission"
)

// The private removal identity is domain + operationUUID16 + SpaceUUID16 +
// profileUUID16 + floorBE64. It never includes transport JTI/request IDs.
func memberRoleRemovalBinding(operation, space, profile uuid.UUID, floor uint64) (memberadmission.RemoveBinding, error) {
	if operation == uuid.Nil || space == uuid.Nil || profile == uuid.Nil || floor == 0 || floor > math.MaxInt64 {
		return memberadmission.RemoveBinding{}, ErrMemberAdmissionConflict
	}
	body := []byte("voice.space.member-role-removal.v1\x00")
	for _, id := range []uuid.UUID{operation, space, profile} {
		body = append(body, id[:]...)
	}
	body = binary.BigEndian.AppendUint64(body, floor)
	digest := sha256.Sum256(body)
	return memberadmission.RemoveBinding{ProtocolVersion: 1, SpaceID: space.String(), ProfileID: profile.String(), RemovalOperationID: operation.String(), RemovalGenerationFloor: floor, RemovalBindingSHA256: digest[:]}, nil
}

func (s *SpaceStore) ListPendingMemberRoleRemovals(ctx context.Context, limit int) ([]memberadmission.RemoveBinding, error) {
	if s == nil || s.Pool == nil || s.tx != nil || limit < 1 || limit > 100 {
		return nil, ErrMemberAdmissionConflict
	}
	rows, err := s.Pool.Query(ctx, `SELECT operation_id,space_id,profile_id,removal_floor FROM space_member_role_cleanup WHERE completed_at IS NULL ORDER BY space_id,profile_id,removal_floor,operation_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []memberadmission.RemoveBinding
	for rows.Next() {
		var operation, space, profile uuid.UUID
		var floor uint64
		if err := rows.Scan(&operation, &space, &profile, &floor); err != nil {
			return nil, err
		}
		binding, err := memberRoleRemovalBinding(operation, space, profile, floor)
		if err != nil {
			return nil, err
		}
		result = append(result, binding)
	}
	return result, rows.Err()
}

// The Role call and exact receipt validation precede this short transaction.
// P3 shares the Space fence; a missing or changed intent never counts as ACK.
func (s *SpaceStore) MarkMemberRoleRemovalCompleted(ctx context.Context, binding memberadmission.RemoveBinding, receipt []byte) error {
	if s == nil || s.Pool == nil || s.tx != nil || memberadmission.ValidateRemove(binding) != nil {
		return ErrMemberAdmissionConflict
	}
	operation, space, profile := uuid.MustParse(binding.RemovalOperationID), uuid.MustParse(binding.SpaceID), uuid.MustParse(binding.ProfileID)
	exact, err := memberRoleRemovalBinding(operation, space, profile, binding.RemovalGenerationFloor)
	if err != nil || !bytes.Equal(exact.RemovalBindingSHA256, binding.RemovalBindingSHA256) {
		return ErrMemberAdmissionConflict
	}
	want, err := memberadmission.RemoveReceiptFingerprint(binding)
	if err != nil || !bytes.Equal(want[:], receipt) {
		return ErrMemberAdmissionConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockLifecycleSpace(ctx, tx, space); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE space_member_role_cleanup SET completed_at=COALESCE(completed_at,clock_timestamp()),receipt_sha256=$5
	 WHERE operation_id=$1 AND space_id=$2 AND profile_id=$3 AND removal_floor=$4 AND (completed_at IS NULL OR receipt_sha256=$5)`, operation, space, profile, binding.RemovalGenerationFloor, receipt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMemberAdmissionConflict
	}
	return tx.Commit(ctx)
}
