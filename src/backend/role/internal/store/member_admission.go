package store

import (
	"bytes"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/memberadmission"
	"voice/backend/role/permissions"
)

var (
	ErrMemberAdmissionConflict    = errors.New("member admission operation binding changed")
	ErrMemberAdmissionStale       = errors.New("member admission generation retired or superseded")
	ErrMemberAdmissionDefaultRole = errors.New("Owner cannot be the member admission default")
)

// MemberAdmissionReceipt is historical evidence, never a current permission.
type MemberAdmissionReceipt struct {
	OperationKind                   string
	ProtocolVersion                 uint32
	SpaceID, ProfileID, OperationID string
	Generation                      uint64
	BindingSHA256                   []byte
	RoleID                          string
	ReceiptSHA256                   []byte
}

// PrepareDefaultMemberRole is the local inert preparation primitive. The RPC
// caller must obtain authenticated PREPARING Space authority outside all Role
// transactions first. No row is inserted into member_roles by this operation.
func (s *RoleStore) PrepareDefaultMemberRole(ctx context.Context, binding memberadmission.EnsureBinding) (MemberAdmissionReceipt, error) {
	return s.prepareDefaultMemberRole(ctx, binding, nil)
}

func (s *RoleStore) prepareDefaultMemberRole(ctx context.Context, binding memberadmission.EnsureBinding, guard func(bool) error) (MemberAdmissionReceipt, error) {
	if err := memberadmission.ValidateEnsure(binding); err != nil {
		return MemberAdmissionReceipt{}, err
	}
	space := uuid.MustParse(binding.SpaceID)
	operation := uuid.MustParse(binding.AdmissionOperationID)
	want := MemberAdmissionReceipt{OperationKind: "ENSURE", ProtocolVersion: binding.ProtocolVersion, SpaceID: binding.SpaceID, ProfileID: binding.ProfileID, OperationID: binding.AdmissionOperationID, Generation: binding.MembershipGeneration, BindingSHA256: binding.AdmissionBindingSHA256}
	var out MemberAdmissionReceipt
	err := s.withLockedTransaction(ctx, operation, []uuid.UUID{space}, func(scoped *RoleStore) error {
		// Retirement and ownership/P3 fences dominate even historical replay.
		return scoped.WithinSpaces(ctx, []uuid.UUID{space}, func(scoped *RoleStore) error {
			if guard != nil {
				if err := scoped.checkQualifiedRoleCatalog(ctx); err != nil {
					return err
				}
				if err := guard(false); err != nil {
					return err
				}
			}
			old, found, err := scoped.memberAdmissionReceipt(ctx, want)
			if err != nil {
				return err
			}
			if found {
				if guard != nil {
					if err := guard(false); err != nil {
						return err
					}
				}
				out = old
				return nil
			}
			var floor, allocated int64
			if err := scoped.db().QueryRow(ctx, `SELECT COALESCE((SELECT retired_through FROM role_member_source_floors WHERE space_id=$1 AND profile_id=$2 AND source_kind='ORDINARY'),0),COALESCE((SELECT MAX(membership_generation) FROM role_member_admission_preparations WHERE space_id=$1 AND profile_id=$2),0)`, space, uuid.MustParse(binding.ProfileID)).Scan(&floor, &allocated); err != nil {
				return err
			}
			if binding.MembershipGeneration <= uint64(floor) || binding.MembershipGeneration < uint64(allocated) {
				return ErrMemberAdmissionStale
			}
			if binding.MembershipGeneration == uint64(allocated) {
				return ErrMemberAdmissionConflict
			}
			role, err := scoped.GetDefaultJoinRole(ctx, space)
			if err != nil {
				return err
			}
			if role == nil || role.SpaceID != space {
				return ErrRoleNotFound
			}
			if role.Managed && role.Name == permissions.RoleOwner {
				return ErrMemberAdmissionDefaultRole
			}
			fingerprint, err := memberadmission.EnsureReceiptFingerprint(binding, role.ID.String())
			if err != nil {
				return err
			}
			out = want
			out.RoleID = role.ID.String()
			out.ReceiptSHA256 = append([]byte(nil), fingerprint[:]...)
			if guard != nil {
				if err := guard(true); err != nil {
					return err
				}
			}
			if err := scoped.insertMemberAdmissionReceipt(ctx, out); err != nil {
				return err
			}
			_, err = scoped.db().Exec(ctx, `INSERT INTO role_member_admission_preparations(space_id,profile_id,membership_generation,admission_operation_id,role_id,receipt_sha256) VALUES($1,$2,$3,$4,$5,$6)`, space, uuid.MustParse(binding.ProfileID), int64(binding.MembershipGeneration), operation, role.ID, out.ReceiptSHA256)
			if err == nil && guard != nil {
				return guard(true)
			}
			return err
		})
	})
	if err != nil {
		return MemberAdmissionReceipt{}, err
	}
	return out, nil
}

