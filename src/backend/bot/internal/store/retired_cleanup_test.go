package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

func TestPurgeBeforeBotDeletionDoesNotCompleteFromMissingTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL purge-before-owner-delete boundary")
	}
	ctx := t.Context()
	st := startBotLifecycleStore(t)
	owner, actor, space, purgeOperation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	bot, token, err := st.CreateBot(ctx, owner, "Retired cleanup obligation", "", `[]`, actor)
	require.NoError(t, err)
	_, err = st.GetBotByTokenHash(ctx, store.HashToken(token))
	require.NoError(t, err, "token lookup is admitted before owner revocation")
	intent, err := st.PrepareInstallation(ctx, bot.ID, space, actor, nil)
	require.NoError(t, err)
	require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
	// Existing minimal state from a distinct historical unrepresented retirement.
	// No Space/actor history or new proof is invented. Positively covering this
	// current Space must not clear that pre-existing uncertainty.
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET retired_cleanup_unresolved=true WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	manifest := &commonv1.ManifestBinding{ManifestId: purgeOperation.String(), ManifestSha256: make([]byte, 32), ItemCount: 0}
	fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: purgeOperation.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest}
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	fence.Generation, fence.DesiredState = 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	_, err = st.ApplySpaceLifecycleFence(ctx, fence)
	require.NoError(t, err)
	_, err = st.PurgeSpace(ctx, &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: purgeOperation.String(), Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, PurgeDecidedAt: timestamppb.New(time.Now()), Manifest: manifest})
	require.ErrorIs(t, err, store.ErrDeletionPending)
	purgeClaim, err := st.ClaimNextBotPurgeHandoff(ctx)
	require.NoError(t, err)
	saveRetirementEvidence(t, st, purgeClaim, bot.ID, true)
	_, err = st.RetireBotPurgeHandoff(ctx, purgeClaim)
	require.NoError(t, err)
	// Current P3 has exact positive proof. The separate pre-existing historical
	// unresolved bit still prevents absence from claiming whole-Bot completion.
	request := &botv1.DeleteBotRequest{BotId: bot.ID.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	// Controlled already-verified initiating facts exercise the existing local
	// atomic decision/revocation boundary, not Gateway/Auth transport admission.
	root, err := st.BeginBotDeletionWithAdmission(ctx, bot.ID, owner, store.DeletionAdmission{AccountID: owner, ProfileID: actor, SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	require.Equal(t, "PENDING", root.State)
	var savedDecision, savedSHA []byte
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT decision_bytes,decision_sha256,state FROM bot_deletion_operations WHERE operation_id=$1`, root.OperationID).Scan(&savedDecision, &savedSHA, &state))
	require.Equal(t, root.DecisionBytes, savedDecision)
	require.Equal(t, root.DecisionSHA256, savedSHA)
	require.Equal(t, "PENDING", state, "immutable root commits despite unresolved retired cleanup")
	claim, err := st.ClaimBotDeletion(ctx, root.OperationID)
	require.NoError(t, err)
	require.ErrorIs(t, st.CompleteBotDeletion(ctx, claim), store.ErrDeletionPending)
	require.ErrorIs(t, st.CompleteBotDeletion(ctx, &store.BotDeletion{OperationID: uuid.New(), ClaimGeneration: 1}), store.ErrDeletionClaimLost, "missing root preserves the existing claim error contract")
	current, err := st.GetBotByID(ctx, bot.ID)
	require.NoError(t, err)
	require.Equal(t, "disabled", current.Status, "uncertain retired cleanup must not prevent owner revocation")
	_, err = st.GetBotByTokenHash(ctx, store.HashToken(token))
	require.ErrorIs(t, err, store.ErrNotFound, "revoked Bot token cannot regain live authority")
	require.NoError(t, st.RetryBotDeletion(ctx, claim))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT decision_bytes,decision_sha256,state FROM bot_deletion_operations WHERE operation_id=$1`, root.OperationID).Scan(&savedDecision, &savedSHA, &state))
	require.Equal(t, "RETRY_WAIT", state)
	require.Equal(t, root.DecisionBytes, savedDecision, "recovery keeps the exact admitted owner decision")
	require.Equal(t, root.DecisionSHA256, savedSHA)
}

