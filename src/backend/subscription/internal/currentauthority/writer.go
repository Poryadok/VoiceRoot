package currentauthority

import (
	"context"
	"crypto/sha256"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
)

// Writer is bound to one actual PostgreSQL transaction. It records source writes,
// not eligibility. There is deliberately no method which promotes an account to
// CURRENT; authenticated owner ingress and all local writers are still required.
type Writer struct {
	tx         pgx.Tx
	generation string
}

// AcquireWriter must precede every provider/aggregate lock. The shared global
// lock prevents an installer from changing the generation until this transaction
// commits. A caller-supplied UUID cannot stand in for installed role evidence.
func AcquireWriter(ctx context.Context, tx pgx.Tx, generation string) (*Writer, error) {
	id, err := uuid.Parse(generation)
	if err != nil || id == uuid.Nil || id.String() != generation {
		return nil, ErrFenceConflict
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock_shared(17006)"); err != nil {
		return nil, err
	}
	var actual, role string
	var exactRole bool
	err = tx.QueryRow(ctx, `SELECT f.generation::text,r.rolname,
 session_user=current_user AND r.rolname=current_user
 FROM public.subscription_authority_fence f JOIN pg_roles r ON r.oid=f.writer_role
 WHERE f.singleton AND f.state='FENCED'`).Scan(&actual, &role, &exactRole)
	if err != nil {
		return nil, err
	}
	if actual != generation {
		return nil, ErrFenceConflict
	}
	if !exactRole {
		return nil, ErrUnsafeRole
	}
	if err = limitedRole(ctx, tx, role); err != nil {
		return nil, err
	}
	if err = noWritePrivilege(ctx, tx, role, "subscription_authority_fence"); err != nil {
		return nil, err
	}
	return &Writer{tx: tx, generation: generation}, nil
}

// RecordProvider accepts only the canonical bytes and outbox outcome just
// committed by this transaction. Call exclusively for a new APPLIED revision;
// replay, STALE and UNCHANGED outcomes never create provenance for old rows.
func (w *Writer) RecordProvider(ctx context.Context, event *eventsv1.SubscriptionStreamEvent, provider, subscription, providerEvent string, version int64) error {
	if w == nil || w.tx == nil || event == nil || version <= 0 {
		return ErrFenceConflict
	}
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(event)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(body)
	tag, err := w.tx.Exec(ctx, `INSERT INTO public.subscription_kernel_writes
 (aggregate_kind,aggregate_id,aggregate_revision,generation,event_id,payload_hash,provider,provider_event_id,provider_version)
 SELECT a.aggregate_kind,a.aggregate_id,a.aggregate_revision,$4,o.event_id,a.payload_hash,$5,$6,$7
 FROM public.subscription_entitlement_aggregates a
 JOIN public.subscription_event_outbox o USING(aggregate_kind,aggregate_id,aggregate_revision)
 JOIN public.subscription_provider_outcomes p ON p.provider=$5 AND p.provider_event_id=$6
 JOIN public.subscription_provider_bindings b ON b.provider=$5 AND b.provider_subscription_id=$11
 AND b.aggregate_kind=a.aggregate_kind AND b.aggregate_id=a.aggregate_id AND b.provider_version=$7 AND b.current_binding
 JOIN public.subscription_provider_versions v ON v.provider=b.provider AND v.provider_subscription_id=b.provider_subscription_id
 AND v.provider_version=b.provider_version AND v.facts_hash=b.facts_hash
 WHERE a.aggregate_kind=$1 AND a.aggregate_id=$2 AND a.aggregate_revision=$3
 AND a.payload=$8 AND a.payload_hash=$9 AND o.payload=$8 AND o.payload_hash=$9
 AND o.event_id=$10 AND p.payload=$8 AND p.disposition='APPLIED'`,
		int16(event.AggregateKind), event.AggregateId, int64(event.AggregateRevision), w.generation, provider, providerEvent, version, body, hash[:], event.EventId, subscription)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrFenceConflict
	}
	return nil
}
