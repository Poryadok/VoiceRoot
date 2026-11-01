package sourcecustody

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// An uncertain close must not let this controller issue another permit even
// when its previous control connection is still usable.
func TestCustodyUncertainClosePoisonsController(t *testing.T) {
	ctx := context.Background()
	control := custodyDatabase(t, ctx)
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	require.NoError(t, err)
	b := custodyBinding()
	_, err = control.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'OPEN',$1,$2,$3,$4,$5)`, b.Epoch, b.SourceID, b.HistoryGeneration, b.UserSourceGeneration, b.KeysetDigest[:])
	require.NoError(t, err)
	c, err := Start(ctx, control, b)
	require.NoError(t, err)
	t.Cleanup(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.leader != nil {
			destroy(c.leader)
			c.leader = nil
		}
	})
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
	require.NoError(t, err)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.Error(t, c.CloseAdmission(canceled))
	p, err := c.Acquire(ctx, b)
	if p != nil {
		_ = p.Release(ctx)
	}
	require.Error(t, err, "uncertain closure must not admit a new permit")
	require.Error(t, c.Shutdown(ctx), "uncertain closure cannot be reported as a clean shutdown")
	require.Error(t, c.Shutdown(ctx), "repeated shutdown must retain the failure")
}

func TestCustodyCleanShutdownStaysCleanAfterRejectedAcquire(t *testing.T) {
	ctx, _, _, binding, c := custodyFixture(t)
	require.NoError(t, c.Shutdown(ctx))
	permit, err := c.Acquire(ctx, binding)
	if permit != nil {
		_ = permit.Release(ctx)
	}
	require.Error(t, err)
	require.NoError(t, c.Shutdown(ctx), "a rejected acquire must not turn a clean shutdown into failure")
}

func TestCustodyWaitRejectsChangedBinding(t *testing.T) {
	ctx := context.Background()
	control := custodyDatabase(t, ctx)
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	require.NoError(t, err)
	b := custodyBinding()
	_, err = control.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'CLOSED',$1,$2,$3,$4,$5)`, b.Epoch, b.SourceID, b.HistoryGeneration, b.UserSourceGeneration, b.KeysetDigest[:])
	require.NoError(t, err)
	c, err := Start(ctx, control, b)
	require.NoError(t, err)
	t.Cleanup(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.leader != nil {
			destroy(c.leader)
			c.leader = nil
		}
	})
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET epoch=epoch+1 WHERE singleton`)
	require.NoError(t, err)
	require.Error(t, c.WaitPermits(ctx), "barrier must not succeed against a changed source binding")
}

func TestCustodyLostPermitPoisonsController(t *testing.T) {
	ctx := context.Background()
	control := custodyDatabase(t, ctx)
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	require.NoError(t, err)
	b := custodyBinding()
	_, err = control.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'CLOSED',$1,$2,$3,$4,$5)`, b.Epoch, b.SourceID, b.HistoryGeneration, b.UserSourceGeneration, b.KeysetDigest[:])
	require.NoError(t, err)
	c, err := Start(ctx, control, b)
	require.NoError(t, err)
	t.Cleanup(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.leader != nil {
			destroy(c.leader)
			c.leader = nil
		}
	})
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
	require.NoError(t, err)
	p, err := c.Acquire(ctx, b)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Release(ctx) })
	var pid int32
	require.NoError(t, control.QueryRow(ctx, `SELECT min(pid) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=($1::bigint >> 32)::oid AND objid=($1::bigint & 4294967295)::oid HAVING count(*)=1`, permitKey).Scan(&pid))
	var terminated bool
	require.NoError(t, control.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated))
	require.True(t, terminated)
	require.Error(t, p.Release(ctx))
	second, err := c.Acquire(ctx, b)
	if second != nil {
		_ = second.Release(ctx)
	}
	require.Error(t, err, "lost permit must prevent further grants")
	require.Error(t, c.Shutdown(ctx), "loss cannot be reported as clean shutdown")
}

