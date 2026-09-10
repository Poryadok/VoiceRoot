package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/permissions"
)

var ErrOwnershipTransferV2InvalidInput = errors.New("invalid ownership transfer v2 input")

type OwnershipTransferV2Input struct {
	ProtocolVersion                                            uint32
	SpaceID, OldOwnerProfileID, NewOwnerProfileID, OperationID uuid.UUID
	IntentBytes, RequestHash                                   []byte
}

type OwnershipTransferV2Receipt struct {
	IntentBytes           []byte
	State                 string
	CurrentOwnerProfileID uuid.UUID
}

func validateOwnershipTransferV2Input(in OwnershipTransferV2Input) error {
	var intent rolev1.OwnershipTransferIntent
	if len(in.IntentBytes) == 0 || len(in.RequestHash) != 32 || proto.Unmarshal(in.IntentBytes, &intent) != nil {
		return ErrOwnershipTransferV2InvalidInput
	}
	canonical, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&intent)
	if err != nil || !bytes.Equal(canonical, in.IntentBytes) || intent.ProtocolVersion != in.ProtocolVersion {
		return ErrOwnershipTransferV2InvalidInput
	}
	for _, field := range []struct {
		raw  string
		want uuid.UUID
	}{
		{intent.SpaceId, in.SpaceID}, {intent.OldOwnerProfileId, in.OldOwnerProfileID},
		{intent.NewOwnerProfileId, in.NewOwnerProfileID}, {intent.OperationId, in.OperationID},
	} {
		id, err := uuid.Parse(field.raw)
		if err != nil || id == uuid.Nil || id != field.want {
			return ErrOwnershipTransferV2InvalidInput
		}
	}
	if in.OldOwnerProfileID == in.NewOwnerProfileID {
		return ErrOwnershipTransferV2InvalidInput
	}
	return nil
}

func (s *RoleStore) PrepareOwnershipTransfer(ctx context.Context, in OwnershipTransferV2Input) (OwnershipTransferV2Receipt, error) {
	return s.transitionOwnershipV2(ctx, "prepared", in)
}

func (s *RoleStore) FinalizeOwnershipTransfer(ctx context.Context, in OwnershipTransferV2Input) (OwnershipTransferV2Receipt, error) {
	return s.transitionOwnershipV2(ctx, "finalized", in)
}

func (s *RoleStore) AbortOwnershipTransfer(ctx context.Context, in OwnershipTransferV2Input) (OwnershipTransferV2Receipt, error) {
	return s.transitionOwnershipV2(ctx, "aborted", in)
}

