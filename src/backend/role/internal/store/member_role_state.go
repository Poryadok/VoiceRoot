package store

import (
	"bytes"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/memberadmission"
	"voice/backend/role/permissions"
)

// readMemberRoleState projects only Role-owned facts from the caller's fenced
// transaction. Native decisions and source2 use this same projection and pure
// qualifier; no Space request or membership inference occurs here.
func readMemberRoleState(ctx context.Context, db scopeExecutor, space string) (authoritysource.RoleStateV2, error) {
	all, err := permissions.AllMask()
	if err != nil {
		return authoritysource.RoleStateV2{}, err
	}
	s := authoritysource.RoleStateV2{SchemaVersion: 2, SpaceID: space, AllPermissions: all,
		Roles: []authoritysource.RoleDefinitionV2{}, Contributions: []authoritysource.RoleContributionV2{},
		Floors: []authoritysource.RoleMembershipFloor{}, Unresolved: []authoritysource.RoleAssignment{},
		ChatOverrides: []authoritysource.RoleOverride{}, VoiceOverrides: []authoritysource.RoleOverride{}, SDKSessionGrants: []authoritysource.SDKSessionGrant{}}
	err = db.QueryRow(ctx, `SELECT r.policy_epoch,s.revision FROM role_voice_policy_epochs r CROSS JOIN role_sdk_authority_revision s WHERE r.space_id=$1 AND s.singleton`, space).Scan(&s.RoleRevision, &s.SDKRevision)
	if err != nil {
		return s, err
	}
	var clockConflict bool
	if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_voice_policy_outbox WHERE space_id=$1 AND policy_epoch>$2)`, space, s.RoleRevision).Scan(&clockConflict); err != nil {
		return s, err
	}
	if clockConflict {
		return s, ErrScopeUnavailable
	}
	err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL),EXISTS(SELECT 1 FROM ownership_transfer_v2 WHERE space_id=$1 AND state='prepared'),COALESCE((SELECT state FROM role_space_deletion_fences WHERE space_id=$1),''),COALESCE((SELECT generation FROM role_space_deletion_fences WHERE space_id=$1),0)`, space).Scan(&s.Retired, &s.OwnershipFrozen, &s.DeletionState, &s.DeletionGeneration)
	if err != nil {
		return s, err
	}
	err = readRoleSourceRows(ctx, db, `SELECT id::text,name,is_system,position,permissions,is_default_join FROM roles WHERE space_id=$1 ORDER BY id LIMIT 10001`, []any{space}, func(row pgx.Rows) error {
		var r authoritysource.RoleDefinitionV2
		if err := row.Scan(&r.ID, &r.Name, &r.Managed, &r.Position, &r.Permissions, &r.DefaultJoin); err != nil {
			return err
		}
		s.Roles = append(s.Roles, r)
		if r.Managed && r.Name == permissions.RoleMember {
			s.CommunityMemberRoleID = r.ID
		}
		return nil
	})
	if err != nil {
		return s, err
	}
	var owner authoritysource.RoleOwnerLink
	err = db.QueryRow(ctx, `SELECT profile_id::text,role_id::text,origin,generation,COALESCE(operation_id::text,'') FROM role_member_owner_authority WHERE space_id=$1`, space).Scan(&owner.ProfileID, &owner.RoleID, &owner.Origin, &owner.Generation, &owner.OperationID)
	if err == nil {
		s.Owner = &owner
		var matches bool
		if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM member_roles WHERE space_id=$1 AND profile_id=$2 AND role_id=$3)`, space, owner.ProfileID, owner.RoleID).Scan(&matches); err != nil {
			return s, err
		}
		if !matches {
			return s, ErrScopeUnavailable
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return s, err
	}
	// A current preparation must agree with every immutable receipt field. An
	// orphan/mismatched witness is unavailable rather than a dropped grant.
	err = readRoleSourceRows(ctx, db, `SELECT p.profile_id::text,p.role_id::text,p.admission_operation_id::text,p.membership_generation,p.receipt_sha256,r.protocol_version,r.binding_sha256,r.receipt_sha256,r.operation_kind,r.space_id::text,r.profile_id::text,r.generation,COALESCE(r.role_id::text,'') FROM role_member_admission_preparations p LEFT JOIN role_member_admission_receipts r ON r.operation_id=p.admission_operation_id WHERE p.space_id=$1 ORDER BY p.admission_operation_id LIMIT 10001`, []any{space}, func(row pgx.Rows) error {
		var c authoritysource.RoleContributionV2
		var generation uint64
		var preparedReceipt []byte
		var kind, receiptSpace, receiptProfile, receiptRole string
		var receiptGeneration uint64
		b := memberadmission.EnsureBinding{SpaceID: space}
		proof := authoritysource.RoleAdmissionProof{}
		if err := row.Scan(&c.ProfileID, &c.RoleID, &c.ID, &generation, &preparedReceipt, &b.ProtocolVersion, &b.AdmissionBindingSHA256, &proof.ReceiptSHA256, &kind, &receiptSpace, &receiptProfile, &receiptGeneration, &receiptRole); err != nil {
			return err
		}
		if kind != "ENSURE" || receiptSpace != space || receiptProfile != c.ProfileID || receiptGeneration != generation || receiptRole != c.RoleID || !equalDigest(preparedReceipt, proof.ReceiptSHA256) {
			return ErrScopeUnavailable
		}
		b.ProfileID = c.ProfileID
		b.AdmissionOperationID = c.ID
		b.MembershipGeneration = generation
		proof.Binding = b
		c.Admission = &proof
		c.AuthorizationID = c.ID
		c.Origin = "ORDINARY_DEFAULT"
		c.Binding = memberadmission.MembershipBinding{Ordinary: &memberadmission.OrdinaryMembershipBinding{MembershipGeneration: generation, PublicationIdentity: c.ID}}
		s.Contributions = append(s.Contributions, c)
		return nil
	})
	if err != nil {
		return s, err
	}
	err = readRoleSourceRows(ctx, db, `SELECT id::text,authorization_id::text,profile_id::text,role_id::text,source_binding,source_kind,membership_generation,owner_generation FROM role_member_contributions WHERE space_id=$1 ORDER BY id LIMIT 10001`, []any{space}, func(row pgx.Rows) error {
		var c authoritysource.RoleContributionV2
		var raw []byte
		var source string
		var generation, ownerGeneration uint64
		if err := row.Scan(&c.ID, &c.AuthorizationID, &c.ProfileID, &c.RoleID, &raw, &source, &generation, &ownerGeneration); err != nil {
			return err
		}
		b, err := memberadmission.ParseMembershipBinding(raw)
		if err != nil {
			return err
		}
		canonical, err := memberadmission.CanonicalMembershipBinding(b)
		if err != nil || !bytes.Equal(raw, canonical) {
			return ErrScopeUnavailable
		}
		switch source {
		case "ORDINARY":
			if b.Ordinary == nil || b.Ordinary.MembershipGeneration != generation || ownerGeneration != 0 {
				return ErrScopeUnavailable
			}
		case "COMMUNITY":
			if b.Community == nil || b.Community.MembershipGeneration != generation || b.Community.OwnerGeneration != ownerGeneration {
				return ErrScopeUnavailable
			}
		default:
			return ErrScopeUnavailable
		}
		c.Origin = "INTERACTIVE"
		c.Binding = b
		s.Contributions = append(s.Contributions, c)
		return nil
	})
	if err != nil {
		return s, err
	}
	err = readRoleSourceRows(ctx, db, `SELECT profile_id::text,source_kind,retired_through FROM role_member_source_floors WHERE space_id=$1 ORDER BY profile_id,source_kind LIMIT 10001`, []any{space}, func(row pgx.Rows) error {
		var f authoritysource.RoleMembershipFloor
		if err := row.Scan(&f.ProfileID, &f.Source, &f.RetiredThrough); err != nil {
			return err
		}
		s.Floors = append(s.Floors, f)
		return nil
	})
	if err != nil {
		return s, err
	}
	err = readRoleSourceRows(ctx, db, `SELECT space_id::text,profile_id::text,role_id::text FROM member_roles WHERE space_id=$1 OR role_id IN(SELECT id FROM roles WHERE space_id=$1) ORDER BY profile_id,role_id LIMIT 10001`, []any{space}, func(row pgx.Rows) error {
		var a authoritysource.RoleAssignment
		var rowSpace string
		if err := row.Scan(&rowSpace, &a.ProfileID, &a.RoleID); err != nil {
			return err
		}
		if rowSpace != space {
			return ErrScopeUnavailable
		}
		if s.Owner == nil || a.ProfileID != s.Owner.ProfileID || a.RoleID != s.Owner.RoleID {
			s.Unresolved = append(s.Unresolved, a)
		}
		return nil
	})
	if err != nil {
		return s, err
	}
	for _, kind := range []struct {
		table, id string
		items     *[]authoritysource.RoleOverride
	}{{"chat_overrides", "chat_id", &s.ChatOverrides}, {"voice_room_overrides", "voice_room_id", &s.VoiceOverrides}} {
		err = readRoleSourceRows(ctx, db, `SELECT o.`+kind.id+`::text,o.role_id::text,o.allow,o.deny FROM `+kind.table+` o JOIN roles r ON r.id=o.role_id WHERE r.space_id=$1 ORDER BY o.`+kind.id+`,o.role_id LIMIT 10001`, []any{space}, func(row pgx.Rows) error {
			var o authoritysource.RoleOverride
			if err := row.Scan(&o.ResourceID, &o.RoleID, &o.Allow, &o.Deny); err != nil {
				return err
			}
			*kind.items = append(*kind.items, o)
			return nil
		})
		if err != nil {
			return s, err
		}
	}
	authoritysource.SortRoleStateV2(&s)
	return s, authoritysource.ValidateRoleStateV2(s)
}

func equalDigest(a, b []byte) bool {
	if len(a) != 32 || len(b) != 32 {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
