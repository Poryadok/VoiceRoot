package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type timingTrace struct {
	start   time.Time
	mu      sync.Mutex
	events  []string
	dropped int
}
type timingKey int
type timingSpan struct {
	start time.Time
	label string
}

const (
	querySpan timingKey = iota
	acquireSpan
)

func (d *timingTrace) event(label string, start time.Time, err error) {
	class := "ok"
	if errors.Is(err, context.DeadlineExceeded) {
		class = "deadline"
	} else if errors.Is(err, context.Canceled) {
		class = "canceled"
	} else if err != nil {
		class = "error"
	}
	line := fmt.Sprintf("at=%s duration=%s stage=%s result=%s", time.Since(d.start).Round(time.Microsecond), time.Since(start).Round(time.Microsecond), label, class)
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.events) < 2000 {
		d.events = append(d.events, line)
	} else {
		d.dropped++
	}
}
func sqlStage(sql string) string {
	s := strings.ToLower(strings.TrimSpace(sql))
	verb := "other"
	for _, v := range []string{"select", "update", "insert", "delete", "begin", "commit", "rollback"} {
		if strings.HasPrefix(s, v) {
			verb = v
			break
		}
	}
	if strings.Contains(s, "set lease_owner=$4,lease_until=now()+interval") {
		return "sql.claim"
	}
	if strings.Contains(s, "next_attempt_at<=now()") && strings.Contains(s, "order by created_at limit 50") {
		return "sql.due"
	}
	if strings.Contains(s, "insert into gis_session_owner_receipts") {
		return "sql.receipt"
	}
	target := "other"
	for _, v := range []string{"gis_session_owner_receipts", "gis_session_operations", "gis_sessions", "gis_authority_revision", "schema_migrations", "player_bindings", "game_resource_binding_chats", "environments", "applications", "pg_catalog"} {
		if strings.Contains(s, v) {
			target = v
			break
		}
	}
	return "sql." + verb + "." + target
}
func (d *timingTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	start := time.Now()
	label := sqlStage(data.SQL)
	d.event(label+".start", start, nil)
	return context.WithValue(ctx, querySpan, timingSpan{start, label})
}
func (d *timingTrace) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if s, ok := ctx.Value(querySpan).(timingSpan); ok {
		d.event(s.label, s.start, data.Err)
	}
}
func (d *timingTrace) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	start := time.Now()
	d.event("pool.acquire.start", start, nil)
	return context.WithValue(ctx, acquireSpan, timingSpan{start: start})
}
func (d *timingTrace) TraceAcquireEnd(ctx context.Context, _ *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
	if s, ok := ctx.Value(acquireSpan).(timingSpan); ok {
		d.event("pool.acquire", s.start, data.Err)
	}
}
func (d *timingTrace) dump(t *testing.T) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, e := range d.events {
		t.Log(e)
	}
	if d.dropped > 0 {
		t.Logf("trace dropped=%d", d.dropped)
	}
}
func tracedTestPool(t *testing.T, ctx context.Context, original *pgxpool.Pool) (*pgxpool.Pool, *timingTrace) {
	t.Helper()
	trace := &timingTrace{start: time.Now()}
	config := original.Config().Copy()
	config.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close(); trace.dump(t) })
	return pool, trace
}

func diagnosticSeconds(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
