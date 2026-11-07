package store

import (
	"bytes"
	"context"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

// Only BeginLifecycleFreeze calls this after loading the locked pending
// aggregate and its validated DELETE proof. The token is bound to that exact
// transition; it is not a general mutation grant for a frozen Space.
func (s *SpaceStore) captureLifecycleFreezeMutation(ctx context.Context, operation lifecycleOperation, generation uint64, manifest *commonv1.ManifestBinding) (*effectiveMemberMutations, error) {
	if s == nil || s.Pool == nil || s.tx == nil || ctx == nil || ctx.Err() != nil || generation == 0 || manifest == nil || operation.method != "DELETE" || operation.state != "SCHEDULE_PENDING" || len(operation.authReceiptBytes) == 0 || operation.validate() != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	var marker, native bool
	if err := s.tx.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL,
 to_regclass('public.space_ordinary_member_heads') IS NOT NULL OR to_regclass('public.space_effective_member_heads') IS NOT NULL OR to_regclass('native_membership_guard.epoch') IS NOT NULL`).Scan(&marker, &native); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if !marker {
		if native {
			return nil, ErrEffectiveEpisodeUnavailable
		}
		return nil, nil
	}
	var single, dirty bool
	var version int64
	if err := s.tx.QueryRow(ctx, `SELECT count(*)=1,COALESCE(max(version),0),COALESCE(bool_or(dirty),true) FROM public.schema_migrations`).Scan(&single, &version, &dirty); err != nil || !single || dirty {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if version <= 34 && !native {
		return nil, nil
	}
	if version == 39 || version == 40 {
		if err := checkMemberAuthorityCatalog(ctx, s.tx); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if version != 41 && version != 42 && version != 44 && version != 45 && version != 50 && version != 51 {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	bound := &effectiveMemberMutations{space: operation.spaceID, scheduleOperation: operation.operationID}
	if err := s.lifecycleScheduleMutationScope(ctx, bound); err != nil {
		return nil, err
	}
	lifecycle, err := loadLifecycle(ctx, s.tx, operation.spaceID, true)
	if err != nil || lifecycle.Generation() != generation {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	capture, err := s.captureEffectiveMemberMutationFacts(ctx, operation.spaceID, []uuid.UUID{operation.actorProfileID})
	if err != nil {
		return nil, err
	}
	capture.purpose = effectiveMutationLifecycleFreeze
	capture.scheduleOperation = operation.operationID
	capture.freezeGeneration = generation
	capture.freezeManifest = proto.Clone(manifest).(*commonv1.ManifestBinding)
	capture.freezeBinding = bytes.Clone(operation.bindingHash)
	capture.freezeReceipt = bytes.Clone(operation.authReceiptHash)
	return capture, nil
}

// Finish is valid only after the same transaction has saved the exact freeze
// transition. Re-reading the binding does not substitute a caller-made grant.
func (s *SpaceStore) lifecycleFreezeMutationScope(ctx context.Context, capture *effectiveMemberMutations) error {
	if capture == nil || capture.scheduleOperation == uuid.Nil || capture.freezeGeneration == 0 || capture.freezeManifest == nil {
		return ErrEffectiveEpisodeUnavailable
	}
	if err := s.effectiveMutationStorageScope(ctx, capture.space); err != nil {
		return err
	}
	lifecycle, err := loadLifecycle(ctx, s.tx, capture.space, true)
	if err != nil || lifecycle.Phase() != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING || lifecycle.Generation() != capture.freezeGeneration || lifecycle.Snapshot().DeletionOperationID != capture.scheduleOperation.String() || !proto.Equal(lifecycle.Snapshot().Manifest, capture.freezeManifest) {
		return ErrEffectiveEpisodeUnavailable
	}
	operation, err := loadLifecycleOperation(ctx, s.tx, capture.scheduleOperation)
	if err != nil || operation.validate() != nil || operation.method != "DELETE" || operation.state != "SCHEDULE_PENDING" || operation.spaceID != capture.space || len(operation.authReceiptBytes) == 0 || !bytes.Equal(operation.bindingHash, capture.freezeBinding) || !bytes.Equal(operation.authReceiptHash, capture.freezeReceipt) {
		return ErrEffectiveEpisodeUnavailable
	}
	ownership, err := readMemberOwnership(ctx, s.tx, capture.space)
	if err != nil || ownership.GetState() != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE {
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}
