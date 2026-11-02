// Package sourcecustody provides a closed-by-default local control kernel.
// It does not qualify a source or authorize a physical source transition.
package sourcecustody

import (
	"context"
	"crypto/rand"
	"errors"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	leaderKey int64 = 6219270163691208961
	permitKey int64 = 6219270163691208962
)

var (
	ErrUnavailable  = errors.New("source custody unavailable")
	sourceIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
)

type Binding struct {
	Epoch                int64
	SourceID             string
	HistoryGeneration    uuid.UUID
	UserSourceGeneration uuid.UUID
	KeysetDigest         [32]byte
}

func (b Binding) valid() bool {
	return b.Epoch > 0 && sourceIDPattern.MatchString(b.SourceID) &&
		b.HistoryGeneration != uuid.Nil && b.UserSourceGeneration != uuid.Nil && b.HistoryGeneration != b.UserSourceGeneration && b.KeysetDigest != [32]byte{}
}

// Valid checks the shape of a configured binding; it does not qualify custody.
func (b Binding) Valid() bool { return b.valid() }

type Controller struct {
	pool                           *pgxpool.Pool
	leader                         *pgxpool.Conn
	binding                        Binding
	controlSystem, controlDatabase string
	controlDatabaseOID             uint32
	activationChallenge            [32]byte
	activationUsed                 bool
	activationPrepared             bool
	authorizedOpen                 bool
	mu                             sync.Mutex
	stopped                        bool
	failed                         bool
	active                         int
	changed                        chan struct{}
	failure                        chan struct{}
	// Test-only synchronization; never used as an authority or production gate.
	afterBarrier        func()
	beforeWait          func()
	afterPrepareIntent  func(uint32)
	afterPrepareEnabled func()
	parent              context.Context
	watchStop           chan struct{}
	watchDone           chan struct{}
	watchOnce           sync.Once
}

type Permit struct {
	mu         sync.Mutex
	conn       *pgxpool.Conn
	physicalTx pgx.Tx
	physical   PhysicalSource
	owner      *Controller
}

// Start takes sole control leadership and commits CLOSED even if the stored
// state was OPEN. ctx is the controller lifetime, and callers must still call
// Shutdown to join its watcher and release leadership. This is not source
// qualification and has no opening API.
func Start(ctx context.Context, pool *pgxpool.Pool, binding Binding) (*Controller, error) {
	if ctx == nil || pool == nil || !binding.valid() {
		return nil, ErrUnavailable
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, ErrUnavailable
	}
	var locked bool
	if err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, leaderKey).Scan(&locked); err != nil || !locked {
		if err != nil {
			destroy(conn)
		} else {
			conn.Release()
		}
		return nil, ErrUnavailable
	}
	var source, state string
	var epoch int64
	var h, u uuid.UUID
	var digest []byte
	err = conn.QueryRow(ctx, `UPDATE public.user_custody_state SET state='CLOSED' WHERE singleton RETURNING state,epoch,source_id,history_generation,user_source_generation,keyset_digest`).Scan(&state, &epoch, &source, &h, &u, &digest)
	if err != nil || state != "CLOSED" || !same(binding, epoch, source, h, u, digest) {
		destroy(conn)
		return nil, ErrUnavailable
	}
	var controlSystem, controlDatabase string
	var controlOID uint32
	if conn.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&controlSystem, &controlDatabase, &controlOID) != nil || !systemID.MatchString(controlSystem) || controlDatabase == "" || controlOID == 0 {
		destroy(conn)
		return nil, ErrUnavailable
	}
	c := &Controller{pool: pool, leader: conn, binding: binding, controlSystem: controlSystem, controlDatabase: controlDatabase, controlDatabaseOID: controlOID, parent: ctx, changed: make(chan struct{}), failure: make(chan struct{})}
	if _, err = rand.Read(c.activationChallenge[:]); err != nil || c.activationChallenge == [32]byte{} {
		destroy(conn)
		return nil, ErrUnavailable
	}
	if ctx.Done() != nil {
		c.watchStop = make(chan struct{})
		c.watchDone = make(chan struct{})
		go c.watchParent()
	}
	return c, nil
}

// ActivationChallenge is freshly generated after this leader closed admission.
// It is not itself an authorization and is invalid after this leader exits.
func (c *Controller) ActivationChallenge() ([32]byte, error) {
	if c == nil {
		return [32]byte{}, ErrUnavailable
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.leader == nil || c.parent.Err() != nil || c.activationUsed {
		return [32]byte{}, ErrUnavailable
	}
	return c.activationChallenge, nil
}

func (c *Controller) matchesControl(ctx context.Context, conn *pgxpool.Conn) bool {
	var system, database string
	var oid uint32
	return conn.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&system, &database, &oid) == nil && system == c.controlSystem && database == c.controlDatabase && oid == c.controlDatabaseOID
}

