// Package gracereminders is the source-disabled revisioned reminder producer.
// A committed outbox or PubAck is not evidence of Notification delivery.
package gracereminders

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/subscription/internal/entitlementoutbox"
)

const Subject = "subscription.grace_reminder"

var ErrLeaseLost = errors.New("grace reminder lease lost")
var ErrContract = errors.New("invalid grace reminder contract")

// Sync runs in the canonical aggregate transition transaction after Append.
// Every successor revision suppresses undelivered prior-revision reminders,
// including already leased rows. No new schedule is made for metadata changes.
func Sync(ctx context.Context, tx pgx.Tx, e *eventsv1.SubscriptionStreamEvent) error {
	if err := entitlementoutbox.Validate(e); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE subscription_grace_reminders SET status='SUPPRESSED',lease_token=NULL,lease_until=NULL
 WHERE aggregate_kind=$1 AND aggregate_id=$2 AND grace_revision<>$3 AND status='PENDING'`, int16(e.AggregateKind), e.AggregateId, int64(e.AggregateRevision))
	if err != nil {
		return err
	}
	s := e.GetEntitlementChanged()
	if s.State != eventsv1.EntitlementState_ENTITLEMENT_STATE_GRACE_PERIOD || s.Reason != eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED {
		return nil
	}
	if s.PurchaserDeleted || s.DeletionCycleId != nil {
		return nil
	}
	start, end := s.EffectiveAt.AsTime(), s.GracePeriodEnd.AsTime()
	if !end.Equal(start.Add(7 * 24 * time.Hour)) {
		return ErrContract
	}
	payer := s.AccountId
	var space *string
	if e.AggregateKind == 2 {
		payer = s.GetPurchaserAccountId()
		id := s.SpaceId
		space = &id
	}
	for i, day := range []int32{1, 3, 7} {
		due := start.Add(time.Duration(day-1) * 24 * time.Hour)
		closes := end
		if i < 2 {
			next := []int32{3, 7}[i]
			closes = start.Add(time.Duration(next-1) * 24 * time.Hour)
		}
		id := uuid.NewSHA1(uuid.MustParse(e.EventId), []byte(fmt.Sprintf("grace/%d", day))).String()
		reminder := &eventsv1.SubscriptionStreamEvent{EventId: id, OccurredAt: e.OccurredAt, ProtocolVersion: 1, AggregateKind: e.AggregateKind, AggregateId: e.AggregateId, AggregateRevision: e.AggregateRevision,
			Payload: &eventsv1.SubscriptionStreamEvent_GraceReminder{GraceReminder: &eventsv1.GraceReminder{AccountId: payer, Plan: s.Plan, Day: day, SpaceId: space}}}
		body, err := proto.MarshalOptions{Deterministic: true}.Marshal(reminder)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(body)
		_, err = tx.Exec(ctx, `INSERT INTO subscription_grace_reminders(event_id,aggregate_kind,aggregate_id,grace_revision,day,subject,payload,payload_hash,due_at,window_end,available_at)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$9)`, id, int16(e.AggregateKind), e.AggregateId, int64(e.AggregateRevision), day, Subject, body, hash[:], due, closes)
		if err != nil {
			return err
		}
	}
	return nil
}

type Store struct{ Pool *pgxpool.Pool }
type Delivery struct {
	EventID, LeaseToken  string
	Payload, PayloadHash []byte
	Attempts             int
}
type candidate struct {
	id, aggregate string
	kind          int16
}

// Claim uses DB time and the canonical aggregate lock before the reminder row.
// A missed window becomes a retained terminal outcome, never a catch-up burst.
func (s Store) Claim(ctx context.Context, limit int, lease time.Duration) ([]Delivery, error) {
	if limit < 1 || limit > 100 || lease < time.Second || lease > time.Minute {
		return nil, ErrContract
	}
	rows, err := s.Pool.Query(ctx, `SELECT event_id::text,aggregate_kind,aggregate_id::text FROM subscription_grace_reminders
 WHERE status='PENDING' AND available_at<=clock_timestamp() AND (lease_until IS NULL OR lease_until<=clock_timestamp())
 ORDER BY available_at,event_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.id, &c.kind, &c.aggregate); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var out []Delivery
	for _, c := range candidates {
		d, err := s.claimOne(ctx, c, lease)
		if err != nil {
			return nil, err
		}
		if d != nil {
			out = append(out, *d)
		}
	}
	return out, nil
}

