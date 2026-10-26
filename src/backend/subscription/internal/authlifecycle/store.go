package authlifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/subscription/internal/currentauthority"
)

var ErrSourceDrift = errors.New("auth lifecycle source drift")

// Ingest owns a local SQL transaction and never performs network I/O under its
// generation/account locks. Only a fresh private client observation is accepted.
// Its result is durable ordered input, never a paid/CURRENT decision or restore.
func Ingest(ctx context.Context, pool *pgxpool.Pool, o *Observation) error {
	if pool == nil || o == nil || o.snapshot == nil || !validID(o.generation) || !validID(o.account) || !validID(o.requestID) || !o.deadline.After(time.Now()) || sha256.Sum256(o.payload) != o.snapshotHash {
		return ErrUnavailable
	}
	ctx, cancel := context.WithDeadline(ctx, o.deadline)
	defer cancel()
	if ctx.Err() != nil {
		return ErrUnavailable
	}
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(o.snapshot)
	if err != nil || !bytes.Equal(body, o.payload) {
		return ErrUnavailable
	}
	request, err := proto.MarshalOptions{Deterministic: true}.Marshal(&authv1.ObserveSubscriptionAccountRequest{AccountId: o.account, MinimumLifecycleRevision: o.minimum})
	if err != nil || sha256.Sum256(request) != o.requestHash {
		return ErrUnavailable
	}
	core := proto.Clone(o.snapshot).(*authv1.ObserveSubscriptionAccountResponse)
	core.ObservedAt = nil
	stateBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(core)
	if err != nil {
		return ErrUnavailable
	}
	stateHash := sha256.Sum256(stateBytes)
	revision := o.snapshot.GetHead().GetSourceRevision()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = currentauthority.AcquireWriter(ctx, tx, o.generation); err != nil {
		if errors.Is(err, currentauthority.ErrFenceConflict) || errors.Is(err, currentauthority.ErrUnsafeRole) {
			return err
		}
		return ErrUnavailable
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,17010))", o.account); err != nil {
		return ErrUnavailable
	}
	var savedAccount, savedGeneration string
	var savedRequest, savedSnapshot []byte
	err = tx.QueryRow(ctx, "SELECT account_id::text,generation::text,request_hash,snapshot_hash FROM public.subscription_auth_observation_receipts WHERE request_id=$1", o.requestID).Scan(&savedAccount, &savedGeneration, &savedRequest, &savedSnapshot)
	if err == nil {
		if savedAccount != o.account || savedGeneration != o.generation || !bytes.Equal(savedRequest, o.requestHash[:]) || !bytes.Equal(savedSnapshot, o.snapshotHash[:]) {
			return ErrSourceDrift
		}
		if tx.Commit(ctx) != nil {
			return ErrUnavailable
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ErrUnavailable
	}
	var oldRevision, oldSecurity, oldEpoch uint64
	var oldState []byte
	var oldObserved time.Time
	err = tx.QueryRow(ctx, "SELECT generation::text,auth_revision,auth_security_revision,auth_session_epoch,state_hash,observed_at FROM public.subscription_auth_account_observations WHERE account_id=$1 FOR UPDATE", o.account).Scan(&savedGeneration, &oldRevision, &oldSecurity, &oldEpoch, &oldState, &oldObserved)
	if err == nil {
		if savedGeneration != o.generation || revision < oldRevision || o.snapshot.SecurityRevision < oldSecurity || o.snapshot.SessionEpoch < oldEpoch || o.snapshot.ObservedAt.AsTime().Before(oldObserved) {
			return ErrSourceDrift
		}
		if revision == oldRevision && o.snapshot.SecurityRevision == oldSecurity && o.snapshot.SessionEpoch == oldEpoch && !bytes.Equal(oldState, stateHash[:]) {
			return ErrSourceDrift
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ErrUnavailable
	}
	var newer bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM public.subscription_auth_lifecycle_facts WHERE account_id=$1 AND (source_revision>$2 OR first_generation<>$3))", o.account, int64(revision), o.generation).Scan(&newer)
	if err != nil {
		return ErrUnavailable
	}
	if newer {
		return ErrSourceDrift
	}
	for _, fact := range o.snapshot.Facts {
		payload, e := proto.MarshalOptions{Deterministic: true}.Marshal(fact)
		if e != nil {
			return ErrUnavailable
		}
		hash := sha256.Sum256(payload)
		tag, e := tx.Exec(ctx, `INSERT INTO public.subscription_auth_lifecycle_facts(account_id,source_revision,event_id,first_generation,payload,payload_hash)
 VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(account_id,source_revision) DO NOTHING`, o.account, int64(fact.SourceRevision), fact.EventId, o.generation, payload, hash[:])
		if e != nil {
			return ErrUnavailable
		}
		if tag.RowsAffected() == 0 {
			var priorBody, priorHash []byte
			var event string
			if e = tx.QueryRow(ctx, "SELECT event_id::text,payload,payload_hash FROM public.subscription_auth_lifecycle_facts WHERE account_id=$1 AND source_revision=$2", o.account, int64(fact.SourceRevision)).Scan(&event, &priorBody, &priorHash); e != nil {
				return ErrUnavailable
			}
			if event != fact.EventId || !bytes.Equal(priorBody, payload) || !bytes.Equal(priorHash, hash[:]) {
				return ErrSourceDrift
			}
		}
	}
	coverage := "CAPTURED"
	if revision == 0 {
		coverage = "UNRECORDED"
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.subscription_auth_account_observations(account_id,generation,auth_revision,auth_security_revision,auth_session_epoch,capture_coverage,payload,payload_hash,state_hash,observed_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(account_id) DO UPDATE SET generation=EXCLUDED.generation,auth_revision=EXCLUDED.auth_revision,
 auth_security_revision=EXCLUDED.auth_security_revision,auth_session_epoch=EXCLUDED.auth_session_epoch,capture_coverage=EXCLUDED.capture_coverage,
 payload=EXCLUDED.payload,payload_hash=EXCLUDED.payload_hash,state_hash=EXCLUDED.state_hash,observed_at=EXCLUDED.observed_at,recorded_at=clock_timestamp()`,
		o.account, o.generation, int64(revision), int64(o.snapshot.SecurityRevision), int64(o.snapshot.SessionEpoch), coverage, o.payload, o.snapshotHash[:], stateHash[:], o.snapshot.ObservedAt.AsTime())
	if err != nil {
		return ErrUnavailable
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.subscription_auth_observation_receipts(request_id,account_id,generation,minimum_revision,auth_revision,request_hash,snapshot_hash)
 VALUES($1,$2,$3,$4,$5,$6,$7)`, o.requestID, o.account, o.generation, int64(o.minimum), int64(revision), o.requestHash[:], o.snapshotHash[:])
	if err != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	if tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}
