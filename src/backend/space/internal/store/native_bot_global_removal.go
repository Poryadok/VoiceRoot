package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
)

func lockBotInstallationOperations(ctx context.Context, tx pgx.Tx, operations []uuid.UUID) error {
	for _, op := range normalizedOwnershipSpaceIDs(operations) {
		digest := sha256.Sum256(append([]byte("voice.space.bot-installation-operation.v1\x00"), op[:]...))
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(binary.BigEndian.Uint64(digest[:8]))); err != nil {
			return err
		}
	}
	return nil
}

func checkRetainedNativeBotActor(ctx context.Context, tx pgx.Tx, bot, profile uuid.UUID) error {
	var conflict bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_bot_installation_attempts WHERE bot_id=$1 AND profile_id<>$2)`, bot, profile).Scan(&conflict); err != nil {
		return ErrMemberAuthorityUnavailable
	}
	if conflict {
		return ErrMemberAdmissionConflict
	}
	return nil
}

func validateSavedGlobalBotReceipt(raw []byte, req *spacev1.ApplyBotInstallationRemovalRequest, requestSHA, evidenceSHA []byte) (*spacev1.BotInstallationRemovalReceipt, error) {
	r := &spacev1.BotInstallationRemovalReceipt{}
	if err := proto.Unmarshal(raw, r); err != nil || len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != 1 || r.OperationId != req.OperationId || r.DecisionId != req.DecisionId || r.Purpose != "global_delete" || r.BotId != req.BotId || r.ActorProfileId != req.ActorProfileId || r.SpaceId != req.SpaceId || r.EvidenceKind != "native" || !bytes.Equal(r.EvidenceSha256, evidenceSHA) || !bytes.Equal(r.FrozenScopeSha256, req.FrozenScopeSha256) || !bytes.Equal(r.RequestSha256, requestSHA) || r.Participant != "space" || r.Outcome != "COMPLETED" || !r.GlobalNativeAddsFenced || r.CompletedAt == nil || r.CompletedAt.CheckValid() != nil || len(r.CompletedAt.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrMemberAdmissionConflict
	}
	if _, err := canonicalLifecycleUUID(r.ReceiptId); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	canonical, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
	if err != nil || !bytes.Equal(canonical, raw) {
		return nil, ErrMemberAuthorityUnavailable
	}
	return r, nil
}

// ApplyBotInstallationRemoval settles only the signed exact native obligation.
// It never adopts legacy0, deletes another membership incarnation, borrows a
// human/default Role, or fabricates the old native-method capability.
func (s *SpaceStore) ApplyBotInstallationRemoval(ctx context.Context, proof *botmembershipprincipal.VerifiedGlobalRemoval, request *spacev1.ApplyBotInstallationRemovalRequest) (*spacev1.BotInstallationRemovalReceipt, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil || request == nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	r := proto.Clone(request).(*spacev1.ApplyBotInstallationRemovalRequest)
	binding, err := proof.Check(r)
	if err != nil {
		return nil, err
	}
	requestRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
	if err != nil || len(requestRaw) > 65536 {
		return nil, ErrMemberAdmissionConflict
	}
	nestedRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r.NativeCancellation)
	if err != nil {
		return nil, ErrMemberAdmissionConflict
	}
	requestSHA, evidenceSHA := sha256.Sum256(requestRaw), sha256.Sum256(nestedRaw)
	in := binding.Cancellation
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockBotInstallationOperations(ctx, tx, []uuid.UUID{binding.OperationID, in.OperationID, in.InstallOperationID}); err != nil {
		return nil, err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 46 {
		return nil, ErrMemberAuthorityUnavailable
	}
	var terminal bool
	if err = tx.QueryRow(ctx, `SELECT public.space_bot_native_terminal($1)`, in.BotID).Scan(&terminal); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if err = checkRetainedNativeBotActor(ctx, tx, in.BotID, in.ProfileID); err != nil {
		return nil, err
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	scoped := *s
	scoped.tx = tx
	if err = scoped.effectiveMutationScope(ctx, in.SpaceID); err != nil {
		return nil, err
	}
	// Completed replay skips capture, so the capture's serving LOGIN/COMPLETE
	// check cannot stand in for this explicit read/response boundary.
	if err=scoped.RequireScopedNativeMembershipReadiness(ctx,in.SpaceID);err!=nil{return nil,err}
	var saved []byte
	if err = tx.QueryRow(ctx, `SELECT public.space_bot_global_removal_saved($1,$2)`, binding.OperationID, in.SpaceID).Scan(&saved); err != nil {
		return nil, ErrMemberAuthorityUnavailable
	}
	if saved != nil {
		if !terminal {
			return nil, ErrMemberAuthorityUnavailable
		}
		out, err := validateSavedGlobalBotReceipt(saved, r, requestSHA[:], evidenceSHA[:])
		if err != nil {
			return nil, err
		}
		if _, err = proof.Check(r); err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, ErrMemberAuthorityUnavailable
		}
		return out, nil
	}
	var conflict bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_bot_installation_attempts WHERE install_operation_id=$1)
 OR EXISTS(SELECT 1 FROM public.space_bot_installation_cancellations WHERE operation_id=$1 AND operation_id<>$2)`, binding.OperationID, in.OperationID).Scan(&conflict); err != nil {
		return nil, err
	}
	if conflict {
		return nil, ErrMemberAdmissionConflict
	}
	var mutatesSource bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_members m JOIN public.space_bot_installation_attempts a ON a.space_id=m.space_id AND a.profile_id=m.profile_id AND a.membership_generation=m.membership_generation AND a.install_operation_id=m.admission_operation_id WHERE a.install_operation_id=$1 AND a.space_id=$2 AND a.profile_id=$3 AND a.state='PUBLISHED')`, in.InstallOperationID, in.SpaceID, in.ProfileID).Scan(&mutatesSource); err != nil {
		return nil, err
	}
	capture, err := scoped.captureNativeEffectiveMemberMutation(ctx, in.SpaceID, []uuid.UUID{in.ProfileID})
	if err != nil {
		return nil, err
	}
	if err = cancelNativeBotInstallation(ctx, tx, in, evidenceSHA[:]); err != nil {
		return nil, err
	}
	if mutatesSource {
		if err = scoped.refreshEffectiveMemberCount(ctx, in.SpaceID); err != nil {
			return nil, err
		}
	}
	var completed time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&completed); err != nil {
		return nil, err
	}
	out := &spacev1.BotInstallationRemovalReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: r.OperationId, DecisionId: r.DecisionId, Purpose: "global_delete", BotId: r.BotId, ActorProfileId: r.ActorProfileId, SpaceId: r.SpaceId, EvidenceKind: "native", EvidenceSha256: evidenceSHA[:], FrozenScopeSha256: append([]byte(nil), r.FrozenScopeSha256...), RequestSha256: requestSHA[:], Participant: "space", Outcome: "COMPLETED", CompletedAt: timestamppb.New(completed), GlobalNativeAddsFenced: true}
	resultRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(out)
	if err != nil {
		return nil, err
	}
	if err = tx.QueryRow(ctx, `SELECT public.space_bot_global_removal_record($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, binding.OperationID, binding.DecisionID, binding.DecisionSHA256[:], in.BotID, in.SpaceID, in.OperationID, in.InstallOperationID, evidenceSHA[:], requestRaw, binding.FrozenScopeSHA256[:], out.ReceiptId, resultRaw, completed).Scan(&saved); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrMemberAdmissionConflict
		}
		return nil, ErrMemberAuthorityUnavailable
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
		return nil, ErrMemberAuthorityUnavailable
	}
	return out, nil
}
