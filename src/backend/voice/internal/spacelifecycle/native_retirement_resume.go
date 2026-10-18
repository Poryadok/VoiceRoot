package spacelifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/spacebarrier"
)

const ownerReceiptDomain = "voice.native-space.retirement.receipt.v1"

type ownerTerminalTarget struct {
	TargetID, Kind string
	TargetHash     retirementproof.Digest
	EffectKind     string
	ProofID        uuid.UUID
	Witness        json.RawMessage
}
type nativeOwnerReceiptDocument struct {
	Protocol                                     uint32
	Kind                                         string
	Work                                         retirementproof.OwnerWorkRef
	Proof                                        retirementproof.ProofRef
	WorkHash, TargetManifestHash, ResolutionHash retirementproof.Digest
	Cut                                          retirementproof.CutBinding
	TerminalTargets                              []ownerTerminalTarget
	PublicP3Receipt                              []byte
	CompletedAt                                  time.Time
}

func (o *NativeRetirementOwner) ResumeCutoverDrain(ctx context.Context, ref retirementproof.OwnerWorkRef, proof retirementproof.ProofRef) (*OwnerDrainReceipt, error) {
	if o == nil || o.Authority == nil || o.Verifier == nil || o.ProjectionFence == nil {
		return nil, ErrUnavailable
	}
	work, err := o.ReadOwnerWork(ctx, ref)
	if err != nil {
		return nil, err
	}
	if err := o.Authority.CheckMaintenance(ctx, work.Cut); err != nil {
		return nil, err
	}
	verified, err := o.Verifier.VerifyForWork(ctx, proof, ref)
	if err != nil {
		return nil, err
	}
	if verified.ProofRef() != proof || verified.ValidateFor(work) != nil {
		return nil, ErrConflict
	}
	record, err := readOwnerRecord(ctx, o.Store.pool, ref.WorkID)
	if err != nil {
		return nil, err
	}
	if record.state == "COMPLETE" {
		return ownerSavedReceipt(record, proof)
	}
	if err := o.acceptOwnerProof(ctx, work, verified); err != nil {
		return nil, err
	}
	// Rotation installs RESTORING before fresh PG. Old tickets cannot follow
	// the maintenance cut into a replacement incarnation, even at the same G.
	if err := o.publishOwnerProjectionGate(ctx, work, false); err != nil {
		return nil, err
	}
	targets, err := validateNativeOwnerManifest(work)
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		if target.Kind != "native" && target.Kind != "legacy" {
			continue
		}
		captured, err := decodeTarget(target.Data)
		if err != nil {
			return nil, err
		}
		if captured.SpaceID == "" {
			captured.SpaceID = work.SpaceID.String()
		}
		if captured.SpaceID != work.SpaceID.String() {
			return nil, ErrConflict
		}
		if target.Kind == "native" {
			if o.NativeDrain == nil {
				return nil, ErrUnavailable
			}
			drain, ok := o.NativeDrain.(interface {
				ResumeRetiredCapturedDrain(context.Context, CapturedTarget, retirementproof.VerifiedRetirement, retirementproof.OwnerWork) error
			})
			if !ok {
				return nil, ErrUnavailable
			}
			err = drain.ResumeRetiredCapturedDrain(ctx, captured, verified, work)
		} else {
			if o.LegacyDrain == nil {
				return nil, errors.Join(retirementproof.ErrPending, ErrUnavailable)
			}
			err = o.LegacyDrain.ResumeCapturedLegacyDrain(ctx, captured)
		}
		if err != nil {
			return nil, errors.Join(retirementproof.ErrPending, err)
		}
		if err := o.markOwnerTargetClean(ctx, work, target, proof); err != nil {
			return nil, err
		}
	}
	for _, target := range targets {
		if target.Kind == "native" || target.Kind == "legacy" {
			continue
		}
		if err := o.cleanOwnerSQLTarget(ctx, work, target, proof); err != nil {
			return nil, errors.Join(retirementproof.ErrPending, err)
		}
	}
	// A qualified cold cut may restore the PG-derived cache while the trusted
	// whole-installation admission barrier remains closed. No public LIVE
	// transition or grant is created; old native members are already terminal.
	if work.Purpose == retirementproof.ColdCutDrain {
		if err := o.publishOwnerProjectionGate(ctx, work, true); err != nil {
			return nil, err
		}
	}
	// Permanent resource retirement is reobserved after local effects and
	// before the final owner transaction. No external IO under SQL locks.
	if err := o.Authority.CheckMaintenance(ctx, work.Cut); err != nil {
		return nil, err
	}
	verified, err = o.Verifier.VerifyForWork(ctx, proof, ref)
	if err != nil {
		return nil, err
	}
	return o.completeOwnerDrain(ctx, work, verified)
}

