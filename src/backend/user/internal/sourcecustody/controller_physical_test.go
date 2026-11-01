package sourcecustody

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type cancelledControlCheckTrace struct {
	checks  atomic.Int32
	entered chan struct{}
}

func (t *cancelledControlCheckTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(query.SQL, "SELECT system_identifier::text,current_database()") && t.checks.Add(1) == 2 {
		close(t.entered)
		<-ctx.Done()
	}
	return ctx
}

func (*cancelledControlCheckTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestCustodyCancelledControlCheckDoesNotPoison(t *testing.T) {
	ctx := context.Background()
	control := custodyDatabase(t, ctx)
	binding := custodyBinding()
	_, err := control.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY,state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
	require.NoError(t, err)
	_, err = control.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'CLOSED',$1,$2,$3,$4,$5)`, binding.Epoch, binding.SourceID, binding.HistoryGeneration, binding.UserSourceGeneration, binding.KeysetDigest[:])
	require.NoError(t, err)
	trace := &cancelledControlCheckTrace{entered: make(chan struct{})}
	config := control.Config().Copy()
	config.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	defer pool.Close()
	controller, err := Start(ctx, pool, binding)
	require.NoError(t, err)
	defer func() { require.NoError(t, controller.Shutdown(ctx)) }()
	_, err = control.Exec(ctx, `UPDATE user_custody_state SET state='OPEN'`)
	require.NoError(t, err)
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, e := controller.Acquire(call, binding); result <- e }()
	select {
	case <-trace.entered:
	case err = <-result:
		t.Fatalf("acquire ended before the control check: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("acquire did not reach the control check")
	}
	cancel()
	select {
	case err = <-result:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled acquire did not return")
	}
	permit, err := controller.Acquire(ctx, binding)
	require.NoError(t, err)
	require.NoError(t, permit.Release(ctx))
}

func TestCustodyRejectsPermitConnectionsToDifferentControlCluster(t *testing.T) {
	for _, target := range []string{"different-cluster", "different-database"} {
		for _, operation := range []string{"acquire", "drain"} {
			t.Run(target+"/"+operation, func(t *testing.T) {
				ctx := context.Background()
				primary := custodyDatabase(t, ctx)
				var other *pgxpool.Pool
				if target == "different-cluster" {
					other = custodyDatabase(t, ctx)
				} else {
					_, err := primary.Exec(ctx, `CREATE DATABASE alternate_control`)
					require.NoError(t, err)
					alternate := primary.Config().Copy()
					alternate.ConnConfig.Database = "alternate_control"
					other, err = pgxpool.NewWithConfig(ctx, alternate)
					require.NoError(t, err)
					defer other.Close()
				}
				binding := custodyBinding()
				_, err := primary.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY,state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
				require.NoError(t, err)
				_, err = primary.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'CLOSED',$1,$2,$3,$4,$5)`, binding.Epoch, binding.SourceID, binding.HistoryGeneration, binding.UserSourceGeneration, binding.KeysetDigest[:])
				require.NoError(t, err)
				_, err = other.Exec(ctx, `CREATE TABLE user_custody_state(singleton boolean PRIMARY KEY,state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL)`)
				require.NoError(t, err)
				_, err = other.Exec(ctx, `INSERT INTO user_custody_state VALUES(true,'OPEN',$1,$2,$3,$4,$5)`, binding.Epoch, binding.SourceID, binding.HistoryGeneration, binding.UserSourceGeneration, binding.KeysetDigest[:])
				require.NoError(t, err)
				config := primary.Config().Copy()
				var connects atomic.Int32
				config.BeforeConnect = func(_ context.Context, connection *pgx.ConnConfig) error {
					if connects.Add(1) > 1 {
						*connection = *other.Config().ConnConfig.Copy()
					}
					return nil
				}
				mixed, err := pgxpool.NewWithConfig(ctx, config)
				require.NoError(t, err)
				defer mixed.Close()
				controller, err := Start(ctx, mixed, binding)
				require.NoError(t, err)
				defer func() { _ = controller.Shutdown(ctx) }()
				if operation == "acquire" {
					_, err = primary.Exec(ctx, `UPDATE user_custody_state SET state='OPEN'`)
					require.NoError(t, err)
					permit, acquireErr := controller.Acquire(ctx, binding)
					if permit != nil {
						_ = permit.Release(ctx)
					}
					require.Error(t, acquireErr, "a permit on another cluster cannot fence the leader")
				} else {
					require.Error(t, controller.WaitPermits(ctx), "a barrier on another cluster cannot drain the leader's permits")
				}
				var state string
				require.NoError(t, primary.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
				require.Equal(t, "CLOSED", state)
			})
		}
	}
}
