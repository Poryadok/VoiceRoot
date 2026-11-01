package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

type monitorCancelTracer struct {
	match  string
	cancel context.CancelFunc
}
type monitorTraceKey struct{}

type monitorTimingKey struct{}
type monitorTimingEventKey struct{}
type monitorTiming struct {
	mu      sync.Mutex
	started time.Time
	events  []string
}
type monitorTimingEvent struct {
	name  string
	start time.Time
}
type monitorTimingTracer struct{}

func (monitorTimingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	trace, ok := ctx.Value(monitorTimingKey{}).(*monitorTiming)
	if !ok {
		return ctx
	}
	name := "other"
	sql := strings.ToLower(strings.TrimSpace(data.SQL))
	switch {
	case strings.HasPrefix(sql, "begin"):
		name = "begin"
	case strings.HasPrefix(sql, "set local"):
		name = "set-path"
	case strings.HasPrefix(sql, "select pg_catalog.current_setting"):
		name = "guard-path"
	case strings.HasPrefix(sql, "select session_user"):
		name = "guard-qualification"
	case strings.HasPrefix(sql, "insert") && strings.Contains(sql, "user_custody_monitor_"):
		name = "store-insert"
	case strings.HasPrefix(sql, "select") && strings.Contains(sql, "user_custody_monitor_"):
		name = "store-read"
	case strings.HasPrefix(sql, "commit"):
		name = "commit"
	case strings.HasPrefix(sql, "rollback"):
		name = "rollback"
	}
	now := time.Now()
	trace.mu.Lock()
	trace.events = append(trace.events, fmt.Sprintf("%s start +%s", name, now.Sub(trace.started)))
	trace.mu.Unlock()
	return context.WithValue(ctx, monitorTimingEventKey{}, monitorTimingEvent{name, now})
}
func (monitorTimingTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	trace, ok := ctx.Value(monitorTimingKey{}).(*monitorTiming)
	if !ok {
		return
	}
	event, ok := ctx.Value(monitorTimingEventKey{}).(monitorTimingEvent)
	if !ok {
		return
	}
	trace.mu.Lock()
	trace.events = append(trace.events, fmt.Sprintf("%s end %s err=%v", event.name, time.Since(event.start), data.Err))
	trace.mu.Unlock()
}
func timedMonitorCall(t *testing.T, ctx context.Context, name string, call func(context.Context) error) error {
	t.Helper()
	trace := &monitorTiming{started: time.Now()}
	err := call(context.WithValue(ctx, monitorTimingKey{}, trace))
	trace.mu.Lock()
	defer trace.mu.Unlock()
	t.Logf("%s elapsed=%s err=%v stages=%v", name, time.Since(trace.started), err, trace.events)
	return err
}

type timedProbeStore struct {
	t     *testing.T
	inner *postgresProbeStore
}

func (s timedProbeStore) PublishProbe(ctx context.Context, p supervisionProbe) error {
	return timedMonitorCall(s.t, ctx, "publish probe", func(c context.Context) error { return s.inner.PublishProbe(c, p) })
}
func (s timedProbeStore) ReadResponses(ctx context.Context, current, last supervisionProbe) (rows []MonitorResponse, err error) {
	err = timedMonitorCall(s.t, ctx, "read responses", func(c context.Context) error {
		var e error
		rows, e = s.inner.ReadResponses(c, current, last)
		return e
	})
	return
}

type timedResponseStore struct {
	t     *testing.T
	inner *postgresResponseStore
}

func (s timedResponseStore) ReadProbe(ctx context.Context, challenge, decision [32]byte, sequence uint64) (p supervisionProbe, found bool, err error) {
	err = timedMonitorCall(s.t, ctx, "read probe", func(c context.Context) error {
		var e error
		p, found, e = s.inner.ReadProbe(c, challenge, decision, sequence)
		return e
	})
	return
}
func (s timedResponseStore) InsertExactResponse(ctx context.Context, r MonitorResponse) error {
	return timedMonitorCall(s.t, ctx, "insert response", func(c context.Context) error { return s.inner.InsertExactResponse(c, r) })
}