func TestRetiredCleanupPurgeRollbackAndFrozenJournalControls(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL purge transaction controls")
	}
	for _, frozen := range []bool{false, true} {
		name := "unrepresented_source"
		if frozen {
			name = "existing_frozen_global_source"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			st := startBotLifecycleStore(t)
			owner, space, op := uuid.New(), uuid.New(), uuid.New()
			bot, _, err := st.CreateBot(ctx, owner, name, "", `[]`, uuid.New())
			require.NoError(t, err)
			intent, err := st.PrepareInstallation(ctx, bot.ID, space, uuid.New(), nil)
			require.NoError(t, err)
			require.NoError(t, st.CompleteInstallation(ctx, intent, nil))
			if frozen {
				_, err = st.BeginBotDeletion(ctx, bot.ID, owner)
				require.NoError(t, err)
			}
			manifest := &commonv1.ManifestBinding{ManifestId: op.String(), ManifestSha256: make([]byte, 32)}
			fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: op.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest}
			_, err = st.ApplySpaceLifecycleFence(ctx, fence)
			require.NoError(t, err)
			fence.Generation, fence.DesiredState = 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
			_, err = st.ApplySpaceLifecycleFence(ctx, fence)
			require.NoError(t, err)
			purge := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: op.String(), Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, PurgeDecidedAt: timestamppb.Now(), Manifest: manifest}
			_, err = st.PurgeSpace(ctx, purge)
			require.ErrorIs(t, err, store.ErrDeletionPending)
			purgeClaim, err := st.ClaimNextBotPurgeHandoff(ctx)
			require.NoError(t, err)
			saveRetirementEvidence(t, st, purgeClaim, bot.ID, true)
			_, err = st.Pool.Exec(ctx, `ALTER TABLE bot_space_lifecycle_purge_receipts ADD CONSTRAINT injected_retired_marker_rollback CHECK(false)`)
			require.NoError(t, err)
			_, err = st.RetireBotPurgeHandoff(ctx, purgeClaim)
			require.Error(t, err)
			var flag bool
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retired_cleanup_unresolved FROM bots WHERE id=$1`, bot.ID).Scan(&flag))
			require.False(t, flag, "failed purge cannot persist a partial marker")
			_, err = st.Pool.Exec(ctx, `ALTER TABLE bot_space_lifecycle_purge_receipts DROP CONSTRAINT injected_retired_marker_rollback`)
			require.NoError(t, err)
			receipt, err := st.RetireBotPurgeHandoff(ctx, purgeClaim)
			require.NoError(t, err)
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retired_cleanup_unresolved FROM bots WHERE id=$1`, bot.ID).Scan(&flag))
			require.False(t, flag, "positively covered current sources do not create a new unresolved obligation")
			again, err := st.PurgeSpace(ctx, purge)
			require.NoError(t, err)
			require.Equal(t, receipt.ReceiptId, again.ReceiptId)
			if frozen {
				effects, e := st.BotDeletionEffects(ctx, func() uuid.UUID {
					var id uuid.UUID
					require.NoError(t, st.Pool.QueryRow(ctx, `SELECT operation_id FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&id))
					return id
				}())
				require.NoError(t, e)
				require.Empty(t, effects, "matched Space effects now use explicit local results, never remote receipt dispatch")
				var count int
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_effects WHERE state='LOCAL_P3_COVERED'`).Scan(&count))
				require.Equal(t, 2, count, "opaque expected effect membership survives")
				require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_terminal_effects WHERE state='PENDING'`).Scan(&count))
				require.Equal(t, 2, count, "P3 Space effects do not discharge global terminal obligations")
			}
			unrelated, _, err := st.CreateBot(ctx, uuid.New(), "Unrelated", "", `[]`, uuid.New())
			require.NoError(t, err)
			root, err := st.BeginBotDeletion(ctx, unrelated.ID, unrelated.OwnerAccountID)
			require.NoError(t, err)
			require.Equal(t, "PENDING", root.State, "both independent global terminal outcomes remain required even with zero Space targets")
		})
	}
}

func TestRetiredCleanupGuardOfficialMigrationAndDownFirstWriter(t *testing.T) {
	if testing.Short() {
		t.Skip("actual migration driver and PostgreSQL lock/isolation")
	}
	t.Run("official_empty_RC", func(t *testing.T) {
		// Exercise the official013 driver transition on its exact historical
		// prefix; down1 on the current016 cohort would test a different migration.
		directory := t.TempDir()
		entries, e := os.ReadDir("../../../migrations/bot_db")
		require.NoError(t, e)
		for _, entry := range entries {
			if entry.Name() < "000014" {
				raw, e := os.ReadFile(filepath.Join("../../../migrations/bot_db", entry.Name()))
				require.NoError(t, e)
				require.NoError(t, os.WriteFile(filepath.Join(directory, entry.Name()), raw, 0600))
			}
		}
		f := integrationtest.NewSourceMigrationFixture(t, t.Context(), directory)
		f.Run(true, "up")
		f.Run(true, "down", "1")
		var exists bool
		require.NoError(t, f.Pool.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='bots' AND column_name='retired_cleanup_unresolved')`).Scan(&exists))
		require.False(t, exists)
	})
	downBytes, err := os.ReadFile("../../../migrations/bot_db/000013_retired_cleanup_guard.down.sql")
	require.NoError(t, err)
	down := string(downBytes)
	t.Run("old_RR", func(t *testing.T) {
		ctx := t.Context()
		st := startBotStore(t)
		bot, _, err := st.CreateBot(ctx, uuid.New(), "Old snapshot", "", `[]`, uuid.New())
		require.NoError(t, err)
		old, err := st.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		require.NoError(t, err)
		defer func() { _ = old.Rollback(context.Background()) }()
		var flag bool
		require.NoError(t, old.QueryRow(ctx, `SELECT retired_cleanup_unresolved FROM bots WHERE id=$1`, bot.ID).Scan(&flag))
		require.False(t, flag)
		_, err = st.Pool.Exec(ctx, `UPDATE bots SET retired_cleanup_unresolved=true WHERE id=$1`, bot.ID)
		require.NoError(t, err)
		_, err = old.Exec(ctx, down)
		require.ErrorContains(t, err, "READ COMMITTED")
		require.NoError(t, old.Rollback(ctx))
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retired_cleanup_unresolved FROM bots WHERE id=$1`, bot.ID).Scan(&flag))
		require.True(t, flag)
	})
	t.Run("retained_first_writer", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		st := startBotStore(t)
		bot, _, err := st.CreateBot(ctx, uuid.New(), "First marker", "", `[]`, uuid.New())
		require.NoError(t, err)
		writer, err := st.Pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = writer.Rollback(context.Background()) }()
		_, err = writer.Exec(ctx, `UPDATE bots SET retired_cleanup_unresolved=true WHERE id=$1`, bot.ID)
		require.NoError(t, err)
		conn, err := st.Pool.Acquire(ctx)
		require.NoError(t, err)
		defer conn.Release()
		var pid int32
		require.NoError(t, conn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
		done := make(chan error, 1)
		go func() { _, e := conn.Exec(ctx, down); done <- e }()
		require.Eventually(t, func() bool {
			var waiting bool
			e := st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, pid).Scan(&waiting)
			return e == nil && waiting
		}, 5*time.Second, 10*time.Millisecond)
		require.NoError(t, writer.Commit(ctx))
		require.ErrorContains(t, <-done, "must not be dropped")
		_, err = conn.Exec(ctx, `ROLLBACK`)
		require.NoError(t, err)
		var flag bool
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT retired_cleanup_unresolved FROM bots WHERE id=$1`, bot.ID).Scan(&flag))
		require.True(t, flag)
	})
}
