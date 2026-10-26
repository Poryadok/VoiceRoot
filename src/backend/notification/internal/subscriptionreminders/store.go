// Package subscriptionreminders owns the durable, source-disabled A7 reminder
// inbox and channel work. QUEUED means committed work, never external delivery.
// Activation requires protected current-state/primary reads and real in-app
// transport; the legacy logging consumer must not ACK using this package alone.
package subscriptionreminders

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
)

var (
	ErrAhead            = errors.New("reminder ahead of entitlement projection")
	ErrContractMismatch = errors.New("notification subscription contract mismatch")
	ErrLeaseLost        = errors.New("notification reminder lease lost")
)

type Store struct{ Pool *pgxpool.Pool }

// Apply commits inbox/projection/channel work atomically. Only a successful
// commit authorizes broker acknowledgement; ErrAhead must be retried.
func (s Store) Apply(ctx context.Context, raw []byte) (string, error) {
	e, err := decode(raw)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Event identity lock precedes aggregate lock, including conflicting replay
	// that changes the aggregate key. All paths follow this one lock order.
	for _, key := range []string{"event/" + e.EventId, fmt.Sprintf("aggregate/%d/%s", e.AggregateKind, e.AggregateId)} {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,118))", key); err != nil {
			return "", err
		}
	}
	var saved []byte
	var outcome string
	err = tx.QueryRow(ctx, "SELECT payload_hash,outcome FROM notification_subscription_inbox WHERE event_id=$1", e.EventId).Scan(&saved, &outcome)
	if err == nil {
		if !bytes.Equal(saved, hash[:]) {
			return "", ErrContractMismatch
		}
		return outcome, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	var revision int64
	var snapshot, snapshotHash []byte
	err = tx.QueryRow(ctx, `SELECT aggregate_revision,snapshot,snapshot_hash FROM notification_subscription_projection WHERE aggregate_kind=$1 AND aggregate_id=$2 FOR UPDATE`, int16(e.AggregateKind), e.AggregateId).Scan(&revision, &snapshot, &snapshotHash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if p := e.GetEntitlementChanged(); p != nil {
		outcome, err = applySnapshot(ctx, tx, e, p, revision, snapshotHash)
	} else {
		outcome, err = applyReminder(ctx, tx, e, revision, snapshot)
	}
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, "INSERT INTO notification_subscription_inbox (event_id,payload_hash,outcome) VALUES ($1,$2,$3)", e.EventId, hash[:], outcome)
	if err != nil {
		return "", err
	}
	return outcome, tx.Commit(ctx)
}

func applySnapshot(ctx context.Context, tx pgx.Tx, e *eventsv1.SubscriptionStreamEvent, p *eventsv1.EntitlementChanged, revision int64, savedHash []byte) (string, error) {
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(p)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(body)
	if e.AggregateRevision < uint64(revision) {
		return "STALE", nil
	}
	if e.AggregateRevision == uint64(revision) {
		if !bytes.Equal(savedHash, hash[:]) {
			return "", ErrContractMismatch
		}
		return "APPLIED", nil
	}
	_, err = tx.Exec(ctx, `INSERT INTO notification_subscription_projection (aggregate_kind,aggregate_id,aggregate_revision,snapshot,snapshot_hash) VALUES ($1,$2,$3,$4,$5)
 ON CONFLICT (aggregate_kind,aggregate_id) DO UPDATE SET aggregate_revision=EXCLUDED.aggregate_revision,snapshot=EXCLUDED.snapshot,snapshot_hash=EXCLUDED.snapshot_hash`, int16(e.AggregateKind), e.AggregateId, int64(e.AggregateRevision), body, hash[:])
	if err != nil {
		return "", err
	}
	// Pending includes currently leased work. Every completion is token-fenced,
	// so a successor snapshot cannot be undone by a former lease holder.
	_, err = tx.Exec(ctx, `UPDATE notification_reminder_dispatch SET status='SUPPRESSED',completed_at=clock_timestamp(),lease_token=NULL,lease_until=NULL WHERE aggregate_kind=$1 AND aggregate_id=$2 AND status='PENDING' AND grace_revision<$3`, int16(e.AggregateKind), e.AggregateId, int64(e.AggregateRevision))
	return "APPLIED", err
}

