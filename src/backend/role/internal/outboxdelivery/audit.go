package outboxdelivery

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"voice/backend/role/internal/store"
)

// AuditEffect is an immutable persisted request plus the current storage lease.
// RequestBytes contain no transport credentials. Each append attempt signs
// freshly, without rebuilding the logical fact.
type AuditEffect = store.RoleAuditEffect

type AuditLeaseStore interface {
	ClaimAuditEffects(context.Context, int, time.Duration) ([]AuditEffect, error)
	CompleteAuditEffect(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	RetryAuditEffect(context.Context, uuid.UUID, uuid.UUID) (bool, error)
}

type AuditAppendSink interface {
	Append(context.Context, uuid.UUID, []byte) error
}

type AuditDispatcher struct {
	Store AuditLeaseStore
	Sink  AuditAppendSink
}

var ErrAuditLeaseLost = errors.New("audit outbox lease is no longer current")

func (d *AuditDispatcher) DispatchOnce(ctx context.Context) error {
	if d == nil || d.Store == nil || d.Sink == nil {
		return errors.New("audit dispatcher is not configured")
	}
	if ctx == nil {
		return errors.New("audit dispatch context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	effects, err := d.Store.ClaimAuditEffects(ctx, 32, 30*time.Second)
	if err != nil {
		return err
	}
	var failures []error
	for _, effect := range effects {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if effect.EventID == uuid.Nil || effect.LeaseToken == uuid.Nil || len(effect.RequestBytes) == 0 {
			failures = append(failures, errors.New("audit effect has invalid identity, lease or request"))
			continue
		}
		// The sink must not be able to alter the storage snapshot reused by a
		// later attempt. Signing credentials are always outside these bytes.
		if err := d.Sink.Append(ctx, effect.EventID, append([]byte(nil), effect.RequestBytes...)); err != nil {
			retried, retryErr := d.Store.RetryAuditEffect(ctx, effect.EventID, effect.LeaseToken)
			if retryErr == nil && !retried {
				retryErr = ErrAuditLeaseLost
			}
			failures = append(failures, errors.Join(err, retryErr))
			continue
		}
		completed, err := d.Store.CompleteAuditEffect(ctx, effect.EventID, effect.LeaseToken)
		if err == nil && !completed {
			err = ErrAuditLeaseLost
		}
		if err != nil {
			// Remote success followed by a local UNKNOWN commit is resolved by
			// durable lease recovery and exact replay, never a payload rewrite.
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
