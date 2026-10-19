// Package lfpconsentstore owns private durable LFP data. It cannot authenticate
// a user, install a restore cohort or turn receipt bytes into MM authority.
package lfpconsentstore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"voice/backend/pkg/storylfpadmission"
)

var (
	ErrUnavailable = errors.New("LFP consent journal unavailable")
	ErrConflict    = errors.New("LFP consent operation conflict")
	ErrStale       = errors.New("LFP offer changed or unavailable")
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
}

// NewBoundStore never creates authority. Only a controller-verified adapter can
// provide admission; nil/unknown authority leaves every qualified API closed.
func NewBoundStore(pool *pgxpool.Pool, barrier *storylfpadmission.Barrier) *Store {
	return &Store{Pool: pool, authority: barrierAdmission{barrier}}
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
	o, release, e := s.authority.enter(ctx)
	if e != nil || !o.valid() || release == nil {
		if release != nil {
			release()
		}
		if errors.Is(e, ErrStale) {
			return nil, origin{}, nil, ErrStale
		}
		return nil, origin{}, nil, ErrUnavailable
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		release()
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
		_ = tx.Rollback(ctx)
		release()
		return nil, origin{}, nil, ErrUnavailable
	}
	if o != dbOrigin {
		_ = tx.Rollback(ctx)
		release()
		return nil, origin{}, nil, ErrStale
	}
	// Shared row lock spans the WHOLE owning transaction, including commit.
	// Controller close updates this row only after external ingress is closed.
	return tx, o, release, nil
}