func (c *Controller) rejectControl(ctx context.Context, conn *pgxpool.Conn) {
	destroy(conn)
	_ = c.CloseAdmission(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.stopped {
		c.poisonLocked()
	}
}

func (c *Controller) watchParent() {
	defer close(c.watchDone)
	select {
	case <-c.watchStop:
		return
	case <-c.parent.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.CloseAdmission(ctx)
	}
}

func (c *Controller) stopWatcher() {
	if c.watchStop == nil {
		return
	}
	c.watchOnce.Do(func() { close(c.watchStop) })
	<-c.watchDone
}

func same(b Binding, epoch int64, source string, h, u uuid.UUID, digest []byte) bool {
	return b.Epoch == epoch && b.SourceID == source && b.HistoryGeneration == h && b.UserSourceGeneration == u && len(digest) == len(b.KeysetDigest) && string(digest) == string(b.KeysetDigest[:])
}

// Acquire retains a control-session permit. A permit is not a bearer grant,
// source provenance, or proof that an independent User transaction stopped.
func (c *Controller) Acquire(ctx context.Context, expected Binding) (*Permit, error) {
	if c == nil || ctx == nil || !expected.valid() || expected != c.binding {
		return nil, ErrUnavailable
	}
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		return nil, ErrUnavailable
	}
	if !c.matchesControl(ctx, conn) {
		if ctx.Err() != nil {
			destroy(conn)
		} else {
			c.rejectControl(ctx, conn)
		}
		return nil, ErrUnavailable
	}
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock_shared($1::bigint)`, permitKey); err != nil {
		destroy(conn)
		return nil, ErrUnavailable
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.parent.Err() != nil || ctx.Err() != nil {
		destroy(conn)
		return nil, ErrUnavailable
	}
	// Request cancellation must not cancel a query on the shared leadership
	// session. A genuinely failed or timed-out leadership check still poisons it.
	pingCtx, stopPing := context.WithTimeout(context.Background(), 3*time.Second)
	leaderErr := ErrUnavailable
	if c.leader != nil {
		leaderErr = c.leader.Ping(pingCtx)
	}
	stopPing()
	if leaderErr != nil {
		c.poisonLocked()
		destroy(conn)
		return nil, ErrUnavailable
	}
	if ctx.Err() != nil || c.parent.Err() != nil {
		destroy(conn)
		return nil, ErrUnavailable
	}
	var state, source string
	var epoch int64
	var h, u uuid.UUID
	var digest []byte
	err = conn.QueryRow(ctx, `SELECT state,epoch,source_id,history_generation,user_source_generation,keyset_digest FROM public.user_custody_state WHERE singleton`).Scan(&state, &epoch, &source, &h, &u, &digest)
	if err != nil || state != "OPEN" || ctx.Err() != nil || c.parent.Err() != nil || !same(expected, epoch, source, h, u, digest) {
		destroy(conn)
		return nil, ErrUnavailable
	}
	c.active++
	return &Permit{conn: conn, owner: c}, nil
}

// AcquireQualified retains the control-plane physical binding under a row
// lock until Release. Its return is not itself a source-currentness assertion;
// callers must compare the binding with their actual source transaction and
// enforce exclusive source and restore custody.
func (c *Controller) AcquireQualified(ctx context.Context, expected Binding) (*Permit, PhysicalSource, error) {
	var physical PhysicalSource
	permit, err := c.Acquire(ctx, expected)
	if err != nil {
		return nil, physical, ErrUnavailable
	}
	cleanup := func() {
		bounded, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = permit.Release(bounded)
	}
	tx, err := permit.conn.Begin(ctx)
	if err != nil {
		cleanup()
		return nil, physical, ErrUnavailable
	}
	permit.physicalTx = tx
	if tx.QueryRow(ctx, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2 FOR SHARE`, expected.SourceID, expected.Epoch).Scan(&physical.SystemIdentifier, &physical.Database, &physical.DatabaseOID) != nil || !systemID.MatchString(physical.SystemIdentifier) || physical.SystemIdentifier == c.controlSystem || !databaseName.MatchString(physical.Database) || physical.DatabaseOID == 0 || permit.Check(ctx) != nil {
		cleanup()
		return nil, PhysicalSource{}, ErrUnavailable
	}
	permit.physical = physical
	return permit, physical, nil
}

// AcquireAttested requires this leader to have completed the signed opening.
func (c *Controller) AcquireAttested(ctx context.Context, expected Binding) (*Permit, PhysicalSource, error) {
	permit, physical, err := c.AcquireQualified(ctx, expected)
	if err != nil {
		return nil, PhysicalSource{}, ErrUnavailable
	}
	c.mu.Lock()
	valid := c.authorizedOpen && !c.stopped && c.parent.Err() == nil
	c.mu.Unlock()
	if !valid {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = permit.Release(cleanup)
		return nil, PhysicalSource{}, ErrUnavailable
	}
	return permit, physical, nil
}

