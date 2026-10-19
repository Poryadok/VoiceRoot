// Package lfpconsentstore owns private durable LFP data. It cannot authenticate
// a user, install a restore cohort or turn receipt bytes into MM authority.
package lfpconsentstore

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/pkg/storylfpadmission"
	"voice/backend/story/internal/lfperasure"
)

var (
	ErrUnavailable = errors.New("LFP consent journal unavailable")
	ErrConflict    = errors.New("LFP consent operation conflict")
	ErrStale       = errors.New("LFP offer changed or unavailable")
	ErrErased      = lfperasure.ErrErased
)

// origin is authenticated deployment provenance, never a public request field.
type origin struct {
	environment, installation, incarnation uuid.UUID
	database                               [32]byte
}

func (o origin) valid() bool {
	return o.environment != uuid.Nil && o.installation != uuid.Nil && o.incarnation != uuid.Nil && o.database != [32]byte{}
}

// admission can only be implemented inside this package by the separately
// reviewed controller adapter. No production adapter currently exists. Its
// enter must hold the actual external close/drain permit until release; a DB
// head, descriptor hash, callback returning true or fixture is not installation
// proof. The controller task owns actual credential/route/replica custody.
type admission interface {
	enter(context.Context) (origin, func(), error)
}

type Store struct {
	Pool      *pgxpool.Pool
	authority admission
	negative  lfperasure.NegativeAuthority
}

// NewBoundStore never creates authority. Only a controller-verified adapter can
// provide admission. The optional negative authority must supply a complete
// authenticated current replay view under its approved finite retention policy;
// omitted/nil/ambiguous authority leaves every API closed, even with a cohort.
func NewBoundStore(pool *pgxpool.Pool, barrier *storylfpadmission.Barrier, negative ...lfperasure.NegativeAuthority) *Store {
	s := &Store{Pool: pool, authority: barrierAdmission{barrier}}
	if len(negative) == 1 {
		s.negative = negative[0]
	}
	return s
}

type barrierAdmission struct{ barrier *storylfpadmission.Barrier }

func (a barrierAdmission) enter(ctx context.Context) (origin, func(), error) {
	if a.barrier == nil {
		return origin{}, nil, ErrUnavailable
	}
	p, err := a.barrier.Enter(ctx)
	if err != nil {
		if errors.Is(err, storylfpadmission.ErrIneligible) {
			return origin{}, nil, ErrStale
		}
		return origin{}, nil, ErrUnavailable
	}
	o := p.Origin()
	return origin{environment: uuid.UUID(o.Environment()), installation: uuid.UUID(o.Installation()), incarnation: uuid.UUID(o.Incarnation()), database: o.DatabaseBinding()}, p.Release, nil
}

func (s *Store) begin(ctx context.Context) (pgx.Tx, origin, func(), error) {
	if s == nil || s.Pool == nil || s.authority == nil {
		return nil, origin{}, nil, ErrUnavailable
	}
	view, e := lfperasure.Enter(ctx, s.negative)
	if e != nil {
		return nil, origin{}, nil, ErrUnavailable
	}
	o, release, e := s.authority.enter(ctx)
	if e != nil || !o.valid() || release == nil {
		if release != nil {
			release()
		}
		view.Release()
		if errors.Is(e, ErrStale) {
			return nil, origin{}, nil, ErrStale
		}
		return nil, origin{}, nil, ErrUnavailable
	}
	refs := negativeReferences(o)
	if e = lfperasure.Check(view, refs); e != nil {
		release()
		view.Release()
		return nil, origin{}, nil, negativeError(e)
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		// BEGIN may have crossed the wire before the caller lost its session.
		// Without a transaction handle there is no confirmed terminal result.
		// Retain restrictive admission until the bound runtime is retired.
		return nil, origin{}, nil, ErrUnavailable
	}
	guard := &completionTx{Tx: tx, negative: view, origin: o}
	var released sync.Once
	confirmedRelease := func() {
		if guard.terminal {
			released.Do(func() { release(); view.Release() })
		}
	}
	// Service-wide local mutation gate precedes operation and Story locks. The
	// erasure worker takes the exclusive form before selecting any closure.
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(2910007)`); e != nil {
		_ = guard.Rollback(ctx)
		confirmedRelease()
		return nil, origin{}, nil, ErrUnavailable
	}
	var dbOrigin origin
	var db []byte
	var phase string
	e = tx.QueryRow(ctx, `SELECT environment_id,installation_id,authority_incarnation,database_binding_sha256,phase FROM story_lfp_admission_head WHERE singleton FOR SHARE`).Scan(&dbOrigin.environment, &dbOrigin.installation, &dbOrigin.incarnation, &db, &phase)
	if len(db) == 32 {
		copy(dbOrigin.database[:], db)
	}
	if e != nil || !dbOrigin.valid() || phase != "open" {
		_ = guard.Rollback(ctx)
		confirmedRelease()
		return nil, origin{}, nil, ErrUnavailable
	}
	if o != dbOrigin {
		_ = guard.Rollback(ctx)
		confirmedRelease()
		return nil, origin{}, nil, ErrStale
	}
	// Shared row lock spans the WHOLE owning transaction, including commit.
	// Controller close updates this row only after external ingress is closed.
	return guard, o, confirmedRelease, nil
}

// completionTx deliberately keeps an unresolved permit restrictive. In pgx,
// Commit marks a transaction closed even on failure; a later ErrTxClosed or an
// asynchronously closed connection does not prove native COMMIT quiescence.
// No timeout or DB marker can release this guard. The actual controller must
// retire the old runtime and its database sessions before admitting replacement
// execution. This package has no operation that pretends that custody exists.
type completionTx struct {
	pgx.Tx
	commitAttempted bool
	terminal        bool
	negative        lfperasure.View
	origin          origin
}

func negativeReferences(o origin) lfperasure.References {
	return lfperasure.References{CohortEnvironment: o.environment, CohortInstallation: o.installation, CohortIncarnation: o.incarnation, DatabaseBinding: o.database}
}

func negativeError(e error) error {
	if errors.Is(e, lfperasure.ErrErased) {
		return ErrErased
	}
	return ErrUnavailable
}

// Only immutable locally acquired snapshot checks occur inside SQL.
func checkNegative(tx pgx.Tx, refs lfperasure.References) error {
	t, ok := tx.(*completionTx)
	if !ok || t.negative == nil {
		return ErrUnavailable
	}
	refs.CohortEnvironment = t.origin.environment
	refs.CohortInstallation = t.origin.installation
	refs.CohortIncarnation = t.origin.incarnation
	refs.DatabaseBinding = t.origin.database
	if e := lfperasure.Check(t.negative, refs); e != nil {
		return negativeError(e)
	}
	return nil
}

func (t *completionTx) Commit(ctx context.Context) error {
	t.commitAttempted = true
	e := t.Tx.Commit(ctx)
	if e == nil {
		t.terminal = true
	}
	return e
}

func (t *completionTx) Rollback(ctx context.Context) error {
	e := t.Tx.Rollback(ctx)
	if e == nil && !t.commitAttempted {
		t.terminal = true
	}
	return e
}