func applyReminder(ctx context.Context, tx pgx.Tx, e *eventsv1.SubscriptionStreamEvent, revision int64, snapshot []byte) (string, error) {
	if e.AggregateRevision > uint64(revision) {
		return "", ErrAhead
	}
	if e.AggregateRevision < uint64(revision) {
		return "STALE", nil
	}
	var p eventsv1.EntitlementChanged
	if err := proto.Unmarshal(snapshot, &p); err != nil {
		return "", ErrContractMismatch
	}
	if p.State != eventsv1.EntitlementState_ENTITLEMENT_STATE_GRACE_PERIOD || p.PurchaserDeleted || p.DeletionCycleId != nil {
		return "STALE", nil
	}
	reminder := e.GetGraceReminder()
	payer := p.AccountId
	if e.AggregateKind == eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_SPACE {
		payer = p.GetPurchaserAccountId()
	}
	if reminder.AccountId != payer || reminder.Plan != p.Plan {
		return "", ErrContractMismatch
	}
	start, end := window(p.EffectiveAt.AsTime(), reminder.Day)
	if !p.GracePeriodEnd.AsTime().Equal(p.EffectiveAt.AsTime().Add(7 * 24 * time.Hour)) {
		return "", ErrContractMismatch
	}
	var now time.Time
	if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return "", err
	}
	if !now.Before(end) {
		return "SUPPRESSED", nil
	}
	if now.Before(start) {
		return "", ErrAhead
	}
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(e)
	if err != nil {
		return "", err
	}
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("voice/reminder/%d/%s/%d/%d", e.AggregateKind, e.AggregateId, e.AggregateRevision, reminder.Day)))
	for _, channel := range []string{"in_app", "push"} {
		var existing string
		err = tx.QueryRow(ctx, "SELECT event_id::text FROM notification_reminder_dispatch WHERE notification_id=$1 AND channel=$2", id, channel).Scan(&existing)
		if err == nil {
			if existing != e.EventId {
				return "", ErrContractMismatch
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
		_, err = tx.Exec(ctx, `INSERT INTO notification_reminder_dispatch (notification_id,channel,event_id,aggregate_kind,aggregate_id,grace_revision,day,payer_account_id,payload,window_start,window_end) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, id, channel, e.EventId, int16(e.AggregateKind), e.AggregateId, int64(e.AggregateRevision), reminder.Day, payer, body, start, end)
		if err != nil {
			return "", err
		}
	}
	return "QUEUED", nil
}

func window(start time.Time, day int32) (time.Time, time.Time) {
	switch day {
	case 1:
		return start, start.Add(2 * 24 * time.Hour)
	case 3:
		return start.Add(2 * 24 * time.Hour), start.Add(6 * 24 * time.Hour)
	default:
		return start.Add(6 * 24 * time.Hour), start.Add(7 * 24 * time.Hour)
	}
}

type Delivery struct {
	NotificationID, Channel, LeaseToken string
	Payload                             []byte
	Attempts                            int
	WindowEnd                           time.Time
}

// Claim returns pending channel work, not a delivery receipt. Dispatch must use
// current protected authority and ordinary policy before creating any effect.
func (s Store) Claim(ctx context.Context, limit int, lease time.Duration) ([]Delivery, error) {
	if limit < 1 || limit > 100 || lease < time.Second || lease > time.Minute {
		return nil, errors.New("invalid reminder claim bounds")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `UPDATE notification_reminder_dispatch SET status='SUPPRESSED',completed_at=clock_timestamp(),lease_token=NULL,lease_until=NULL WHERE status='PENDING' AND window_end<=clock_timestamp()`)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `WITH ready AS (SELECT notification_id,channel FROM notification_reminder_dispatch WHERE status='PENDING' AND available_at<=clock_timestamp() AND window_start<=clock_timestamp() AND window_end>clock_timestamp() AND (lease_until IS NULL OR lease_until<=clock_timestamp()) ORDER BY available_at,notification_id,channel LIMIT $1 FOR UPDATE SKIP LOCKED)
 UPDATE notification_reminder_dispatch d SET lease_token=gen_random_uuid(),lease_until=clock_timestamp()+$2*interval '1 millisecond',attempts=attempts+1 FROM ready WHERE d.notification_id=ready.notification_id AND d.channel=ready.channel RETURNING d.notification_id::text,d.channel,d.lease_token::text,d.payload,d.attempts,d.window_end`, limit, lease.Milliseconds())
	if err != nil {
		return nil, err
	}
	var result []Delivery
	for rows.Next() {
		var d Delivery
		if err = rows.Scan(&d.NotificationID, &d.Channel, &d.LeaseToken, &d.Payload, &d.Attempts, &d.WindowEnd); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}

// Complete is for a confirmed real channel effect only. The activation adapter
// must not call it after recognized events, failed authority, or no-op senders.
func (s Store) Complete(ctx context.Context, d Delivery) error { return s.finish(ctx, d, false) }
func (s Store) Retry(ctx context.Context, d Delivery) error    { return s.finish(ctx, d, true) }
func (s Store) finish(ctx context.Context, d Delivery, retry bool) error {
	query := `UPDATE notification_reminder_dispatch SET status='DELIVERED',completed_at=clock_timestamp(),lease_token=NULL,lease_until=NULL WHERE notification_id=$1 AND channel=$2 AND lease_token=$3 AND lease_until>clock_timestamp() AND window_end>clock_timestamp() AND status='PENDING'`
	if retry {
		query = `UPDATE notification_reminder_dispatch SET available_at=clock_timestamp()+LEAST(300,power(2,LEAST(attempts,8)))*interval '1 second',lease_token=NULL,lease_until=NULL WHERE notification_id=$1 AND channel=$2 AND lease_token=$3 AND lease_until>clock_timestamp() AND status='PENDING'`
	}
	tag, err := s.Pool.Exec(ctx, query, d.NotificationID, d.Channel, d.LeaseToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return nil
}