func (o *NativeRetirementOwner) ReadCutoverDrain(ctx context.Context, ref retirementproof.OwnerWorkRef) (*OwnerDrainReceipt, error) {
	if _, err := o.ReadOwnerWork(ctx, ref); err != nil {
		return nil, err
	}
	record, err := readOwnerRecord(ctx, o.Store.pool, ref.WorkID)
	if err != nil {
		return nil, err
	}
	if record.state != "COMPLETE" {
		return nil, retirementproof.ErrPending
	}
	if o.Verifier == nil {
		return nil, ErrUnavailable
	}
	var doc nativeOwnerReceiptDocument
	if json.Unmarshal(record.receipt, &doc) != nil {
		return nil, ErrConflict
	}
	verified, err := o.Verifier.VerifyForWork(ctx, doc.Proof, ref)
	if err != nil {
		return nil, err
	}
	if err := verified.ValidateFor(record.work); err != nil {
		return nil, err
	}
	return ownerSavedReceipt(record, doc.Proof)
}

func ownerSavedReceipt(record ownerRecord, proof retirementproof.ProofRef) (*OwnerDrainReceipt, error) {
	var doc nativeOwnerReceiptDocument
	if record.state != "COMPLETE" || json.Unmarshal(record.receipt, &doc) != nil {
		return nil, ErrConflict
	}
	canonical, err := json.Marshal(doc)
	hash := retirementproof.Hash(ownerReceiptDomain, record.receipt)
	workBytes, workErr := json.Marshal(record.work)
	if workErr != nil || doc.WorkHash != retirementproof.Hash(retirementproof.WorkDomain, workBytes) || uint64(len(doc.TerminalTargets)) != record.work.SealedTargetCount {
		return nil, ErrConflict
	}
	if (record.work.Purpose == retirementproof.P3FrozenCompletion) != (len(doc.PublicP3Receipt) != 0) {
		return nil, ErrConflict
	}
	if err != nil || !bytes.Equal(canonical, record.receipt) || !bytes.Equal(hash[:], record.receiptHash) || doc.Protocol != retirementproof.ProtocolV1 || doc.Kind != "NATIVE_SPACE_DRAINED" || doc.Work != record.work.Ref || doc.Proof != proof || doc.Cut != record.work.Cut || doc.TargetManifestHash != record.work.SealedTargetManifestHash {
		return nil, ErrConflict
	}
	return &OwnerDrainReceipt{Work: doc.Work, Proof: proof, CanonicalBytes: append([]byte(nil), record.receipt...), Hash: hash}, nil
}