func (s *RoleStore) transitionOwnershipV2(ctx context.Context, action string, in OwnershipTransferV2Input) (OwnershipTransferV2Receipt, error) {
	if err := validateOwnershipTransferV2Input(in); err != nil {
		return OwnershipTransferV2Receipt{}, err
	}
	var result OwnershipTransferV2Receipt
	err := s.withLockedTransaction(ctx, in.OperationID, []uuid.UUID{in.SpaceID}, func(scoped *RoleStore) error {
		db := scoped.db()
		var legacy bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ownership_transfer_role_receipts WHERE operation_id=$1)`, in.OperationID).Scan(&legacy); err != nil {
			return err
		}
		if legacy {
			return ErrOwnershipTransferConflict
		}
		var row ownershipV2Row
		err := db.QueryRow(ctx, `SELECT space_id, protocol_version, old_owner_profile_id, new_owner_profile_id,
            intent_bytes, intent_hash, state, prepare_request_hash, finalize_request_hash, abort_request_hash
            FROM ownership_transfer_v2 WHERE operation_id=$1`, in.OperationID).Scan(
			&row.spaceID, &row.version, &row.oldOwner, &row.newOwner, &row.intent, &row.intentHash,
			&row.state, &row.prepareHash, &row.finalizeHash, &row.abortHash)
		exists := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		intentHash := sha256.Sum256(in.IntentBytes)
		if exists && (row.spaceID != in.SpaceID || row.version != in.ProtocolVersion || row.oldOwner != in.OldOwnerProfileID || row.newOwner != in.NewOwnerProfileID || !bytes.Equal(row.intent, in.IntentBytes) || !bytes.Equal(row.intentHash, intentHash[:])) {
			return ErrOwnershipTransferConflict
		}
		if !exists && in.ProtocolVersion != 2 {
			return ErrOwnershipTransferV2InvalidInput
		}
		if exists {
			accepted := row.actionHash(action)
			if accepted != nil && !bytes.Equal(accepted, in.RequestHash) {
				return ErrOwnershipTransferConflict
			}
		}
		var retired, active bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL)`, in.SpaceID).Scan(&retired); err != nil {
			return err
		}
		if retired {
			return ErrSpaceRetired
		}
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ownership_transfer_v2 WHERE space_id=$1 AND state='prepared' AND operation_id<>$2)`, in.SpaceID, in.OperationID).Scan(&active); err != nil {
			return err
		}
		if active {
			return ErrSpaceFrozen
		}
		if exists {
			if row.state == action {
				result = ownershipV2Receipt(in, row.state, row.intent)
				return nil
			}
			if row.state != "prepared" || action == "prepared" {
				return ErrOwnershipTransferState
			}
			roleID, err := scoped.soleOwnershipV2Owner(ctx, in)
			if err != nil {
				return err
			}
			if action == "finalized" {
				changed, err := db.Exec(ctx, `UPDATE member_roles SET profile_id=$4,assigned_by=$3,assigned_at=now()
                    WHERE space_id=$1 AND role_id=$2 AND profile_id=$3`, in.SpaceID, roleID, in.OldOwnerProfileID, in.NewOwnerProfileID)
				if err != nil {
					return err
				}
				if changed.RowsAffected() != 1 {
					return ErrOwnershipTransferState
				}
				_, err = db.Exec(ctx, `UPDATE ownership_transfer_v2 SET state='finalized',finalize_request_hash=$2,updated_at=now() WHERE operation_id=$1`, in.OperationID, in.RequestHash)
				if err != nil {
					return err
				}
			} else {
				_, err = db.Exec(ctx, `UPDATE ownership_transfer_v2 SET state='aborted',abort_request_hash=$2,updated_at=now() WHERE operation_id=$1`, in.OperationID, in.RequestHash)
				if err != nil {
					return err
				}
			}
			result = ownershipV2Receipt(in, action, row.intent)
			return nil
		}
		if action == "finalized" {
			return ErrOwnershipTransferMissing
		}
		if _, err := scoped.soleOwnershipV2Owner(ctx, in); err != nil {
			return err
		}
		var prepareHash, abortHash []byte
		if action == "prepared" {
			prepareHash = in.RequestHash
		} else {
			abortHash = in.RequestHash
		}
		_, err = db.Exec(ctx, `INSERT INTO ownership_transfer_v2
            (operation_id,space_id,protocol_version,old_owner_profile_id,new_owner_profile_id,intent_bytes,intent_hash,state,prepare_request_hash,abort_request_hash)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, in.OperationID, in.SpaceID, in.ProtocolVersion, in.OldOwnerProfileID, in.NewOwnerProfileID, in.IntentBytes, intentHash[:], action, prepareHash, abortHash)
		if err != nil {
			return err
		}
		result = ownershipV2Receipt(in, action, in.IntentBytes)
		return nil
	})
	if err != nil {
		return OwnershipTransferV2Receipt{}, err
	}
	return result, nil
}

type ownershipV2Row struct {
	spaceID, oldOwner, newOwner          uuid.UUID
	version                              uint32
	intent, intentHash                   []byte
	state                                string
	prepareHash, finalizeHash, abortHash []byte
}

func (r ownershipV2Row) actionHash(action string) []byte {
	switch action {
	case "prepared":
		return r.prepareHash
	case "finalized":
		return r.finalizeHash
	default:
		return r.abortHash
	}
}

func ownershipV2Receipt(in OwnershipTransferV2Input, state string, intent []byte) OwnershipTransferV2Receipt {
	out := OwnershipTransferV2Receipt{IntentBytes: append([]byte(nil), intent...), State: state}
	if state == "finalized" {
		out.CurrentOwnerProfileID = in.NewOwnerProfileID
	}
	if state == "aborted" {
		out.CurrentOwnerProfileID = in.OldOwnerProfileID
	}
	return out
}

// The space serialization lock is held before validating membership rows. Count
// every assignment of this Owner role, including malformed foreign-space rows.
func (s *RoleStore) soleOwnershipV2Owner(ctx context.Context, in OwnershipTransferV2Input) (uuid.UUID, error) {
	var roleID uuid.UUID
	err := s.db().QueryRow(ctx, `SELECT id FROM roles WHERE space_id=$1 AND name=$2 AND is_system=true FOR UPDATE`, in.SpaceID, permissions.RoleOwner).Scan(&roleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrOwnershipTransferState
	}
	if err != nil {
		return uuid.Nil, err
	}
	rows, err := s.db().Query(ctx, `SELECT space_id,profile_id FROM member_roles WHERE role_id=$1 FOR UPDATE`, roleID)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	count := 0
	valid := true
	for rows.Next() {
		var spaceID, profileID uuid.UUID
		if err := rows.Scan(&spaceID, &profileID); err != nil {
			return uuid.Nil, err
		}
		count++
		valid = valid && spaceID == in.SpaceID && profileID == in.OldOwnerProfileID
	}
	if err := rows.Err(); err != nil {
		return uuid.Nil, err
	}
	if count != 1 || !valid {
		return uuid.Nil, ErrOwnershipTransferState
	}
	return roleID, nil
}
