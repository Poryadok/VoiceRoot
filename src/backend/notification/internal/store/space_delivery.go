package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type notificationDeliveryTxKey struct{}
type notificationDeliveryTx struct {
	store *SettingsStore
	tx    pgx.Tx
}

func (s *SettingsStore) scopeTransaction(ctx context.Context, scopeType string, scopeID *uuid.UUID) (pgx.Tx, bool, error) {
	if scopeID == nil || (scopeType != "space" && scopeType != "chat" && scopeType != "channel") {
		return nil, false, nil
	}
	if s == nil || s.Pool == nil {
		return nil, false, ErrNotImplemented
	}
	shared, reused := ctx.Value(notificationDeliveryTxKey{}).(notificationDeliveryTx)
	var tx pgx.Tx
	var err error
	if reused && shared.store == s {
		tx = shared.tx
	} else {
		reused = false
		tx, err = s.Pool.Begin(ctx)
		if err != nil {
			return nil, false, err
		}
	}
	fail := func(err error) (pgx.Tx, bool, error) {
		if !reused {
			_ = tx.Rollback(context.Background())
		}
		return nil, false, err
	}
	if err = notificationLifecycleLock(ctx, tx, *scopeID); err != nil {
		return fail(err)
	}
	spaceID := *scopeID
	if scopeType != "space" {
		var blocked bool
		err = tx.QueryRow(ctx, `SELECT space_id,blocked FROM notification_space_chat_fences WHERE chat_id=$1`, *scopeID).Scan(&spaceID, &blocked)
		if errors.Is(err, pgx.ErrNoRows) {
			return tx, !reused, nil
		}
		if err != nil {
			return fail(err)
		}
		if blocked {
			return fail(ErrSpaceLifecycleNotLive)
		}
		if err = notificationLifecycleLock(ctx, tx, spaceID); err != nil {
			return fail(err)
		}
	}
	var state string
	var generation uint64
	err = tx.QueryRow(ctx, `SELECT state,generation FROM notification_space_lifecycle_fences WHERE space_id=$1 FOR SHARE`, spaceID).Scan(&state, &generation)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fail(err)
	}
	if err == nil && state != "LIVE" || errors.Is(err, pgx.ErrNoRows) && scopeType != "space" {
		return fail(ErrSpaceLifecycleNotLive)
	}
	var importing bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM notification_space_chat_manifests WHERE space_id=$1 AND schedule_generation>$2 AND retain_until IS NULL)`, spaceID, generation).Scan(&importing); err != nil {
		return fail(err)
	}
	if importing {
		return fail(ErrSpaceLifecycleNotLive)
	}
	return tx, !reused, nil
}

// WithChatDelivery drains admitted external dispatches before import/fence
// acknowledgement. Frozen/purged queued work is a suppressed no-op.
func (s *SettingsStore) WithChatDelivery(ctx context.Context, chatID string, dispatch func(context.Context) error) error {
	if chatID == "" {
		return dispatch(ctx)
	}
	id, err := notificationLifecycleUUID(chatID)
	if err != nil {
		return err
	}
	return s.withScopeDelivery(ctx, "chat", id, dispatch)
}

func (s *SettingsStore) WithSpaceDelivery(ctx context.Context, spaceID string, dispatch func(context.Context) error) error {
	id, err := notificationLifecycleUUID(spaceID)
	if err != nil {
		return err
	}
	return s.withScopeDelivery(ctx, "space", id, dispatch)
}

func (s *SettingsStore) withScopeDelivery(ctx context.Context, scope string, id uuid.UUID, dispatch func(context.Context) error) error {
	tx, owned, err := s.scopeTransaction(ctx, scope, &id)
	if errors.Is(err, ErrSpaceLifecycleNotLive) {
		return nil
	}
	if err != nil {
		return err
	}
	if !owned {
		return dispatch(ctx)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = dispatch(context.WithValue(ctx, notificationDeliveryTxKey{}, notificationDeliveryTx{store: s, tx: tx})); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
