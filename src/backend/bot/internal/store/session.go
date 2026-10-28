package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrSessionRevoked = errors.New("bot session revoked")
var ErrRotationDenied = errors.New("bot credential rotation denied")

// BotSchemaVersion is the exact migrated schema supported by this binary.
const BotSchemaVersion = 9

// Session is an internal generation binding, never a public bearer credential.
type Session struct {
	BotID uuid.UUID
	Epoch int64
}
type Authority struct {
	Session
	TokenHash string
}

func (b *BotRow) Session() Session { return Session{BotID: b.ID, Epoch: b.SessionEpoch} }
func (b *BotRow) Authority() Authority {
	return Authority{Session: b.Session(), TokenHash: b.TokenHash}
}

// RotateCredential commits credential replacement and capability revocation
// together. Callers must not automatically repeat an ambiguous Commit error.
func (s *BotStore) RotateCredential(ctx context.Context, botID, owner uuid.UUID, webhookSecret bool) (string, int64, error) {
	plain, hash, err := NewToken()
	if err != nil {
		return "", 0, err
	}
	if webhookSecret {
		var bytes [32]byte
		if _, err = rand.Read(bytes[:]); err != nil {
			return "", 0, err
		}
		plain = hex.EncodeToString(bytes[:])
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", 0, err
	}
	defer rollbackSession(tx)
	var currentOwner uuid.UUID
	var state string
	var epoch int64
	err = tx.QueryRow(ctx, `SELECT owner_account_id,status,session_epoch FROM bots WHERE id=$1 FOR UPDATE`, botID).Scan(&currentOwner, &state, &epoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, ErrNotFound
	}
	if err != nil {
		return "", 0, err
	}
	if currentOwner != owner || state != "live" || epoch < 1 || epoch == math.MaxInt64 {
		return "", 0, ErrRotationDenied
	}
	epoch++
	if webhookSecret {
		_, err = tx.Exec(ctx, `UPDATE bots SET webhook_secret=$2,session_epoch=$3,updated_at=clock_timestamp() WHERE id=$1`, botID, plain, epoch)
	} else {
		_, err = tx.Exec(ctx, `UPDATE bots SET token_hash=$2,session_epoch=$3,updated_at=clock_timestamp() WHERE id=$1`, botID, hash, epoch)
	}
	if err != nil {
		return "", 0, err
	}
	if err = cancelSessionEvents(ctx, tx, botID, epoch); err != nil {
		return "", 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", 0, err
	}
	return plain, epoch, nil
}

func cancelSessionEvents(ctx context.Context, tx pgx.Tx, botID uuid.UUID, epoch int64) error {
	_, err := tx.Exec(ctx, `WITH revoked AS (
 SELECT id FROM bot_event_log WHERE bot_id=$1 AND event_type<>'message'
 AND (session_epoch IS NULL OR session_epoch<$2) AND delivery_status IN ('pending','deferred')
 ORDER BY id FOR UPDATE
) UPDATE bot_event_log e SET delivery_status='canceled',payload='{}'::jsonb,claimed_until=NULL
FROM revoked r WHERE e.id=r.id`, botID, epoch)
	return err
}

// RevokeBot preserves DeleteBot idempotence while fencing the old incarnation.
func (s *BotStore) RevokeBot(ctx context.Context, botID, owner uuid.UUID) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer rollbackSession(tx)
	var storedOwner uuid.UUID
	var state string
	var epoch int64
	err = tx.QueryRow(ctx, `SELECT owner_account_id,status,session_epoch FROM bots WHERE id=$1 FOR UPDATE`, botID).Scan(&storedOwner, &state, &epoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if storedOwner != owner || epoch < 1 {
		return 0, ErrRotationDenied
	}
	if state == "disabled" {
		return epoch, nil
	}
	if epoch == math.MaxInt64 {
		return 0, ErrRotationDenied
	}
	epoch++
	if _, err = tx.Exec(ctx, `UPDATE bots SET status='disabled',session_epoch=$2,updated_at=clock_timestamp() WHERE id=$1`, botID, epoch); err != nil {
		return 0, err
	}
	if err = cancelSessionEvents(ctx, tx, botID, epoch); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return epoch, nil
}

