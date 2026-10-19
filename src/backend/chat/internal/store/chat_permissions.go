package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrChatPermissionScope = errors.New("ordinary permissions require a standalone human-owned chat")

type ChatPermissions struct {
	CanPostAsChat          bool
	CanAddMembers          bool
	CanManagePermissions   bool
	MemberAdditionsEnabled bool
	ExplicitPostAsChat     bool
}

func standalonePermissions(role string, additions, explicit bool) ChatPermissions {
	p := ChatPermissions{MemberAdditionsEnabled: additions}
	if role != "owner" && role != "admin" && role != "member" {
		return p
	}
	p.ExplicitPostAsChat = explicit
	p.CanManagePermissions = role == "owner" || role == "admin"
	p.CanPostAsChat = p.CanManagePermissions || explicit
	p.CanAddMembers = p.CanManagePermissions || additions
	return p
}

// GetChatPermissions reads membership and settings from one PostgreSQL snapshot.
func (s *DMStore) GetChatPermissions(ctx context.Context, chatID, actorID uuid.UUID) (ChatPermissions, error) {
	if s == nil || s.Pool == nil {
		return ChatPermissions{}, errors.New("dm store: pool not configured")
	}
	var role, kind string
	var spaceID *uuid.UUID
	var managed, squad, additions, explicit bool
	err := s.Pool.QueryRow(ctx, `
 SELECT c.type,c.space_id,c.managed_by_application_id IS NOT NULL,
 EXISTS(SELECT 1 FROM chat_match_squad_operations q WHERE q.chat_id=c.id),
 c.member_additions_enabled,COALESCE(m.role,''),COALESCE(m.post_as_chat,false)
 FROM chats c LEFT JOIN chat_members m ON m.chat_id=c.id AND m.profile_id=$2
 WHERE c.id=$1`, chatID, actorID).Scan(&kind, &spaceID, &managed, &squad, &additions, &role, &explicit)
	if err != nil {
		return ChatPermissions{}, err
	}
	if (kind != "group" && kind != "channel") || spaceID != nil || managed || squad {
		return ChatPermissions{}, ErrChatPermissionScope
	}
	if role != "owner" && role != "admin" && role != "member" {
		return ChatPermissions{}, ErrNotGroupMember
	}
	return standalonePermissions(role, additions, explicit), nil
}

// lockOrdinaryChat is also used by mutations, always before membership locks.
func lockOrdinaryChat(ctx context.Context, tx pgx.Tx, chatID uuid.UUID) error {
	var kind string
	var spaceID *uuid.UUID
	var managed, squad bool
	err := tx.QueryRow(ctx, `
 SELECT type,space_id,managed_by_application_id IS NOT NULL,
 EXISTS(SELECT 1 FROM chat_match_squad_operations q WHERE q.chat_id=chats.id)
 FROM chats WHERE id=$1 FOR UPDATE`, chatID).Scan(&kind, &spaceID, &managed, &squad)
	if err != nil {
		return err
	}
	if (kind != "group" && kind != "channel") || spaceID != nil || managed || squad {
		return ErrChatPermissionScope
	}
	return nil
}

func (s *DMStore) SetChatMemberPostAsChat(ctx context.Context, chatID, actorID, targetID uuid.UUID, allowed bool) error {
	if s == nil || s.Pool == nil {
		return errors.New("dm store: pool not configured")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockOrdinaryChat(ctx, tx, chatID); err != nil {
		return err
	}
	roles, err := lockGroupMemberRoles(ctx, tx, chatID, actorID, targetID)
	if err != nil {
		return err
	}
	if roles[actorID] != "owner" && roles[actorID] != "admin" {
		return ErrRoleChangeForbidden
	}
	targetRole, ok := roles[targetID]
	if !ok {
		return pgx.ErrNoRows
	}
	if targetRole != "member" {
		return ErrRoleChangeInvalid
	}
	_, err = tx.Exec(ctx, "UPDATE chat_members SET post_as_chat=$3 WHERE chat_id=$1 AND profile_id=$2", chatID, targetID, allowed)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
