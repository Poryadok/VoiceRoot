package partysearch

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	admission "voice/backend/matchmaking/internal/mmadmission"
	wire "voice/backend/pkg/mmadmissionwire"
)

// A code-owned LOCAL_OWNED receipt fixture proves canonical database validation,
// not controller custody or an installed admission.
func historyReceiptFixture(t *testing.T, pool *pgxpool.Pool) (admission.Binding, wire.ArmedReceiptV1, wire.Hash) {
	t.Helper()
	b := admissionFixture()
	wb := admission.WireBinding(b)
	wb.AdmissionReceiptHash = wire.Hash{}
	receipt := wire.ArmedReceiptV1{Version: 1, AuthorityScope: wire.LocalOwned, ScopeID: wire.ID(uuid.New()), ControllerID: wire.ID(uuid.New()), TransitionID: wb.TransitionID, DatabaseLineageID: wire.ID(uuid.New()), Binding: wb, LaunchSpecHash: wb.LaunchManifestHash, ClosureEvidenceHash: wire.Hash{1}, PreparationEvidenceHash: wire.Hash{2}}
	hash, err := wire.ArmedHash(receipt)
	require.NoError(t, err)
	b.AdmissionReceiptHash = [32]byte(hash)
	root, err := wire.HistoryGenesisHash(hash, receipt.DatabaseLineageID)
	require.NoError(t, err)
	raw, err := json.Marshal(receipt)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `INSERT INTO mm_native_admission_receipts VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, hash[:], raw, uuid.UUID(receipt.DatabaseLineageID), b.AuthorityIncarnation, b.TransitionID, int64(b.AdmissionGeneration), b.EnvironmentID, b.InstallationID)
	require.NoError(t, err)
	insertAdmissionFixture(t, pool, b)
	_, err = pool.Exec(context.Background(), `UPDATE mm_native_admission_head SET database_lineage_id=$1,continuity_history_root_sha256=$2`, uuid.UUID(receipt.DatabaseLineageID), root[:])
	require.NoError(t, err)
	return b, receipt, root
}

func TestAdmissionHistoryValidatesCanonicalReceiptInSameCut(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	b, receipt, root := historyReceiptFixture(t, pool)
	read := func(expected wire.Binding) (admission.HistoryCut, error) {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		return admission.ReadHistory(ctx, tx, expected)
	}
	wb := admission.WireBinding(b)
	cut, err := read(wb)
	require.NoError(t, err)
	expectedHash, err := wire.BindingHash(wb)
	require.NoError(t, err)
	require.Equal(t, wire.AdmissionStampV1{Version: 1, BindingHash: expectedHash, DatabaseLineageID: receipt.DatabaseLineageID, ContinuityHistoryRootHash: root}, cut.Stamp())
	scope, scopeID, controllerID := cut.Authority()
	require.Equal(t, receipt.AuthorityScope, scope)
	require.Equal(t, receipt.ScopeID, scopeID)
	require.Equal(t, receipt.ControllerID, controllerID)
	origin := wire.OriginV1{Version: 1, EnvironmentID: receipt.Binding.EnvironmentID, InstallationID: receipt.Binding.InstallationID, AuthorityIncarnation: receipt.Binding.AuthorityIncarnation, DatabaseLineageID: receipt.DatabaseLineageID, OriginTransitionID: receipt.TransitionID, OriginAdmissionGeneration: receipt.Binding.AdmissionGeneration, OriginAdmissionReceiptHash: wb.AdmissionReceiptHash, SourceKind: "NATIVE_VOICE", OwnerSourceReceiptOrCaptureID: wire.ID(uuid.New()), PartyID: wire.ID(uuid.New()), CreationOperationID: wire.ID(uuid.New()), CreationRequestHash: wire.Hash{1}, PrivateSourceManifestHash: wire.Hash{2}}
	origin.OriginHash, err = wire.OriginHash(origin)
	require.NoError(t, err)
	require.True(t, cut.AllowsOrigin(origin, admission.NativeSourceVersion))
	require.False(t, cut.AllowsOrigin(origin, "UNIMPLEMENTED"))
	wb.AdmissionGeneration++
	_, err = read(wb)
	require.ErrorIs(t, err, admission.ErrUnavailable)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET continuity_history_root_sha256=$1`, make([]byte, 32))
	require.NoError(t, err)
	_, err = read(admission.WireBinding(b))
	require.ErrorIs(t, err, admission.ErrUnavailable)

	previous := wire.Hash(b.AdmissionReceiptHash)
	next := receipt
	next.Binding.TransitionID = wire.ID(uuid.New())
	next.TransitionID = next.Binding.TransitionID
	next.Binding.AdmissionGeneration++
	next.PredecessorReceiptHash = &previous
	next.PreviousReceiptHash = &previous
	nextHash, err := wire.ArmedHash(next)
	require.NoError(t, err)
	raw, err := json.Marshal(next)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO mm_native_admission_receipts VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, nextHash[:], raw, uuid.UUID(next.DatabaseLineageID), b.AuthorityIncarnation, uuid.UUID(next.TransitionID), int64(next.Binding.AdmissionGeneration), b.EnvironmentID, b.InstallationID)
	require.NoError(t, err)
	nextBinding, err := wire.ReceiptBinding(next)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET transition_id=$1,admission_generation=$2,admission_receipt_sha256=$3,continuity_history_root_sha256=$4`, uuid.UUID(next.TransitionID), int64(next.Binding.AdmissionGeneration), nextHash[:], root[:])
	require.NoError(t, err)
	_, err = read(nextBinding)
	require.ErrorIs(t, err, admission.ErrUnavailable, "missing continuity cannot be treated as genesis")
	record := wire.ContinuityRecordV1{Version: 1, EnvironmentID: next.Binding.EnvironmentID, InstallationID: next.Binding.InstallationID, AuthorityIncarnation: next.Binding.AuthorityIncarnation, DatabaseLineageID: next.DatabaseLineageID, PredecessorReceiptHash: previous, SuccessorReceiptHash: nextHash, SupportedSourceVersions: []string{admission.NativeSourceVersion}, PreviousHistoryRootHash: root}
	nextRoot, err := wire.ContinuityHash(record)
	require.NoError(t, err)
	raw, err = json.Marshal(record)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO mm_native_continuity_records VALUES($1,$2,$3,$4,$1)`, nextRoot[:], raw, previous[:], nextHash[:])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET continuity_history_root_sha256=$1`, nextRoot[:])
	require.NoError(t, err)
	rolled, err := read(nextBinding)
	require.NoError(t, err)
	require.NotEqual(t, cut.Stamp(), rolled.Stamp())
	require.True(t, rolled.AllowsOrigin(origin, admission.NativeSourceVersion))
	_, err = read(admission.WireBinding(b))
	require.ErrorIs(t, err, admission.ErrUnavailable, "old current stamp is not reusable after rollout")
	changedOrigin := origin
	changedOrigin.AuthorityIncarnation = wire.ID(uuid.New())
	changedOrigin.OriginHash, err = wire.OriginHash(changedOrigin)
	require.NoError(t, err)
	require.False(t, rolled.AllowsOrigin(changedOrigin, admission.NativeSourceVersion))
	changedOrigin = origin
	changedOrigin.SourceKind = "MM_LFP_CONSENT"
	changedOrigin.OriginHash, err = wire.OriginHash(changedOrigin)
	require.NoError(t, err)
	require.False(t, rolled.AllowsOrigin(changedOrigin, admission.NativeSourceVersion), "unimplemented LFP version cannot qualify by family reinterpretation")

	closer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	require.NoError(t, admission.AcquireCloseGate(ctx, closer))
	forward, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	_, err = admission.ReadHistory(bounded, forward, nextBinding)
	cancel()
	require.ErrorIs(t, err, admission.ErrUnavailable, "RC history cannot wait past a closer")
	require.NoError(t, forward.Rollback(ctx))
	require.NoError(t, closer.Rollback(ctx))
	forward, err = pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	_, err = admission.ReadHistory(ctx, forward, nextBinding)
	require.NoError(t, err)
	require.NoError(t, forward.Rollback(ctx))

	// Self-consistent SQL indexes and head cannot bless substituted receipt
	// bytes: canonical receipt hashes still have to match their selected key.
	forged := receipt
	forged.Binding.TransitionID = wire.ID(uuid.New())
	forged.TransitionID = forged.Binding.TransitionID
	forged.Binding.AdmissionGeneration += 2
	forgedHash, err := wire.ArmedHash(forged)
	require.NoError(t, err)
	forgedHash[31] ^= 1
	raw, err = json.Marshal(forged)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO mm_native_admission_receipts VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, forgedHash[:], raw, uuid.UUID(forged.DatabaseLineageID), b.AuthorityIncarnation, uuid.UUID(forged.TransitionID), int64(forged.Binding.AdmissionGeneration), b.EnvironmentID, b.InstallationID)
	require.NoError(t, err)
	forgedRoot, err := wire.HistoryGenesisHash(forgedHash, forged.DatabaseLineageID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET transition_id=$1,admission_generation=$2,admission_receipt_sha256=$3,continuity_history_root_sha256=$4`, uuid.UUID(forged.TransitionID), int64(forged.Binding.AdmissionGeneration), forgedHash[:], forgedRoot[:])
	require.NoError(t, err)
	forgedBinding, err := wire.ReceiptBinding(forged)
	require.NoError(t, err)
	forgedBinding.AdmissionReceiptHash = forgedHash
	_, err = read(forgedBinding)
	require.ErrorIs(t, err, admission.ErrUnavailable)
}
