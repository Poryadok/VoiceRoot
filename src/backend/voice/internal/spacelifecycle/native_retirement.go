package spacelifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/spacebarrier"
)

const NativeOwnerProtocol = "voice.native-space.retirement.v1"

// MaintenanceAuthority is implemented by the protected cut issuer composition,
// using distinct scoped credentials. A serving role has no permissive fallback.
type MaintenanceAuthority interface {
	CheckMaintenance(context.Context, retirementproof.CutBinding) error
}

type ExpectedOwnerBinding struct {
	WorkID, SpaceID uuid.UUID
	Purpose         string
	Head            retirementproof.OwnerHeadBinding
	ExistingP3      *retirementproof.P3Binding
}

type NativeRetirementOwner struct {
	Store       *PostgresStore
	Authority   MaintenanceAuthority
	Verifier    retirementproof.Verifier
	NativeDrain interface {
		ResumeCapturedDrain(context.Context, CapturedTarget) error
	}
	// LegacyDrain is an exact local owner operation. It must not call a
	// replacement media realm; absent legacy ownership remains pending.
	LegacyDrain interface {
		ResumeCapturedLegacyDrain(context.Context, CapturedTarget) error
	}
	ProjectionFence interface {
		SetSpaceLifecycle(context.Context, string, int64, bool) error
	}
}

type OwnerDrainReceipt struct {
	Work           retirementproof.OwnerWorkRef
	Proof          retirementproof.ProofRef
	CanonicalBytes []byte
	Hash           retirementproof.Digest
}

func NativeOwnerContractHash() retirementproof.Digest {
	return retirementproof.Hash("voice.native-space.owner-contract.v1", []byte("voice017;immutable-exhaustive-targets;closed-maintenance;opaque-issuance-resolution;exact-owner-cleanup;real-p3-only;retained-provenance"))
}

func (o *NativeRetirementOwner) CheckSchema(ctx context.Context) error {
	if o == nil || o.Store == nil || o.Store.pool == nil {
		return ErrUnavailable
	}
	if err := o.Store.CheckSchema(ctx); err != nil {
		return err
	}
	var ready bool
	if err := o.Store.pool.QueryRow(ctx, `SELECT to_regclass('voice_native_owner_works') IS NOT NULL AND to_regclass('voice_native_owner_targets') IS NOT NULL AND to_regclass('voice_native_owner_acceptances') IS NOT NULL`).Scan(&ready); err != nil || !ready {
		return ErrUnavailable
	}
	return nil
}

