package ordinarycall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"voice/backend/voice/internal/admissionlock"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUnavailable = errors.New("ordinary call authority unavailable")
	ErrNotFound    = errors.New("ordinary call not found")
	ErrStaleLease  = errors.New("ordinary call stale lease")
)

type EncodedEvent struct {
	ID      uuid.UUID
	Subject string
	Payload []byte
}

type EventEncoder func(*Room, Event) (EncodedEvent, error)

type OutboxItem struct {
	EncodedEvent
	RoomID     uuid.UUID
	Ordinal    int64
	LeaseToken uuid.UUID
}

type RecoveryItem struct {
	RoomID     uuid.UUID
	LeaseToken uuid.UUID
	Attempts   int64
}

// ClaimRecovery leases scheduling, not SDK authority. Every completion still
// checks the native operation/generation/effect revision under the room lock.
func (s *PostgresStore) ClaimRecovery(ctx context.Context, leaseFor time.Duration) (*RecoveryItem, error) {
	if leaseFor < time.Millisecond || leaseFor > time.Minute {
		return nil, ErrInvalidIntent
	}
	var item RecoveryItem
	err := s.pool.QueryRow(ctx, `WITH candidate AS (
SELECT room_id FROM voice_ordinary_rooms WHERE next_attempt_at<=clock_timestamp()
AND (lease_until IS NULL OR lease_until<=clock_timestamp())
ORDER BY next_attempt_at,room_id FOR UPDATE SKIP LOCKED LIMIT 1)
UPDATE voice_ordinary_rooms r SET lease_token=$1,lease_until=clock_timestamp()+$2::bigint*interval '1 millisecond',
recovery_attempts=recovery_attempts+1 FROM candidate c WHERE r.room_id=c.room_id
RETURNING r.room_id,r.lease_token,r.recovery_attempts`, uuid.New(), leaseFor.Milliseconds()).Scan(&item.RoomID, &item.LeaseToken, &item.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *PostgresStore) ReleaseRecovery(ctx context.Context, item RecoveryItem, retryAfter time.Duration, success bool) error {
	if retryAfter < time.Millisecond || retryAfter > time.Minute {
		return ErrInvalidIntent
	}
	tag, err := s.pool.Exec(ctx, `UPDATE voice_ordinary_rooms SET lease_token=NULL,lease_until=NULL,
next_attempt_at=clock_timestamp()+$3::bigint*interval '1 millisecond',
recovery_attempts=CASE WHEN $4 THEN 0 ELSE recovery_attempts END
WHERE room_id=$1 AND lease_token=$2 AND lease_until>clock_timestamp()`, item.RoomID, item.LeaseToken, retryAfter.Milliseconds(), success)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

type PostgresStore struct {
	pool   *pgxpool.Pool
	encode EventEncoder
}

func NewPostgresStore(pool *pgxpool.Pool, encode EventEncoder) *PostgresStore {
	return &PostgresStore{pool: pool, encode: encode}
}

func (s *PostgresStore) CheckSchema(ctx context.Context) error {
	if s == nil || s.pool == nil || s.encode == nil {
		return ErrUnavailable
	}
	var ready bool
	err := s.pool.QueryRow(ctx, `SELECT to_regclass('voice_ordinary_rooms') IS NOT NULL
AND to_regclass('voice_ordinary_outbox') IS NOT NULL
AND to_regclass('voice_ordinary_memberships') IS NOT NULL
AND EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema()
AND table_name='voice_sdk_conversion_fence_receipts' AND column_name='ordinary_source_snapshot')
AND EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema()
AND table_name='voice_ordinary_memberships' AND column_name='session_epoch')
AND EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema()
AND table_name='voice_ordinary_rooms' AND column_name='roster_version')
AND to_regclass('voice_account_voice_fences') IS NOT NULL
AND EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema()
AND table_name='voice_account_voice_fences' AND column_name='admission_generation')`).Scan(&ready)
	if err != nil {
		return fmt.Errorf("ordinary schema: %w", err)
	}
	if !ready {
		return ErrUnavailable
	}
	return nil
}

func (s *PostgresStore) Create(ctx context.Context, room *Room) error {
	if s == nil || s.pool == nil || s.encode == nil || room == nil || room.Stage != StagePreparing ||
		len(room.Events) != 0 {
		return ErrInvalidIntent
	}
	if err := validateRoom(room); err != nil {
		return err
	}
	body, err := json.Marshal(room)
	if err != nil {
		return err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer rollback(tx)
	_, err = tx.Exec(ctx, `INSERT INTO voice_ordinary_rooms(room_id,operation_id,chat_id,is_group,state,version,document,next_attempt_at)
VALUES($1,$2,$3,$4,$5,$6,$7,clock_timestamp()+interval '30 seconds')`,
		room.Intent.RoomID, room.Intent.OperationID, room.Intent.ChatID, room.Intent.Group, string(room.Stage), room.Version, body)
	if err != nil {
		return mapStoreError(err)
	}
	if err := lockAdmissions(ctx, tx, nil, room); err != nil {
		return err
	}
	if err := syncFences(ctx, tx, nil, room); err != nil {
		return err
	}
	if err := syncMemberships(ctx, tx, room); err != nil {
		return err
	}
	// A failed COMMIT acknowledgement is not a rollback receipt. The caller
	// retains operation/room IDs and resolves through readback/recovery.
	return tx.Commit(ctx)
}

func (s *PostgresStore) Load(ctx context.Context, roomID uuid.UUID) (*Room, error) {
	if s == nil || s.pool == nil || roomID == uuid.Nil {
		return nil, ErrUnavailable
	}
	var body []byte
	var observedAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT document,clock_timestamp() FROM voice_ordinary_rooms WHERE room_id=$1`, roomID).Scan(&body, &observedAt)
	if err != nil {
		return nil, mapStoreError(err)
	}
	room, err := decodeRoom(body, roomID)
	if err == nil {
		room.TransactionTime = observedAt
	}
	return room, err
}

// Mutate locks the room, then the complete sorted identity union, then fences. The callback
// must perform no IO; room/fence/outbox state commits in the same transaction.
func (s *PostgresStore) Mutate(ctx context.Context, roomID uuid.UUID, change func(*Room) error) (*Room, error) {
	if s == nil || s.pool == nil || s.encode == nil || change == nil {
		return nil, ErrUnavailable
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollback(tx)
	var body []byte
	if err := tx.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1 FOR UPDATE`, roomID).Scan(&body); err != nil {
		return nil, mapStoreError(err)
	}
	before, err := decodeRoom(body, roomID)
	if err != nil {
		return nil, err
	}
	room, err := decodeRoom(body, roomID)
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&room.TransactionTime); err != nil {
		return nil, err
	}
	if err := change(room); err != nil {
		return nil, err
	}
	if room.Intent.RoomID != before.Intent.RoomID || room.Intent.OperationID != before.Intent.OperationID ||
		room.Intent.ChatID != before.Intent.ChatID || room.Intent.Group != before.Intent.Group ||
		!reflect.DeepEqual(room.Intent, before.Intent) ||
		room.LiveKitRoomName != before.LiveKitRoomName || room.Version < before.Version || room.RosterVersion < before.RosterVersion || len(room.Events) < len(before.Events) {
		return nil, ErrInvalidIntent
	}
	for index := range before.Events {
		if !reflect.DeepEqual(room.Events[index], before.Events[index]) {
			return nil, ErrInvalidIntent
		}
	}
	if err := lockAdmissions(ctx, tx, before, room); err != nil {
		return nil, err
	}
	// Identity-lock waits can cross the ring deadline even when the request's
	// earlier clock sample passed it. The final transaction must check again
	// with the database clock after the barrier, never accept on that old sample.
	var decisionAt time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&decisionAt); err != nil {
		return nil, err
	}
	if before.Stage == StageRinging && room.Stage == StageActive && !decisionAt.Before(before.ExpiresAt) {
		return nil, ErrExpired
	}
	if before.Stage == StagePreparing && (room.Stage == StageRinging || room.Stage == StageActive) {
		room.ReadyAt = decisionAt
		if room.Stage == StageRinging {
			room.ExpiresAt = decisionAt.Add(30 * time.Second)
		}
	}
	room.TransactionTime = decisionAt
	for index := len(before.Events); index < len(room.Events); index++ {
		room.Events[index].OccurredAt = decisionAt
	}
	for id, admission := range room.Admissions {
		if _, existed := before.Admissions[id]; !existed {
			admission.CreatedAt = decisionAt
			room.Admissions[id] = admission
		}
	}
	for profile, member := range room.Members {
		prior := before.Members[profile]
		if member.State == MemberJoined && (prior.State != MemberJoined || prior.Generation != member.Generation) {
			member.JoinedAt = decisionAt
		}
		if !member.LeftAt.IsZero() && prior.LeftAt.IsZero() {
			member.LeftAt = decisionAt
		}
		room.Members[profile] = member
	}
	if !room.EndedAt.IsZero() && before.EndedAt.IsZero() {
		room.EndedAt = decisionAt
	}
	if err := validateRoom(room); err != nil {
		return nil, err
	}
	if err := syncFences(ctx, tx, before, room); err != nil {
		return nil, err
	}
	if err := syncMemberships(ctx, tx, room); err != nil {
		return nil, err
	}
	body, err = json.Marshal(room)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE voice_ordinary_rooms SET state=$2,version=$3,document=$4,roster_version=$5,
next_attempt_at=CASE WHEN $2='PREPARING' THEN clock_timestamp()+interval '30 seconds' ELSE clock_timestamp() END,
updated_at=clock_timestamp() WHERE room_id=$1`, roomID, string(room.Stage), room.Version, body, room.RosterVersion)
	if err != nil {
		return nil, err
	}
	for index := len(before.Events); index < len(room.Events); index++ {
		event, err := s.encode(room, room.Events[index])
		if err != nil {
			return nil, err
		}
		if event.ID != room.Events[index].ID || event.ID == uuid.Nil || len(event.Payload) == 0 {
			return nil, ErrInvalidIntent
		}
		var rosterVersion any
		if room.Events[index].RosterVersion > 0 {
			rosterVersion = room.Events[index].RosterVersion
		}
		_, err = tx.Exec(ctx, `INSERT INTO voice_ordinary_outbox(event_id,room_id,room_version,ordinal,subject,payload,roster_version)
VALUES($1,$2,$3,$4,$5,$6,$7)`, event.ID, roomID, room.Version, index+1, event.Subject, event.Payload, rosterVersion)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return room, nil
}

func decodeRoom(body []byte, roomID uuid.UUID) (*Room, error) {
	var room Room
	if err := json.Unmarshal(body, &room); err != nil || room.Intent.RoomID != roomID || room.Version == 0 ||
		room.Members == nil || room.Admissions == nil {
		return nil, ErrInvalidIntent
	}
	if err := validateRoom(&room); err != nil {
		return nil, err
	}
	return &room, nil
}

func lockAdmissions(ctx context.Context, tx pgx.Tx, before, room *Room) error {
	identities := make([]admissionlock.Identity, 0, len(room.Members)*2)
	for _, r := range []*Room{before, room} {
		if r == nil {
			continue
		}
		for _, m := range r.Members {
			identities = append(identities, admissionlock.Identity{AccountID: m.AccountID, ProfileID: m.ProfileID})
		}
	}
	if err := admissionlock.Lock(ctx, tx, identities); err != nil {
		return err
	}
	// This fresh READ COMMITTED statement runs AFTER waiting for the identity
	// locks. Cleanup/replay stays possible while a conversion fence is held.
	for id, admission := range room.Admissions {
		if admission.State == AdmissionAborted {
			continue
		}
		if before != nil {
			prior, exists := before.Admissions[id]
			if exists && prior.State == admission.State {
				continue
			}
		}
		for profile := range admission.Generations {
			member, exists := room.Members[profile]
			if !exists {
				return ErrInvalidIntent
			}
			var fenced bool
			err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_sdk_conversion_fence_receipts
WHERE (state IN ('pending','fenced') AND (source_profile_id=$1 OR target_profile_id=$1 OR target_account_id=$2))
OR (state='activated' AND source_profile_id=$1))`, member.ProfileID, member.AccountID).Scan(&fenced)
			if err != nil {
				return err
			}
			if fenced {
				return ErrConflict
			}
		}
	}
	return nil
}

