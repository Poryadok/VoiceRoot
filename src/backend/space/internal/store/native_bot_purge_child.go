package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
)

func nativeP3ChildError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return ErrMemberAdmissionConflict
		}
		if pgErr.Code == "55000" {
			return ErrMemberAuthorityUnavailable
		}
	}
	return err
}

func validateSavedNativeP3Child(raw []byte, request *spacev1.ReconcileBotPurgeInstallationRequest, requestSHA []byte) (*spacev1.BotPurgeInstallationReceipt, error) {
	r := &spacev1.BotPurgeInstallationReceipt{}
	if err := proto.Unmarshal(raw, r); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != 1 || r.OperationId != request.OperationId || !bytes.Equal(r.RequestSha256, requestSHA) || r.SpaceId != request.SpaceId || r.DeletionOperationId != request.DeletionOperationId || r.Generation != request.Generation || !proto.Equal(r.Manifest, request.Manifest) || r.BotId != request.BotId || r.InstallationId != request.InstallationId || !proto.Equal(r.NativeCancellation, request.NativeCancellation) || r.CompletedAt == nil || r.CompletedAt.CheckValid() != nil || r.CompletedAt.Seconds <= 0 || len(r.CompletedAt.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrMemberAuthorityUnavailable
	}
	if _, err := canonicalLifecycleUUID(r.ReceiptId); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	switch r.Disposition {
	case spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED,
		spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_ALREADY_CANCELED,
		spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_NEGATIVE_INTENT_FENCED:
	default:
		return nil, ErrMemberAuthorityUnavailable
	}
	canonical, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
	if err != nil || !bytes.Equal(canonical, raw) {
		return nil, ErrMemberAuthorityUnavailable
	}
	return r, nil
}

