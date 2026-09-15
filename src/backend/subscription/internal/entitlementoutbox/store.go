package entitlementoutbox

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
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
)

// Append commits neither transaction nor billing. The caller must include its
// normalized billing transition and this snapshot in the same transaction and
// roll back on any error. Replayed bytes reuse their event and revision.
func Append(ctx context.Context, tx pgx.Tx, expected int64, e *eventsv1.SubscriptionStreamEvent) error {
	if err := Validate(e); err != nil {
		return err
	}
	if expected < 0 || expected == math.MaxInt64 || uint64(expected+1) != e.AggregateRevision {
		return ErrRevisionConflict
	}
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(e)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(body)
	// Lock even a not-yet-created permanent aggregate. Hash collisions merely
	// serialize unrelated transactions; the actual key remains the UUID pair.
	key := fmt.Sprintf("%d/%s", e.AggregateKind, e.AggregateId)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,17007))", key); err != nil {
		return err
	}
	var saved []byte
	err = tx.QueryRow(ctx, "SELECT payload FROM subscription_event_outbox WHERE event_id=$1", e.EventId).Scan(&saved)
	if err == nil {
		if bytes.Equal(saved, body) {
			return nil
		}
		return ErrContractMismatch
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var revision int64
	err = tx.QueryRow(ctx, `SELECT aggregate_revision FROM subscription_entitlement_aggregates WHERE aggregate_kind=$1 AND aggregate_id=$2 FOR UPDATE`, int16(e.AggregateKind), e.AggregateId).Scan(&revision)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if revision != expected {
		return ErrRevisionConflict
	}
	_, err = tx.Exec(ctx, `INSERT INTO subscription_entitlement_aggregates
		(aggregate_kind,aggregate_id,aggregate_revision,payload,payload_hash) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (aggregate_kind,aggregate_id) DO UPDATE SET aggregate_revision=EXCLUDED.aggregate_revision,payload=EXCLUDED.payload,payload_hash=EXCLUDED.payload_hash`, int16(e.AggregateKind), e.AggregateId, int64(e.AggregateRevision), body, hash[:])
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO subscription_event_outbox
		(event_id,aggregate_kind,aggregate_id,aggregate_revision,event_kind,subject,payload,payload_hash)
		VALUES ($1,$2,$3,$4,'entitlement_changed',$5,$6,$7)`, e.EventId, int16(e.AggregateKind), e.AggregateId, int64(e.AggregateRevision), Subject, body, hash[:])
	return err
}

type Store struct{ Pool *pgxpool.Pool }

type Delivery struct {
	EventID     string
	Payload     []byte
	PayloadHash []byte
	LeaseToken  string
	Attempts    int
}

type Ack struct {
	Stream   string
	Sequence uint64
}

// Claim uses DB time and SKIP LOCKED to share bounded work across replicas.
func (s Store) Claim(ctx context.Context, limit int, lease time.Duration) ([]Delivery, error) {
	if limit < 1 || limit > 100 || lease < time.Second || lease > time.Minute {
		return nil, errors.New("invalid outbox claim bounds")
	}
	rows, err := s.Pool.Query(ctx, `WITH ready AS (
		SELECT event_id FROM subscription_event_outbox
		WHERE delivered_at IS NULL AND available_at<=clock_timestamp()
		AND (lease_until IS NULL OR lease_until<=clock_timestamp())
		ORDER BY available_at,created_at,event_id LIMIT $1 FOR UPDATE SKIP LOCKED)
		UPDATE subscription_event_outbox o SET lease_token=gen_random_uuid(),
		lease_until=clock_timestamp()+$2*interval '1 millisecond',attempts=attempts+1
		FROM ready WHERE o.event_id=ready.event_id
		RETURNING o.event_id::text,o.payload,o.payload_hash,o.lease_token::text,o.attempts`, limit, lease.Milliseconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Delivery
	for rows.Next() {
		var d Delivery
		if err := rows.Scan(&d.EventID, &d.Payload, &d.PayloadHash, &d.LeaseToken, &d.Attempts); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (s Store) Complete(ctx context.Context, d Delivery, ack Ack) error {
	if ack.Stream != "subscription_events" || ack.Sequence == 0 || ack.Sequence > math.MaxInt64 {
		return ErrContractMismatch
	}
	if !validID(d.EventID) || !validID(d.LeaseToken) {
		return ErrLeaseLost
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE subscription_event_outbox SET delivered_at=clock_timestamp(),
		published_stream=$3,published_sequence=$4,lease_token=NULL,lease_until=NULL,last_error=NULL
		WHERE event_id=$1 AND lease_token=$2 AND lease_until>clock_timestamp() AND delivered_at IS NULL`, d.EventID, d.LeaseToken, ack.Stream, int64(ack.Sequence))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return nil
}

// Retry never stores provider/error text: it can contain identities or secrets.
// All ambiguous publish outcomes retry the existing bytes with bounded backoff.
func (s Store) Retry(ctx context.Context, d Delivery) error {
	if _, err := uuid.Parse(d.LeaseToken); err != nil {
		return ErrLeaseLost
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE subscription_event_outbox SET
		available_at=clock_timestamp()+LEAST(300,power(2,LEAST(attempts,8))) * interval '1 second',
		last_error='PUBLISH_UNCONFIRMED',lease_token=NULL,lease_until=NULL
		WHERE event_id=$1 AND lease_token=$2 AND lease_until>clock_timestamp() AND delivered_at IS NULL`, d.EventID, d.LeaseToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrLeaseLost
	}
	return nil
}
