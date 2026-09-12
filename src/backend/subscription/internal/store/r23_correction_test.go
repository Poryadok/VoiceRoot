package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestR23CorrectionLifecycleReplayRequiresStoredRequestBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL lifecycle replay contract requires testcontainers")
	}
	ctx := context.Background()
	pool := r23StartSubscriptionPostgres(t, ctx)
	st := &SubscriptionStore{Pool: pool}

	t.Run("fence", func(t *testing.T) {
		spaceID, deletionID := uuid.New(), uuid.New()
		input := correctionFenceInput(spaceID, deletionID, []byte("fence-request"))
		_, err := st.ApplySpaceLifecycleFence(ctx, input)
		require.NoError(t, err)

		changed := input
		changed.RequestBytes = []byte("different-fence-request")
		_, err = st.ApplySpaceLifecycleFence(ctx, changed)
		require.ErrorIs(t, err, ErrLifecycleBinding)

		_, err = st.CleanupSpaceLifecycleEvidence(ctx, input.AppliedAt.Add(31*24*time.Hour))
		require.NoError(t, err)
		_, err = st.ApplySpaceLifecycleFence(ctx, input)
		require.ErrorIs(t, err, ErrLifecycleBinding, "compacted evidence cannot prove byte equality")
	})

	t.Run("purge", func(t *testing.T) {
		spaceID, deletionID := uuid.New(), uuid.New()
		require.NoError(t, r23InsertLifecycleFence(ctx, pool, spaceID, deletionID, 4, "PURGE_DECIDED"))
		input := correctionPurgeInput(spaceID, deletionID, []byte("purge-request"))
		_, err := st.PurgeSpace(ctx, input, nil)
		require.NoError(t, err)

		changed := input
		changed.RequestBytes = []byte("different-purge-request")
		_, err = st.PurgeSpace(ctx, changed, nil)
		require.ErrorIs(t, err, ErrLifecycleBinding)

		_, err = st.CleanupSpaceLifecycleEvidence(ctx, input.CompletedAt.Add(31*24*time.Hour))
		require.NoError(t, err)
		_, err = st.PurgeSpace(ctx, input, nil)
		require.ErrorIs(t, err, ErrLifecycleBinding, "compacted evidence cannot prove byte equality")
	})
}

func TestR23CorrectionProviderRotationPreservesReplayConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL provider rotation conflict requires testcontainers")
	}
	ctx := context.Background()
	pool := r23StartSubscriptionPostgres(t, ctx)
	st := &SubscriptionStore{Pool: pool}
	eventID := []byte("provider-event-across-rotation")
	oldKey := ProviderEventHMACKey{Version: "kms-old", Key: []byte("old-provider-key")}
	newKey := ProviderEventHMACKey{Version: "kms-current", Key: []byte("current-provider-key")}
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	replayed, err := st.RecordProviderEventWithKeysTx(ctx, tx, "paddle", eventID, "accepted", []ProviderEventHMACKey{oldKey})
	require.NoError(t, err)
	require.False(t, replayed)
	require.NoError(t, tx.Commit(ctx))

	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = st.RecordProviderEventWithKeysTx(ctx, tx, "paddle", eventID, "conflicting", []ProviderEventHMACKey{newKey, oldKey})
	require.ErrorIs(t, err, ErrLifecycleBinding)
}

