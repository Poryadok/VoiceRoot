package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"voice/backend/pkg/memberadmission"
)

var (
	ErrSpaceChatAuthorityChanged  = errors.New("space chat Role authority changed")
	ErrSpaceChatOperationConflict = errors.New("space chat operation binding changed")
	ErrInvalidSpaceChatMutation   = errors.New("invalid space chat mutation")
)

type SpaceChatPermissionSnapshot struct {
	Revision               uint64
	MemberAdditionsEnabled bool
	ExplicitPostGrant      bool
}

// SpaceChatMutation contains both the caller's optional selector and the exact
// source selected by fresh owner authority. ResolvedBinding is not request
// identity: an omitted selector is pinned in the first committed receipt.
type SpaceChatMutation struct {
	OperationID          uuid.UUID
	ActorProfileID       uuid.UUID
	SpaceID              uuid.UUID
	ChatID               uuid.UUID
	ChatScopeIncarnation uuid.UUID
	TargetProfileID      uuid.UUID
	Kind                 string
	Allowed              bool
	RequestedBinding     *memberadmission.MembershipBinding
	ResolvedBinding      *memberadmission.MembershipBinding
}

type SpaceChatMutationReceipt struct {
	OperationID       uuid.UUID
	Revision          uint64
	Allowed           bool
	MembershipBinding *memberadmission.MembershipBinding
	Historical        bool
}

func (s *RoleStore) ReadSpaceChatPermissions(ctx context.Context, space, chat, incarnation, target uuid.UUID, binding memberadmission.MembershipBinding) (SpaceChatPermissionSnapshot, error) {
	var snapshot SpaceChatPermissionSnapshot
	encoded, err := memberadmission.CanonicalMembershipBinding(binding)
	if err != nil || space == uuid.Nil || chat == uuid.Nil || incarnation == uuid.Nil || target == uuid.Nil {
		return snapshot, ErrInvalidSpaceChatMutation
	}
	err = s.WithinSpaces(ctx, []uuid.UUID{space}, func(scoped *RoleStore) error {
		var revision int64
		if err := scoped.db().QueryRow(ctx, `SELECT policy_epoch FROM role_voice_policy_epochs WHERE space_id=$1`, space).Scan(&revision); err != nil {
			return err
		}
		if revision <= 0 {
			return ErrScopeUnavailable
		}
		snapshot.Revision = uint64(revision)
		return scoped.db().QueryRow(ctx, `SELECT
COALESCE((SELECT member_additions_enabled FROM role_space_chat_policies WHERE space_id=$1 AND chat_id=$2 AND chat_scope_incarnation=$3),TRUE),
COALESCE((SELECT allowed FROM role_space_chat_grants WHERE space_id=$1 AND chat_id=$2 AND chat_scope_incarnation=$3 AND target_profile_id=$4 AND membership_binding=$5::jsonb),FALSE)`, space, chat, incarnation, target, string(encoded)).Scan(&snapshot.MemberAdditionsEnabled, &snapshot.ExplicitPostGrant)
	})
	return snapshot, err
}

