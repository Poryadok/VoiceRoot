package ordinarycall

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"

	"github.com/jackc/pgx/v5"

	"voice/backend/voice/internal/retirementproof"
)

// AcceptCutoverProof retains qualified resource evidence for this exact work.
// Acceptance alone performs no cleanup and does not issue a terminal receipt.
func (o *CutoverDrainOwner) AcceptCutoverProof(ctx context.Context, work retirementproof.OwnerWorkRef, proof retirementproof.ProofRef) error {
	if o == nil || o.Authority == nil || o.Verifier == nil {
		return ErrUnavailable
	}
	sealed, err := o.ReadOwnerWork(ctx, work)
	if err != nil {
		return err
	}
	if err = o.Authority.CheckMaintenance(ctx, sealed.Cut); err != nil {
		return err
	}
	// Every invocation, including a lost-ACK replay, must observe the actual
	// protected resource scope again. A stored acceptance is not a live fence.
	verified, err := o.Verifier.VerifyForWork(ctx, proof, work)
	if err != nil {
		return err
	}
	if verified.ProofRef() != proof {
		return ErrConflict
	}
	if err = verified.ValidateFor(sealed); err != nil {
		return err
	}
	var request cutoverRequest
	if err = json.Unmarshal(sealed.OwnerRequestBytes, &request); err != nil {
		return ErrConflict
	}
	tx, err := o.Store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer rollback(tx)
	// The same native room -> private work order as Prepare. Do not replace
	// native generation/state checks with hashes or a generic proof boolean.
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT document FROM voice_ordinary_rooms WHERE room_id=$1 FOR UPDATE`, request.Expected.RoomID).Scan(&raw); err != nil {
		return mapStoreError(err)
	}
	native, err := decodeRoom(raw, request.Expected.RoomID)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,hashtext($2))`, int32(0x4f524443), work.WorkID.String()); err != nil {
		return err
	}
	current, err := readCutoverWork(ctx, tx, work.WorkID)
	if err != nil {
		return err
	}
	if current.Ref != work {
		return ErrConflict
	}
	if err = verified.ValidateFor(current); err != nil {
		return err
	}
	targets, err := validateCutoverManifest(current)
	if err != nil {
		return err
	}
	var closure *Room
	for _, target := range targets {
		if target.Kind == "closure" {
			closure, err = decodeRoom(target.Bytes, request.Expected.RoomID)
			if err != nil {
				return err
			}
		}
	}
	if closure == nil || !reflect.DeepEqual(native.Intent, closure.Intent) || native.Version < closure.Version || (native.Stage != StageClosing && native.Stage != StageAborting && native.Stage != StageClosed) {
		return ErrConflict
	}
	proofBytes, err := json.Marshal(proof)
	if err != nil {
		return err
	}
	resolution, resolutionHash := verified.DomainResolutionBytes(), verified.DomainResolutionHash()
	if len(resolution) == 0 || retirementproof.Hash(retirementproof.ResolutionDomain, resolution) != resolutionHash {
		return ErrConflict
	}
	workHash, targetHash := verified.WorkHash(), verified.TargetManifestHash()
	_, err = tx.Exec(ctx, `INSERT INTO voice_ordinary_cutover_acceptances(work_id,proof_id,proof_ref_bytes,resource_receipt_sha256,work_sha256,target_manifest_sha256,resolution_bytes,resolution_sha256)
VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(work_id,proof_id) DO NOTHING`, work.WorkID, proof.ProofID, proofBytes, proof.ReceiptHash[:], workHash[:], targetHash[:], resolution, resolutionHash[:])
	if err != nil {
		return err
	}
	var savedProof, savedReceiptHash, savedWorkHash, savedTargetHash, savedResolution, savedResolutionHash []byte
	if err = tx.QueryRow(ctx, `SELECT proof_ref_bytes,resource_receipt_sha256,work_sha256,target_manifest_sha256,resolution_bytes,resolution_sha256 FROM voice_ordinary_cutover_acceptances WHERE work_id=$1 AND proof_id=$2`, work.WorkID, proof.ProofID).Scan(&savedProof, &savedReceiptHash, &savedWorkHash, &savedTargetHash, &savedResolution, &savedResolutionHash); err != nil {
		return err
	}
	if !bytes.Equal(savedProof, proofBytes) || !bytes.Equal(savedReceiptHash, proof.ReceiptHash[:]) || !bytes.Equal(savedWorkHash, workHash[:]) || !bytes.Equal(savedTargetHash, targetHash[:]) || !bytes.Equal(savedResolution, resolution) || !bytes.Equal(savedResolutionHash, resolutionHash[:]) {
		return ErrConflict
	}
	return tx.Commit(ctx)
}