// RemoveOrdinaryMemberRoles retires only the ordinary source. Persisting the
// floor is mandatory even if no preparation exists; community floors are never
// compared or modified. Exact replay does not erase later incarnations.
func (s *RoleStore) RemoveOrdinaryMemberRoles(ctx context.Context, binding memberadmission.RemoveBinding) (MemberAdmissionReceipt, error) {
	return s.removeOrdinaryMemberRoles(ctx, binding, nil)
}

func (s *RoleStore) removeOrdinaryMemberRoles(ctx context.Context, binding memberadmission.RemoveBinding, guard func() error) (MemberAdmissionReceipt, error) {
	if err := memberadmission.ValidateRemove(binding); err != nil {
		return MemberAdmissionReceipt{}, err
	}
	space, profile, operation := uuid.MustParse(binding.SpaceID), uuid.MustParse(binding.ProfileID), uuid.MustParse(binding.RemovalOperationID)
	want := MemberAdmissionReceipt{OperationKind: "REMOVE", ProtocolVersion: binding.ProtocolVersion, SpaceID: binding.SpaceID, ProfileID: binding.ProfileID, OperationID: binding.RemovalOperationID, Generation: binding.RemovalGenerationFloor, BindingSHA256: binding.RemovalBindingSHA256}
	var out MemberAdmissionReceipt
	err := s.withLockedTransaction(ctx, operation, []uuid.UUID{space}, func(scoped *RoleStore) error {
		return scoped.WithinSpaces(ctx, []uuid.UUID{space}, func(scoped *RoleStore) error {
			if guard != nil {
				if err := scoped.checkQualifiedRoleCatalog(ctx); err != nil {
					return err
				}
				if err := guard(); err != nil {
					return err
				}
			}
			old, found, err := scoped.memberAdmissionReceipt(ctx, want)
			if err != nil {
				return err
			}
			if found {
				if guard != nil {
					if err := guard(); err != nil {
						return err
					}
				}
				out = old
				return nil
			}
			fingerprint, err := memberadmission.RemoveReceiptFingerprint(binding)
			if err != nil {
				return err
			}
			out = want
			out.ReceiptSHA256 = append([]byte(nil), fingerprint[:]...)
			if err := scoped.insertMemberAdmissionReceipt(ctx, out); err != nil {
				return err
			}
			if _, err := scoped.db().Exec(ctx, `INSERT INTO role_member_source_floors(space_id,profile_id,source_kind,retired_through) VALUES($1,$2,'ORDINARY',$3) ON CONFLICT(space_id,profile_id,source_kind) DO UPDATE SET retired_through=EXCLUDED.retired_through WHERE role_member_source_floors.retired_through<EXCLUDED.retired_through`, space, profile, int64(binding.RemovalGenerationFloor)); err != nil {
				return err
			}
			_, err = scoped.db().Exec(ctx, `DELETE FROM role_member_admission_preparations WHERE space_id=$1 AND profile_id=$2 AND membership_generation<=$3`, space, profile, int64(binding.RemovalGenerationFloor))
			if err == nil && guard != nil {
				return guard()
			}
			return err
		})
	})
	if err != nil {
		return MemberAdmissionReceipt{}, err
	}
	return out, nil
}

func (s *RoleStore) memberAdmissionReceipt(ctx context.Context, want MemberAdmissionReceipt) (MemberAdmissionReceipt, bool, error) {
	var out MemberAdmissionReceipt
	var space, profile, operation uuid.UUID
	var role *uuid.UUID
	var version int32
	var generation int64
	err := s.db().QueryRow(ctx, `SELECT operation_id,operation_kind,protocol_version,space_id,profile_id,generation,binding_sha256,role_id,receipt_sha256 FROM role_member_admission_receipts WHERE operation_id=$1`, uuid.MustParse(want.OperationID)).Scan(&operation, &out.OperationKind, &version, &space, &profile, &generation, &out.BindingSHA256, &role, &out.ReceiptSHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	out.OperationID, out.SpaceID, out.ProfileID = operation.String(), space.String(), profile.String()
	out.ProtocolVersion, out.Generation = uint32(version), uint64(generation)
	if role != nil {
		out.RoleID = role.String()
	}
	if out.OperationKind != want.OperationKind || out.ProtocolVersion != want.ProtocolVersion || out.SpaceID != want.SpaceID || out.ProfileID != want.ProfileID || out.Generation != want.Generation || !bytes.Equal(out.BindingSHA256, want.BindingSHA256) {
		return MemberAdmissionReceipt{}, false, ErrMemberAdmissionConflict
	}
	return out, true, nil
}

func (s *RoleStore) insertMemberAdmissionReceipt(ctx context.Context, value MemberAdmissionReceipt) error {
	var role *uuid.UUID
	if value.RoleID != "" {
		id := uuid.MustParse(value.RoleID)
		role = &id
	}
	_, err := s.db().Exec(ctx, `INSERT INTO role_member_admission_receipts(operation_id,operation_kind,protocol_version,space_id,profile_id,generation,binding_sha256,role_id,receipt_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, uuid.MustParse(value.OperationID), value.OperationKind, int32(value.ProtocolVersion), uuid.MustParse(value.SpaceID), uuid.MustParse(value.ProfileID), int64(value.Generation), value.BindingSHA256, role, value.ReceiptSHA256)
	return err
}
