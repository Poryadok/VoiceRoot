package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
)

func pendingPurgeRecoverySource(t *testing.T, st *store.BotStore) (uuid.UUID, *commonv1.SpacePurgeRequest) {
	t.Helper()
	actor, space := uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(t.Context(), uuid.New(), "P3 recovery", "", `[]`, actor)
	require.NoError(t, err)
	_, err = st.PrepareInstallation(t.Context(), bot.ID, space, actor, nil)
	require.NoError(t, err)
	purge := preparePurgeHandoffDecision(t, st, space)
	_, err = st.PurgeSpace(t.Context(), purge)
	require.ErrorIs(t, err, store.ErrDeletionPending)
	return bot.ID, purge
}

func purgeRecoveryRoleProof(t *testing.T, p *commonv1.SpacePurgeRequest) store.BotPurgeCoverageProof {
	t.Helper()
	p = proto.Clone(p).(*commonv1.SpacePurgeRequest)
	q := &spacev1.GetBotPurgeHandoffRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding)}
	original := &rolev1.RetireSpaceRequest{ProtocolVersion: 1, SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding), PurgeDecidedAt: proto.Clone(p.PurgeDecidedAt).(*timestamppb.Timestamp)}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(original)
	require.NoError(t, err)
	requestSHA := sha256.Sum256(append([]byte("voice.role.v1.RetireSpaceRequest\x00"), body...))
	r := &rolev1.RetireSpaceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, State: rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, RequestSha256: requestSHA[:], ManifestSha256: append([]byte(nil), p.Manifest.ManifestSha256...), RetiredAt: timestamppb.Now()}
	receipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	require.NoError(t, err)
	receiptSHA := sha256.Sum256(append([]byte("voice.role.v1.RetireSpaceReceipt\x00"), receipt...))
	response := &spacev1.GetBotPurgeHandoffResponse{ProtocolVersion: 1, RequestId: q.RequestId, SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding), Role: &spacev1.BotPurgeRoleHandoff{OriginalReceipt: r, OriginalRequestBytes: body, OriginalRequestSha256: requestSHA[:], OriginalReceiptBytes: receipt, OriginalReceiptSha256: receiptSHA[:]}}
	// Explicitly controlled producer originals: this tests local persistence and
	// source/claim validation, never protected relay authentication or Role effect.
	return store.BotPurgeCoverageProof{Purge: p, HandoffRequest: q, Handoff: response}
}