// CloseAdmission persists CLOSED without waiting for outstanding permits.
func (c *Controller) CloseAdmission(ctx context.Context) error {
	if c == nil || ctx == nil {
		return ErrUnavailable
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authorizedOpen = false
	if c.stopped || c.leader == nil {
		return ErrUnavailable
	}
	var state, source string
	var epoch int64
	var h, u uuid.UUID
	var digest []byte
	if err := c.leader.QueryRow(ctx, `UPDATE public.user_custody_state SET state='CLOSED' WHERE singleton RETURNING state,epoch,source_id,history_generation,user_source_generation,keyset_digest`).Scan(&state, &epoch, &source, &h, &u, &digest); err != nil || state != "CLOSED" || !same(c.binding, epoch, source, h, u, digest) {
		c.poisonLocked()
		return ErrUnavailable
	}
	return nil
}

// WaitPermits only waits for control sessions. Lost sessions can leave User
// transactions running; source replacement requires independent fencing.
func (c *Controller) WaitPermits(ctx context.Context) error {
	if c == nil || ctx == nil {
		return ErrUnavailable
	}
	for {
		done, changed, err := c.waitPermitPass(ctx)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if c.beforeWait != nil {
			c.beforeWait()
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ErrUnavailable
		}
	}
}

func (c *Controller) waitPermitPass(ctx context.Context) (bool, <-chan struct{}, error) {
	c.mu.Lock()
	if c.stopped || c.leader == nil || ctx.Err() != nil {
		c.mu.Unlock()
		return false, nil, ErrUnavailable
	}
	var state, source string
	var epoch int64
	var h, u uuid.UUID
	var digest []byte
	observe, stopObserve := context.WithTimeout(context.Background(), 3*time.Second)
	err := c.leader.QueryRow(observe, `SELECT state,epoch,source_id,history_generation,user_source_generation,keyset_digest FROM public.user_custody_state WHERE singleton`).Scan(&state, &epoch, &source, &h, &u, &digest)
	stopObserve()
	if err != nil || !same(c.binding, epoch, source, h, u, digest) {
		c.poisonLocked()
		c.mu.Unlock()
		return false, nil, ErrUnavailable
	}
	c.mu.Unlock()
	if state != "CLOSED" || ctx.Err() != nil {
		return false, nil, ErrUnavailable
	}
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		return false, nil, ErrUnavailable
	}
	if !c.matchesControl(ctx, conn) {
		if ctx.Err() != nil {
			destroy(conn)
		} else {
			c.rejectControl(ctx, conn)
		}
		return false, nil, ErrUnavailable
	}
	if _, err = conn.Exec(ctx, `SELECT pg_advisory_lock($1::bigint)`, permitKey); err != nil {
		destroy(conn)
		return false, nil, ErrUnavailable
	}
	if c.afterBarrier != nil {
		c.afterBarrier()
	}
	// The leader can die or be shut down while the exclusive lock is waiting.
	// Recheck under controller serialization before reporting a barrier result.
	c.mu.Lock()
	if c.stopped || ctx.Err() != nil {
		c.mu.Unlock()
		destroy(conn)
		return false, nil, ErrUnavailable
	}
	observe, stopObserve = context.WithTimeout(context.Background(), 3*time.Second)
	valid := c.leader != nil && c.leader.Ping(observe) == nil
	if valid {
		err = c.leader.QueryRow(observe, `SELECT state,epoch,source_id,history_generation,user_source_generation,keyset_digest FROM public.user_custody_state WHERE singleton`).Scan(&state, &epoch, &source, &h, &u, &digest)
		valid = err == nil && state == "CLOSED" && same(c.binding, epoch, source, h, u, digest)
	}
	stopObserve()
	if !valid {
		c.poisonLocked()
		c.mu.Unlock()
		destroy(conn)
		return false, nil, ErrUnavailable
	}
	done := c.active == 0
	changed := c.changed
	var unlocked bool
	cleanup, stopCleanup := context.WithTimeout(context.Background(), 3*time.Second)
	err = conn.QueryRow(cleanup, `SELECT pg_advisory_unlock($1::bigint)`, permitKey).Scan(&unlocked)
	stopCleanup()
	if err != nil || !unlocked {
		c.poisonLocked()
		c.mu.Unlock()
		destroy(conn)
		return false, nil, ErrUnavailable
	}
	c.mu.Unlock()
	conn.Release()
	if ctx.Err() != nil {
		return false, nil, ErrUnavailable
	}
	return done, changed, nil
}