// This is a transactional lookup index over the native journal, not another
// account exclusivity authority. Consumers must validate exact journal ownership.
func syncMemberships(ctx context.Context, tx pgx.Tx, room *Room) error {
	for _, m := range room.Members {
		_, err := tx.Exec(ctx, `INSERT INTO voice_ordinary_memberships(room_id,profile_id,account_id,
membership_generation,fence_operation_id,media_identity,state,session_epoch) VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT(room_id,profile_id) DO UPDATE SET account_id=EXCLUDED.account_id,
membership_generation=EXCLUDED.membership_generation,fence_operation_id=EXCLUDED.fence_operation_id,
media_identity=EXCLUDED.media_identity,state=EXCLUDED.state,session_epoch=EXCLUDED.session_epoch`, room.Intent.RoomID, m.ProfileID, m.AccountID,
			m.Generation, m.FenceOperationID, m.MediaIdentity, string(m.State), m.SessionEpoch)
		if err != nil {
			return err
		}
	}
	return nil
}

func syncFences(ctx context.Context, tx pgx.Tx, before, room *Room) error {
	members := make([]Member, 0, len(room.Members))
	for _, member := range room.Members {
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool { return members[i].AccountID.String() < members[j].AccountID.String() })
	for _, member := range members {
		if room.Stage == StageClosed || member.State == MemberLeft || member.State == MemberAborted {
			_, err := tx.Exec(ctx, `DELETE FROM voice_account_voice_fences WHERE account_id=$1 AND profile_id=$2
AND room_id=$3 AND admission_operation_id=$4 AND admission_generation=$5`, member.AccountID,
				member.ProfileID, room.Intent.RoomID.String(), member.FenceOperationID, member.Generation.String())
			if err != nil {
				return err
			}
			continue
		}
		_, err := tx.Exec(ctx, `INSERT INTO voice_profile_account_mappings(profile_id,account_id)
VALUES($1,$2) ON CONFLICT(profile_id) DO NOTHING`, member.ProfileID, member.AccountID)
		if err != nil {
			return err
		}
		var account uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT account_id FROM voice_profile_account_mappings WHERE profile_id=$1 FOR UPDATE`, member.ProfileID).Scan(&account); err != nil {
			return err
		}
		if account != member.AccountID {
			return ErrConflict
		}
		prior, hadPrior := Member{}, false
		if before != nil {
			prior, hadPrior = before.Members[member.ProfileID.String()]
		}
		if hadPrior && prior.Generation == member.Generation {
			var valid bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_account_voice_fences WHERE account_id=$1
AND profile_id=$2 AND room_id=$3 AND admission_operation_id=$4 AND admission_generation=$5
AND state='active' AND reservation_expires_at IS NULL)`, member.AccountID,
				member.ProfileID, room.Intent.RoomID.String(), member.FenceOperationID, member.Generation.String()).Scan(&valid); err != nil {
				return err
			}
			if !valid {
				return ErrConflict
			}
			continue
		}
		// The existing operation/generation columns are shared ownership, not a
		// second account table. Space/Game or another ordinary generation cannot
		// be taken over. Pending ordinary effects use non-expiring active fences.
		tag, err := tx.Exec(ctx, `INSERT INTO voice_account_voice_fences(account_id,profile_id,room_id,state,
reservation_expires_at,admission_operation_id,admission_generation) VALUES($1,$2,$3,'active',NULL,$4,$5)
ON CONFLICT(account_id) DO UPDATE SET profile_id=EXCLUDED.profile_id,room_id=EXCLUDED.room_id,
state='active',reservation_expires_at=NULL,admission_operation_id=EXCLUDED.admission_operation_id,
admission_generation=EXCLUDED.admission_generation,updated_at=clock_timestamp()
WHERE voice_account_voice_fences.admission_operation_id IS NULL AND voice_account_voice_fences.state='reserving'
AND voice_account_voice_fences.reservation_expires_at<=clock_timestamp()`, member.AccountID,
			member.ProfileID, room.Intent.RoomID.String(), member.FenceOperationID, member.Generation.String())
		if err != nil {
			return mapStoreError(err)
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
	}
	return nil
}

