package store

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type v2RaceResult struct {
	receipt OwnershipTransferV2Receipt
	err     error
}

// Both real connections must reach a database lock barrier before it opens.
// No scheduler sleeps or optimistic goroutine starts are treated as concurrency.
func v2Race(t *testing.T, s *RoleStore, lockOperation bool, in OwnershipTransferV2Input, calls [2]func(*RoleStore, context.Context) (OwnershipTransferV2Receipt, error)) [2]v2RaceResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	barrier, err := s.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = barrier.Rollback(context.Background()) }()
	if lockOperation {
		_, err = barrier.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,hashtext($2))`, int32(0x524f5032), in.OperationID.String())
	} else {
		_, err = barrier.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, in.SpaceID.String())
	}
	require.NoError(t, err)
	var pools [2]*pgxpool.Pool
	var pids [2]int32
	for i := range pools {
		config := s.Pool.Config()
		config.MaxConns = 1
		config.MinConns = 0
		pools[i], err = pgxpool.NewWithConfig(ctx, config)
		require.NoError(t, err)
		defer pools[i].Close()
		require.NoError(t, pools[i].QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pids[i]))
	}
	var results [2]v2RaceResult
	var wg sync.WaitGroup
	finished := make(chan struct{})
	wg.Add(2)
	for i := range calls {
		go func(i int) {
			defer wg.Done()
			results[i].receipt, results[i].err = calls[i](&RoleStore{Pool: pools[i]}, ctx)
		}(i)
	}
	go func() { wg.Wait(); close(finished) }()
	defer func() {
		cancel()
		_ = barrier.Rollback(context.Background())
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("ownership race goroutines did not finish")
		}
	}()
	require.Eventually(t, func() bool {
		var count int
		err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE pid IN ($1,$2) AND wait_event_type='Lock'`, pids[0], pids[1]).Scan(&count)
		return err == nil && count == 2
	}, 5*time.Second, 20*time.Millisecond, "both requests must be waiting inside PostgreSQL")
	if lockOperation {
		var spaceLocks int
		require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE pid IN ($1,$2) AND locktype='advisory' AND granted AND objsubid=1`, pids[0], pids[1]).Scan(&spaceLocks))
		require.Zero(t, spaceLocks, "operation lock must precede any bigint space lock")
	}
	require.NoError(t, barrier.Commit(ctx))
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("ownership race did not finish")
	}
	return results
}

func TestOwnershipTransferV2_FinalConcurrentReservationsSerialize(t *testing.T) {
	for _, kind := range []string{"different operations same space", "same operation different spaces"} {
		t.Run(kind, func(t *testing.T) {
			s, first := v2Fixture(t)
			second := first
			global := kind == "same operation different spaces"
			if global {
				second.SpaceID = uuid.New()
				require.NoError(t, s.BootstrapSpaceRoles(context.Background(), second.SpaceID, second.OldOwnerProfileID))
			} else {
				second.OperationID = uuid.New()
			}
			second.IntentBytes = v2IntentBytes(t, second)
			results := v2Race(t, s, global, first, [2]func(*RoleStore, context.Context) (OwnershipTransferV2Receipt, error){
				func(st *RoleStore, c context.Context) (OwnershipTransferV2Receipt, error) {
					return st.PrepareOwnershipTransfer(c, first)
				},
				func(st *RoleStore, c context.Context) (OwnershipTransferV2Receipt, error) {
					return st.PrepareOwnershipTransfer(c, second)
				},
			})
			wins := 0
			for _, r := range results {
				if r.err == nil {
					wins++
					require.Equal(t, "prepared", r.receipt.State)
				} else if global {
					require.ErrorIs(t, r.err, ErrOwnershipTransferConflict)
				} else {
					require.ErrorIs(t, r.err, ErrSpaceFrozen)
				}
			}
			require.Equal(t, 1, wins)
			var count int
			require.NoError(t, s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM ownership_transfer_v2`).Scan(&count))
			require.Equal(t, 1, count)
			v2RequireOwner(t, s, first, first.OldOwnerProfileID)
			v2RequireOwner(t, s, second, second.OldOwnerProfileID)
		})
	}
}