func (o *NativeRetirementOwner) acceptOwnerProof(ctx context.Context, work retirementproof.OwnerWork, verified retirementproof.VerifiedRetirement) error {
	tx, err := o.Store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockOwnerWork(ctx, tx, work.SpaceID, work.Ref.WorkID); err != nil {
		return err
	}
	record, err := readOwnerRecord(ctx, tx, work.Ref.WorkID)
	if err != nil {
		return err
	}
	if record.work.Ref != work.Ref || verified.ValidateFor(record.work) != nil {
		return ErrConflict
	}
	head, _, err := readOwnerHead(ctx, tx, work.SpaceID)
	if err != nil || !sameOwnerHead(head, work.CapturedHead) {
		return ErrConflict
	}
	proof := verified.ProofRef()
	var oldID uuid.UUID
	var oldRef, oldWork, oldTargets, oldResolution, oldResolutionHash []byte
	err = tx.QueryRow(ctx, `SELECT proof_id,proof_ref_bytes,work_sha256,target_manifest_sha256,resolution_bytes,resolution_sha256 FROM voice_native_owner_acceptances WHERE work_id=$1`, work.Ref.WorkID).Scan(&oldID, &oldRef, &oldWork, &oldTargets, &oldResolution, &oldResolutionHash)
	refBytes, marshalErr := json.Marshal(proof)
	if marshalErr != nil {
		return marshalErr
	}
	workHash, targetHash, resolutionHash := verified.WorkHash(), verified.TargetManifestHash(), verified.DomainResolutionHash()
	resolution := verified.DomainResolutionBytes()
	if len(resolution) == 0 || retirementproof.Hash(retirementproof.ResolutionDomain, resolution) != resolutionHash {
		return ErrConflict
	}
	if err == nil {
		if oldID != proof.ProofID || !bytes.Equal(oldRef, refBytes) || !bytes.Equal(oldWork, workHash[:]) || !bytes.Equal(oldTargets, targetHash[:]) || !bytes.Equal(oldResolution, resolution) || !bytes.Equal(oldResolutionHash, resolutionHash[:]) {
			return ErrConflict
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO voice_native_owner_acceptances(work_id,proof_id,proof_ref_bytes,resource_receipt_sha256,work_sha256,target_manifest_sha256,resolution_bytes,resolution_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, work.Ref.WorkID, proof.ProofID, refBytes, proof.ReceiptHash[:], workHash[:], targetHash[:], resolution, resolutionHash[:])
		if err != nil {
			return err
		}
	} else {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE voice_native_owner_targets SET effect_kind='REALM_RETIRED',effect_proof_id=$2 WHERE work_id=$1 AND source_kind<>'event' AND effect_proof_id IS NULL`, work.Ref.WorkID, proof.ProofID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (o *NativeRetirementOwner) publishOwnerProjectionGate(ctx context.Context, work retirementproof.OwnerWork, live bool) error {
	gates, ok := o.ProjectionFence.(interface {
		ReinitializeSpaceLifecycle(context.Context, string, func(context.Context, string) (int64, bool, error)) error
	})
	if !ok {
		return ErrUnavailable
	}
	return gates.ReinitializeSpaceLifecycle(ctx, work.SpaceID.String(), func(ctx context.Context, _ string) (int64, bool, error) {
		tx, err := o.Store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		if err != nil {
			return 0, false, err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if err := spacebarrier.Lock(ctx, tx, work.SpaceID.String()); err != nil {
			return 0, false, err
		}
		head, current, err := readOwnerHead(ctx, tx, work.SpaceID)
		if err != nil || !sameOwnerHead(head, work.CapturedHead) {
			return 0, false, ErrConflict
		}
		generation := int64(current.Generation)
		if live {
			generation, err = spacebarrier.Live(ctx, tx, work.SpaceID.String())
			if err != nil {
				return 0, false, err
			}
		}
		return generation, live, tx.Commit(ctx)
	})
}

func (o *NativeRetirementOwner) markOwnerTargetClean(ctx context.Context, work retirementproof.OwnerWork, target nativeOwnerTarget, proof retirementproof.ProofRef) error {
	tag, err := o.Store.pool.Exec(ctx, `UPDATE voice_native_owner_targets SET cleanup_completed_at=COALESCE(cleanup_completed_at,clock_timestamp()) WHERE work_id=$1 AND target_id=$2 AND (effect_proof_id=$3 OR source_kind='event')`, work.Ref.WorkID, target.ID, proof.ProofID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func (o *NativeRetirementOwner) cleanOwnerSQLTarget(ctx context.Context, work retirementproof.OwnerWork, target nativeOwnerTarget, proof retirementproof.ProofRef) error {
	tx, err := o.Store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockOwnerWork(ctx, tx, work.SpaceID, work.Ref.WorkID); err != nil {
		return err
	}
	head, _, err := readOwnerHead(ctx, tx, work.SpaceID)
	if err != nil || !sameOwnerHead(head, work.CapturedHead) {
		return ErrConflict
	}
	if target.Kind == "head" {
		var h struct {
			VoiceRoomID    string `json:"voice_room_id"`
			RoomGeneration uint64 `json:"room_generation"`
			SpaceID        string `json:"space_id"`
			RoomID         string `json:"call_room_id"`
			Creator        string `json:"creator_operation_id"`
		}
		if json.Unmarshal(target.Data, &h) != nil || h.SpaceID != work.SpaceID.String() || h.RoomGeneration == 0 {
			return ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE voice_space_media_room_heads h SET state='ENDED',updated_at=clock_timestamp() WHERE h.voice_room_id=$1 AND h.room_generation=$2 AND h.space_id=$3 AND h.call_room_id=$4 AND h.creator_operation_id=$5 AND h.state='CLOSING' AND NOT EXISTS(SELECT 1 FROM voice_space_media_admissions a WHERE a.voice_room_id=h.voice_room_id AND a.room_generation=h.room_generation AND a.participant_state<>'ENDED')`, h.VoiceRoomID, int64(h.RoomGeneration), work.SpaceID, h.RoomID, h.Creator)
		if err != nil {
			return err
		}
	} else if target.Kind == "legacy_head" {
		var h struct {
			RoomID      string `json:"room_id"`
			SpaceID     string `json:"space_id"`
			VoiceRoomID string `json:"voice_room_id"`
			SFURoom     string `json:"livekit_room_name"`
		}
		if json.Unmarshal(target.Data, &h) != nil || h.SpaceID != work.SpaceID.String() {
			return ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE voice_room_instances r SET state='closed',closed_at=COALESCE(closed_at,clock_timestamp()),updated_at=clock_timestamp() WHERE room_id=$1 AND space_id=$2 AND voice_room_id=$3 AND livekit_room_name=$4 AND NOT EXISTS(SELECT 1 FROM voice_room_memberships m WHERE m.room_id=r.room_id)`, h.RoomID, work.SpaceID, h.VoiceRoomID, h.SFURoom)
		if err != nil {
			return err
		}
	}
	if _, err := ownerTargetTerminal(ctx, tx, work, target); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE voice_native_owner_targets SET cleanup_completed_at=COALESCE(cleanup_completed_at,clock_timestamp()) WHERE work_id=$1 AND target_id=$2 AND (effect_proof_id=$3 OR source_kind='event')`, work.Ref.WorkID, target.ID, proof.ProofID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return tx.Commit(ctx)
}

func (o *NativeRetirementOwner) completeOwnerDrain(ctx context.Context, work retirementproof.OwnerWork, verified retirementproof.VerifiedRetirement) (*OwnerDrainReceipt, error) {
	tx, err := o.Store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockOwnerWork(ctx, tx, work.SpaceID, work.Ref.WorkID); err != nil {
		return nil, err
	}
	record, err := readOwnerRecord(ctx, tx, work.Ref.WorkID)
	if err != nil {
		return nil, err
	}
	if record.work.Ref != work.Ref || verified.ValidateFor(record.work) != nil {
		return nil, ErrConflict
	}
	proof := verified.ProofRef()
	if record.state == "COMPLETE" {
		receipt, err := ownerSavedReceipt(record, proof)
		if err != nil {
			return nil, err
		}
		return receipt, tx.Commit(ctx)
	}
	head, _, err := readOwnerHead(ctx, tx, work.SpaceID)
	if err != nil || !sameOwnerHead(head, work.CapturedHead) {
		return nil, ErrConflict
	}
	var accepted bool
	workHash, targetHash, resolutionHash := verified.WorkHash(), verified.TargetManifestHash(), verified.DomainResolutionHash()
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM voice_native_owner_acceptances WHERE work_id=$1 AND proof_id=$2 AND resource_receipt_sha256=$3 AND work_sha256=$4 AND target_manifest_sha256=$5 AND resolution_sha256=$6)`, work.Ref.WorkID, proof.ProofID, proof.ReceiptHash[:], workHash[:], targetHash[:], resolutionHash[:]).Scan(&accepted); err != nil || !accepted {
		return nil, ErrConflict
	}
	targets, err := validateNativeOwnerManifest(work)
	if err != nil {
		return nil, err
	}
	terminal := make([]ownerTerminalTarget, 0, len(targets))
	for _, target := range targets {
		var complete bool
		var effect *string
		var proofID *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT cleanup_completed_at IS NOT NULL,effect_kind,effect_proof_id FROM voice_native_owner_targets WHERE work_id=$1 AND target_id=$2`, work.Ref.WorkID, target.ID).Scan(&complete, &effect, &proofID); err != nil || !complete {
			return nil, retirementproof.ErrPending
		}
		kind := "LOCAL_INTENT_RETAINED"
		boundProof := uuid.Nil
		if target.Kind != "event" {
			if effect == nil || *effect != "REALM_RETIRED" || proofID == nil || *proofID != proof.ProofID {
				return nil, ErrConflict
			}
			kind = *effect
			boundProof = *proofID
		}
		witness, err := ownerTargetTerminal(ctx, tx, work, target)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(target)
		if err != nil {
			return nil, err
		}
		terminal = append(terminal, ownerTerminalTarget{target.ID, target.Kind, retirementproof.Hash(ownerTargetEntryDomain, encoded), kind, boundProof, witness})
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return nil, err
	}
	var publicP3 []byte
	if work.Purpose == retirementproof.P3FrozenCompletion {
		publicP3, err = completeQualifiedOwnerP3(ctx, tx, work, proof, now)
		if err != nil {
			return nil, err
		}
	}
	doc := nativeOwnerReceiptDocument{retirementproof.ProtocolV1, "NATIVE_SPACE_DRAINED", work.Ref, proof, workHash, targetHash, resolutionHash, work.Cut, terminal, publicP3, now.UTC()}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	digest := retirementproof.Hash(ownerReceiptDomain, encoded)
	tag, err := tx.Exec(ctx, `UPDATE voice_native_owner_works SET state='COMPLETE',receipt_bytes=$2,receipt_sha256=$3,completed_at=$4 WHERE work_id=$1 AND state='PREPARED' AND work_sha256=$5`, work.Ref.WorkID, encoded, digest[:], now.UTC(), workHash[:])
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &OwnerDrainReceipt{work.Ref, proof, encoded, digest}, nil
}

