package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const DefaultPresenceTTL = 90 * time.Second

const DailyChatCreateLimit = 10

var ErrBotChatCreateRequestConflict = errors.New("bot chat create request id already used with different request")

var botChatRequestHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// TouchPresence records bot liveness (heartbeat / poll / webhook activity).
func (s *BotStore) TouchPresence(ctx context.Context, botID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
INSERT INTO bot_presence (bot_id, last_seen_at) VALUES ($1, now())
ON CONFLICT (bot_id) DO UPDATE SET last_seen_at = now()`, botID)
	return err
}

// IsBotOnline reports whether the bot was seen within ttl.
func (s *BotStore) IsBotOnline(ctx context.Context, botID uuid.UUID, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = DefaultPresenceTTL
	}
	var last time.Time
	err := s.Pool.QueryRow(ctx, `SELECT last_seen_at FROM bot_presence WHERE bot_id = $1`, botID).Scan(&last)
	if err != nil {
		return false, nil
	}
	return time.Since(last) <= ttl, nil
}

// DailyChatCreateCount returns today's create count for the bot (0 if none).
func (s *BotStore) DailyChatCreateCount(ctx context.Context, botID uuid.UUID) (int, error) {
	var count int
	err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(count, 0) FROM bot_daily_chat_creates
WHERE bot_id = $1 AND day = CURRENT_DATE`, botID).Scan(&count)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

// ReserveDailyChatCreate atomically claims one of the bot's daily chat-create slots.
// The returned day identifies the reservation for safe release across a date boundary.
func (s *BotStore) ReserveDailyChatCreate(ctx context.Context, botID uuid.UUID) (day time.Time, reserved bool, err error) {
	err = s.Pool.QueryRow(ctx, `
INSERT INTO bot_daily_chat_creates (bot_id, day, count) VALUES ($1, CURRENT_DATE, 1)
ON CONFLICT (bot_id, day) DO UPDATE
SET count = bot_daily_chat_creates.count + 1
WHERE bot_daily_chat_creates.count < $2
RETURNING day`, botID, DailyChatCreateLimit).Scan(&day)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return day, true, nil
}

// ReserveDailyChatCreateForRequest reserves a slot once for a stable request ID.
// A retry with the same hash is admitted to continue Chat-side reconciliation but
// does not increment the quota again. A changed request for the key is rejected.
func (s *BotStore) ReserveDailyChatCreateForRequest(ctx context.Context, botID, requestID uuid.UUID, requestHash string) (day time.Time, admitted, replayed bool, err error) {
	if botID == uuid.Nil || requestID == uuid.Nil || !botChatRequestHashPattern.MatchString(requestHash) {
		return time.Time{}, false, false, errors.New("invalid bot chat create request identity")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return time.Time{}, false, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	created := false
	replayed = false
	for retry := 0; retry < 4; retry++ {
		err = tx.QueryRow(ctx, `
		INSERT INTO bot_chat_create_requests (bot_id, request_id, request_hash, day, active_attempts)
VALUES ($1, $2, $3, CURRENT_DATE, 1)
ON CONFLICT (bot_id, request_id) DO NOTHING
RETURNING day
`, botID, requestID, requestHash).Scan(&day)
		if err == nil {
			created = true
			break
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, false, err
		}
		var previousHash string
		var activeAttempts int
		var retainReservation, quotaReserved bool
		err = tx.QueryRow(ctx, `SELECT day, request_hash, active_attempts, retain_reservation, quota_reserved FROM bot_chat_create_requests WHERE bot_id=$1 AND request_id=$2 FOR UPDATE`, botID, requestID).Scan(&day, &previousHash, &activeAttempts, &retainReservation, &quotaReserved)
		if errors.Is(err, pgx.ErrNoRows) {
			continue // The prior row disappeared between conflict detection and its locked read.
		}
		if err != nil {
			return time.Time{}, false, false, err
		}
		if previousHash != requestHash {
			return time.Time{}, false, false, ErrBotChatCreateRequestConflict
		}
		if quotaReserved {
			if _, err := tx.Exec(ctx, `UPDATE bot_chat_create_requests SET active_attempts=active_attempts+1 WHERE bot_id=$1 AND request_id=$2`, botID, requestID); err != nil {
				return time.Time{}, false, false, err
			}
			replayed = true
			break
		}
		if activeAttempts != 0 || retainReservation {
			return time.Time{}, false, false, errors.New("released bot chat create request has inconsistent reservation state")
		}
		var reservedDay time.Time
		err = tx.QueryRow(ctx, `
INSERT INTO bot_daily_chat_creates (bot_id, day, count) VALUES ($1, CURRENT_DATE, 1)
ON CONFLICT (bot_id, day) DO UPDATE
SET count = bot_daily_chat_creates.count + 1
WHERE bot_daily_chat_creates.count < $2
RETURNING day
`, botID, DailyChatCreateLimit).Scan(&reservedDay)
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, false, nil
		}
		if err != nil {
			return time.Time{}, false, false, err
		}
		if _, err := tx.Exec(ctx, `UPDATE bot_chat_create_requests SET day=$3, active_attempts=1, quota_reserved=true WHERE bot_id=$1 AND request_id=$2`, botID, requestID, reservedDay); err != nil {
			return time.Time{}, false, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return time.Time{}, false, false, err
		}
		return reservedDay, true, true, nil
	}
	if !created && !replayed {
		return time.Time{}, false, false, errors.New("could not register bot chat create attempt")
	}
	if replayed {
		if err := tx.Commit(ctx); err != nil {
			return time.Time{}, false, false, err
		}
		return day, false, true, nil
	}
	var reservedDay time.Time
	err = tx.QueryRow(ctx, `
INSERT INTO bot_daily_chat_creates (bot_id, day, count) VALUES ($1, CURRENT_DATE, 1)
ON CONFLICT (bot_id, day) DO UPDATE
SET count = bot_daily_chat_creates.count + 1
WHERE bot_daily_chat_creates.count < $2
RETURNING day
`, botID, DailyChatCreateLimit).Scan(&reservedDay)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, false, nil
	}
	if err != nil {
		return time.Time{}, false, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, false, false, err
	}
	return reservedDay, true, false, nil
}

