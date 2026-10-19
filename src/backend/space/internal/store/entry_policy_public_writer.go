package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

// settingsMembership captures only qualified current local source identities.
// Readiness is a real persisted guard/cutover check, not table/head presence.
func (s *SpaceStore) settingsMembership(ctx context.Context, in JoinOperationInput) ([]*rolev1.MembershipBinding, error) {
	if err := s.RequireScopedNativeMembershipReadiness(ctx, in.SpaceID); err != nil {
		return nil, err
	}
	bindings, err := s.ReadScopedMemberSourceBindings(ctx, in.SpaceID, in.ActorProfileID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJoinAttemptDenied
	}
	if err != nil {
		return nil, ErrEntryPolicyUnavailable
	}
	var banned bool
	if err = s.db().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_bans WHERE space_id=$1 AND account_id=$2)`, in.SpaceID, in.ActorAccountID).Scan(&banned); err != nil {
		return nil, err
	}
	if banned {
		return nil, ErrJoinAttemptDenied
	}
	return bindings, nil
}

// CommitEntryPolicySet consumes the actual constructor-private capabilities.
// All owner I/O happens before this actor-operation -> Space transaction.
func (s *SpaceStore) CommitEntryPolicySet(ctx context.Context, d *VerifiedEntryPolicySet) (*spacev1.SetEntryPolicyResponse, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || d == nil || d.proposal == nil {
		return nil, ErrEntryPolicyUnavailable
	}
	p := d.proposal
	if err := p.ingress(); err != nil {
		return nil, err
	}
	in, policy, err := entryPolicyPublicInput(p.request, p.operation.Input.ActorAccountID, p.operation.Input.ActorProfileID)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if err = lockJoinPublicOperation(ctx, tx, in); err != nil {
		return nil, err
	}
	local := *s
	local.tx = tx
	saved, ack, found, err := local.lookupEntryPolicyPublicOperation(ctx, in, true)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrEntryPolicyUnavailable
	}
	if saved.State == "COMPLETED" {
		// No Subject/Role/current membership/policy gate on an exact completed
		// acknowledgement, including a concurrent winner while we waited.
		if err = p.ingress(); err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return ack, nil
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	if err = d.check(); err != nil {
		return nil, err
	}
	before, err := local.settingsMembership(ctx, in)
	if err != nil {
		return nil, err
	}
	// A reserved public operation cannot adopt an independently written legacy
	// receipt. Only this transaction may write both legacy CAS and public ack.
	var legacy bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_entry_policy_operations WHERE actor_profile_id=$1 AND operation_id=$2)`, in.ActorProfileID, in.OperationID).Scan(&legacy); err != nil || legacy {
		return nil, ErrEntryPolicyUnavailable
	}
	outcome, err := local.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: in.SpaceID, ActorProfileID: in.ActorProfileID, OperationID: in.OperationID, ExpectedVersion: int64(p.request.ExpectedPolicyVersion), Policy: policy})
	if err != nil {
		return nil, err
	}
	ack = &spacev1.SetEntryPolicyResponse{OperationId: in.OperationID.String(), EntryPolicyVersion: uint64(outcome.Version)}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(ack)
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `UPDATE space_join_operations SET state='COMPLETED',outcome_code=0,outcome_proto=$3 WHERE actor_profile_id=$1 AND operation_id=$2 AND actor_account_id=$4 AND space_id=$5 AND request_method=$6 AND request_sha256=$7 AND state='RESERVED' AND selected_attempt_id IS NULL AND decision_basis IS NULL AND observed_basis_sha256 IS NULL AND publication_operation_id IS NULL`, in.ActorProfileID, in.OperationID, raw, in.ActorAccountID, in.SpaceID, in.Method, in.RequestSHA256)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, ErrEntryPolicyConflict
	}
	after, err := local.settingsMembership(ctx, in)
	if err != nil {
		return nil, err
	}
	if len(before) != len(after) {
		return nil, ErrJoinAttemptDenied
	}
	for index := range before {
		if !proto.Equal(before[index], after[index]) {
			return nil, ErrJoinAttemptDenied
		}
	}
	// These are pure checks after all SQL, including the receipt/ack writes and
	// final native source reads. Failure rolls back both CAS and public outcome.
	if err = d.check(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ack, nil
}