func TestCustodyShutdownCannotHideUnsettledLostPermit(t *testing.T) {
	ctx, control, _, binding, c := custodyFixtureWithFailure(t, true)
	p, err := c.Acquire(ctx, binding)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Release(ctx) })
	var pid int32
	require.NoError(t, control.QueryRow(ctx, `SELECT min(pid) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=($1::bigint >> 32)::oid AND objid=($1::bigint & 4294967295)::oid HAVING count(*)=1`, permitKey).Scan(&pid))
	var terminated bool
	require.NoError(t, control.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated))
	require.True(t, terminated)
	require.Eventually(t, func() bool {
		var alive bool
		err := control.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1)`, pid).Scan(&alive)
		return err == nil && !alive
	}, 5*time.Second, 20*time.Millisecond)
	bounded, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	require.Error(t, c.Shutdown(bounded), "lost lock is not a settled permit")
	require.Error(t, p.Release(ctx))
	require.Error(t, c.Shutdown(ctx), "failed permit settlement cannot become clean")
}

func TestCustodyPoisonWakesPermitWaiter(t *testing.T) {
	ctx, control, _, binding, c := custodyFixtureWithFailure(t, true)
	p, err := c.Acquire(ctx, binding)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Release(ctx) })
	var pid int32
	require.NoError(t, control.QueryRow(ctx, `SELECT min(pid) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=($1::bigint >> 32)::oid AND objid=($1::bigint & 4294967295)::oid HAVING count(*)=1`, permitKey).Scan(&pid))
	var terminated bool
	require.NoError(t, control.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, pid).Scan(&terminated))
	require.True(t, terminated)
	require.NoError(t, c.CloseAdmission(ctx))
	reached, resume := make(chan struct{}), make(chan struct{})
	var resumeOnce sync.Once
	t.Cleanup(func() { resumeOnce.Do(func() { close(resume) }) })
	c.beforeWait = func() { close(reached); <-resume }
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result := make(chan error, 1)
	got := false
	go func() { result <- c.WaitPermits(bounded) }()
	t.Cleanup(func() {
		cancel()
		resumeOnce.Do(func() { close(resume) })
		if !got {
			select {
			case <-result:
			case <-time.After(5 * time.Second):
				t.Error("permit waiter did not terminate")
			}
		}
	})
	select {
	case <-reached:
	case <-bounded.Done():
		t.Fatal("waiter did not reach local settlement wait")
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	require.Error(t, c.CloseAdmission(canceled))
	resumeOnce.Do(func() { close(resume) })
	select {
	case err = <-result:
		got = true
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("poisoned controller did not wake waiter")
	}
	require.Error(t, p.Release(ctx))
}

func TestCustodyWaitRechecksBindingAfterBarrier(t *testing.T) {
	ctx := context.Background()
	control := custodyDatabase(t, ctx)
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	require.NoError(t, err)
	b := custodyBinding()
	_, err = control.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'CLOSED',$1,$2,$3,$4,$5)`, b.Epoch, b.SourceID, b.HistoryGeneration, b.UserSourceGeneration, b.KeysetDigest[:])
	require.NoError(t, err)
	c, err := Start(ctx, control, b)
	require.NoError(t, err)
	t.Cleanup(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.leader != nil {
			destroy(c.leader)
			c.leader = nil
		}
	})
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
	require.NoError(t, err)
	p, err := c.Acquire(ctx, b)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Release(ctx) })
	require.NoError(t, c.CloseAdmission(ctx))
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result := make(chan error, 1)
	got := false
	go func() { result <- c.WaitPermits(bounded) }()
	t.Cleanup(func() {
		cancel()
		_ = p.Release(ctx)
		if !got {
			select {
			case <-result:
			case <-time.After(5 * time.Second):
				t.Error("barrier waiter did not terminate")
			}
		}
	})
	require.Eventually(t, func() bool {
		var waiting bool
		err := control.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND mode='ExclusiveLock' AND NOT granted AND objsubid=1 AND classid=($1::bigint >> 32)::oid AND objid=($1::bigint & 4294967295)::oid)`, permitKey).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET epoch=epoch+1 WHERE singleton`)
	require.NoError(t, err)
	require.NoError(t, p.Release(ctx))
	select {
	case err = <-result:
		got = true
		require.Error(t, err, "binding change during wait must deny barrier")
	case <-bounded.Done():
		t.Fatal("barrier waiter did not terminate")
	}
}

