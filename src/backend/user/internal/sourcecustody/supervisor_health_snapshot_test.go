package sourcecustody

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type healthStateObservation struct {
	entered, release chan struct{}
	once             sync.Once
}

func (o *healthStateObservation) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, o, data.SQL)
}
func (o *healthStateObservation) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	sql, _ := ctx.Value(o).(string)
	if strings.HasPrefix(sql, "SELECT state,epoch,source_id") {
		o.once.Do(func() {
			close(o.entered)
			select {
			case <-o.release:
			case <-ctx.Done():
			}
		})
	}
}

func TestSupervisorHealthUsesOneControlSnapshotAcrossOpening(t *testing.T) {
	testSupervisorHealthOpeningTransition(t, true)
}

func TestSupervisorHealthAcceptsOnlySignedPrepareAcrossSourceEnable(t *testing.T) {
	testSupervisorHealthOpeningTransition(t, false)
}

func testSupervisorHealthOpeningTransition(t *testing.T, prepared bool) {
	ctx, control, admin, source, keys, controller, claim, public, _, _ := openSourceFixture(t)
	if prepared {
		require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	}
	trace := &healthStateObservation{entered: make(chan struct{}), release: make(chan struct{})}
	config := control.Config().Copy()
	config.ConnConfig.Tracer = trace
	observed, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer observed.Close()
	health := &supervisorHealth{control: observed, maintenance: admin, claim: claim}
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- health.check(call, false) }()
	var released sync.Once
	release := func() { released.Do(func() { close(trace.release) }) }
	defer release()
	select {
	case <-trace.entered:
	case err := <-result:
		t.Fatalf("health returned before state observation: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("state observation was not reached")
	}
	if prepared {
		require.NoError(t, controller.OpenQualified(ctx, admin, source, keys, claim, public))
	} else {
		require.NoError(t, controller.PrepareOpenSource(ctx, admin, claim, public))
	}
	release()
	select {
	case err := <-result:
		require.NoError(t, err, "CLOSED from one cut and ACTIVE from another must not revoke an honest opening")
	case <-time.After(10 * time.Second):
		t.Fatal("health did not finish after opening")
	}
}

func TestSupervisorHealthRejectsStableDormantSourceMismatch(t *testing.T) {
	ctx, control, admin, _, _, _, claim, _, _, _ := openSourceFixture(t)
	_, err := admin.Exec(ctx, `ALTER DATABASE `+pgx.Identifier{claim.Physical.Database}.Sanitize()+` WITH ALLOW_CONNECTIONS true`)
	require.NoError(t, err)
	health := &supervisorHealth{control: control, maintenance: admin, claim: claim}
	require.Error(t, health.check(ctx, false), "a source enabled without a signed PREPARING intent is not a transition")
	require.Error(t, health.check(ctx, true), "initial readiness always requires the dormant source")
	var evidence int
	require.NoError(t, control.QueryRow(ctx, `SELECT count(*) FROM user_custody_open_evidence`).Scan(&evidence))
	require.Zero(t, evidence)
}