type BotChatCreateAttemptOutcome int

const (
	BotChatCreateAttemptRejected BotChatCreateAttemptOutcome = iota
	BotChatCreateAttemptSucceeded
	BotChatCreateAttemptUncertain
)

// FinishDailyChatCreateRequestAttempt records one external attempt result.
// A definite rejection releases the slot only after every concurrently
// admitted attempt also definitively rejects. Any uncertain result is sticky:
// later rejections must not release a slot that may correspond to a committed
// Chat create. The request row and quota decrement commit atomically.
func (s *BotStore) FinishDailyChatCreateRequestAttempt(ctx context.Context, botID, requestID uuid.UUID, outcome BotChatCreateAttemptOutcome) (bool, error) {
	if outcome != BotChatCreateAttemptRejected && outcome != BotChatCreateAttemptSucceeded && outcome != BotChatCreateAttemptUncertain {
		return false, errors.New("invalid bot chat create attempt outcome")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var day time.Time
	var active int
	var retainReservation bool
	err = tx.QueryRow(ctx, `SELECT day, active_attempts, retain_reservation FROM bot_chat_create_requests WHERE bot_id=$1 AND request_id=$2 FOR UPDATE`, botID, requestID).Scan(&day, &active, &retainReservation)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if active <= 0 {
		return false, errors.New("bot chat create request has no active attempt")
	}
	active--
	if outcome == BotChatCreateAttemptUncertain || outcome == BotChatCreateAttemptSucceeded {
		retainReservation = true
	}
	if outcome == BotChatCreateAttemptRejected && active == 0 && !retainReservation {
		if _, err := tx.Exec(ctx, `UPDATE bot_chat_create_requests SET active_attempts=0, quota_reserved=false WHERE bot_id=$1 AND request_id=$2`, botID, requestID); err != nil {
			return false, err
		}
	} else if _, err := tx.Exec(ctx, `UPDATE bot_chat_create_requests SET active_attempts=$3, retain_reservation=$4 WHERE bot_id=$1 AND request_id=$2`, botID, requestID, active, retainReservation); err != nil {
		return false, err
	} else {
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	var count int
	if err := tx.QueryRow(ctx, `UPDATE bot_daily_chat_creates SET count=count-1 WHERE bot_id=$1 AND day=$2::date AND count>0 RETURNING count`, botID, day).Scan(&count); err != nil {
		return false, fmt.Errorf("release bot chat quota reservation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// ReleaseDailyChatCreate returns a reserved slot after an outcome known to have
// been rejected before Chat persistence. Ambiguous outcomes must keep the slot.
func (s *BotStore) ReleaseDailyChatCreate(ctx context.Context, botID uuid.UUID, day time.Time) error {
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_daily_chat_creates
SET count = count - 1
WHERE bot_id = $1 AND day = $2::date AND count > 0`, botID, day)
	return err
}

// IncrementDailyChatCreates returns the new count for today; enforces platform limit externally.
func (s *BotStore) IncrementDailyChatCreates(ctx context.Context, botID uuid.UUID) (int, error) {
	var count int
	err := s.Pool.QueryRow(ctx, `
INSERT INTO bot_daily_chat_creates (bot_id, day, count) VALUES ($1, CURRENT_DATE, 1)
ON CONFLICT (bot_id, day) DO UPDATE SET count = bot_daily_chat_creates.count + 1
RETURNING count`, botID).Scan(&count)
	return count, err
}

// MarkEventDeferred sets delivery_status to deferred for an interaction token.
func (s *BotStore) MarkEventDeferred(ctx context.Context, botID uuid.UUID, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = 'deferred'
WHERE bot_id = $1 AND interaction_token = $2 AND delivery_status = 'pending'`, botID, token)
	return err
}

// AbandonStaleDeferred marks deferred events older than ttl as abandoned.
func (s *BotStore) AbandonStaleDeferred(ctx context.Context, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	cutoff := time.Now().Add(-ttl)
	_, err := s.Pool.Exec(ctx, `
UPDATE bot_event_log SET delivery_status = 'abandoned'
WHERE delivery_status = 'deferred' AND created_at < $1`, cutoff)
	return err
}

// ListDeferredTokens returns interaction tokens awaiting async completion.
func (s *BotStore) ListDeferredTokens(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT interaction_token FROM bot_event_log
WHERE delivery_status = 'deferred' AND interaction_token IS NOT NULL AND interaction_token <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		out = append(out, token)
	}
	return out, rows.Err()
}

const PrivilegedScopeReadHistory = "TEXT_CHAT_READ_HISTORY"
const PrivilegedScopeManageRoles = "SPACE_MANAGE_ROLES"

// HasPrivilegedScope reports whether scopes JSON includes a privileged install scope.
func HasPrivilegedScope(scopesJSON string) bool {
	return ScopeAllows(scopesJSON, PrivilegedScopeReadHistory) ||
		ScopeAllows(scopesJSON, PrivilegedScopeManageRoles)
}
