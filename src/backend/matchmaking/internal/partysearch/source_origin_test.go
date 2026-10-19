package partysearch

import (
	"context"
	"testing"

	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	admission "voice/backend/matchmaking/internal/mmadmission"
	"voice/backend/matchmaking/internal/store"
	wire "voice/backend/pkg/mmadmissionwire"
)

func TestNativeSourcePartyOriginIsAtomicAndNeverBackfilled(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	historicalParams := partyParams(t, pool)
	historical, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, historicalParams)
	require.NoError(t, err, "pre-cut typed source remains history")
	binding, receipt, root := historyReceiptFixture(t, pool)
	parties := &store.PartyStore{Pool: pool, NativeAdmission: &binding}
	params := partyParams(t, pool)
	first, err := parties.CreateSourceParty(ctx, params)
	require.NoError(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM mm_native_party_origins WHERE party_id=$1`, first.Party.ID).Scan(&count))
	require.Equal(t, 1, count, "native source origin must commit with all sessions and queue intent")
	var raw, hash []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT origin_bytes,origin_sha256 FROM mm_native_party_origins WHERE party_id=$1`, first.Party.ID).Scan(&raw, &hash))
	var origin wire.OriginV1
	require.NoError(t, wire.Decode(raw, &origin))
	require.Equal(t, origin.OriginHash[:], hash)
	require.Equal(t, wire.ID(first.Party.ID), origin.PartyID)
	require.Equal(t, wire.ID(params.SearchOperationID), origin.CreationOperationID)
	require.Equal(t, wire.ID(params.SearchOperationID), origin.OwnerSourceReceiptOrCaptureID)
	require.Equal(t, wire.Hash(params.SearchRequestHash), origin.CreationRequestHash)
	require.Equal(t, receipt.DatabaseLineageID, origin.DatabaseLineageID)
	require.Equal(t, wire.Hash(binding.AdmissionReceiptHash), origin.OriginAdmissionReceiptHash)
	replay, err := parties.CreateSourceParty(ctx, params)
	require.NoError(t, err)
	require.True(t, replay.Replay)
	require.Equal(t, first.Party.ID, replay.Party.ID)
	var after []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT origin_bytes FROM mm_native_party_origins WHERE party_id=$1`, first.Party.ID).Scan(&after))
	require.Equal(t, raw, after)
	nextBinding := advanceOriginHistoryFixture(t, pool, binding, receipt, root)
	parties.NativeAdmission = &nextBinding
	replay, err = parties.CreateSourceParty(ctx, params)
	require.NoError(t, err, "compatible rollout must preserve the original source on replay")
	require.True(t, replay.Replay)
	require.NoError(t, pool.QueryRow(ctx, `SELECT origin_bytes FROM mm_native_party_origins WHERE party_id=$1`, first.Party.ID).Scan(&after))
	require.Equal(t, raw, after, "current admission rotation must not rewrite original origin")
	conflicting := params
	conflicting.SearchOperationID = partyParams(t, pool).SearchOperationID
	_, err = parties.CreateSourceParty(ctx, conflicting)
	require.ErrorIs(t, err, store.ErrActiveSearchExists)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM mm_native_party_origins`).Scan(&count))
	require.Equal(t, 1, count, "failed session insertion must roll back its new origin")
	_, err = parties.CreateSourceParty(ctx, historicalParams)
	require.Error(t, err, "historical capture cannot gain native origin by replay")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM mm_native_party_origins WHERE party_id=$1`, historical.Party.ID).Scan(&count))
	require.Zero(t, count)
}

// This fixture is a canonical LOCAL_OWNED history transition with no provider
// effects or installed launch; it cannot certify real controller retirement.
func advanceOriginHistoryFixture(t *testing.T, pool *pgxpool.Pool, b admission.Binding, r wire.ArmedReceiptV1, root wire.Hash) admission.Binding {
	t.Helper()
	ctx := context.Background()
	previous := wire.Hash(b.AdmissionReceiptHash)
	next := r
	next.Binding.TransitionID = wire.ID(uuid.New())
	next.TransitionID = next.Binding.TransitionID
	next.Binding.AdmissionGeneration++
	next.PredecessorReceiptHash = &previous
	next.PreviousReceiptHash = &previous
	hash, err := wire.ArmedHash(next)
	require.NoError(t, err)
	record := wire.ContinuityRecordV1{Version: 1, EnvironmentID: r.Binding.EnvironmentID, InstallationID: r.Binding.InstallationID, AuthorityIncarnation: r.Binding.AuthorityIncarnation, DatabaseLineageID: r.DatabaseLineageID, PredecessorReceiptHash: previous, SuccessorReceiptHash: hash, SupportedSourceVersions: []string{admission.NativeSourceVersion}, PreviousHistoryRootHash: root}
	nextRoot, err := wire.ContinuityHash(record)
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, admission.AcquireCloseGate(ctx, tx))
	_, err = tx.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	raw, err := json.Marshal(next)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO mm_native_admission_receipts VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, hash[:], raw, uuid.UUID(next.DatabaseLineageID), b.AuthorityIncarnation, uuid.UUID(next.TransitionID), int64(next.Binding.AdmissionGeneration), b.EnvironmentID, b.InstallationID)
	require.NoError(t, err)
	raw, err = json.Marshal(record)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO mm_native_continuity_records VALUES($1,$2,$3,$4,$1)`, nextRoot[:], raw, previous[:], hash[:])
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE mm_native_admission_head SET state='ADMITTED',transition_id=$1,admission_generation=$2,admission_receipt_sha256=$3,continuity_history_root_sha256=$4`, uuid.UUID(next.TransitionID), int64(next.Binding.AdmissionGeneration), hash[:], nextRoot[:])
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	b.TransitionID = uuid.UUID(next.TransitionID)
	b.AdmissionGeneration = next.Binding.AdmissionGeneration
	b.AdmissionReceiptHash = [32]byte(hash)
	return b
}