func TestOwnershipTransferV2_FinalConcurrentTerminalDecisionAndSameActionReplay(t *testing.T) {
	for _, pair := range [][2]string{{"finalized", "aborted"}, {"finalized", "finalized"}, {"aborted", "aborted"}} {
		t.Run(pair[0]+"/"+pair[1], func(t *testing.T) {
			s, in := v2Fixture(t)
			_, err := s.PrepareOwnershipTransfer(context.Background(), in)
			require.NoError(t, err)
			results := v2Race(t, s, false, in, [2]func(*RoleStore, context.Context) (OwnershipTransferV2Receipt, error){
				func(st *RoleStore, c context.Context) (OwnershipTransferV2Receipt, error) {
					return v2Terminal(st, pair[0])(c, v2ActionInput(in, pair[0]))
				},
				func(st *RoleStore, c context.Context) (OwnershipTransferV2Receipt, error) {
					return v2Terminal(st, pair[1])(c, v2ActionInput(in, pair[1]))
				},
			})
			wins := 0
			var winner OwnershipTransferV2Receipt
			for _, r := range results {
				if r.err == nil {
					wins++
					winner = r.receipt
				} else {
					require.ErrorIs(t, r.err, ErrOwnershipTransferState)
				}
			}
			if pair[0] == pair[1] {
				require.Equal(t, 2, wins)
				require.Equal(t, results[0].receipt, results[1].receipt)
			} else {
				require.Equal(t, 1, wins)
			}
			state, _, finalHash, abortHash := v2ReadActionHashes(t, s, in.OperationID)
			require.Equal(t, winner.State, state)
			if state == "finalized" {
				require.Nil(t, abortHash)
				require.Equal(t, v2ActionInput(in, "finalized").RequestHash, finalHash)
			} else {
				require.Nil(t, finalHash)
				require.Equal(t, v2ActionInput(in, "aborted").RequestHash, abortHash)
			}
			v2RequireOwner(t, s, in, winner.CurrentOwnerProfileID)
		})
	}
}

func TestOwnershipTransferV2_FinalSeparatePoolRestartsRetainUnknownIntent(t *testing.T) {
	for _, terminal := range []string{"finalized", "aborted"} {
		t.Run(terminal, func(t *testing.T) {
			s, in := v2Fixture(t)
			ctx := context.Background()
			in.IntentBytes = append(in.IntentBytes, 0xa0, 0x06, 0x07)
			config := s.Pool.Config()
			pool1, err := pgxpool.NewWithConfig(ctx, config.Copy())
			require.NoError(t, err)
			defer pool1.Close()
			prepared, err := (&RoleStore{Pool: pool1}).PrepareOwnershipTransfer(ctx, in)
			pool1.Close()
			require.NoError(t, err)
			pool2, err := pgxpool.NewWithConfig(ctx, config.Copy())
			require.NoError(t, err)
			defer pool2.Close()
			second := &RoleStore{Pool: pool2}
			replay, err := second.PrepareOwnershipTransfer(ctx, in)
			require.NoError(t, err)
			require.Equal(t, prepared, replay)
			terminalInput := v2ActionInput(in, terminal)
			receipt, err := v2Terminal(second, terminal)(ctx, terminalInput)
			pool2.Close()
			require.NoError(t, err)
			pool3, err := pgxpool.NewWithConfig(ctx, config.Copy())
			require.NoError(t, err)
			defer pool3.Close()
			got, err := v2Terminal(&RoleStore{Pool: pool3}, terminal)(ctx, terminalInput)
			require.NoError(t, err)
			require.Equal(t, receipt, got)
			require.Equal(t, in.IntentBytes, got.IntentBytes)
			v2RequireOwner(t, s, in, receipt.CurrentOwnerProfileID)
		})
	}
}

