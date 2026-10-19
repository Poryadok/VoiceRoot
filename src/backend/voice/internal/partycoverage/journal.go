package partycoverage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"math"
)

var ErrConflict = errors.New("voice party cutover conflict")

// These storage methods are private: only the resource-observing executor may
// advance them. They are not a public operator-proof or runtime minting API.
type cutPlan struct {
	OperationID uuid.UUID
	Binding     RuntimeBinding
	Scope       []byte
}
type cutState struct {
	OperationID, CoverageID uuid.UUID
	Phase                   string
	Revision                uint64
}
type journal struct{ conn *pgx.Conn }

func (j journal) prepare(ctx context.Context, p cutPlan) (cutState, error) {
	if j.conn == nil || p.OperationID == uuid.Nil || p.Binding.Validate() != nil || !json.Valid(p.Scope) {
		return cutState{}, ErrUnavailable
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return cutState{}, err
	}
	sum := sha256.Sum256(raw)
	tx, err := j.conn.Begin(ctx)
	if err != nil {
		return cutState{}, err
	}
	defer tx.Rollback(ctx)
	// The bounded cutover lock is never taken by native membership writers.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(191015191015)`); err != nil {
		return cutState{}, err
	}
	old, stateErr := lockedState(ctx, tx)
	if stateErr != nil && !errors.Is(stateErr, pgx.ErrNoRows) {
		return cutState{}, stateErr
	}
	if stateErr == nil && old.OperationID == p.OperationID {
		var saved, hash []byte
		if err = tx.QueryRow(ctx, `SELECT plan_bytes,plan_sha256 FROM voice_party_coverage_operations WHERE operation_id=$1`, p.OperationID).Scan(&saved, &hash); err != nil {
			return cutState{}, err
		}
		if !bytes.Equal(raw, saved) || !bytes.Equal(sum[:], hash) {
			return cutState{}, ErrConflict
		}
		return old, tx.Commit(ctx)
	}
	revision := uint64(1)
	if stateErr == nil {
		var epoch int64
		if old.Phase != "SUSPENDED" || old.Revision >= math.MaxInt64 {
			return cutState{}, ErrConflict
		}
		if err = tx.QueryRow(ctx, `SELECT coverage_epoch FROM voice_party_coverage_head WHERE singleton`).Scan(&epoch); err != nil {
			return cutState{}, err
		}
		if p.Binding.CoverageEpoch <= uint64(epoch) {
			return cutState{}, ErrConflict
		}
		revision = old.Revision + 1
	}
	if _, err = tx.Exec(ctx, `INSERT INTO voice_party_coverage_operations VALUES($1,$2,$3,$4,$5,$6)`, p.OperationID, p.Binding.EnvironmentID, p.Binding.AuthorityIncarnation, int64(p.Binding.CoverageEpoch), raw, sum[:]); err != nil {
		return cutState{}, err
	}
	if stateErr == nil {
		_, err = tx.Exec(ctx, `UPDATE voice_party_coverage_head SET environment_id=$1,authority_incarnation=$2,coverage_epoch=$3,phase='PREPARING',coverage_id=NULL,current_operation_id=$4,revision=$5 WHERE singleton`, p.Binding.EnvironmentID, p.Binding.AuthorityIncarnation, int64(p.Binding.CoverageEpoch), p.OperationID, int64(revision))
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO voice_party_coverage_head VALUES(true,$1,$2,$3,'PREPARING',NULL,$4,1)`, p.Binding.EnvironmentID, p.Binding.AuthorityIncarnation, int64(p.Binding.CoverageEpoch), p.OperationID)
	}
	if err != nil {
		return cutState{}, err
	}
	return cutState{OperationID: p.OperationID, Phase: "PREPARING", Revision: revision}, tx.Commit(ctx)
}
func (j journal) advance(ctx context.Context, s cutState, phase string, proof []byte) (cutState, error) {
	predecessor := map[string]string{"FROZEN": "PREPARING", "DRAINED": "FROZEN", "VALIDATED": "DRAINED", "SUSPENDED": "ACTIVE"}[phase]
	if j.conn == nil || predecessor == "" || s.OperationID == uuid.Nil || s.Revision == 0 || len(proof) == 0 {
		return cutState{}, ErrUnavailable
	}
	sum := sha256.Sum256(proof)
	tx, err := j.conn.Begin(ctx)
	if err != nil {
		return cutState{}, err
	}
	defer tx.Rollback(ctx)
	actual, err := lockedState(ctx, tx)
	if err != nil {
		return cutState{}, err
	}
	if actual.OperationID != s.OperationID {
		return cutState{}, ErrConflict
	}
	target := phase
	if phase == "VALIDATED" {
		target = "DRAINED"
	}
	var existing, hash []byte
	stepErr := tx.QueryRow(ctx, `SELECT proof_bytes,proof_sha256 FROM voice_party_coverage_steps WHERE operation_id=$1 AND phase=$2`, s.OperationID, phase).Scan(&existing, &hash)
	if stepErr == nil {
		if !bytes.Equal(proof, existing) || !bytes.Equal(sum[:], hash) || actual.Phase != target {
			return cutState{}, ErrConflict
		}
		return actual, tx.Commit(ctx)
	}
	if !errors.Is(stepErr, pgx.ErrNoRows) {
		return cutState{}, stepErr
	}
	if actual.Revision != s.Revision || actual.Phase != predecessor || actual.Revision >= math.MaxInt64 {
		return cutState{}, ErrConflict
	}
	if _, err = tx.Exec(ctx, `INSERT INTO voice_party_coverage_steps VALUES($1,$2,$3,$4)`, s.OperationID, phase, proof, sum[:]); err != nil {
		return cutState{}, err
	}
	actual.Phase = target
	actual.Revision++
	if target == "SUSPENDED" {
		actual.CoverageID = uuid.Nil
	}
	_, err = tx.Exec(ctx, `UPDATE voice_party_coverage_head SET phase=$1,revision=$2,coverage_id=CASE WHEN $1='SUSPENDED' THEN NULL ELSE coverage_id END WHERE singleton`, target, int64(actual.Revision))
	if err != nil {
		return cutState{}, err
	}
	return actual, tx.Commit(ctx)
}
func (j journal) activate(ctx context.Context, s cutState) (cutState, error) {
	if j.conn == nil || s.OperationID == uuid.Nil || s.Revision == 0 {
		return cutState{}, ErrUnavailable
	}
	tx, err := j.conn.Begin(ctx)
	if err != nil {
		return cutState{}, err
	}
	defer tx.Rollback(ctx)
	actual, err := lockedState(ctx, tx)
	if err != nil {
		return cutState{}, err
	}
	if actual.OperationID != s.OperationID {
		return cutState{}, ErrConflict
	}
	if actual.Phase == "ACTIVE" {
		return actual, tx.Commit(ctx)
	}
	if actual.Phase != "DRAINED" || actual.Revision != s.Revision || actual.Revision >= math.MaxInt64 {
		return cutState{}, ErrConflict
	}
	var planRaw, planHash []byte
	if err = tx.QueryRow(ctx, `SELECT plan_bytes,plan_sha256 FROM voice_party_coverage_operations WHERE operation_id=$1`, s.OperationID).Scan(&planRaw, &planHash); err != nil {
		return cutState{}, err
	}
	sum := sha256.Sum256(planRaw)
	if !bytes.Equal(sum[:], planHash) {
		return cutState{}, ErrUnavailable
	}
	var plan cutPlan
	if json.Unmarshal(planRaw, &plan) != nil || plan.OperationID != s.OperationID || plan.Binding.Validate() != nil {
		return cutState{}, ErrUnavailable
	}
	proofs := map[string][32]byte{}
	rows, err := tx.Query(ctx, `SELECT phase,proof_bytes,proof_sha256 FROM voice_party_coverage_steps WHERE operation_id=$1 AND phase IN ('FROZEN','DRAINED','VALIDATED')`, s.OperationID)
	if err != nil {
		return cutState{}, err
	}
	for rows.Next() {
		var phase string
		var raw, hash []byte
		if err = rows.Scan(&phase, &raw, &hash); err != nil {
			rows.Close()
			return cutState{}, err
		}
		digest := sha256.Sum256(raw)
		if len(raw) == 0 || !bytes.Equal(digest[:], hash) {
			rows.Close()
			return cutState{}, ErrUnavailable
		}
		proofs[phase] = digest
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return cutState{}, err
	}
	if len(proofs) != 3 {
		return cutState{}, ErrUnavailable
	}
	id := uuid.NewSHA1(s.OperationID, []byte("voice.party.coverage.v1"))
	d := receiptDocument{ProtocolVersion: 1, CoverageID: id, OperationID: s.OperationID, Binding: plan.Binding, FenceProofHash: proofs["FROZEN"], DrainProofHash: proofs["DRAINED"], NativeProofHash: proofs["VALIDATED"]}
	raw, err := canonicalReceipt(d)
	if err != nil {
		return cutState{}, err
	}
	hash := sha256.Sum256(raw)
	b := plan.Binding
	if _, err = tx.Exec(ctx, `INSERT INTO voice_party_coverage_receipts VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, id, s.OperationID, b.EnvironmentID, b.AuthorityIncarnation, int64(b.CoverageEpoch), b.DatabaseIdentity[:], b.SourceCatalogHash[:], b.SchemaContractHash[:], b.WriterManifestHash[:], d.FenceProofHash[:], d.DrainProofHash[:], d.NativeProofHash[:], raw, hash[:]); err != nil {
		return cutState{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO voice_party_coverage_steps VALUES($1,'ACTIVE',$2,$3)`, s.OperationID, raw, hash[:]); err != nil {
		return cutState{}, err
	}
	actual.CoverageID = id
	actual.Phase = "ACTIVE"
	actual.Revision++
	if _, err = tx.Exec(ctx, `UPDATE voice_party_coverage_head SET phase='ACTIVE',coverage_id=$1,revision=$2 WHERE singleton`, id, int64(actual.Revision)); err != nil {
		return cutState{}, err
	}
	return actual, tx.Commit(ctx)
}

func lockedState(ctx context.Context, tx pgx.Tx) (cutState, error) {
	var s cutState
	var rev int64
	err := tx.QueryRow(ctx, `SELECT current_operation_id,coalesce(coverage_id,'00000000-0000-0000-0000-000000000000'),phase,revision FROM voice_party_coverage_head WHERE singleton FOR UPDATE`).Scan(&s.OperationID, &s.CoverageID, &s.Phase, &rev)
	if err != nil {
		return s, err
	}
	if rev <= 0 {
		return s, ErrUnavailable
	}
	s.Revision = uint64(rev)
	return s, nil
}
