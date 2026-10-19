package ordinarycall

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"voice/backend/voice/internal/retirementproof"
)

const CutoverEffectPlanProtocol = "voice.native-ordinary.effect-plan.v1"

type CutoverEffectStep string

const (
	CutoverPreserveOrCancelEnsure CutoverEffectStep = "PRESERVE_OR_CANCEL_ENSURE"
	CutoverQualifyMediaRemoval    CutoverEffectStep = "QUALIFY_MEDIA_REMOVAL"
	CutoverVerifyNativeFence      CutoverEffectStep = "VERIFY_NATIVE_FENCE"
)

type CutoverEffectPlanRef struct {
	Work  retirementproof.OwnerWorkRef
	Hash  retirementproof.Digest
	Count uint64
}

type CutoverPlannedEffect struct {
	Effect        Effect
	StepKind      CutoverEffectStep
	OriginalBytes []byte
	OriginalHash  retirementproof.Digest
	TargetHash    retirementproof.Digest
}

// CutoverEffectPlan is source intent, not a completed effect or execution permit.
// Original Applied values retain their SDK/native meaning without modification.
type CutoverEffectPlan struct {
	Ref            CutoverEffectPlanRef
	CanonicalBytes []byte
	Effects        []CutoverPlannedEffect
}

type cutoverEffectPlanWire struct {
	Protocol                    string
	Work                        retirementproof.OwnerWorkRef
	WorkHash, ClosureTargetHash retirementproof.Digest
	RoomID, Incarnation         uuid.UUID
	CapturedVersion             uint64
	Effects                     []CutoverPlannedEffect
}

type cutoverEffectPlanTransaction struct {
	tx   pgx.Tx
	wire cutoverEffectPlanWire
	plan CutoverEffectPlan
}