// The wrapper deliberately holds a real PostgreSQL read past cancellation to
// exercise the independent expiry path and rejection of a late result.
type heldMonitorRead struct {
	*postgresProbeStore
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *heldMonitorRead) unblock() { s.once.Do(func() { close(s.release) }) }

func startHeldRead(t *testing.T, ctx context.Context, poller *monitorPoller, held *heldMonitorRead) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	finished := make(chan struct{})
	t.Cleanup(func() {
		held.unblock()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("held poller did not exit during cleanup")
		}
	})
	go func() { defer close(finished); done <- poller.step(ctx) }()
	select {
	case <-held.entered:
	case err := <-done:
		t.Fatalf("poller exited before held read: %v", err)
	case <-ctx.Done():
		t.Fatalf("waiting for held read: %v", ctx.Err())
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for held read")
	}
	return done
}

func awaitHeldResult(t *testing.T, ctx context.Context, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		t.Fatalf("waiting for held poller: %v", ctx.Err())
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for held poller")
	}
	return ErrUnavailable
}

func (s *heldMonitorRead) ReadResponses(_ context.Context, current, last supervisionProbe) ([]MonitorResponse, error) {
	close(s.entered)
	<-s.release
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return s.postgresProbeStore.ReadResponses(ctx, current, last)
}

func (t *monitorCancelTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, monitorTraceKey{}, data.SQL)
}
func (t *monitorCancelTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	sql, _ := ctx.Value(monitorTraceKey{}).(string)
	if t.cancel != nil && data.Err == nil && strings.HasPrefix(strings.ToLower(sql), t.match) && (t.match != "commit" || data.CommandTag.String() == "COMMIT") {
		t.cancel()
	}
}