func (o *NativeRetirementOwner) PrepareCutoverDrain(ctx context.Context, cut retirementproof.CutBinding, expected ExpectedOwnerBinding) (retirementproof.OwnerWorkRef, error) {
	var zero retirementproof.OwnerWorkRef
	if o == nil || o.Authority == nil {
		return zero, ErrUnavailable
	}
	if expected.WorkID == uuid.Nil || expected.SpaceID == uuid.Nil || !validOwnerCut(cut) || !validOwnerHead(expected.Head) {
		return zero, ErrInvalidRequest
	}
	if (expected.Purpose != retirementproof.ColdCutDrain && expected.Purpose != retirementproof.P3FrozenCompletion) || (expected.Purpose == retirementproof.ColdCutDrain && expected.ExistingP3 != nil) || (expected.Purpose == retirementproof.P3FrozenCompletion && expected.ExistingP3 == nil) {
		return zero, ErrInvalidRequest
	}
	// No external/resource observations while holding PostgreSQL locks.
	if err := o.Authority.CheckMaintenance(ctx, cut); err != nil {
		return zero, err
	}
	if err := o.CheckSchema(ctx); err != nil {
		return zero, err
	}
	tx, err := o.Store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockOwnerWork(ctx, tx, expected.SpaceID, expected.WorkID); err != nil {
		return zero, err
	}
	old, err := readOwnerRecord(ctx, tx, expected.WorkID)
	if err == nil {
		if old.work.Cut != cut || old.work.SpaceID != expected.SpaceID || old.work.Purpose != expected.Purpose || !sameOwnerHead(old.work.CapturedHead, expected.Head) || !sameJSON(old.work.ExistingP3, expected.ExistingP3) {
			return zero, ErrConflict
		}
		if old.state != "COMPLETE" {
			head, _, err := readOwnerHead(ctx, tx, expected.SpaceID)
			if err != nil || !sameOwnerHead(head, old.work.CapturedHead) {
				return zero, ErrConflict
			}
		}
		return old.work.Ref, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return zero, err
	}
	head, current, err := readOwnerHead(ctx, tx, expected.SpaceID)
	if err != nil {
		return zero, err
	}
	if !sameOwnerHead(head, expected.Head) {
		return zero, ErrConflict
	}
	work := retirementproof.OwnerWork{Ref: retirementproof.OwnerWorkRef{Protocol: retirementproof.ProtocolV1, OwnerProtocol: NativeOwnerProtocol, WorkID: expected.WorkID}, Purpose: expected.Purpose, SourceKind: "NATIVE_SPACE", Cut: cut, SpaceID: expected.SpaceID, CapturedHead: head, SourceManifestHash: NativeOwnerContractHash()}
	if expected.Purpose == retirementproof.ColdCutDrain {
		if head.State == retirementproof.HeadPresent && current.State != "LIVE" {
			return zero, ErrConflict
		}
		work.OwnerRequestDomain = retirementproof.OwnerRequestDomain
		work.OwnerRequestBytes, err = json.Marshal(struct {
			Protocol string
			Cut      retirementproof.CutBinding
			Expected ExpectedOwnerBinding
		}{NativeOwnerProtocol, cut, expected})
		if err != nil {
			return zero, err
		}
		work.Ref.RequestHash = retirementproof.Hash(work.OwnerRequestDomain, work.OwnerRequestBytes)
	} else {
		binding, err := readPendingOwnerP3(ctx, tx, expected.SpaceID, current)
		if err != nil {
			return zero, err
		}
		if !sameJSON(binding, expected.ExistingP3) {
			return zero, ErrConflict
		}
		work.ExistingP3 = binding
		work.OwnerRequestDomain, work.OwnerRequestBytes, work.Ref.RequestHash = binding.RequestDomain, binding.CanonicalRequestBytes, binding.RequestHash
	}
	targets, err := captureOwnerTargets(ctx, tx, work)
	if err != nil {
		return zero, err
	}
	work.SealedTargetManifestBytes, err = json.Marshal(targets)
	if err != nil {
		return zero, err
	}
	work.SealedTargetCount = uint64(len(targets))
	work.SealedTargetManifestHash = retirementproof.Hash(retirementproof.TargetDomain, work.SealedTargetManifestBytes)
	work.CapturedProviderBindings, work.UnknownIssuanceMarkers = ownerIssuanceBindings(targets)
	encoded, err := json.Marshal(work)
	if err != nil {
		return zero, err
	}
	digest := retirementproof.Hash(retirementproof.WorkDomain, encoded)
	if _, err := tx.Exec(ctx, `INSERT INTO voice_native_owner_works(work_id,space_id,owner_protocol,purpose,request_sha256,work_bytes,work_sha256) VALUES($1,$2,$3,$4,$5,$6,$7)`, work.Ref.WorkID, work.SpaceID, work.Ref.OwnerProtocol, work.Purpose, work.Ref.RequestHash[:], encoded, digest[:]); err != nil {
		return zero, err
	}
	for _, target := range targets {
		encoded, err := json.Marshal(target)
		if err != nil {
			return zero, err
		}
		digest := retirementproof.Hash(ownerTargetEntryDomain, encoded)
		if _, err := tx.Exec(ctx, `INSERT INTO voice_native_owner_targets(work_id,target_id,source_kind,target_bytes,target_sha256) VALUES($1,$2,$3,$4,$5)`, work.Ref.WorkID, target.ID, target.Kind, encoded, digest[:]); err != nil {
			return zero, err
		}
	}
	if err := o.closeCapturedOwnerRooms(ctx, tx, work.SpaceID); err != nil {
		return zero, err
	}
	return work.Ref, tx.Commit(ctx)
}