func rollbackSession(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

type guardedSessionKey struct{}
type guardedSession struct {
	tx          pgx.Tx
	bot         *BotRow
	chat, space uuid.UUID
	spaces      []uuid.UUID
}

func GuardedChatSpace(ctx context.Context) (uuid.UUID, bool) {
	held, ok := ctx.Value(guardedSessionKey{}).(*guardedSession)
	if !ok || held.chat == uuid.Nil {
		return uuid.Nil, false
	}
	return held.space, true
}

// CheckGuardedSession never rehydrates authority from a pool after guard loss.
func (s *BotStore) CheckGuardedSession(ctx context.Context, session Session) error {
	held, ok := ctx.Value(guardedSessionKey{}).(*guardedSession)
	if !ok || held.bot.Session() != session {
		return ErrSessionRevoked
	}
	var epoch int64
	err := held.tx.QueryRow(ctx, `SELECT session_epoch FROM bots WHERE id=$1 AND status='live'`, session.BotID).Scan(&epoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSessionRevoked
	}
	if err != nil {
		return err
	}
	if epoch != session.Epoch {
		return ErrSessionRevoked
	}
	return nil
}

type sessionQuery interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *BotStore) sessionDB(ctx context.Context) sessionQuery {
	if held, ok := ctx.Value(guardedSessionKey{}).(*guardedSession); ok {
		return held.tx
	}
	return s.Pool
}

func (s *BotStore) beginSessionDB(ctx context.Context) (pgx.Tx, error) {
	if held, ok := ctx.Value(guardedSessionKey{}).(*guardedSession); ok {
		return held.tx.Begin(ctx)
	}
	return s.Pool.Begin(ctx)
}

// WithSession holds the authority cutover guard through the actual synchronous
// callback. Chat discovery is a hint; its binding is reread after ordered locks.
func (s *BotStore) WithSession(ctx context.Context, authority Authority, chat uuid.UUID, effect func(context.Context, pgx.Tx, *BotRow) error) error {
	return s.WithSessionSpaces(ctx, authority, chat, nil, effect)
}

func (s *BotStore) WithSessionSpaces(ctx context.Context, authority Authority, chat uuid.UUID, spaces []uuid.UUID, effect func(context.Context, pgx.Tx, *BotRow) error) error {
	spaces = append([]uuid.UUID(nil), spaces...)
	if authority.BotID == uuid.Nil || authority.Epoch < 1 || effect == nil {
		return ErrSessionRevoked
	}
	if held, ok := ctx.Value(guardedSessionKey{}).(*guardedSession); ok {
		if held.bot.Session() != authority.Session || (authority.TokenHash != "" && held.bot.TokenHash != authority.TokenHash) || (chat != uuid.Nil && held.chat != chat) {
			return ErrSessionRevoked
		}
		for _, space := range spaces {
			found := space == uuid.Nil
			for _, saved := range held.spaces {
				found = found || saved == space
			}
			if !found {
				return ErrMessageBindingChanged
			}
		}
		return effect(ctx, held.tx, held.bot)
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer rollbackSession(tx)
	space := uuid.Nil
	if chat != uuid.Nil {
		space, err = messageWhitelistSpace(ctx, tx, authority.BotID, chat)
		if err != nil {
			return err
		}
		spaces = append(append([]uuid.UUID{}, spaces...), space)
	}
	if err = lockMessageSpaces(ctx, tx, spaces); err != nil {
		return err
	}
	bot, err := scanBot(tx.QueryRow(ctx, `SELECT id,owner_account_id,name,description,avatar_url,token_hash,webhook_url,webhook_secret,
is_polling_mode,scopes::text,status,actor_profile_id,slug,created_at,updated_at,session_epoch
FROM bots WHERE id=$1 FOR SHARE`, authority.BotID))
	if errors.Is(err, ErrNotFound) {
		return ErrSessionRevoked
	}
	if err != nil {
		return err
	}
	if bot.Status != "live" || bot.Session() != authority.Session || (authority.TokenHash != "" && authority.TokenHash != bot.TokenHash) {
		return ErrSessionRevoked
	}
	for _, space := range spaces {
		if space == uuid.Nil {
			continue
		}
		live, e := messageSpaceLive(ctx, tx, space)
		if e != nil {
			return e
		}
		if !live {
			return ErrMessageSendDenied
		}
	}
	if chat != uuid.Nil {
		if space == uuid.Nil {
			return ErrMessageSendDenied
		}
		enabled, e := lockMessageWhitelist(ctx, tx, bot.ID, chat, space)
		if e != nil {
			return e
		}
		if !enabled {
			return ErrMessageSendDenied
		}
	}
	call := context.WithValue(ctx, guardedSessionKey{}, &guardedSession{tx: tx, bot: bot, chat: chat, space: space, spaces: spaces})
	if err = effect(call, tx, bot); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