func (s *PostgresStore) MarkProjected(ctx context.Context, roomID uuid.UUID, version uint64) error {
	tag, err := s.pool.Exec(ctx, `UPDATE voice_ordinary_rooms SET projection_version=$2
WHERE room_id=$1 AND version=$2`, roomID, version)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (s *PostgresStore) ClaimOutbox(ctx context.Context, leaseFor time.Duration) (*OutboxItem, error) {
	if leaseFor < time.Millisecond || leaseFor > time.Minute {
		return nil, ErrInvalidIntent
	}
	token := uuid.New()
	var item OutboxItem
	err := s.pool.QueryRow(ctx, `WITH candidate AS (
SELECT o.event_id FROM voice_ordinary_outbox o JOIN voice_ordinary_rooms r USING(room_id)
WHERE o.delivered_at IS NULL AND o.next_attempt_at<=clock_timestamp()
AND (o.lease_until IS NULL OR o.lease_until<=clock_timestamp()) AND r.projection_version>=o.room_version
AND NOT EXISTS(SELECT 1 FROM voice_ordinary_outbox prior WHERE prior.room_id=o.room_id
AND prior.ordinal<o.ordinal AND prior.delivered_at IS NULL)
ORDER BY o.created_at,o.room_id,o.ordinal FOR UPDATE OF o SKIP LOCKED LIMIT 1)
UPDATE voice_ordinary_outbox o SET lease_token=$1,lease_until=clock_timestamp()+$2::bigint*interval '1 millisecond',attempts=attempts+1
FROM candidate c WHERE o.event_id=c.event_id RETURNING o.event_id,o.room_id,o.ordinal,o.subject,o.payload,o.lease_token`,
		token, leaseFor.Milliseconds()).Scan(&item.ID, &item.RoomID, &item.Ordinal, &item.Subject, &item.Payload, &item.LeaseToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *PostgresStore) ReleaseOutbox(ctx context.Context, item OutboxItem, next time.Time) error {
	if next.IsZero() {
		next = time.Unix(0, 0)
	}
	tag, err := s.pool.Exec(ctx, `UPDATE voice_ordinary_outbox SET lease_token=NULL,lease_until=NULL,next_attempt_at=$3
WHERE event_id=$1 AND lease_token=$2 AND lease_until>clock_timestamp() AND delivered_at IS NULL`, item.ID, item.LeaseToken, next)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

func (s *PostgresStore) MarkDelivered(ctx context.Context, item OutboxItem) error {
	tag, err := s.pool.Exec(ctx, `UPDATE voice_ordinary_outbox SET delivered_at=clock_timestamp(),lease_token=NULL,lease_until=NULL
WHERE event_id=$1 AND lease_token=$2 AND lease_until>clock_timestamp() AND delivered_at IS NULL`, item.ID, item.LeaseToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrStaleLease
	}
	return nil
}

func mapStoreError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var sql *pgconn.PgError
	if errors.As(err, &sql) && sql.Code == "23505" {
		return ErrConflict
	}
	return err
}

func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