func TestMonitorStoreExactRetriesAndIndependentStop(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable TLS PostgreSQL")
	}
	integrationtest.ConfigureDockerTesting()
	for _, version := range []string{"16-alpine", "18-alpine"} {
		t.Run(version, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			dir := t.TempDir()
			pki := processTLS(t, dir, "monitor")
			configFile := filepath.Join(dir, "postgres.conf")
			require.NoError(t, os.WriteFile(configFile, []byte("listen_addresses='*'\nssl=on\nssl_ca_file='/tmp/testcontainers-go/postgres/ca_cert.pem'\nssl_cert_file='/tmp/testcontainers-go/postgres/server.cert'\nssl_key_file='/tmp/testcontainers-go/postgres/server.key'\n"), 0600))
			pg, err := postgres.Run(ctx, "postgres:"+version, postgres.BasicWaitStrategies(), postgres.WithDatabase("monitor_fixture"), postgres.WithUsername("guard_admin"), postgres.WithPassword("fixture_only"), postgres.WithConfigFile(configFile), postgres.WithSSLCert(pki.caFile, pki.serverCert, pki.serverKey))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pg.Terminate(context.Background())) })
			raw, err := pg.ConnectionString(ctx)
			require.NoError(t, err)
			u, err := url.Parse(raw)
			require.NoError(t, err)
			_, port, err := net.SplitHostPort(u.Host)
			require.NoError(t, err)
			u.Host = net.JoinHostPort("127.0.0.1", port)
			q := u.Query()
			q.Set("sslmode", "verify-full")
			q.Set("sslrootcert", pki.caFile)
			u.RawQuery = q.Encode()
			pool, err := pgxpool.New(ctx, u.String())
			require.NoError(t, err)
			defer pool.Close()
			_, err = pool.Exec(ctx, `CREATE TABLE public.user_custody_monitor_probe(challenge bytea NOT NULL,sequence bigint NOT NULL,decision_digest bytea NOT NULL,nonce bytea NOT NULL,PRIMARY KEY(challenge,sequence),UNIQUE(challenge,sequence,nonce));
CREATE TABLE public.user_custody_monitor_response(challenge bytea NOT NULL,sequence bigint NOT NULL,kind smallint NOT NULL,nonce bytea NOT NULL,incarnation uuid NOT NULL,signature bytea NOT NULL,PRIMARY KEY(challenge,sequence,kind),FOREIGN KEY(challenge,sequence,nonce) REFERENCES public.user_custody_monitor_probe(challenge,sequence,nonce));
CREATE UNIQUE INDEX monitor_one_stop ON public.user_custody_monitor_response(challenge) WHERE kind=3`)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `CREATE ROLE monitor_controller LOGIN PASSWORD 'fixture_controller';
CREATE ROLE monitor_responder LOGIN PASSWORD 'fixture_responder';
GRANT USAGE ON SCHEMA public TO monitor_controller,monitor_responder;
GRANT SELECT,INSERT ON public.user_custody_monitor_probe TO monitor_controller;
GRANT SELECT ON public.user_custody_monitor_response TO monitor_controller;
GRANT SELECT ON public.user_custody_monitor_probe TO monitor_responder;
GRANT SELECT,INSERT ON public.user_custody_monitor_response TO monitor_responder;
REVOKE EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO monitor_controller,monitor_responder`)
			require.NoError(t, err)
			rolePool := func(user, password string) *pgxpool.Pool {
				roleURL := *u
				roleURL.User = url.UserPassword(user, password)
				cfg, configErr := pinnedPostgresConfig(roleURL.String())
				require.NoError(t, configErr)
				cfg.ConnConfig.Tracer = monitorTimingTracer{}
				// Test a small, explicitly prewarmed pool. It is not an exclusive
				// runtime reservation or a guarantee against later connection loss.
				cfg.MaxConns = 2
				p, poolErr := pgxpool.NewWithConfig(ctx, cfg)
				require.NoError(t, poolErr)
				t.Cleanup(p.Close)
				first, acquireErr := p.Acquire(ctx)
				require.NoError(t, acquireErr)
				second, acquireErr := p.Acquire(ctx)
				if acquireErr != nil {
					first.Release()
					require.NoError(t, acquireErr)
				}
				first.Release()
				second.Release()
				return p
			}
			controllerPool := rolePool("monitor_controller", "fixture_controller")
			responderPool := rolePool("monitor_responder", "fixture_responder")
			var system, database string
			var oid uint32
			require.NoError(t, pool.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&system, &database, &oid))
			makePin := func(user, password string) *monitorConnectionPin {
				roleURL := *u
				roleURL.User = url.UserPassword(user, password)
				pin, pinErr := newMonitorConnectionPin(roleURL.String(), user, system, database, oid, []string{"monitor_controller", "monitor_responder"}, "guard_admin", "127.0.0.1")
				require.NoError(t, pinErr)
				return pin
			}
			public, private, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)
			store := &postgresProbeStore{pool: controllerPool}
			responder := &postgresResponseStore{pool: responderPool}
			p := supervisionProbe{Challenge: supervisionNonce(1), DecisionDigest: supervisionNonce(2), Sequence: 1, Nonce: supervisionNonce(3)}
			require.Error(t, store.PublishProbe(ctx, p), "unqualified store must reject writes")
			_, _, err = responder.ReadProbe(ctx, p.Challenge, p.DecisionDigest, p.Sequence)
			require.Error(t, err)
			store.pin = makePin("monitor_controller", "fixture_controller")
			responder.pin = makePin("monitor_responder", "fixture_responder")
			started := time.Now()
			var before time.Duration
			require.NoError(t, withMonitorGuardTx(ctx, controllerPool, store.pin, func(tx monitorGuardTx) error {
				before = time.Since(started)
				return nil
			}))
			t.Logf("guard qualification: before callback %s, after callback through commit %s (poller budget 500ms)", before, time.Since(started)-before)
			wrongPin := *store.pin
			wrongPin.oid++
			unqualified := &postgresProbeStore{pool: controllerPool, pin: &wrongPin}
			require.Error(t, unqualified.PublishProbe(ctx, p))
			_, err = unqualified.ReadResponses(ctx, p, supervisionProbe{})
			require.Error(t, err)
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM public.user_custody_monitor_probe`).Scan(&count))
			require.Zero(t, count)
			require.NoError(t, store.PublishProbe(ctx, p))
			require.NoError(t, store.PublishProbe(ctx, p))
			tracedURL := *u
			tracedURL.User = url.UserPassword("monitor_controller", "fixture_controller")
			tracedCfg, err := pinnedPostgresConfig(tracedURL.String())
			require.NoError(t, err)
			tracer := &monitorCancelTracer{}
			tracedCfg.ConnConfig.Tracer = tracer
			tracedPool, err := pgxpool.NewWithConfig(ctx, tracedCfg)
			require.NoError(t, err)
			defer tracedPool.Close()
			traced := &postgresProbeStore{pool: tracedPool, pin: store.pin}
			committed := p
			committed.Challenge = supervisionNonce(90)
			cancelCtx, cancelCommit := context.WithCancel(ctx)
			tracer.match, tracer.cancel = "commit", cancelCommit
			require.Error(t, traced.PublishProbe(cancelCtx, committed), "cancelled acknowledged commit must not report success")
			tracer.cancel = nil
			var durable int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM public.user_custody_monitor_probe WHERE challenge=$1 AND sequence=$2`, committed.Challenge[:], int64(committed.Sequence)).Scan(&durable))
			require.Equal(t, 1, durable)
			require.NoError(t, traced.PublishProbe(ctx, committed), "an uncertain commit must allow exact retry")
			readProbe, found, err := responder.ReadProbe(ctx, p.Challenge, p.DecisionDigest, p.Sequence)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, p, readProbe)
			_, found, err = responder.ReadProbe(ctx, p.Challenge, p.DecisionDigest, 2)
			require.NoError(t, err)
			require.False(t, found)
			_, _, err = responder.ReadProbe(ctx, p.Challenge, supervisionNonce(8), p.Sequence)
			require.Error(t, err)
			_, err = pool.Exec(ctx, `INSERT INTO public.user_custody_monitor_probe(challenge,sequence,decision_digest,nonce) VALUES($1,3,$2,$3)`, p.Challenge[:], p.DecisionDigest[:], make([]byte, 31))
			require.NoError(t, err)
			_, _, err = responder.ReadProbe(ctx, p.Challenge, p.DecisionDigest, 3)
			require.Error(t, err, "malformed durable rows must not become probes")
			conflict := p
			conflict.Nonce = supervisionNonce(4)
			require.Error(t, store.PublishProbe(ctx, conflict))
			id := uuid.New()
			armed := signedProbeResponse(t, p, id, MonitorArmed, private)
			stop := signedProbeResponse(t, p, id, MonitorStop, private)
			require.NoError(t, responder.InsertExactResponse(ctx, armed))
			require.NoError(t, responder.InsertExactResponse(ctx, armed))
			wrongDigest := armed
			wrongDigest.DecisionDigest = supervisionNonce(8)
			require.Error(t, responder.InsertExactResponse(ctx, wrongDigest), "a retry must match the signed probe digest")
			other := signedProbeResponse(t, p, uuid.New(), MonitorArmed, private)
			require.Error(t, responder.InsertExactResponse(ctx, other))
			require.NoError(t, responder.InsertExactResponse(ctx, stop))
			p2 := p
			p2.Sequence, p2.Nonce = 2, supervisionNonce(5)
			require.NoError(t, store.PublishProbe(ctx, p2))
			wrongProbe := p2
			wrongProbe.DecisionDigest = supervisionNonce(8)
			require.Error(t, responder.InsertExactResponse(ctx, signedProbeResponse(t, wrongProbe, id, MonitorHeartbeat, private)))
			heartbeat := signedProbeResponse(t, p2, id, MonitorHeartbeat, private)
			results := make(chan error, 8)
			for i := 0; i < cap(results); i++ {
				go func() { results <- responder.InsertExactResponse(ctx, heartbeat) }()
			}
			for i := 0; i < cap(results); i++ {
				require.NoError(t, <-results)
			}
			assertDenied := func(err error) {
				t.Helper()
				var pgErr *pgconn.PgError
				require.ErrorAs(t, err, &pgErr)
				require.Equal(t, "42501", pgErr.Code)
			}
			_, err = controllerPool.Exec(ctx, `DELETE FROM public.user_custody_monitor_response`)
			assertDenied(err)
			_, err = controllerPool.Exec(ctx, `INSERT INTO public.user_custody_monitor_response SELECT * FROM public.user_custody_monitor_response LIMIT 1`)
			assertDenied(err)
			_, err = responderPool.Exec(ctx, `UPDATE public.user_custody_monitor_probe SET sequence=2`)
			assertDenied(err)
			_, err = responderPool.Exec(ctx, `DELETE FROM public.user_custody_monitor_response`)
			assertDenied(err)
			_, err = responderPool.Exec(ctx, `UPDATE public.user_custody_monitor_response SET kind=3`)
			assertDenied(err)
			rows, err := store.ReadResponses(ctx, p, supervisionProbe{})
			require.NoError(t, err)
			require.Len(t, rows, 2)
			readCtx, cancelRead := context.WithCancel(ctx)
			tracer.match, tracer.cancel = "select r.challenge", cancelRead
			suppressed, readErr := traced.ReadResponses(readCtx, p, supervisionProbe{})
			tracer.cancel = nil
			require.Error(t, readErr)
			require.Nil(t, suppressed, "post-read cancellation must suppress accumulated responses")
			for _, row := range rows {
				require.NoError(t, VerifyMonitorResponse(row, public))
			}
			bad := p2
			bad.DecisionDigest = supervisionNonce(9)
			badRows, err := store.ReadResponses(ctx, p, bad)
			require.Error(t, err)
			require.Nil(t, badRows, "a failed scan must not expose accumulated responses")

			// This exercises the transport and both workers with independent login
			// roles. It does not establish source identity or physical watchdog health.
			challenge, decision := supervisionNonce(21), supervisionNonce(22)
			now := time.Unix(1000, 0)
			session, err := newSupervisionSession(challenge, decision, public, func() time.Time { return now })
			require.NoError(t, err)
			monitor, err := newMonitorResponder(challenge, decision, uuid.New(), private)
			require.NoError(t, err)
			poller := newMonitorPoller(session, timedProbeStore{t, store}, rand.Reader)
			worker := newMonitorResponseWorker(monitor, timedResponseStore{t, responder}, func(context.Context) error { return nil })
			require.NoError(t, poller.step(ctx))
			require.False(t, session.live())
			require.NoError(t, worker.step(ctx))
			require.NoError(t, poller.step(ctx))
			require.True(t, session.live())
			now = now.Add(time.Second)
			require.NoError(t, poller.step(ctx))
			require.NoError(t, worker.step(ctx))
			require.NoError(t, poller.step(ctx))
			require.True(t, session.live())
			now = now.Add(2100 * time.Millisecond)
			require.True(t, session.live(), "the heartbeat must renew beyond the original ARMED deadline")
			require.NoError(t, worker.stop())
			require.NoError(t, poller.step(ctx))
			require.False(t, session.live(), "a durable STOP must revoke the same session")

			// Real monotonic time, three successive renewals, and bounded competing
			// database work exercise the existing 1s interval and 3s deadline.
			loadCtx, stopLoad := context.WithCancel(ctx)
			loadDone := make(chan error, 1)
			go func() {
				for {
					if loadCtx.Err() != nil {
						loadDone <- nil
						return
					}
					if _, e := pool.Exec(loadCtx, `SELECT pg_sleep(0.005)`); e != nil {
						if loadCtx.Err() != nil {
							loadDone <- nil
						} else {
							loadDone <- e
						}
						return
					}
					select {
					case <-loadCtx.Done():
						loadDone <- nil
						return
					case <-time.After(45 * time.Millisecond):
					}
				}
			}()
			defer func() { stopLoad(); require.NoError(t, <-loadDone) }()
			makeReal := func(tag byte) (*supervisionSession, *monitorPoller, *monitorResponseWorker) {
				s, e := newSupervisionSession(supervisionNonce(tag), supervisionNonce(tag+1), public, time.Now)
				require.NoError(t, e)
				r, e := newMonitorResponder(supervisionNonce(tag), supervisionNonce(tag+1), uuid.New(), private)
				require.NoError(t, e)
				return s, newMonitorPoller(s, timedProbeStore{t, store}, rand.Reader), newMonitorResponseWorker(r, timedResponseStore{t, responder}, func(context.Context) error { return nil })
			}
			realSession, realPoller, realWorker := makeReal(40)
			for cycle := 0; cycle < 4; cycle++ {
				if cycle > 0 {
					realSession.mu.Lock()
					next := realSession.issued.Add(supervisionInterval)
					realSession.mu.Unlock()
					if delay := time.Until(next); delay > 0 {
						time.Sleep(delay)
					}
					require.True(t, realSession.live(), "lease expired before renewal")
				}
				started := time.Now()
				require.NoError(t, realPoller.step(ctx))
				require.NoError(t, realWorker.step(ctx))
				require.NoError(t, realPoller.step(ctx))
				require.True(t, realSession.live(), "renewal must complete before previous deadline")
				t.Logf("real renewal %d: %s", cycle, time.Since(started))
			}
			// STOP is delivered after a deliberately held read resumes, before its
			// call deadline, and cannot be hidden by the already accepted heartbeat.
			held := &heldMonitorRead{postgresProbeStore: store, entered: make(chan struct{}), release: make(chan struct{})}
			heldPoller := newMonitorPoller(realSession, held, rand.Reader)
			stopDone := startHeldRead(t, ctx, heldPoller, held)
			require.NoError(t, realWorker.stop())
			held.unblock()
			require.NoError(t, awaitHeldResult(t, ctx, stopDone))
			require.False(t, realSession.live())

			expSession, expPoller, expWorker := makeReal(50)
			require.NoError(t, expPoller.step(ctx))
			require.NoError(t, expWorker.step(ctx))
			require.NoError(t, expPoller.step(ctx))
			require.True(t, expSession.live())
			expHeld := &heldMonitorRead{postgresProbeStore: store, entered: make(chan struct{}), release: make(chan struct{})}
			expBlocked := newMonitorPoller(expSession, expHeld, rand.Reader)
			blockedDone := startHeldRead(t, ctx, expBlocked, expHeld)
			ticker := time.NewTicker(20 * time.Millisecond)
			expiryCtx, cancelExpiry := context.WithCancel(ctx)
			expiryDone := make(chan struct{})
			t.Cleanup(func() {
				cancelExpiry()
				ticker.Stop()
				select {
				case <-expiryDone:
				case <-time.After(3 * time.Second):
					t.Error("expiry loop did not exit during cleanup")
				}
			})
			go func() { supervisionExpiryLoop(expiryCtx, expSession, ticker.C); close(expiryDone) }()
			select {
			case <-expSession.revoked():
			case <-time.After(supervisionDeadline + time.Second):
				t.Fatal("expiry blocked by database read")
			}
			ticker.Stop()
			expHeld.unblock()
			require.Error(t, awaitHeldResult(t, ctx, blockedDone))
			select {
			case <-expiryDone:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(3 * time.Second):
				t.Fatal("expiry loop did not exit")
			}
			require.False(t, expSession.live(), "late read cannot revive expired session")
		})
	}
}
