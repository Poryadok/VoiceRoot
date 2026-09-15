package providerlifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/subscription/internal/entitlementoutbox"
)

type Store struct{ Pool *pgxpool.Pool }

type Result struct {
	Event       *eventsv1.SubscriptionStreamEvent
	Disposition string
}

type binding struct {
	kind    int16
	id      string
	version int64
	facts   []byte
	current bool
}

func commandBytes(c Command) ([]byte, []byte, error) {
	c.EffectiveAt = c.EffectiveAt.UTC()
	c.PeriodStart = c.PeriodStart.UTC()
	c.PeriodEnd = c.PeriodEnd.UTC()
	request, err := json.Marshal(c)
	if err != nil {
		return nil, nil, err
	}
	c.EventID = ""
	c.RawBody = nil
	facts, err := json.Marshal(c)
	if err != nil {
		return nil, nil, err
	}
	return request, facts, nil
}

// Apply owns the transaction. A successful replay returns the original result,
// not today's projection. Contract conflicts commit only a durable quarantine
// record; errors must never be converted to webhook success by a future adapter.
func (s Store) Apply(ctx context.Context, c Command) (Result, error) {
	if err := validateCommand(c); err != nil {
		return Result{}, err
	}
	request, facts, err := commandBytes(c)
	if err != nil {
		return Result{}, err
	}
	requestHash := sha256.Sum256(request)
	factsHash := sha256.Sum256(facts)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// A fixed lock order covers absent rows, cross-target replay, and parallel
	// provider bindings. Aggregate seed/key exactly match S1 Append.
	for _, lock := range []struct {
		key  string
		seed int64
	}{
		{c.Provider + "/" + c.EventID, 17008}, {c.Provider + "/" + c.SubscriptionID, 17009},
		{fmt.Sprintf("%d/%s", c.Kind, c.AggregateID), 17007},
	} {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,$2))", lock.key, lock.seed); err != nil {
			return Result{}, err
		}
	}
	reject := func() (Result, error) {
		_, e := tx.Exec(ctx, `INSERT INTO subscription_provider_conflicts(provider,provider_event_id,request_hash,request_bytes,error_class)
 VALUES($1,$2,$3,$4,'CONTRACT_MISMATCH') ON CONFLICT DO NOTHING`, c.Provider, c.EventID, requestHash[:], request)
		if e == nil {
			e = tx.Commit(ctx)
		}
		if e != nil {
			return Result{}, e
		}
		return Result{}, ErrContractMismatch
	}
	var savedHash, payload []byte
	var disposition string
	err = tx.QueryRow(ctx, `SELECT request_hash,payload,disposition FROM subscription_provider_outcomes WHERE provider=$1 AND provider_event_id=$2`, c.Provider, c.EventID).Scan(&savedHash, &payload, &disposition)
	if err == nil {
		if !bytes.Equal(savedHash, requestHash[:]) {
			return reject()
		}
		event, e := decode(payload)
		if e != nil {
			return Result{}, e
		}
		if e = tx.Commit(ctx); e != nil {
			return Result{}, e
		}
		return Result{Event: event, Disposition: disposition}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}
	var b binding
	err = tx.QueryRow(ctx, `SELECT aggregate_kind,aggregate_id::text,provider_version,facts_hash,current_binding FROM subscription_provider_bindings
 WHERE provider=$1 AND provider_subscription_id=$2 FOR UPDATE`, c.Provider, c.SubscriptionID).Scan(&b.kind, &b.id, &b.version, &b.facts, &b.current)
	known := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, err
	}
	if known && (b.kind != int16(c.Kind) || b.id != c.AggregateID) {
		return reject()
	}
	current, err := loadCurrent(ctx, tx, c)
	if err != nil {
		return Result{}, err
	}
	result := Result{Event: current}
	switch {
	case known && current == nil:
		return Result{}, ErrNeedsReconciliation
	case known && !b.current:
		result.Disposition = "RETIRED"
	case known && c.Version < b.version:
		result.Disposition = "STALE"
	case known && c.Version == b.version:
		if !bytes.Equal(b.facts, factsHash[:]) {
			return reject()
		}
		result.Disposition = "UNCHANGED"
	default:
		if !known && (c.Reason != started || (current != nil && current.GetEntitlementChanged().State != inactive)) {
			return Result{}, ErrNeedsReconciliation
		}
		var previous *eventsv1.EntitlementChanged
		var revision uint64
		if current != nil {
			previous = current.GetEntitlementChanged()
			revision = current.AggregateRevision
		}
		if revision >= math.MaxInt64 {
			return Result{}, ErrNeedsReconciliation
		}
		snapshot, e := transition(previous, c, IDs{Entitlement: uuid.NewString(), DeletionFence: uuid.NewString(), DowngradeCycle: uuid.NewString()})
		if errors.Is(e, ErrContractMismatch) {
			return reject()
		}
		if e != nil {
			return Result{}, e
		}
		result.Disposition = "UNCHANGED"
		if !proto.Equal(previous, snapshot) {
			var now time.Time
			if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
				return Result{}, err
			}
			event := &eventsv1.SubscriptionStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.New(now), ProtocolVersion: 1, AggregateKind: c.Kind, AggregateId: c.AggregateID, AggregateRevision: revision + 1,
				Payload: &eventsv1.SubscriptionStreamEvent_EntitlementChanged{EntitlementChanged: snapshot}}
			if err = entitlementoutbox.Append(ctx, tx, int64(revision), event); err != nil {
				return Result{}, err
			}
			result = Result{Event: event, Disposition: "APPLIED"}
		}
		if !known {
			if _, err = tx.Exec(ctx, `UPDATE subscription_provider_bindings SET current_binding=false,updated_at=clock_timestamp() WHERE aggregate_kind=$1 AND aggregate_id=$2 AND current_binding`, int16(c.Kind), c.AggregateID); err != nil {
				return Result{}, err
			}
			_, err = tx.Exec(ctx, `INSERT INTO subscription_provider_bindings(provider,provider_subscription_id,aggregate_kind,aggregate_id,provider_version,facts_hash,current_binding) VALUES($1,$2,$3,$4,$5,$6,true)`, c.Provider, c.SubscriptionID, int16(c.Kind), c.AggregateID, c.Version, factsHash[:])
		} else {
			_, err = tx.Exec(ctx, `UPDATE subscription_provider_bindings SET provider_version=$3,facts_hash=$4,updated_at=clock_timestamp() WHERE provider=$1 AND provider_subscription_id=$2`, c.Provider, c.SubscriptionID, c.Version, factsHash[:])
		}
		if err != nil {
			return Result{}, err
		}
	}
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(result.Event)
	if err != nil {
		return Result{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO subscription_provider_outcomes(provider,provider_event_id,request_hash,disposition,payload) VALUES($1,$2,$3,$4,$5)`, c.Provider, c.EventID, requestHash[:], result.Disposition, body); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return result, nil
}

func decode(payload []byte) (*eventsv1.SubscriptionStreamEvent, error) {
	e := new(eventsv1.SubscriptionStreamEvent)
	if err := proto.Unmarshal(payload, e); err != nil {
		return nil, ErrContractMismatch
	}
	if err := entitlementoutbox.Validate(e); err != nil {
		return nil, ErrContractMismatch
	}
	return e, nil
}

func loadCurrent(ctx context.Context, tx pgx.Tx, c Command) (*eventsv1.SubscriptionStreamEvent, error) {
	var body, hash []byte
	err := tx.QueryRow(ctx, `SELECT payload,payload_hash FROM subscription_entitlement_aggregates WHERE aggregate_kind=$1 AND aggregate_id=$2 FOR UPDATE`, int16(c.Kind), c.AggregateID).Scan(&body, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	actual := sha256.Sum256(body)
	if !bytes.Equal(actual[:], hash) {
		return nil, ErrContractMismatch
	}
	e, err := decode(body)
	if err != nil {
		return nil, err
	}
	if e.AggregateKind != c.Kind || e.AggregateId != c.AggregateID {
		return nil, ErrContractMismatch
	}
	return e, nil
}