func lockOwnerWork(ctx context.Context, tx pgx.Tx, space, work uuid.UUID) error {
	if err := spacebarrier.Lock(ctx, tx, space.String()); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,hashtext($2))`, int32(0x4f574e52), work.String())
	return err
}

func validOwnerCut(c retirementproof.CutBinding) bool {
	return c.EnvironmentID != uuid.Nil && c.AuthorityIncarnation != uuid.Nil && c.CutoverOperationID != uuid.Nil && c.CoverageEpoch > 0 && c.CoverageEpoch <= uint64(^uint64(0)>>1) && c.PlanHash != (retirementproof.Digest{}) && c.DatabaseIdentity != (retirementproof.Digest{}) && c.SourceCatalogHash != (retirementproof.Digest{}) && c.SchemaContractHash != (retirementproof.Digest{}) && c.WriterManifestHash != (retirementproof.Digest{}) && c.ProviderManifestHash != (retirementproof.Digest{}) && c.CredentialManifestHash != (retirementproof.Digest{}) && c.ClosedAdmissionBarrierID != uuid.Nil && c.ClosedAdmissionBarrierHash != (retirementproof.Digest{}) && c.IssuerImplementationManifestHash != (retirementproof.Digest{})
}

func validOwnerHead(h retirementproof.OwnerHeadBinding) bool {
	if h.State == retirementproof.HeadAbsent {
		return len(h.CanonicalBytes) == 0 && h.Hash == retirementproof.Hash(retirementproof.HeadDomain, []byte(retirementproof.HeadAbsent))
	}
	return h.State == retirementproof.HeadPresent && len(h.CanonicalBytes) > 0 && h.Hash == retirementproof.Hash(retirementproof.HeadDomain, h.CanonicalBytes)
}

func sameOwnerHead(a, b retirementproof.OwnerHeadBinding) bool {
	return a.State == b.State && a.Hash == b.Hash && bytes.Equal(a.CanonicalBytes, b.CanonicalBytes)
}

func sameJSON(a, b any) bool {
	x, err := json.Marshal(a)
	if err != nil {
		return false
	}
	y, err := json.Marshal(b)
	return err == nil && bytes.Equal(x, y)
}

type ownerQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}
type ownerRecord struct {
	work                 retirementproof.OwnerWork
	state                string
	receipt, receiptHash []byte
}

func readOwnerRecord(ctx context.Context, q ownerQuery, id uuid.UUID) (ownerRecord, error) {
	var record ownerRecord
	var encoded, digest []byte
	if err := q.QueryRow(ctx, `SELECT work_bytes,work_sha256,state,receipt_bytes,receipt_sha256 FROM voice_native_owner_works WHERE work_id=$1`, id).Scan(&encoded, &digest, &record.state, &record.receipt, &record.receiptHash); err != nil {
		return record, err
	}
	h := retirementproof.Hash(retirementproof.WorkDomain, encoded)
	if !bytes.Equal(h[:], digest) || json.Unmarshal(encoded, &record.work) != nil || record.work.Ref.WorkID != id {
		return ownerRecord{}, fmt.Errorf("%w: native owner journal integrity", ErrConflict)
	}
	canonical, err := json.Marshal(record.work)
	if err != nil || !bytes.Equal(canonical, encoded) || record.work.Ref.OwnerProtocol != NativeOwnerProtocol || record.work.Ref.Protocol != retirementproof.ProtocolV1 || record.work.SourceKind != "NATIVE_SPACE" || record.work.SourceManifestHash != NativeOwnerContractHash() || retirementproof.Hash(record.work.OwnerRequestDomain, record.work.OwnerRequestBytes) != record.work.Ref.RequestHash {
		return ownerRecord{}, ErrConflict
	}
	targets, err := validateNativeOwnerManifest(record.work)
	if err != nil {
		return ownerRecord{}, err
	}
	if err := compareOwnerTargetRows(ctx, q, record.work.Ref.WorkID, targets); err != nil {
		return ownerRecord{}, err
	}
	return record, nil
}

func (o *NativeRetirementOwner) ReadOwnerWork(ctx context.Context, ref retirementproof.OwnerWorkRef) (retirementproof.OwnerWork, error) {
	if err := o.CheckSchema(ctx); err != nil {
		return retirementproof.OwnerWork{}, err
	}
	record, err := readOwnerRecord(ctx, o.Store.pool, ref.WorkID)
	if err != nil {
		return retirementproof.OwnerWork{}, err
	}
	if record.work.Ref != ref {
		return retirementproof.OwnerWork{}, ErrConflict
	}
	return record.work, nil
}