// ownerTargetTerminal rechecks actual owner state in the final transaction;
// progress timestamps or a resource receipt alone cannot substitute for it.
func ownerTargetTerminal(ctx context.Context, tx pgx.Tx, work retirementproof.OwnerWork, target nativeOwnerTarget) ([]byte, error) {
	var terminal bool
	switch target.Kind {
	case "native":
		t, err := decodeTarget(target.Data)
		if err != nil {
			return nil, err
		}
		if t.SpaceID == "" {
			t.SpaceID = work.SpaceID.String()
		}
		err = tx.QueryRow(ctx, `SELECT participant_state='ENDED' AND (state='COMMITTED' OR (state='ABORTING' AND cleanup_completed_at IS NOT NULL)) AND NOT EXISTS(SELECT 1 FROM voice_account_voice_fences f WHERE f.admission_operation_id=a.operation_id AND f.admission_generation=a.generation) FROM voice_space_media_admissions a WHERE operation_id=$1 AND generation=$2 AND account_id=$3 AND profile_id=$4 AND space_id=$5 AND room_id=$6 AND voice_room_id=$7 AND room_generation=$8 AND livekit_identity=$9`, t.OperationID, t.Generation, t.AccountID, t.ProfileID, work.SpaceID, t.RoomID, t.VoiceRoomID, int64(t.RoomGeneration), t.Identity).Scan(&terminal)
		if err != nil {
			return nil, err
		}
	case "head":
		var h struct {
			VoiceRoomID    string `json:"voice_room_id"`
			RoomGeneration uint64 `json:"room_generation"`
			RoomID         string `json:"call_room_id"`
			Creator        string `json:"creator_operation_id"`
		}
		if json.Unmarshal(target.Data, &h) != nil {
			return nil, ErrConflict
		}
		if err := tx.QueryRow(ctx, `SELECT state='ENDED' AND NOT EXISTS(SELECT 1 FROM voice_space_media_admissions a WHERE a.voice_room_id=h.voice_room_id AND a.room_generation=h.room_generation AND a.participant_state<>'ENDED') FROM voice_space_media_room_heads h WHERE voice_room_id=$1 AND room_generation=$2 AND space_id=$3 AND call_room_id=$4 AND creator_operation_id=$5`, h.VoiceRoomID, int64(h.RoomGeneration), work.SpaceID, h.RoomID, h.Creator).Scan(&terminal); err != nil {
			return nil, err
		}
	case "legacy":
		t, err := decodeTarget(target.Data)
		if err != nil {
			return nil, err
		}
		if err := tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM voice_room_memberships WHERE room_id=$1 AND profile_id=$2 AND media_epoch=$3)`, t.RoomID, t.ProfileID, t.MediaEpoch).Scan(&terminal); err != nil {
			return nil, err
		}
	case "legacy_head":
		var h struct {
			RoomID      string `json:"room_id"`
			SFURoom     string `json:"livekit_room_name"`
			VoiceRoomID string `json:"voice_room_id"`
		}
		if json.Unmarshal(target.Data, &h) != nil {
			return nil, ErrConflict
		}
		if err := tx.QueryRow(ctx, `SELECT state='closed' AND NOT EXISTS(SELECT 1 FROM voice_room_memberships m WHERE m.room_id=r.room_id) FROM voice_room_instances r WHERE room_id=$1 AND space_id=$2 AND voice_room_id=$3 AND livekit_room_name=$4`, h.RoomID, work.SpaceID, h.VoiceRoomID, h.SFURoom).Scan(&terminal); err != nil {
			return nil, err
		}
	case "grant":
		var grant struct {
			GrantID string `json:"grant_id"`
		}
		if json.Unmarshal(target.Data, &grant) != nil {
			return nil, ErrConflict
		}
		if err := tx.QueryRow(ctx, `SELECT (to_jsonb(g)||jsonb_build_object('space_id',a.space_id,'account_id',a.account_id,'profile_id',a.profile_id,'room_id',a.room_id,'voice_room_id',a.voice_room_id,'room_generation',a.room_generation,'identity',a.livekit_identity))=$2::jsonb FROM voice_space_media_grants g JOIN voice_space_media_admissions a USING(operation_id) WHERE grant_id=$1 AND a.space_id=$3`, grant.GrantID, string(target.Data), work.SpaceID).Scan(&terminal); err != nil {
			return nil, err
		}
	case "event":
		var event struct {
			EventID     string `json:"event_id"`
			OperationID string `json:"operation_id"`
			Subject     string `json:"subject"`
			Payload     string `json:"payload"`
		}
		if json.Unmarshal(target.Data, &event) != nil {
			return nil, ErrConflict
		}
		if err := tx.QueryRow(ctx, `SELECT o.subject=$3 AND encode(o.payload,'hex')=substring($4 from 3) AND a.participant_state='ENDED' AND (a.state='COMMITTED' OR (a.state='ABORTING' AND a.cleanup_completed_at IS NOT NULL)) FROM voice_space_media_admission_outbox o JOIN voice_space_media_admissions a USING(operation_id) WHERE o.event_id=$1 AND o.operation_id=$2 AND a.space_id=$5`, event.EventID, event.OperationID, event.Subject, event.Payload, work.SpaceID).Scan(&terminal); err != nil {
			return nil, err
		}
	case "history":
		terminal = true
	default:
		return nil, ErrConflict
	}
	if !terminal {
		return nil, retirementproof.ErrPending
	}
	return json.Marshal(struct{ TargetID, OwnerState string }{target.ID, "TERMINAL"})
}

var _ retirementproof.OwnerWorkReader = (*NativeRetirementOwner)(nil)