func TestOwnershipTransferV2_FinalMissingOwnerRejectsAbsentPrepareAndAbort(t *testing.T) {
	for _, absence := range []string{"role", "membership"} {
		t.Run(absence, func(t *testing.T) {
			s, in := v2Fixture(t)
			ctx := context.Background()
			_, err := s.Pool.Exec(ctx, `DELETE FROM member_roles WHERE space_id=$1`, in.SpaceID)
			require.NoError(t, err)
			if absence == "role" {
				_, err = s.Pool.Exec(ctx, `DELETE FROM roles WHERE space_id=$1`, in.SpaceID)
				require.NoError(t, err)
			}
			for _, fn := range []func(context.Context, OwnershipTransferV2Input) (OwnershipTransferV2Receipt, error){s.PrepareOwnershipTransfer, s.AbortOwnershipTransfer} {
				_, err := fn(ctx, in)
				require.ErrorIs(t, err, ErrOwnershipTransferState)
				v2RequireNoLedger(t, s)
			}
			var owners int
			require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM member_roles WHERE space_id=$1`, in.SpaceID).Scan(&owners))
			require.Zero(t, owners)
		})
	}
}

func TestOwnershipTransferV2_FinalLedgerWriteFailureRollsBackMovedOwner(t *testing.T) {
	s, in := v2Fixture(t)
	ctx := context.Background()
	_, err := s.PrepareOwnershipTransfer(ctx, in)
	require.NoError(t, err)
	// The trigger proves the membership UPDATE happened before injecting failure.
	// Any change escaping the shared transaction would remain visible afterward.
	_, err = s.Pool.Exec(ctx, `CREATE FUNCTION reject_v2_terminal() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.state='finalized' THEN
  IF NOT EXISTS(SELECT 1 FROM member_roles mr JOIN roles r ON r.id=mr.role_id WHERE mr.space_id=NEW.space_id AND r.name='Owner' AND mr.profile_id=NEW.new_owner_profile_id) THEN RAISE EXCEPTION 'owner not moved before ledger write'; END IF;
  RAISE EXCEPTION USING MESSAGE='injected ledger failure after owner move', ERRCODE='P0001';
 END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_v2_terminal BEFORE UPDATE ON ownership_transfer_v2 FOR EACH ROW EXECUTE FUNCTION reject_v2_terminal()`)
	require.NoError(t, err)
	_, err = s.FinalizeOwnershipTransfer(ctx, v2ActionInput(in, "failed terminal"))
	require.ErrorContains(t, err, "injected ledger failure after owner move")
	v2RequireOwner(t, s, in, in.OldOwnerProfileID)
	state, _, finalHash, abortHash := v2ReadActionHashes(t, s, in.OperationID)
	require.Equal(t, "prepared", state)
	require.Nil(t, finalHash)
	require.Nil(t, abortHash)
	_, err = s.Pool.Exec(ctx, `DROP TRIGGER reject_v2_terminal ON ownership_transfer_v2; DROP FUNCTION reject_v2_terminal()`)
	require.NoError(t, err)
	_, err = s.FinalizeOwnershipTransfer(ctx, v2ActionInput(in, "first accepted terminal"))
	require.NoError(t, err)
	v2RequireOwner(t, s, in, in.NewOwnerProfileID)
}

func TestOwnershipTransferV2_FinalDownEmptySuccessAndDurableRefusal(t *testing.T) {
	for _, existing := range []string{"empty", "receipt", "retirement"} {
		t.Run(existing, func(t *testing.T) {
			s, in := v2Fixture(t)
			ctx := context.Background()
			down, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "role_db", "000010_ownership_v2.down.sql"))
			require.NoError(t, err)
			if existing == "receipt" {
				_, err = s.AbortOwnershipTransfer(ctx, in)
				require.NoError(t, err)
			}
			if existing == "retirement" {
				_, err = s.Pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,now())`, in.SpaceID)
				require.NoError(t, err)
			}
			_, err = s.Pool.Exec(ctx, string(down))
			var tables int
			if existing == "empty" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename IN ('ownership_transfer_v2','role_space_lifecycle')`).Scan(&tables))
			if existing == "empty" {
				require.Zero(t, tables)
			} else {
				require.Equal(t, 2, tables)
			}
		})
	}
}

func TestOwnershipTransferV2_FinalRetirementFencesEveryStoredStateAndUUIDReuse(t *testing.T) {
	for _, state := range []string{"prepared", "finalized", "aborted"} {
		t.Run(state, func(t *testing.T) {
			s, in := v2Fixture(t)
			ctx := context.Background()
			_, err := s.PrepareOwnershipTransfer(ctx, in)
			require.NoError(t, err)
			if state != "prepared" {
				_, err = v2Terminal(s, state)(ctx, v2ActionInput(in, state))
				require.NoError(t, err)
			}
			_, err = s.Pool.Exec(ctx, `INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES($1,now());`, in.SpaceID)
			require.NoError(t, err)
			_, err = s.Pool.Exec(ctx, `DELETE FROM member_roles WHERE space_id=$1`, in.SpaceID)
			require.NoError(t, err)
			_, err = s.Pool.Exec(ctx, `DELETE FROM roles WHERE space_id=$1`, in.SpaceID)
			require.NoError(t, err)
			for _, operation := range []uuid.UUID{in.OperationID, uuid.New()} {
				request := in
				request.OperationID = operation
				request.IntentBytes = v2IntentBytes(t, request)
				for _, action := range []string{"prepared", "finalized", "aborted"} {
					call := request
					if action != "prepared" {
						call = v2ActionInput(request, action)
					}
					fn := s.PrepareOwnershipTransfer
					if action != "prepared" {
						fn = v2Terminal(s, action)
					}
					_, err := fn(ctx, call)
					require.ErrorIs(t, err, ErrSpaceRetired)
				}
			}
			stored, _, _, _ := v2ReadActionHashes(t, s, in.OperationID)
			require.Equal(t, state, stored)
			var retired int
			require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM role_space_lifecycle WHERE space_id=$1 AND retired_at IS NOT NULL`, in.SpaceID).Scan(&retired))
			require.Equal(t, 1, retired)
		})
	}
}