func lock(ctx context.Context, tx pgx.Tx, c candidate) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,17007))", fmt.Sprintf("%d/%s", c.kind, c.aggregate))
	return err
}

// current verifies the authoritative snapshot and immutable reminder against
// the same aggregate revision. Caller holds the aggregate advisory lock.
func current(ctx context.Context, tx pgx.Tx, c candidate, d Delivery, revision int64) (bool, error) {
	var body, hash []byte
	var actual int64
	err := tx.QueryRow(ctx, `SELECT aggregate_revision,payload,payload_hash FROM subscription_entitlement_aggregates WHERE aggregate_kind=$1 AND aggregate_id=$2 FOR UPDATE`, c.kind, c.aggregate).Scan(&actual, &body, &hash)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(body)
	if !bytes.Equal(sum[:], hash) {
		return false, ErrContract
	}
	e := new(eventsv1.SubscriptionStreamEvent)
	if proto.Unmarshal(body, e) != nil || entitlementoutbox.Validate(e) != nil {
		return false, ErrContract
	}
	if actual != revision {
		return false, nil
	}
	s := e.GetEntitlementChanged()
	if s.State != eventsv1.EntitlementState_ENTITLEMENT_STATE_GRACE_PERIOD || s.PurchaserDeleted || s.DeletionCycleId != nil {
		return false, nil
	}
	sum = sha256.Sum256(d.Payload)
	if !bytes.Equal(sum[:], d.PayloadHash) {
		return false, ErrContract
	}
	r := new(eventsv1.SubscriptionStreamEvent)
	if proto.Unmarshal(d.Payload, r) != nil || r.EventId != d.EventID || r.ProtocolVersion != 1 || r.AggregateKind != e.AggregateKind || r.AggregateId != e.AggregateId || r.AggregateRevision != e.AggregateRevision {
		return false, ErrContract
	}
	p := r.GetGraceReminder()
	if p == nil || (p.Day != 1 && p.Day != 3 && p.Day != 7) || p.Plan != s.Plan {
		return false, ErrContract
	}
	payer := s.AccountId
	if c.kind == 2 {
		payer = s.GetPurchaserAccountId()
		if p.GetSpaceId() != s.SpaceId {
			return false, ErrContract
		}
	} else if p.SpaceId != nil {
		return false, ErrContract
	}
	if p.AccountId != payer {
		return false, ErrContract
	}
	return true, nil
}

func (s Store) claimOne(ctx context.Context, c candidate, lease time.Duration) (*Delivery, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var acquired bool
	if err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(hashtextextended($1,17007))", fmt.Sprintf("%d/%s", c.kind, c.aggregate)).Scan(&acquired); err != nil {
		return nil, err
	}
	if !acquired {
		return nil, nil
	}
	var d Delivery
	var revision int64
	var expired bool
	err = tx.QueryRow(ctx, `SELECT event_id::text,payload,payload_hash,grace_revision,window_end<=clock_timestamp() FROM subscription_grace_reminders
 WHERE event_id=$1 AND status='PENDING' AND due_at<=clock_timestamp() AND available_at<=clock_timestamp() AND (lease_until IS NULL OR lease_until<=clock_timestamp()) FOR UPDATE SKIP LOCKED`, c.id).Scan(&d.EventID, &d.Payload, &d.PayloadHash, &revision, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	valid, err := current(ctx, tx, c, d, revision)
	if err != nil {
		return nil, err
	}
	if !valid || expired {
		status := "SUPPRESSED"
		if valid && expired {
			status = "EXPIRED"
		}
		_, err = tx.Exec(ctx, `UPDATE subscription_grace_reminders SET status=$2,lease_token=NULL,lease_until=NULL WHERE event_id=$1`, c.id, status)
		if err != nil {
			return nil, err
		}
		return nil, tx.Commit(ctx)
	}
	err = tx.QueryRow(ctx, `UPDATE subscription_grace_reminders SET lease_token=gen_random_uuid(),lease_until=clock_timestamp()+$2*interval '1 millisecond',attempts=attempts+1 WHERE event_id=$1 RETURNING lease_token::text,attempts`, c.id, lease.Milliseconds()).Scan(&d.LeaseToken, &d.Attempts)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &d, nil
}