// Planning writes source intent only. It requires maintenance custody but never
// substitutes it for resource proof, accepted R/P, or native completion authority.
func (o *CutoverDrainOwner) PrepareCutoverEffectPlan(ctx context.Context, ref retirementproof.OwnerWorkRef) (CutoverEffectPlanRef, error) {
	state, err := o.effectPlanTransaction(ctx, ref, false)
	if err != nil {
		return CutoverEffectPlanRef{}, err
	}
	defer rollback(state.tx)
	old, err := readStoredEffectPlan(ctx, state.tx, state.plan, state.wire)
	if err == nil {
		return old.Ref, state.tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CutoverEffectPlanRef{}, err
	}
	_, err = state.tx.Exec(ctx, `INSERT INTO voice_ordinary_cutover_effect_plans(work_id,protocol,work_sha256,closure_target_sha256,plan_bytes,plan_sha256,effect_count) VALUES($1,$2,$3,$4,$5,$6,$7)`, ref.WorkID, CutoverEffectPlanProtocol, state.wire.WorkHash[:], state.wire.ClosureTargetHash[:], state.plan.CanonicalBytes, state.plan.Ref.Hash[:], int64(state.plan.Ref.Count))
	if err != nil {
		return CutoverEffectPlanRef{}, err
	}
	for _, item := range state.plan.Effects {
		_, err = state.tx.Exec(ctx, `INSERT INTO voice_ordinary_cutover_effect_plan_items(work_id,effect_id,effect_revision,effect_kind,step_kind,source_applied,effect_bytes,effect_sha256,target_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, ref.WorkID, item.Effect.ID, int64(item.Effect.Revision), string(item.Effect.Kind), string(item.StepKind), item.Effect.Applied, item.OriginalBytes, item.OriginalHash[:], item.TargetHash[:])
		if err != nil {
			return CutoverEffectPlanRef{}, err
		}
	}
	return state.plan.Ref, state.tx.Commit(ctx)
}

// Read rejects a changed current effect set/revision. Returning a plan is not
// authorization to execute it; qualified commands must recheck under their own
// native transaction and preserve separate external resource continuity.
func (o *CutoverDrainOwner) ReadCutoverEffectPlan(ctx context.Context, ref CutoverEffectPlanRef) (CutoverEffectPlan, error) {
	state, err := o.effectPlanTransaction(ctx, ref.Work, true)
	if err != nil {
		return CutoverEffectPlan{}, err
	}
	defer rollback(state.tx)
	if state.plan.Ref != ref {
		return CutoverEffectPlan{}, ErrConflict
	}
	plan, err := readStoredEffectPlan(ctx, state.tx, state.plan, state.wire)
	if err != nil {
		return CutoverEffectPlan{}, err
	}
	return plan, state.tx.Commit(ctx)
}

func (o *CutoverDrainOwner) effectPlanTransaction(ctx context.Context, ref retirementproof.OwnerWorkRef, read bool) (result *cutoverEffectPlanTransaction, err error) {
	if o == nil || o.Authority == nil {
		return nil, ErrUnavailable
	}
	work, err := o.ReadOwnerWork(ctx, ref)
	if err != nil {
		return nil, err
	}
	if err = o.Authority.CheckMaintenance(ctx, work.Cut); err != nil {
		return nil, err
	}
	var installed bool
	if err = o.Store.pool.QueryRow(ctx, `SELECT to_regclass('voice_ordinary_cutover_effect_plans') IS NOT NULL AND to_regclass('voice_ordinary_cutover_effect_plan_items') IS NOT NULL`).Scan(&installed); err != nil {
		return nil, err
	}
	if !installed {
		return nil, ErrUnavailable
	}
	var request cutoverRequest
	if json.Unmarshal(work.OwnerRequestBytes, &request) != nil {
		return nil, ErrConflict
	}
	tx, err := o.Store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if result == nil {
			rollback(tx)
		}
	}()
	lock := " FOR UPDATE"
	if read {
		lock = " FOR SHARE"
	}
	var body []byte
	if err = tx.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1`+lock, request.Expected.RoomID).Scan(&body); err != nil {
		return nil, mapStoreError(err)
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,hashtext($2))`, int32(0x4f524443), ref.WorkID.String()); err != nil {
		return nil, err
	}
	locked, err := readCutoverWork(ctx, tx, ref.WorkID)
	if err != nil {
		return nil, err
	}
	a, _ := json.Marshal(work)
	b, _ := json.Marshal(locked)
	if locked.Ref != ref || !bytes.Equal(a, b) {
		return nil, ErrConflict
	}
	var status string
	if err = tx.QueryRow(ctx, `SELECT state FROM voice_ordinary_cutover_works WHERE work_id=$1`, ref.WorkID).Scan(&status); err != nil {
		return nil, err
	}
	if !read && status != "PREPARED" {
		return nil, ErrConflict
	}
	wire, plan, err := deriveCutoverEffectPlan(locked)
	if err != nil {
		return nil, err
	}
	current, err := decodeRoom(body, request.Expected.RoomID)
	if err != nil {
		return nil, err
	}
	if current.Intent.OperationID != wire.Incarnation || current.Version < wire.CapturedVersion || (current.Stage != StageClosing && current.Stage != StageAborting && current.Stage != StageClosed) || len(current.Effects) != len(plan.Effects) {
		return nil, ErrConflict
	}
	actual := map[uuid.UUID][]byte{}
	for _, effect := range current.Effects {
		if _, exists := actual[effect.ID]; exists {
			return nil, ErrConflict
		}
		actual[effect.ID], err = json.Marshal(effect)
		if err != nil {
			return nil, err
		}
	}
	for _, item := range plan.Effects {
		if !bytes.Equal(actual[item.Effect.ID], item.OriginalBytes) {
			return nil, ErrConflict
		}
	}
	return &cutoverEffectPlanTransaction{tx: tx, wire: wire, plan: plan}, nil
}

func deriveCutoverEffectPlan(work retirementproof.OwnerWork) (cutoverEffectPlanWire, CutoverEffectPlan, error) {
	var wire cutoverEffectPlanWire
	var plan CutoverEffectPlan
	targets, err := validateCutoverManifest(work)
	if err != nil {
		return wire, plan, err
	}
	var request cutoverRequest
	if json.Unmarshal(work.OwnerRequestBytes, &request) != nil {
		return wire, plan, ErrConflict
	}
	var closure *Room
	var targetHash retirementproof.Digest
	for _, target := range targets {
		if target.Kind != "closure" {
			continue
		}
		closure, err = decodeRoom(target.Bytes, request.Expected.RoomID)
		if err != nil {
			return wire, plan, err
		}
		body, e := json.Marshal(target)
		if e != nil {
			return wire, plan, e
		}
		targetHash = retirementproof.Hash(cutoverEntryDomain, body)
	}
	if closure == nil || len(closure.Effects) == 0 {
		return wire, plan, ErrConflict
	}
	encoded, err := json.Marshal(work)
	if err != nil {
		return wire, plan, err
	}
	wire = cutoverEffectPlanWire{Protocol: CutoverEffectPlanProtocol, Work: work.Ref, WorkHash: retirementproof.Hash(retirementproof.WorkDomain, encoded), ClosureTargetHash: targetHash, RoomID: closure.Intent.RoomID, Incarnation: closure.Intent.OperationID, CapturedVersion: closure.Version}
	seen := map[uuid.UUID]bool{}
	for _, effect := range closure.Effects {
		if effect.ID == uuid.Nil || effect.OperationID == uuid.Nil || effect.Revision > math.MaxInt64 || seen[effect.ID] {
			return cutoverEffectPlanWire{}, plan, ErrConflict
		}
		seen[effect.ID] = true
		var step CutoverEffectStep
		switch effect.Kind {
		case EffectEnsure:
			step = CutoverPreserveOrCancelEnsure
		case EffectClose, EffectRemoveMember:
			step = CutoverQualifyMediaRemoval
		case EffectReleaseFence:
			step = CutoverVerifyNativeFence
		default:
			return cutoverEffectPlanWire{}, plan, ErrConflict
		}
		body, e := json.Marshal(effect)
		if e != nil {
			return cutoverEffectPlanWire{}, plan, e
		}
		wire.Effects = append(wire.Effects, CutoverPlannedEffect{Effect: effect, StepKind: step, OriginalBytes: body, OriginalHash: retirementproof.Hash("voice.native-ordinary.effect-plan.effect.v1", body), TargetHash: targetHash})
	}
	sort.Slice(wire.Effects, func(i, j int) bool {
		return bytes.Compare(wire.Effects[i].Effect.ID[:], wire.Effects[j].Effect.ID[:]) < 0
	})
	encoded, err = json.Marshal(wire)
	if err != nil {
		return cutoverEffectPlanWire{}, plan, err
	}
	plan = CutoverEffectPlan{Ref: CutoverEffectPlanRef{Work: work.Ref, Hash: retirementproof.Hash(CutoverEffectPlanProtocol, encoded), Count: uint64(len(wire.Effects))}, CanonicalBytes: encoded, Effects: wire.Effects}
	return wire, plan, nil
}

func readStoredEffectPlan(ctx context.Context, tx pgx.Tx, expected CutoverEffectPlan, wire cutoverEffectPlanWire) (CutoverEffectPlan, error) {
	var protocol string
	var body, hash, workHash, targetHash []byte
	var count int64
	err := tx.QueryRow(ctx, `SELECT protocol,plan_bytes,plan_sha256,work_sha256,closure_target_sha256,effect_count FROM voice_ordinary_cutover_effect_plans WHERE work_id=$1`, expected.Ref.Work.WorkID).Scan(&protocol, &body, &hash, &workHash, &targetHash, &count)
	if err != nil {
		return CutoverEffectPlan{}, err
	}
	if protocol != CutoverEffectPlanProtocol || count != int64(expected.Ref.Count) || !bytes.Equal(body, expected.CanonicalBytes) || !bytes.Equal(hash, expected.Ref.Hash[:]) || !bytes.Equal(workHash, wire.WorkHash[:]) || !bytes.Equal(targetHash, wire.ClosureTargetHash[:]) {
		return CutoverEffectPlan{}, ErrConflict
	}
	rows, err := tx.Query(ctx, `SELECT effect_id,effect_revision,effect_kind,step_kind,source_applied,effect_bytes,effect_sha256,target_sha256 FROM voice_ordinary_cutover_effect_plan_items WHERE work_id=$1 ORDER BY effect_id`, expected.Ref.Work.WorkID)
	if err != nil {
		return CutoverEffectPlan{}, err
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var id uuid.UUID
		var revision int64
		var kind, step string
		var applied bool
		var effectBytes, effectHash, targetHash []byte
		if err = rows.Scan(&id, &revision, &kind, &step, &applied, &effectBytes, &effectHash, &targetHash); err != nil {
			return CutoverEffectPlan{}, err
		}
		if i >= len(expected.Effects) {
			return CutoverEffectPlan{}, ErrConflict
		}
		item := expected.Effects[i]
		if id != item.Effect.ID || revision != int64(item.Effect.Revision) || kind != string(item.Effect.Kind) || step != string(item.StepKind) || applied != item.Effect.Applied || !bytes.Equal(effectBytes, item.OriginalBytes) || !bytes.Equal(effectHash, item.OriginalHash[:]) || !bytes.Equal(targetHash, item.TargetHash[:]) {
			return CutoverEffectPlan{}, ErrConflict
		}
		i++
	}
	if err = rows.Err(); err != nil {
		return CutoverEffectPlan{}, err
	}
	if i != len(expected.Effects) {
		return CutoverEffectPlan{}, ErrConflict
	}
	return expected, nil
}
