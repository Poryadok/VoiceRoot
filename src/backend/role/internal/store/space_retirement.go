package store

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrSpaceRetirementInvalidInput = errors.New("invalid space retirement input")
	ErrSpaceRetirementConflict     = errors.New("space retirement conflicts with permanent evidence")
)

type SpaceRetirementInput struct {
	ProtocolVersion     uint32
	SpaceID             uuid.UUID
	DeletionOperationID uuid.UUID
	Generation          uint64
	PurgeDecidedAt      time.Time
	ManifestID          uuid.UUID
	ManifestItemCount   uint64
	RequestSHA256       []byte
	ManifestSHA256      []byte
	RequestBytes        []byte
}

type SpaceRetirementReceipt struct {
	ProtocolVersion     uint32
	ReceiptID           uuid.UUID
	SpaceID             uuid.UUID
	DeletionOperationID uuid.UUID
	Generation          uint64
	RequestSHA256       []byte
	ManifestSHA256      []byte
	RetiredAt           time.Time
	ReceiptBytes        []byte
}

func (s *RoleStore) RetireSpace(ctx context.Context, in SpaceRetirementInput, buildReceipt func(SpaceRetirementReceipt) ([]byte, error)) (SpaceRetirementReceipt, error) {
	if s == nil || s.Pool == nil || buildReceipt == nil || in.ProtocolVersion != 1 || in.SpaceID == uuid.Nil || in.DeletionOperationID == uuid.Nil || in.Generation == 0 || in.PurgeDecidedAt.IsZero() || in.ManifestID == uuid.Nil || len(in.RequestSHA256) != 32 || len(in.ManifestSHA256) != 32 || len(in.RequestBytes) == 0 {
		return SpaceRetirementReceipt{}, ErrSpaceRetirementInvalidInput
	}
	var result SpaceRetirementReceipt
	err := s.withLockedTransaction(ctx, in.DeletionOperationID, []uuid.UUID{in.SpaceID}, func(scoped *RoleStore) error {
		db := scoped.db()
		var stored SpaceRetirementReceipt
		var storedGeneration int64
		var requestBytes []byte
		var storedPurgeDecidedAt time.Time
		var storedManifestID uuid.UUID
		var storedManifestItemCount uint64
		err := db.QueryRow(ctx, `SELECT protocol_version,receipt_id,space_id,deletion_operation_id,generation,
 purge_decided_at,manifest_id,manifest_item_count,request_sha256,manifest_sha256,retired_at,receipt_bytes,request_bytes
 FROM role_space_retirement_receipts WHERE space_id=$1`, in.SpaceID).Scan(
			&stored.ProtocolVersion, &stored.ReceiptID, &stored.SpaceID, &stored.DeletionOperationID, &storedGeneration,
			&storedPurgeDecidedAt, &storedManifestID, &storedManifestItemCount, &stored.RequestSHA256, &stored.ManifestSHA256,
			&stored.RetiredAt, &stored.ReceiptBytes, &requestBytes)
		if err == nil {
			stored.Generation = uint64(storedGeneration)
			if stored.ProtocolVersion != in.ProtocolVersion || stored.SpaceID != in.SpaceID || stored.DeletionOperationID != in.DeletionOperationID || stored.Generation != in.Generation || !storedPurgeDecidedAt.Equal(in.PurgeDecidedAt) || storedManifestID != in.ManifestID || storedManifestItemCount != in.ManifestItemCount || !bytes.Equal(stored.RequestSHA256, in.RequestSHA256) || !bytes.Equal(stored.ManifestSHA256, in.ManifestSHA256) || (requestBytes != nil && !bytes.Equal(requestBytes, in.RequestBytes)) {
				return ErrSpaceRetirementConflict
			}
			if stored.ReceiptBytes == nil {
				var buildErr error
				stored.ReceiptBytes, buildErr = buildReceipt(stored)
				if buildErr != nil {
					return buildErr
				}
			}
			result = stored
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		var retired, prepared, operationUsed bool
		if err := db.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL),
 EXISTS(SELECT 1 FROM ownership_transfer_v2 WHERE space_id=$1 AND state='prepared'),
 EXISTS(SELECT 1 FROM role_space_retirement_receipts WHERE deletion_operation_id=$2)`, in.SpaceID, in.DeletionOperationID).Scan(&retired, &prepared, &operationUsed); err != nil {
			return err
		}
		if retired || operationUsed {
			return ErrSpaceRetirementConflict
		}
		if prepared {
			return ErrSpaceFrozen
		}

		result = SpaceRetirementReceipt{
			ProtocolVersion: in.ProtocolVersion, ReceiptID: uuid.New(), SpaceID: in.SpaceID,
			DeletionOperationID: in.DeletionOperationID, Generation: in.Generation,
			RequestSHA256: append([]byte(nil), in.RequestSHA256...), ManifestSHA256: append([]byte(nil), in.ManifestSHA256...),
		}
		if err := db.QueryRow(ctx, `SELECT GREATEST(clock_timestamp(),$1::timestamptz)`, in.PurgeDecidedAt).Scan(&result.RetiredAt); err != nil {
			return err
		}
		result.RetiredAt = result.RetiredAt.UTC()
		var buildErr error
		result.ReceiptBytes, buildErr = buildReceipt(result)
		if buildErr != nil {
			return buildErr
		}
		if _, err := db.Exec(ctx, `SELECT set_config('voice.role_retirement_space_id',$1,true)`, in.SpaceID.String()); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,$2)
 ON CONFLICT(space_id) DO UPDATE SET retired_at=EXCLUDED.retired_at WHERE role_space_lifecycle.retired_at IS NULL`, in.SpaceID, result.RetiredAt); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `DELETE FROM member_roles WHERE space_id=$1`, in.SpaceID); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `DELETE FROM roles WHERE space_id=$1`, in.SpaceID); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `INSERT INTO role_space_retirement_receipts
	 (space_id,deletion_operation_id,protocol_version,generation,purge_decided_at,manifest_id,manifest_item_count,receipt_id,
	  request_sha256,manifest_sha256,request_bytes,receipt_bytes,retired_at,full_bytes_retain_until)
	 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`,
			in.SpaceID, in.DeletionOperationID, in.ProtocolVersion, int64(in.Generation), in.PurgeDecidedAt,
			in.ManifestID, in.ManifestItemCount, result.ReceiptID, in.RequestSHA256, in.ManifestSHA256,
			in.RequestBytes, result.ReceiptBytes, result.RetiredAt); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return SpaceRetirementReceipt{}, err
	}
	return result, nil
}