// Dispatch rechecks current state immediately before network publication while
// holding the aggregate lock. A recovery already committed suppresses the row;
// recovery after this comparison cannot recall an already sent provider request.
// Any unknown PubAck leaves the saved ID/bytes for a fenced lease retry.
func (s Store) Dispatch(ctx context.Context, js nats.JetStreamContext, d Delivery) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var c candidate
	err = tx.QueryRow(ctx, `SELECT event_id::text,aggregate_kind,aggregate_id::text FROM subscription_grace_reminders WHERE event_id=$1`, d.EventID).Scan(&c.id, &c.kind, &c.aggregate)
	if err != nil {
		return err
	}
	if err = lock(ctx, tx, c); err != nil {
		return err
	}
	var saved Delivery
	var revision int64
	var expired bool
	err = tx.QueryRow(ctx, `SELECT event_id::text,payload,payload_hash,grace_revision,window_end<=clock_timestamp() FROM subscription_grace_reminders
 WHERE event_id=$1 AND status='PENDING' AND lease_token=$2 AND lease_until>clock_timestamp() FOR UPDATE`, d.EventID, d.LeaseToken).Scan(&saved.EventID, &saved.Payload, &saved.PayloadHash, &revision, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLeaseLost
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(saved.Payload, d.Payload) || !bytes.Equal(saved.PayloadHash, d.PayloadHash) {
		return ErrContract
	}
	valid, err := current(ctx, tx, c, saved, revision)
	if err != nil {
		return err
	}
	if !valid || expired {
		status := "SUPPRESSED"
		if valid && expired {
			status = "EXPIRED"
		}
		_, err = tx.Exec(ctx, `UPDATE subscription_grace_reminders SET status=$2,lease_token=NULL,lease_until=NULL WHERE event_id=$1`, c.id, status)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	msg := nats.NewMsg(Subject)
	msg.Data = saved.Payload
	msg.Header.Set(nats.MsgIdHdr, saved.EventID)
	ack, publishErr := js.PublishMsg(msg, nats.Context(ctx))
	if publishErr == nil && (ack == nil || ack.Stream != "subscription_events" || ack.Sequence == 0 || ack.Sequence > math.MaxInt64) {
		publishErr = ErrContract
	}
	if publishErr != nil {
		return publishErr
	}
	tag, err := tx.Exec(ctx, `UPDATE subscription_grace_reminders SET status='DELIVERED',delivered_at=clock_timestamp(),published_stream=$3,published_sequence=$4,lease_token=NULL,lease_until=NULL
 WHERE event_id=$1 AND lease_token=$2 AND lease_until>clock_timestamp()`, d.EventID, d.LeaseToken, ack.Stream, int64(ack.Sequence))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return tx.Commit(ctx)
}

func (s Store) Retry(ctx context.Context, d Delivery) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE subscription_grace_reminders SET available_at=clock_timestamp()+LEAST(300,power(2,LEAST(attempts,8)))*interval '1 second',lease_token=NULL,lease_until=NULL
 WHERE event_id=$1 AND lease_token=$2 AND lease_until>clock_timestamp() AND status='PENDING'`, d.EventID, d.LeaseToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return nil
}