func TestR23CorrectionFirstFrozenFenceSerializesUnfencedGovernedPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL absent-row lifecycle races require testcontainers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := r23StartSubscriptionPostgres(t, ctx)
	st := &SubscriptionStore{Pool: pool}

	for _, tc := range []struct {
		name   string
		status string
		call   func(context.Context, *SubscriptionStore, uuid.UUID, uuid.UUID, uuid.UUID) error
	}{
		{"read by space", "active", func(ctx context.Context, st *SubscriptionStore, spaceID, _, _ uuid.UUID) error {
			row, err := st.GetSpaceSubscriptionBySpaceID(ctx, spaceID)
			if err == nil && row != nil {
				return errors.New("FROZEN read exposed entitlement")
			}
			return err
		}},
		{"active by space", "active", func(ctx context.Context, st *SubscriptionStore, spaceID, _, _ uuid.UUID) error {
			active, err := st.HasActiveSpaceProForSpace(ctx, spaceID)
			if err == nil && active {
				return errors.New("FROZEN space reported active entitlement")
			}
			return err
		}},
		{"active by purchaser", "active", func(ctx context.Context, st *SubscriptionStore, _, purchaserID, _ uuid.UUID) error {
			active, err := st.HasActiveSpaceProForPurchaser(ctx, purchaserID)
			if err == nil && active {
				return errors.New("FROZEN purchaser reported active entitlement")
			}
			return err
		}},
		{"activate", "active", func(ctx context.Context, st *SubscriptionStore, spaceID, purchaserID, _ uuid.UUID) error {
			_, err := st.ActivateSpacePro(ctx, spaceID, purchaserID, "evt-absent-fence-activate", json.RawMessage(`{}`))
			if !errors.Is(err, ErrLifecycleState) {
				return fmt.Errorf("activation error = %v, want %v", err, ErrLifecycleState)
			}
			return nil
		}},
		{"provider activate", "active", func(ctx context.Context, st *SubscriptionStore, spaceID, purchaserID, _ uuid.UUID) error {
			_, _, err := st.ActivateSpaceProProviderEventWithKeys(ctx, "paddle", []byte("evt-absent-fence-provider-activate"), "space_pro_activated", []ProviderEventHMACKey{{Version: "kms-current", Key: []byte("provider-key")}}, spaceID, purchaserID, json.RawMessage(`{}`))
			if !errors.Is(err, ErrLifecycleState) {
				return fmt.Errorf("provider activation error = %v, want %v", err, ErrLifecycleState)
			}
			return nil
		}},
		{"mark cancelled", "active", func(ctx context.Context, st *SubscriptionStore, spaceID, _, _ uuid.UUID) error {
			_, err := st.MarkSpaceProCancelled(ctx, spaceID, "evt-absent-fence-cancel", json.RawMessage(`{}`))
			if !errors.Is(err, ErrLifecycleState) {
				return fmt.Errorf("cancellation error = %v, want %v", err, ErrLifecycleState)
			}
			return nil
		}},
		{"sweeper selection", "pending_cancel", func(ctx context.Context, st *SubscriptionStore, spaceID, _, _ uuid.UUID) error {
			rows, err := st.ListPendingSpaceProCancellations(ctx, time.Now().UTC())
			if err != nil {
				return err
			}
			for _, row := range rows {
				if row.SpaceID == spaceID {
					return errors.New("FROZEN worker selected entitlement")
				}
			}
			return nil
		}},
		{"sweeper finalization", "pending_cancel", func(ctx context.Context, st *SubscriptionStore, spaceID, _, subID uuid.UUID) error {
			_, err := st.FinalizeSpaceProCancellation(ctx, spaceID, subID)
			if !errors.Is(err, ErrLifecycleState) {
				return fmt.Errorf("finalization error = %v, want %v", err, ErrLifecycleState)
			}
			return nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spaceID, purchaserID, subID := correctionSeedUnfencedSpace(t, ctx, pool, tc.status)
			input := correctionFenceInput(spaceID, uuid.New(), []byte("first-frozen-request"))

			barrier, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = barrier.Rollback(context.Background()) }()
			_, err = barrier.Exec(ctx, `
INSERT INTO subscription_space_lifecycle_operations(
 space_id,deletion_operation_id,generation,operation_kind,request_sha256,request_bytes,
 receipt_id,receipt_bytes,terminal_state,completed_at,retain_until,provider_cancel_attempts)
VALUES($1,$2,$3,'FENCE',$4,'barrier-request',$5,'barrier-receipt','COMPLETED',$6,$6::timestamptz+interval '30 days',0)`,
				input.SpaceID, input.DeletionOperationID, input.Generation, input.RequestSHA256, uuid.New(), input.AppliedAt)
			require.NoError(t, err)

			fenceDone := make(chan error, 1)
			go func() {
				_, applyErr := st.ApplySpaceLifecycleFence(ctx, input)
				fenceDone <- applyErr
			}()
			correctionWaitForBlocked(t, ctx, pool, 1)

			governedDone := make(chan error, 1)
			go func() { governedDone <- tc.call(ctx, st, spaceID, purchaserID, subID) }()
			if completedErr, completed := correctionWaitForBlockedOrCompleted(t, ctx, pool, governedDone, 2); completed {
				require.Failf(t, "governed path bypassed first FROZEN transaction", "%s completed before the first FROZEN fence committed: %v", tc.name, completedErr)
			}

			require.NoError(t, barrier.Rollback(ctx))
			require.NoError(t, <-fenceDone)
			require.NoError(t, <-governedDone)
			require.Equal(t, "FROZEN", r23FenceState(t, ctx, pool, spaceID))
		})
	}
}