// CommitSpaceChatMutation is the final LOCAL CAS stage, not an authorization
// endpoint. The owner must finish the bounded remote double collection first.
// authorize must use only the supplied scoped store and perform no remote IO.
func (s *RoleStore) CommitSpaceChatMutation(ctx context.Context, mutation SpaceChatMutation, expectedRevision uint64, authorize func(*RoleStore) error) (SpaceChatMutationReceipt, error) {
	var receipt SpaceChatMutationReceipt
	request, resolved, err := mutation.canonical()
	if err != nil || expectedRevision == 0 || expectedRevision > math.MaxInt64 || authorize == nil {
		return receipt, ErrInvalidSpaceChatMutation
	}
	err = s.withLockedTransaction(ctx, mutation.OperationID, []uuid.UUID{mutation.SpaceID}, func(scoped *RoleStore) error {
		var storedRequest, storedBinding []byte
		var revision int64
		err := scoped.db().QueryRow(ctx, `SELECT request_bytes,resolved_binding,allowed,role_revision FROM role_space_chat_operations WHERE operation_id=$1`, mutation.OperationID).Scan(&storedRequest, &storedBinding, &receipt.Allowed, &revision)
		if err == nil {
			if !bytes.Equal(request, storedRequest) {
				return ErrSpaceChatOperationConflict
			}
			receipt.OperationID, receipt.Revision, receipt.Historical = mutation.OperationID, uint64(revision), true
			if len(storedBinding) != 0 {
				binding, err := memberadmission.ParseMembershipBinding(storedBinding)
				if err != nil {
					return err
				}
				receipt.MembershipBinding = &binding
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return scoped.WithinSpaces(ctx, []uuid.UUID{mutation.SpaceID}, func(guarded *RoleStore) error {
			if err := guarded.db().QueryRow(ctx, `SELECT policy_epoch FROM role_voice_policy_epochs WHERE space_id=$1`, mutation.SpaceID).Scan(&revision); err != nil {
				return err
			}
			if revision <= 0 || uint64(revision) != expectedRevision {
				return ErrSpaceChatAuthorityChanged
			}
			if err := authorize(guarded); err != nil {
				return err
			}
			if mutation.Kind == "post_as_chat" {
				_, err = guarded.db().Exec(ctx, `INSERT INTO role_space_chat_grants(space_id,chat_id,chat_scope_incarnation,target_profile_id,membership_binding,allowed,operation_id,actor_profile_id)
VALUES($1,$2,$3,$4,$5::jsonb,$6,$7,$8)
ON CONFLICT(space_id,chat_id,chat_scope_incarnation,target_profile_id,membership_binding)
DO UPDATE SET allowed=EXCLUDED.allowed,operation_id=EXCLUDED.operation_id,actor_profile_id=EXCLUDED.actor_profile_id,updated_at=clock_timestamp()`, mutation.SpaceID, mutation.ChatID, mutation.ChatScopeIncarnation, mutation.TargetProfileID, string(resolved), mutation.Allowed, mutation.OperationID, mutation.ActorProfileID)
			} else {
				_, err = guarded.db().Exec(ctx, `INSERT INTO role_space_chat_policies(space_id,chat_id,chat_scope_incarnation,member_additions_enabled,operation_id,actor_profile_id)
VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(space_id,chat_id,chat_scope_incarnation)
DO UPDATE SET member_additions_enabled=EXCLUDED.member_additions_enabled,operation_id=EXCLUDED.operation_id,actor_profile_id=EXCLUDED.actor_profile_id,updated_at=clock_timestamp()`, mutation.SpaceID, mutation.ChatID, mutation.ChatScopeIncarnation, mutation.Allowed, mutation.OperationID, mutation.ActorProfileID)
			}
			if err != nil {
				return err
			}
			if err = guarded.db().QueryRow(ctx, `SELECT policy_epoch FROM role_voice_policy_epochs WHERE space_id=$1`, mutation.SpaceID).Scan(&revision); err != nil {
				return err
			}
			var bindingValue any
			if len(resolved) != 0 {
				bindingValue = string(resolved)
			}
			_, err = guarded.db().Exec(ctx, `INSERT INTO role_space_chat_operations(operation_id,space_id,request_bytes,resolved_binding,allowed,role_revision) VALUES($1,$2,$3,$4::jsonb,$5,$6)`, mutation.OperationID, mutation.SpaceID, request, bindingValue, mutation.Allowed, revision)
			if err != nil {
				return err
			}
			_, err = guarded.db().Exec(ctx, `INSERT INTO role_space_chat_outbox(operation_id,space_id,role_revision) VALUES($1,$2,$3)`, mutation.OperationID, mutation.SpaceID, revision)
			if err != nil {
				return err
			}
			receipt = SpaceChatMutationReceipt{OperationID: mutation.OperationID, Revision: uint64(revision), Allowed: mutation.Allowed}
			if len(resolved) != 0 {
				binding, err := memberadmission.ParseMembershipBinding(resolved)
				if err != nil {
					return err
				}
				receipt.MembershipBinding = &binding
			}
			return nil
		})
	})
	if err != nil {
		return SpaceChatMutationReceipt{}, err
	}
	return receipt, nil
}

func (m SpaceChatMutation) canonical() ([]byte, []byte, error) {
	if m.OperationID == uuid.Nil || m.ActorProfileID == uuid.Nil || m.SpaceID == uuid.Nil || m.ChatID == uuid.Nil || m.ChatScopeIncarnation == uuid.Nil {
		return nil, nil, ErrInvalidSpaceChatMutation
	}
	var resolved []byte
	if m.Kind == "post_as_chat" {
		if m.TargetProfileID == uuid.Nil || m.ResolvedBinding == nil {
			return nil, nil, ErrInvalidSpaceChatMutation
		}
		var err error
		resolved, err = memberadmission.CanonicalMembershipBinding(*m.ResolvedBinding)
		if err != nil {
			return nil, nil, ErrInvalidSpaceChatMutation
		}
		if m.RequestedBinding != nil {
			requested, err := memberadmission.CanonicalMembershipBinding(*m.RequestedBinding)
			if err != nil || !bytes.Equal(requested, resolved) {
				return nil, nil, ErrInvalidSpaceChatMutation
			}
		}
	} else if m.Kind != "add_members" || m.TargetProfileID != uuid.Nil || m.RequestedBinding != nil || m.ResolvedBinding != nil {
		return nil, nil, ErrInvalidSpaceChatMutation
	}
	// Exclude server resolution from immutable REQUEST identity. Exact retries
	// with an omitted selector retain the originally saved source and outcome.
	m.ResolvedBinding = nil
	request, err := json.Marshal(m)
	return request, resolved, err
}