func (p *Permit) Release(ctx context.Context) error {
	if p == nil || ctx == nil {
		return ErrUnavailable
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn == nil {
		return ErrUnavailable
	}
	conn := p.conn
	p.conn = nil
	var unlocked bool
	var err error
	// Cleanup must still run if the caller expired while waiting for p.mu.
	// An actual rollback or unlock failure remains an uncertain transition.
	cleanup, stopCleanup := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCleanup()
	if p.physicalTx != nil {
		err = p.physicalTx.Rollback(cleanup)
		p.physicalTx = nil
	}
	if err == nil {
		err = conn.QueryRow(cleanup, `SELECT pg_advisory_unlock_shared($1::bigint)`, permitKey).Scan(&unlocked)
	}
	if err != nil || !unlocked {
		destroy(conn)
	} else {
		conn.Release()
	}
	if p.owner != nil {
		p.owner.mu.Lock()
		p.owner.active--
		close(p.owner.changed)
		p.owner.changed = make(chan struct{})
		if err != nil || !unlocked {
			p.owner.poisonLocked()
		}
		p.owner.mu.Unlock()
	}
	if err != nil || !unlocked {
		return ErrUnavailable
	}
	return nil
}

// Check observes the still-held control session and the controller's current
// local binding. It is not a fence for an independent User transaction.
func (p *Permit) Check(ctx context.Context) error {
	if p == nil || ctx == nil {
		return ErrUnavailable
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn == nil || p.owner == nil {
		return ErrUnavailable
	}
	c := p.owner
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.parent.Err() != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	observe, stopObserve := context.WithTimeout(context.Background(), 3*time.Second)
	defer stopObserve()
	if c.leader == nil || c.leader.Ping(observe) != nil || p.conn.Ping(observe) != nil {
		c.poisonLocked()
		return ErrUnavailable
	}
	if p.physicalTx != nil && p.physical != (PhysicalSource{}) {
		var physical PhysicalSource
		if p.physicalTx.QueryRow(observe, `SELECT system_identifier,database_name,database_oid FROM public.user_custody_physical_source WHERE source_id=$1 AND epoch=$2`, c.binding.SourceID, c.binding.Epoch).Scan(&physical.SystemIdentifier, &physical.Database, &physical.DatabaseOID) != nil || physical != p.physical {
			c.poisonLocked()
			return ErrUnavailable
		}
	}
	var state, source string
	var epoch int64
	var h, u uuid.UUID
	var digest []byte
	err := c.leader.QueryRow(observe, `SELECT state,epoch,source_id,history_generation,user_source_generation,keyset_digest FROM public.user_custody_state WHERE singleton`).Scan(&state, &epoch, &source, &h, &u, &digest)
	if err != nil || (state != "OPEN" && state != "CLOSED") || !same(c.binding, epoch, source, h, u, digest) {
		c.poisonLocked()
		return ErrUnavailable
	}
	if ctx.Err() != nil || c.parent.Err() != nil {
		return ErrUnavailable
	}
	return nil
}

func (c *Controller) Shutdown(ctx context.Context) error {
	if c == nil || ctx == nil {
		return ErrUnavailable
	}
	defer c.stopWatcher()
	c.mu.Lock()
	if c.stopped {
		failed := c.failed
		c.mu.Unlock()
		if failed {
			return ErrUnavailable
		}
		return nil
	}
	c.mu.Unlock()
	if err := c.CloseAdmission(ctx); err != nil {
		return err
	}
	if err := c.WaitPermits(ctx); err != nil {
		c.mu.Lock()
		c.poisonLocked()
		c.mu.Unlock()
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		if c.failed {
			return ErrUnavailable
		}
		return nil
	}
	c.stopped = true
	conn := c.leader
	c.leader = nil
	var unlocked bool
	if err := conn.QueryRow(ctx, `SELECT pg_advisory_unlock($1::bigint)`, leaderKey).Scan(&unlocked); err != nil || !unlocked {
		c.failed = true
		destroy(conn)
		return ErrUnavailable
	}
	conn.Release()
	return nil
}

// poisonLocked prevents future grants after a failed or uncertain control
// operation. It does not claim the durable CLOSED update committed.
func (c *Controller) poisonLocked() {
	c.authorizedOpen = false
	if !c.failed && c.failure != nil {
		close(c.failure)
	}
	c.failed = true
	c.stopped = true
	if c.changed != nil {
		close(c.changed)
		c.changed = make(chan struct{})
	}
	if c.leader != nil {
		destroy(c.leader)
		c.leader = nil
	}
}

func destroy(conn *pgxpool.Conn) {
	if conn == nil {
		return
	}
	raw := conn.Hijack()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = raw.Close(ctx)
}