func TestCustodyParentCancellationClosesAdmission(t *testing.T) {
	ctx := context.Background()
	control := custodyDatabase(t, ctx)
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	require.NoError(t, err)
	b := custodyBinding()
	_, err = control.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'CLOSED',$1,$2,$3,$4,$5)`, b.Epoch, b.SourceID, b.HistoryGeneration, b.UserSourceGeneration, b.KeysetDigest[:])
	require.NoError(t, err)
	parent, cancel := context.WithCancel(ctx)
	c, err := Start(parent, control, b)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		c.stopWatcher()
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.leader != nil {
			destroy(c.leader)
			c.leader = nil
		}
	})
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
	require.NoError(t, err)
	cancel()
	require.Eventually(t, func() bool {
		var state string
		return control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state) == nil && state == "CLOSED"
	}, 5*time.Second, 20*time.Millisecond, "parent cancellation must close admission")
	p, err := c.Acquire(ctx, b)
	if p != nil {
		_ = p.Release(ctx)
	}
	require.Error(t, err)
}

func TestCustodyAcquireRejectsCancellationDuringStateRead(t *testing.T) {
	ctx := context.Background()
	control := custodyDatabase(t, ctx)
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	require.NoError(t, err)
	b := custodyBinding()
	_, err = control.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'CLOSED',$1,$2,$3,$4,$5)`, b.Epoch, b.SourceID, b.HistoryGeneration, b.UserSourceGeneration, b.KeysetDigest[:])
	require.NoError(t, err)
	parent, cancel := context.WithCancel(ctx)
	c, err := Start(parent, control, b)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		c.stopWatcher()
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.leader != nil {
			destroy(c.leader)
			c.leader = nil
		}
	})
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
	require.NoError(t, err)
	blocker, err := control.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocker.Rollback(ctx) })
	_, err = blocker.Exec(ctx, `LOCK TABLE user_custody_state IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	type result struct {
		permit *Permit
		err    error
	}
	resultCh := make(chan result, 1)
	acquireCtx, acquireCancel := context.WithTimeout(ctx, 10*time.Second)
	got := false
	var received result
	go func() { p, e := c.Acquire(acquireCtx, b); resultCh <- result{p, e} }()
	t.Cleanup(func() {
		acquireCancel()
		_ = blocker.Rollback(ctx)
		if !got {
			select {
			case received = <-resultCh:
			case <-time.After(5 * time.Second):
				t.Error("acquire goroutine did not terminate")
			}
		}
		if received.permit != nil {
			_ = received.permit.Release(ctx)
		}
	})
	require.Eventually(t, func() bool {
		var waiting bool
		err := control.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='public.user_custody_state'::regclass AND mode='AccessShareLock' AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	cancel()
	require.NoError(t, blocker.Commit(ctx))
	select {
	case received = <-resultCh:
		got = true
	case <-acquireCtx.Done():
		t.Fatal("acquire goroutine did not return")
	}
	require.Error(t, received.err, "cancellation during state read must not return a permit")
}

func TestCustodyWaitAfterConcurrentCleanShutdownDoesNotPoison(t *testing.T) {
	ctx, control, _, binding, c := custodyFixture(t)
	p, err := c.Acquire(ctx, binding)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Release(ctx) })
	require.NoError(t, c.CloseAdmission(ctx))
	var calls atomic.Int32
	reached, resume := make(chan struct{}), make(chan struct{})
	firstResume := make(chan struct{})
	var resumeOnce sync.Once
	t.Cleanup(func() { resumeOnce.Do(func() { close(resume) }) })
	var firstOnce sync.Once
	t.Cleanup(func() { firstOnce.Do(func() { close(firstResume) }) })
	c.afterBarrier = func() {
		switch calls.Add(1) {
		case 1:
			<-firstResume
		case 2:
			close(reached)
			<-resume
		}
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	shutdown := make(chan error, 1)
	go func() { shutdown <- c.Shutdown(bounded) }()
	waiting := func(want int) bool {
		var count int
		err := control.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ExclusiveLock' AND NOT granted AND objsubid=1 AND classid=($1::bigint >> 32)::oid AND objid=($1::bigint & 4294967295)::oid`, permitKey).Scan(&count)
		return err == nil && count >= want
	}
	require.Eventually(t, func() bool { return waiting(1) }, 5*time.Second, 20*time.Millisecond)
	second := make(chan error, 1)
	go func() { second <- c.WaitPermits(bounded) }()
	require.Eventually(t, func() bool { return waiting(2) }, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, p.Release(ctx))
	firstOnce.Do(func() { close(firstResume) })
	select {
	case <-reached:
	case <-bounded.Done():
		t.Fatal("second waiter did not acquire the barrier")
	}
	require.NoError(t, <-shutdown)
	resumeOnce.Do(func() { close(resume) })
	require.Error(t, <-second, "an old waiter must not report a live controller")
	require.NoError(t, c.Shutdown(ctx), "old waiter must not poison clean shutdown")
}

// A drained permit barrier is not a successful controller observation if the
// leader was lost while the barrier waited.
func TestCustodyWaitRejectsLeaderLossDuringDrain(t *testing.T) {
	ctx := context.Background()
	control := custodyDatabase(t, ctx)
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY CHECK(singleton),state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	require.NoError(t, err)
	b := custodyBinding()
	_, err = control.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'OPEN',$1,$2,$3,$4,$5)`, b.Epoch, b.SourceID, b.HistoryGeneration, b.UserSourceGeneration, b.KeysetDigest[:])
	require.NoError(t, err)
	c, err := Start(ctx, control, b)
	require.NoError(t, err)
	t.Cleanup(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.leader != nil {
			destroy(c.leader)
			c.leader = nil
		}
	})
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN' WHERE singleton`)
	require.NoError(t, err)
	p, err := c.Acquire(ctx, b)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Release(ctx) })
	require.NoError(t, c.CloseAdmission(ctx))
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- c.WaitPermits(waitCtx) }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := control.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND mode='ExclusiveLock' AND NOT granted AND objsubid=1 AND classid=($1::bigint >> 32)::oid AND objid=($1::bigint & 4294967295)::oid)`, permitKey).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	var leaderPID int32
	require.NoError(t, control.QueryRow(ctx, `SELECT min(pid) FROM pg_locks WHERE locktype='advisory' AND mode='ExclusiveLock' AND granted AND objsubid=1 AND classid=($1::bigint >> 32)::oid AND objid=($1::bigint & 4294967295)::oid HAVING count(*)=1`, leaderKey).Scan(&leaderPID))
	var terminated bool
	require.NoError(t, control.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, leaderPID).Scan(&terminated))
	require.True(t, terminated)
	require.NoError(t, p.Release(ctx))
	require.Error(t, <-result, "drain cannot succeed after leadership loss")
}