func TestBotPurgeClaimSkipsLockedWorkAndFencesOldRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL P3 claims")
	}
	st := startBotLifecycleStore(t)
	ctx := t.Context()
	_, p1 := pendingPurgeRecoverySource(t, st)
	_, p2 := pendingPurgeRecoverySource(t, st)
	tx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, `SELECT 1 FROM bot_space_purge_handoffs WHERE space_id=$1 FOR UPDATE`, p1.SpaceId)
	require.NoError(t, err)
	c2, err := st.ClaimNextBotPurgeHandoff(ctx)
	require.NoError(t, err)
	require.Equal(t, p2.SpaceId, c2.PurgeRequest().SpaceId, "busy earlier work must not block the due independent source")
	require.NoError(t, tx.Rollback(ctx))
	c1, err := st.ClaimNextBotPurgeHandoff(ctx)
	require.NoError(t, err)
	require.Equal(t, p1.SpaceId, c1.PurgeRequest().SpaceId)
	_, err = st.ClaimNextBotPurgeHandoff(ctx)
	require.ErrorIs(t, err, store.ErrDeletionClaimLost)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_space_purge_handoffs SET claimed_until=clock_timestamp()-interval '1 second' WHERE space_id=$1`, p2.SpaceId)
	require.NoError(t, err)
	c3, err := st.ClaimNextBotPurgeHandoff(ctx)
	require.NoError(t, err)
	require.Equal(t, p2.SpaceId, c3.PurgeRequest().SpaceId)
	require.Equal(t, c2.Sources(), c3.Sources())
	require.ErrorIs(t, st.RetryBotPurgeHandoff(ctx, c2), store.ErrDeletionClaimLost)
	require.NoError(t, st.RetryBotPurgeHandoff(ctx, c3))
	require.ErrorIs(t, st.RetryBotPurgeHandoff(ctx, c3), store.ErrDeletionClaimLost, "a released lease is already fenced even before another generation claims it")
	copy := c1.PurgeRequest()
	copy.Manifest.ManifestSha256[0] ^= 1
	require.True(t, proto.Equal(p1, c1.PurgeRequest()))
	sources := c1.Sources()
	sources[0].Intent.OperationID = uuid.New()
	require.NotEqual(t, sources[0].Intent.OperationID, c1.Sources()[0].Intent.OperationID)
}

func TestBotPurgeObservationOwnsBytesAcrossActualSpaceWait(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL P3 observation ownership")
	}
	st := startBotLifecycleStore(t)
	ctx := t.Context()
	bot, purge := pendingPurgeRecoverySource(t, st)
	claim, err := st.ClaimNextBotPurgeHandoff(ctx)
	require.NoError(t, err)
	proof := purgeRecoveryRoleProof(t, purge)
	original := proto.Clone(proof.Handoff).(*spacev1.GetBotPurgeHandoffResponse)
	blocker, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer blocker.Release()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,71431))`, purge.SpaceId)
	require.NoError(t, err)
	defer func() {
		_, _ = blocker.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,71431))`, purge.SpaceId)
	}()
	done := make(chan error, 1)
	go func() {
		done <- st.SaveBotPurgeCoverageObservation(ctx, claim, bot, []store.BotPurgeCoverageProof{proof})
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted)`).Scan(&waiting) == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	proof.Handoff.Role.OriginalReceipt.ReceiptId = uuid.NewString()
	proof.Purge.Manifest.ManifestSha256[0] ^= 1
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_unlock(hashtextextended($1,71431))`, purge.SpaceId)
	require.NoError(t, err)
	require.NoError(t, <-done)
	var body []byte
	var state string
	var covered *time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT coverage_bytes,state,covered_at FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, purge.SpaceId, bot).Scan(&body, &state, &covered))
	var envelope struct {
		Kind   string                     `json:"kind"`
		Proofs []struct{ Handoff []byte } `json:"proofs"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	require.Equal(t, "P3_COVERAGE", envelope.Kind)
	require.Len(t, envelope.Proofs, 1)
	var saved spacev1.GetBotPurgeHandoffResponse
	require.NoError(t, proto.Unmarshal(envelope.Proofs[0].Handoff, &saved))
	require.True(t, proto.Equal(original, &saved))
	require.Equal(t, "PENDING", state)
	require.Nil(t, covered)
	_, err = st.PurgeSpace(ctx, purge)
	require.ErrorIs(t, err, store.ErrDeletionPending, "valid Role observation is not native drain or full P3 completion")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_lifecycle_purge_receipts WHERE space_id=$1`, purge.SpaceId).Scan(&count))
	require.Zero(t, count)
}

func TestBotPurgeObservationRejectsExpiryAndChangedSource(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL P3 stale worker fencing")
	}
	t.Run("lease expires while Space guard is held", func(t *testing.T) {
		st := startBotLifecycleStore(t)
		ctx := t.Context()
		bot, purge := pendingPurgeRecoverySource(t, st)
		claim, err := st.ClaimNextBotPurgeHandoff(ctx)
		require.NoError(t, err)
		proof := purgeRecoveryRoleProof(t, purge)
		blocker, err := st.Pool.Acquire(ctx)
		require.NoError(t, err)
		defer blocker.Release()
		_, err = blocker.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1,71431))`, purge.SpaceId)
		require.NoError(t, err)
		defer func() {
			_, _ = blocker.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtextextended($1,71431))`, purge.SpaceId)
		}()
		done := make(chan error, 1)
		go func() {
			done <- st.SaveBotPurgeCoverageObservation(ctx, claim, bot, []store.BotPurgeCoverageProof{proof})
		}()
		require.Eventually(t, func() bool {
			var waiting bool
			return st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted)`).Scan(&waiting) == nil && waiting
		}, 5*time.Second, 10*time.Millisecond)
		_, err = st.Pool.Exec(ctx, `UPDATE bot_space_purge_handoffs SET claimed_until=clock_timestamp()-interval '1 second' WHERE space_id=$1`, purge.SpaceId)
		require.NoError(t, err)
		_, err = blocker.Exec(ctx, `SELECT pg_advisory_unlock(hashtextextended($1,71431))`, purge.SpaceId)
		require.NoError(t, err)
		require.ErrorIs(t, <-done, store.ErrDeletionClaimLost)
		var count int
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_sources WHERE space_id=$1 AND coverage_bytes IS NOT NULL`, purge.SpaceId).Scan(&count))
		require.Zero(t, count)
	})
	t.Run("a changed frozen source cannot borrow the old claim", func(t *testing.T) {
		st := startBotLifecycleStore(t)
		ctx := t.Context()
		bot, purge := pendingPurgeRecoverySource(t, st)
		claim, err := st.ClaimNextBotPurgeHandoff(ctx)
		require.NoError(t, err)
		source := claim.Sources()[0]
		source.ActorProfileID = uuid.New()
		raw, err := json.Marshal(source)
		require.NoError(t, err)
		digest := sha256.Sum256(raw)
		_, err = st.Pool.Exec(ctx, `UPDATE bot_space_purge_sources SET source_bytes=$3,source_sha256=$4 WHERE space_id=$1 AND bot_id=$2`, purge.SpaceId, bot, raw, digest[:])
		require.NoError(t, err)
		require.ErrorIs(t, st.SaveBotPurgeCoverageObservation(ctx, claim, bot, []store.BotPurgeCoverageProof{purgeRecoveryRoleProof(t, purge)}), store.ErrSpaceLifecycleConflict)
		var count int
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_sources WHERE space_id=$1 AND coverage_bytes IS NOT NULL`, purge.SpaceId).Scan(&count))
		require.Zero(t, count)
	})
	t.Run("an extra source cannot hide behind the old header count", func(t *testing.T) {
		st := startBotLifecycleStore(t)
		ctx := t.Context()
		bot, purge := pendingPurgeRecoverySource(t, st)
		claim, err := st.ClaimNextBotPurgeHandoff(ctx)
		require.NoError(t, err)
		// Explicit consistency fault, not a legitimate source-discovery writer.
		source := claim.Sources()[0]
		source.BotID = uuid.New()
		raw, err := json.Marshal(source)
		require.NoError(t, err)
		digest := sha256.Sum256(raw)
		_, err = st.Pool.Exec(ctx, `INSERT INTO bot_space_purge_sources(space_id,bot_id,source_bytes,source_sha256) VALUES($1,$2,$3,$4)`, purge.SpaceId, source.BotID, raw, digest[:])
		require.NoError(t, err)
		require.ErrorIs(t, st.SaveBotPurgeCoverageObservation(ctx, claim, bot, []store.BotPurgeCoverageProof{purgeRecoveryRoleProof(t, purge)}), store.ErrSpaceLifecycleConflict)
		var count int
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_sources WHERE space_id=$1 AND coverage_bytes IS NOT NULL`, purge.SpaceId).Scan(&count))
		require.Zero(t, count)
	})
}