func TestR23CorrectionSpacePublicPathsLinearizeBehindFence(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL lifecycle races require testcontainers")
	}
	for _, tc := range []struct {
		name string
		call func(context.Context, *SubscriptionStore, uuid.UUID, uuid.UUID) error
	}{
		{"read", func(ctx context.Context, st *SubscriptionStore, spaceID, _ uuid.UUID) error {
			row, err := st.GetSpaceSubscriptionBySpaceID(ctx, spaceID)
			if err == nil && row != nil {
				return errors.New("FROZEN read exposed entitlement")
			}
			return err
		}},
		{"mutation", func(ctx context.Context, st *SubscriptionStore, spaceID, _ uuid.UUID) error {
			_, err := st.MarkSpaceProCancelled(ctx, spaceID, "evt-cancel", json.RawMessage(`{}`))
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			pool := r23StartSubscriptionPostgres(t, ctx)
			st := &SubscriptionStore{Pool: pool}
			spaceID, subID := correctionSeedLiveSpace(t, ctx, pool, "active")
			barrier, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = barrier.Rollback(context.Background()) }()
			var locked uuid.UUID
			require.NoError(t, barrier.QueryRow(ctx, `SELECT space_id FROM subscription_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&locked))

			done := make(chan error, 1)
			go func() { done <- tc.call(ctx, st, spaceID, subID) }()
			correctionWaitForBlocked(t, ctx, pool, 1)
			require.NoError(t, enableEvidenceMutation(ctx, barrier))
			_, err = barrier.Exec(ctx, `UPDATE subscription_space_lifecycle_fences SET state='FROZEN' WHERE space_id=$1`, spaceID)
			require.NoError(t, err)
			require.NoError(t, barrier.Commit(ctx))
			err = <-done
			if tc.name == "mutation" {
				require.ErrorIs(t, err, ErrLifecycleState)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestR23CorrectionSpaceWorkerSelectionAndFinalizeLinearizeBehindFence(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL lifecycle races require testcontainers")
	}
	for _, tc := range []struct {
		name string
		call func(context.Context, *SubscriptionStore, uuid.UUID, uuid.UUID) error
	}{
		{"selection", func(ctx context.Context, st *SubscriptionStore, spaceID, _ uuid.UUID) error {
			rows, err := st.ListPendingSpaceProCancellations(ctx, time.Now().UTC())
			if err != nil {
				return err
			}
			for _, row := range rows {
				if row.SpaceID == spaceID {
					return errors.New("FROZEN worker selected entitlement")
				}
			}
			return nil
		}},
		{"finalize", func(ctx context.Context, st *SubscriptionStore, spaceID, subID uuid.UUID) error {
			_, err := st.FinalizeSpaceProCancellation(ctx, spaceID, subID)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			pool := r23StartSubscriptionPostgres(t, ctx)
			st := &SubscriptionStore{Pool: pool}
			spaceID, subID := correctionSeedLiveSpace(t, ctx, pool, "pending_cancel")
			barrier, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = barrier.Rollback(context.Background()) }()
			var locked uuid.UUID
			require.NoError(t, barrier.QueryRow(ctx, `SELECT space_id FROM subscription_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&locked))

			done := make(chan error, 1)
			go func() { done <- tc.call(ctx, st, spaceID, subID) }()
			correctionWaitForBlocked(t, ctx, pool, 1)
			require.NoError(t, enableEvidenceMutation(ctx, barrier))
			_, err = barrier.Exec(ctx, `UPDATE subscription_space_lifecycle_fences SET state='FROZEN' WHERE space_id=$1`, spaceID)
			require.NoError(t, err)
			require.NoError(t, barrier.Commit(ctx))
			err = <-done
			if tc.name == "finalize" {
				require.ErrorIs(t, err, ErrLifecycleState)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func correctionFenceInput(spaceID, deletionID uuid.UUID, requestBytes []byte) LifecycleFenceInput {
	now := time.Now().UTC().Add(-32 * 24 * time.Hour)
	return LifecycleFenceInput{SpaceID: spaceID, DeletionOperationID: deletionID, Generation: 1, State: "FROZEN", ManifestID: "manifest", ManifestSHA256: bytes.Repeat([]byte{1}, 32), ManifestItemCount: 0, RequestSHA256: bytes.Repeat([]byte{2}, 32), RequestBytes: requestBytes, ReceiptID: uuid.New(), ReceiptBytes: []byte("receipt"), AppliedAt: now}
}

func correctionPurgeInput(spaceID, deletionID uuid.UUID, requestBytes []byte) PurgeInput {
	now := time.Now().UTC().Add(-32 * 24 * time.Hour)
	return PurgeInput{SpaceID: spaceID, DeletionOperationID: deletionID, Generation: 4, ManifestID: deletionID.String(), ManifestSHA256: bytes.Repeat([]byte{1}, 32), ManifestItemCount: 0, RequestSHA256: bytes.Repeat([]byte{2}, 32), RequestBytes: requestBytes, ReceiptID: uuid.New(), ReceiptBytes: []byte("receipt"), CompletedAt: now}
}

func correctionSeedLiveSpace(t *testing.T, ctx context.Context, pool *pgxpool.Pool, status string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	spaceID, deletionID, subID := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, r23InsertLifecycleFence(ctx, pool, spaceID, deletionID, 1, "LIVE"))
	_, err := pool.Exec(ctx, `INSERT INTO space_subscriptions(id,space_id,purchaser_account_id,plan,billing_period,status,provider,provider_subscription_id,current_period_start,current_period_end) VALUES($1,$2,$3,'space_pro','monthly',$4,'paddle','sub',now()-interval '1 month',now()-interval '1 minute')`, subID, spaceID, uuid.New(), status)
	require.NoError(t, err)
	return spaceID, subID
}

func correctionSeedUnfencedSpace(t *testing.T, ctx context.Context, pool *pgxpool.Pool, status string) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	spaceID, purchaserID, subID := uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO space_subscriptions(id,space_id,purchaser_account_id,plan,billing_period,status,provider,provider_subscription_id,current_period_start,current_period_end) VALUES($1,$2,$3,'space_pro','monthly',$4,'paddle','sub',now()-interval '1 month',now()-interval '1 minute')`, subID, spaceID, purchaserID, status)
	require.NoError(t, err)
	return spaceID, purchaserID, subID
}

func correctionWaitForBlockedOrCompleted(t *testing.T, ctx context.Context, pool *pgxpool.Pool, done <-chan error, want int) (error, bool) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return err, true
		default:
		}
		var blocked int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock'`).Scan(&blocked))
		if blocked >= want {
			return nil, false
		}
		select {
		case <-ctx.Done():
			t.Fatalf("only %d of %d Subscription transactions reached lifecycle lock", blocked, want)
		case err := <-done:
			return err, true
		case <-ticker.C:
		}
	}
}

func correctionWaitForBlocked(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock'`).Scan(&blocked))
		if blocked >= want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("only %d of %d Subscription transactions reached lifecycle lock", blocked, want)
		case <-ticker.C:
		}
	}
}