// Fresh destructive authority comes only from this exact own irreversible P3
// decision and its accepted Role-first barrier. This private scope is distinct
// from general maintenance and cannot admit an ordinary non-LIVE mutation.
func (s *SpaceStore) nativeP3ChildMutationScope(ctx context.Context, space uuid.UUID, binding botmembershipprincipal.PurgeInstallationBinding) error {
	if space != binding.SpaceID {
		return ErrMemberAuthorityUnavailable
	}
	if err := s.effectiveMutationStorageScope(ctx, space); err != nil {
		return err
	}
	aggregate, err := loadLifecycle(ctx, s.tx, space, true)
	if err != nil {
		return ErrMemberAuthorityUnavailable
	}
	snapshot := aggregate.Snapshot()
	if snapshot.DeletionOperationID != binding.DeletionOperationID.String() || snapshot.Generation != binding.Generation || snapshot.Manifest == nil || snapshot.Manifest.ManifestId != binding.ManifestID.String() || !bytes.Equal(snapshot.Manifest.ManifestSha256, binding.ManifestSHA256[:]) || snapshot.Manifest.ItemCount != binding.ItemCount {
		return ErrLifecycleConflict
	}
	if snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING || snapshot.RoleReceipt == nil || snapshot.PurgeDecidedAt.IsZero() {
		return ErrMemberAuthorityUnavailable
	}
	operation, err := loadLifecycleOperation(ctx, s.tx, binding.DeletionOperationID)
	if err != nil || operation.validate() != nil || operation.method != "DELETE" || operation.spaceID != space || len(operation.authReceiptBytes) == 0 {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}

func (s *SpaceStore) captureNativeP3ChildMutation(ctx context.Context, binding botmembershipprincipal.PurgeInstallationBinding) (*effectiveMemberMutations, error) {
	if err := s.nativeP3ChildMutationScope(ctx, binding.SpaceID, binding); err != nil {
		return nil, err
	}
	capture, err := s.captureEffectiveMemberMutationFacts(ctx, binding.SpaceID, []uuid.UUID{binding.Cancellation.ProfileID})
	if err != nil {
		return nil, err
	}
	capture.purpose, capture.purgeBinding = effectiveMutationNativeP3Child, &binding
	return capture, nil
}

// ReconcileBotPurgeInstallation is native-only, receiver-owned P3 convergence.
// No global owner root, ordinary installation capability or missing-source
// success substitutes for the own decision and exact native effect.
func (s *SpaceStore) ReconcileBotPurgeInstallation(ctx context.Context, proof *botmembershipprincipal.VerifiedPurgeInstallation, request *spacev1.ReconcileBotPurgeInstallationRequest) (*spacev1.BotPurgeInstallationReceipt, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil || request == nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	r := proto.Clone(request).(*spacev1.ReconcileBotPurgeInstallationRequest)
	binding, err := proof.Check(r)
	if err != nil {
		return nil, err
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
	if err != nil || len(raw) > 65536 {
		return nil, ErrMemberAdmissionConflict
	}
	nativeRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r.NativeCancellation)
	if err != nil {
		return nil, ErrMemberAdmissionConflict
	}
	requestSHA, nativeSHA := sha256.Sum256(raw), sha256.Sum256(nativeRaw)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	in := binding.Cancellation
	if err = lockBotInstallationOperations(ctx, tx, []uuid.UUID{binding.OperationID, binding.DeletionOperationID, in.OperationID, in.InstallOperationID}); err != nil {
		return nil, err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 49 {
		return nil, ErrMemberAuthorityUnavailable
	}
	var terminal bool
	if err = tx.QueryRow(ctx, `SELECT public.space_bot_native_terminal($1)`, in.BotID).Scan(&terminal); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	// Negative P3 convergence remains valid if the independent global Bot fence
	// already exists. This query acquires its common Bot lock, never a grant.
	if err = lockLifecycleSpace(ctx, tx, binding.SpaceID); err != nil {
		return nil, err
	}
	scoped := *s
	scoped.tx = tx
	var saved []byte
	if err = tx.QueryRow(ctx, `SELECT public.space_bot_purge_child_saved($1,$2,$3)`, binding.OperationID, binding.SpaceID, raw).Scan(&saved); err != nil {
		return nil, nativeP3ChildError(err)
	}
	if saved != nil {
		result, err := validateSavedNativeP3Child(saved, r, requestSHA[:])
		if err != nil {
			return nil, err
		}
		if _, err = proof.Check(r); err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return result, nil
	}
	if err = scoped.nativeP3ChildMutationScope(ctx, binding.SpaceID, binding); err != nil {
		return nil, err
	}
	var state string
	err = tx.QueryRow(ctx, `SELECT state FROM public.space_bot_installation_attempts WHERE install_operation_id=$1`, in.InstallOperationID).Scan(&state)
	disposition := spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		disposition = spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_NEGATIVE_INTENT_FENCED
	case err != nil:
		return nil, err
	case state == "CANCELED":
		disposition = spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_ALREADY_CANCELED
	case state != "PUBLISHED":
		return nil, ErrMemberAuthorityUnavailable
	}
	capture, err := scoped.captureNativeP3ChildMutation(ctx, binding)
	if err != nil {
		return nil, err
	}
	if err = cancelNativeBotInstallation(ctx, tx, in, nativeSHA[:]); err != nil {
		return nil, err
	}
	if err = scoped.refreshEffectiveMemberCount(ctx, binding.SpaceID); err != nil {
		return nil, err
	}
	var completed time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&completed); err != nil {
		return nil, err
	}
	result := &spacev1.BotPurgeInstallationReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: r.OperationId, RequestSha256: requestSHA[:], SpaceId: r.SpaceId, DeletionOperationId: r.DeletionOperationId, Generation: r.Generation, Manifest: proto.Clone(r.Manifest).(*commonv1.ManifestBinding), BotId: r.BotId, InstallationId: r.InstallationId, NativeCancellation: proto.Clone(r.NativeCancellation).(*spacev1.RemoveBotMemberRequest), Disposition: disposition, CompletedAt: timestamppb.New(completed)}
	resultRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(result)
	if err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, `SELECT public.space_bot_purge_child_record($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, binding.OperationID, binding.SpaceID, binding.DeletionOperationID, int64(binding.Generation), binding.ManifestID, binding.ManifestSHA256[:], int64(binding.ItemCount), in.BotID, in.InstallationID, in.ProfileID, in.InstallOperationID, in.OperationID, int64(in.Generation), nativeSHA[:], raw, result.ReceiptId, resultRaw, completed).Scan(&saved); err != nil {
		return nil, nativeP3ChildError(err)
	}
	if !bytes.Equal(saved, resultRaw) {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = scoped.finishEffectiveMemberMutations(ctx, capture); err != nil {
		return nil, err
	}
	if err = scoped.flushMemberAdmissionDeferredChecks(ctx, capture); err != nil {
		return nil, err
	}
	if _, err = proof.Check(r); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}
