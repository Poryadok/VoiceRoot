package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"math"
	"time"
	commonv1 "voice.app/voice/common/v1"
)

var ErrMessageSendDenied = errors.New("current Bot message send authority denied")

// NewMessageAuthorityStore reserves independent connections for callback reads.
// Producer transactions may occupy every connection in the ordinary pool while
// waiting for Messaging to call back. The owner must close the returned pool.
func NewMessageAuthorityStore(ctx context.Context, producer *pgxpool.Pool) (*BotStore, error) {
	if producer == nil {
		return nil, ErrMembershipSource
	}
	cfg := producer.Config().Copy()
	cfg.MaxConns = 2
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &BotStore{Pool: pool}, nil
}

func ValidateMessageSendBinding(b *commonv1.BotMessageSendBinding) error {
	if b == nil || b.ProtocolVersion != 1 || b.InstallationGeneration == 0 || b.InstallationGeneration > math.MaxInt64 || len(b.ProtoReflect().GetUnknown()) != 0 {
		return ErrMessageSendDenied
	}
	for _, raw := range []string{b.BotId, b.ActorProfileId, b.SpaceId, b.InstallationId, b.InstallOperationId, b.ChatId} {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || id.String() != raw {
			return ErrMessageSendDenied
		}
	}
	return nil
}

// One MVCC statement: callbacks never acquire producer guards or row locks.
const currentMessageSendSQL = `SELECT h.installation_id,h.actor_profile_id,h.generation,h.add_operation_id
FROM bot_space_membership_heads h
JOIN bot_space_installations i ON i.id=h.installation_id AND i.bot_id=h.bot_id AND i.space_id=h.space_id
JOIN bots b ON b.id=h.bot_id AND b.actor_profile_id=h.actor_profile_id
JOIN bot_chat_whitelist w ON w.bot_id=h.bot_id AND w.space_id=h.space_id AND w.chat_id=$3
JOIN bot_space_membership_completions c ON c.add_operation_id=h.add_operation_id AND c.completion_revision=h.completion_revision
LEFT JOIN bot_space_lifecycle_heads l ON l.space_id=h.space_id
WHERE h.bot_id=$1 AND h.space_id=$2 AND h.desired_state='INSTALL' AND h.phase='ACTIVE'
AND c.completed AND b.status='live' AND b.scopes @> '["TEXT_CHAT_SEND_MESSAGES"]'::jsonb
AND w.enabled AND (l.space_id IS NULL OR l.desired_state='LIVE')`

type messageSendQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func currentMessageSendBinding(ctx context.Context, q messageSendQuery, bot, space, chat uuid.UUID) (*commonv1.BotMessageSendBinding, error) {
	var installation, actor, operation uuid.UUID
	var generation uint64
	err := q.QueryRow(ctx, currentMessageSendSQL, bot, space, chat).Scan(&installation, &actor, &generation, &operation)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMessageSendDenied
	}
	if err != nil {
		return nil, err
	}
	b := &commonv1.BotMessageSendBinding{ProtocolVersion: 1, BotId: bot.String(), SpaceId: space.String(), ChatId: chat.String(), ActorProfileId: actor.String(), InstallationId: installation.String(), InstallOperationId: operation.String(), InstallationGeneration: generation}
	return b, ValidateMessageSendBinding(b)
}

func (s *BotStore) CheckCurrentMessageSendAuthority(ctx context.Context, b *commonv1.BotMessageSendBinding) (bool, error) {
	if err := ValidateMessageSendBinding(b); err != nil {
		return false, err
	}
	if s == nil || s.Pool == nil {
		return false, ErrMembershipSource
	}
	current, err := currentMessageSendBinding(ctx, s.Pool, uuid.MustParse(b.BotId), uuid.MustParse(b.SpaceId), uuid.MustParse(b.ChatId))
	if errors.Is(err, ErrMessageSendDenied) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return proto.Equal(current, b), nil
}

// WithMessageSend holds the existing revocation guards across the actual remote
// attempt. The caller must not publish outside this callback or retain authority.
func (s *BotStore) WithMessageSend(ctx context.Context, bot, actor, chat uuid.UUID, send func(context.Context, *commonv1.BotMessageSendBinding) error) error {
	if s == nil || s.Pool == nil || send == nil {
		return ErrMembershipSource
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), time.Second)
		defer done()
		_ = tx.Rollback(cleanup)
	}()
	space, err := messageWhitelistSpace(ctx, tx, bot, chat)
	if err != nil {
		return err
	}
	if space == uuid.Nil {
		return ErrMessageSendDenied
	}
	if err = lockMessageSpaces(ctx, tx, []uuid.UUID{space}); err != nil {
		return err
	}
	var lockedActor uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT actor_profile_id FROM bots WHERE id=$1 FOR SHARE`, bot).Scan(&lockedActor); err != nil {
		return err
	}
	if lockedActor != actor {
		return ErrMessageSendDenied
	}
	enabled, err := lockMessageWhitelist(ctx, tx, bot, chat, space)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrMessageSendDenied
	}
	binding, err := currentMessageSendBinding(ctx, tx, bot, space, chat)
	if err != nil {
		return err
	}
	if err = send(ctx, binding); err != nil {
		return err
	}
	return ctx.Err()
}
