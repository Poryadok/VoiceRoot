package store

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
)

func (s *SpaceStore) AddBotMemberWithInstallation(ctx context.Context, proof *botmembershipprincipal.VerifiedInstallation, request *spacev1.AddBotMemberRequest) error {
	if request == nil {
		return ErrMemberAdmissionConflict
	}
	return s.mutateBotInstallation(ctx, proof, proto.Clone(request), false)
}
func (s *SpaceStore) RemoveBotMemberWithInstallation(ctx context.Context, proof *botmembershipprincipal.VerifiedInstallation, request *spacev1.RemoveBotMemberRequest) error {
	if request == nil {
		return ErrMemberAdmissionConflict
	}
	return s.mutateBotInstallation(ctx, proof, proto.Clone(request), true)
}

// No ambient account/profile metadata, caller boolean, default-role receipt or
// RPC belongs inside this transaction. Only the private listener capability can
// select the immutable producer intent; final checks follow all waiting SQL.
func (s *SpaceStore) mutateBotInstallation(ctx context.Context, proof *botmembershipprincipal.VerifiedInstallation, request proto.Message, remove bool) error {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil {
		return ErrMemberAuthorityUnavailable
	}
	in, err := proof.Check(request)
	if err != nil {
		return err
	}
	if in.Remove != remove {
		return ErrMemberAdmissionConflict
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return err
	}
	requestSHA := sha256.Sum256(raw)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer rollbackLifecycleTx(ctx, tx)
	// Operation identities are global, including cross-Space conflicts. Acquire
	// them before the Space fence so opposite cancel/add calls cannot deadlock.
	for _, operation := range normalizedOwnershipSpaceIDs([]uuid.UUID{in.OperationID, in.InstallOperationID}) {
		digest := sha256.Sum256(append([]byte("voice.space.bot-installation-operation.v1\x00"), operation[:]...))
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(binary.BigEndian.Uint64(digest[:8]))); err != nil {
			return err
		}
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return err
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || (version != 42 && version != 44 && version != 45 && version != 50 && version != 51) {
		return ErrMemberAuthorityUnavailable
	}
	scoped := *s
	scoped.tx = tx
	if err = scoped.effectiveMutationScope(ctx, in.SpaceID); err != nil {
		return err
	}
	if _, err = proof.Check(request); err != nil {
		return err
	}
	// Operation UUIDs never switch between installation and cancellation roles.
	var conflict bool
	if remove {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_bot_installation_attempts WHERE install_operation_id=$1) OR EXISTS(SELECT 1 FROM space_bot_installation_cancellations WHERE operation_id=$2)`, in.OperationID, in.InstallOperationID).Scan(&conflict)
	} else {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_bot_installation_cancellations WHERE operation_id=$1)`, in.OperationID).Scan(&conflict)
	}
	if err != nil {
		return err
	}
	if conflict {
		return ErrMemberAdmissionConflict
	}
	mutatesSource := !remove
	if remove {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members m JOIN space_bot_installation_attempts a ON a.space_id=m.space_id AND a.profile_id=m.profile_id AND a.membership_generation=m.membership_generation AND a.install_operation_id=m.admission_operation_id WHERE a.install_operation_id=$1 AND a.space_id=$2 AND a.profile_id=$3 AND a.state='PUBLISHED')`, in.InstallOperationID, in.SpaceID, in.ProfileID).Scan(&mutatesSource); err != nil {
			return err
		}
	}
	var capture *effectiveMemberMutations
	// Guarded44 journals and cancellation tombstones are native source state
	// even when no current membership is removed. Complete the full DB cohort
	// for those writes too; the domain primitive still rejects unknown sources.
	if mutatesSource || version == 44 || version == 45 || (version == 50 || version == 51) {
		capture, err = scoped.captureNativeEffectiveMemberMutation(ctx, in.SpaceID, []uuid.UUID{in.ProfileID})
		if err != nil {
			return err
		}
	}
	if remove {
		err = cancelNativeBotInstallation(ctx, tx, in, requestSHA[:])
	} else {
		err = publishNativeBotInstallation(ctx, tx, in, requestSHA[:])
	}
	if err != nil {
		return err
	}
	if mutatesSource {
		if err = scoped.refreshEffectiveMemberCount(ctx, in.SpaceID); err != nil {
			return err
		}
	}
	if capture != nil {
		if err = scoped.finishEffectiveMemberMutations(ctx, capture); err != nil {
			return err
		}
	}
	if _, err = proof.Check(request); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
