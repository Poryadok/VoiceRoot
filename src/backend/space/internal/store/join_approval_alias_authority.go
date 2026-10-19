package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
)

type joinApprovalAliasSnapshot struct {
	attempt     uuid.UUID
	input       JoinAttemptInput
	basis       []byte
	publication *uuid.UUID
	published   bool
	original    JoinOperationInput
}

type JoinApprovalAliasProposal struct {
	operation           *JoinApprovalOperation
	snapshot            *joinApprovalAliasSnapshot
	applicant, reviewer *authv1.ObserveSpaceAdmissionSubjectRequest
}

type VerifiedJoinApprovalAlias struct {
	proposal   *JoinApprovalAliasProposal
	applicant  *admissionauthority.VerifiedSubject
	permission *admissionauthority.VerifiedReviewerPermission
}

// A nil proposal means the selected A still needs an original decision, or
// this exact operation is itself the original winner. No owner allow follows.
func (s *SpaceStore) PrepareJoinApprovalAlias(ctx context.Context, p *JoinApprovalOperation) (*JoinApprovalAliasProposal, error) {
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if err := p.ingress(); err != nil {
		return nil, err
	}
	if _, err := s.resolveJoinRequestApprovalObservation(ctx, p.input, p.requestID, p.explicit); err != nil {
		return nil, err
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{p.input.SpaceID}); err != nil {
		return nil, err
	}
	local := *s
	local.tx = tx
	snapshot, err := local.joinApprovalAliasSnapshot(ctx, p)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if err = p.ingress(); err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	binding, err := joinApprovalAliasContextSHA256(p.input, p.requestID, snapshot.attempt, snapshot.basis)
	if err != nil {
		return nil, err
	}
	actor, err := p.call.ActorFor(p.request)
	if err != nil || actor.SessionEpoch <= 0 {
		return nil, ErrJoinAttemptUnavailable
	}
	phase := authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_APPROVE
	if snapshot.published {
		phase = authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ
	}
	epoch := uint64(actor.SessionEpoch)
	proposal := &JoinApprovalAliasProposal{operation: p, snapshot: snapshot, reviewer: &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: phase, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER, AccountId: p.input.ActorAccountID.String(), ProfileId: p.input.ActorProfileID.String(), SpaceId: p.input.SpaceID.String(), AdmissionBindingSha256: slices.Clone(binding[:]), InteractiveSessionEpoch: &epoch}}
	if !snapshot.published {
		proposal.applicant = &authv1.ObserveSpaceAdmissionSubjectRequest{ProtocolVersion: 1, Phase: phase, SubjectRole: authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT, AccountId: snapshot.input.AccountID.String(), ProfileId: snapshot.input.ProfileID.String(), SpaceId: p.input.SpaceID.String(), AdmissionBindingSha256: slices.Clone(binding[:])}
	}
	return proposal, nil
}

// Only historical journal/selection/parent integrity is established here.
func (s *SpaceStore) joinApprovalAliasSnapshot(ctx context.Context, p *JoinApprovalOperation) (*joinApprovalAliasSnapshot, error) {
	if s == nil || s.tx == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	op, err := scanJoinOperation(s.db().QueryRow(ctx, `SELECT `+joinOperationColumns+` FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, p.input.ActorProfileID, p.input.OperationID))
	if err != nil || op.AttemptID == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if !sameJoinRequestOperation(op.Input, p.input) {
		return nil, ErrJoinAttemptConflict
	}
	if err = s.VerifyJoinRequestApprovalSelection(ctx, p.input, p.requestID, *op.AttemptID); err != nil {
		return nil, err
	}
	selection, err := s.loadJoinRequestSelection(ctx, p.input)
	if err != nil || !sameOptionalJoinAttempt(selection.explicitA, p.explicit) {
		return nil, ErrJoinAttemptConflict
	}
	a, err := scanJoinAttempt(s.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, *op.AttemptID))
	if err != nil || a.Input.SpaceID != p.input.SpaceID {
		return nil, ErrJoinAttemptUnavailable
	}
	if a.State == "PENDING" {
		return nil, nil
	}
	if (a.State != "READY" && a.State != "CLAIMED" && a.State != "PUBLISHED") || a.approvedAccount == nil || a.approvedProfile == nil || a.approvalOperation == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if *a.approvedAccount == p.input.ActorAccountID && *a.approvedProfile == p.input.ActorProfileID && *a.approvalOperation == p.input.OperationID {
		return nil, nil
	}
	var basis, observed []byte
	if err = s.db().QueryRow(ctx, `SELECT decision_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 AND actor_account_id=$3 AND space_id=$4 AND selected_attempt_id=$5 AND observed_basis_sha256 IS NULL`, *a.approvedProfile, *a.approvalOperation, *a.approvedAccount, a.Input.SpaceID, a.ID).Scan(&basis); err != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	native := &MemberAdmissionIntent{Input: MemberAdmissionInput{SpaceID: a.Input.SpaceID, AccountID: a.Input.AccountID, ProfileID: a.Input.ProfileID, JoinAttempt: &JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}, DecisionBasisSHA256: basis}}
	original, q, _, err := s.joinApprovalOriginalOperation(ctx, native)
	if err != nil || q != p.requestID {
		return nil, ErrJoinAttemptUnavailable
	}
	if a.publication != nil {
		intent, requestSHA, e := scanMemberAdmission(s.db().QueryRow(ctx, `SELECT `+memberAdmissionColumns+` FROM space_member_admission_intents WHERE operation_id=$1`, *a.publication))
		if e != nil || intent.Input.SpaceID != a.Input.SpaceID || intent.Input.ProfileID != a.Input.ProfileID || intent.Input.AccountID != a.Input.AccountID || intent.Input.Guest != a.Input.Guest || !sameOptionalJoinAttempt(intent.Input.InviteID, a.Input.InviteID) || intent.Input.JoinAttempt == nil || intent.Input.JoinAttempt.AttemptID != a.ID || !bytes.Equal(intent.Input.JoinAttempt.EvidenceSHA256, a.EvidenceSHA256) || !bytes.Equal(intent.Input.DecisionBasisSHA256, basis) || intent.PolicyVersion != a.PolicyVersion {
			return nil, ErrJoinAttemptUnavailable
		}
		expected := admissionRequestHash(intent.Input)
		if !bytes.Equal(expected[:], requestSHA) || (a.State == "CLAIMED" && intent.State != "PREPARING") || (a.State == "PUBLISHED" && intent.State != "PUBLISHED") {
			return nil, ErrJoinAttemptUnavailable
		}
	}
	var ownBasis, publicationState *string
	var ownPublication *uuid.UUID
	var outcome *int32
	var raw []byte
	if err = s.db().QueryRow(ctx, `SELECT decision_basis_sha256::text,observed_basis_sha256,publication_operation_id,publication_state,outcome_code,outcome_proto FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, p.input.ActorProfileID, p.input.OperationID).Scan(&ownBasis, &observed, &ownPublication, &publicationState, &outcome, &raw); err != nil {
		return nil, err
	}
	if ownBasis != nil || ownPublication != nil || publicationState != nil || outcome != nil || raw != nil || (op.State != "RESERVED" && op.State != "DECIDED") || (op.State == "RESERVED" && observed != nil) || (op.State == "DECIDED" && !bytes.Equal(observed, basis)) {
		return nil, ErrJoinAttemptUnavailable
	}
	return &joinApprovalAliasSnapshot{attempt: a.ID, input: a.Input, basis: slices.Clone(basis), publication: a.publication, published: a.State == "PUBLISHED", original: original}, nil
}

// Published reads deliberately have no applicant owner request.
func (p *JoinApprovalAliasProposal) OwnerRequests() (*authv1.ObserveSpaceAdmissionSubjectRequest, *authv1.ObserveSpaceAdmissionSubjectRequest, error) {
	if p == nil || p.operation == nil || p.snapshot == nil {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	if err := p.operation.ingress(); err != nil {
		return nil, nil, err
	}
	var applicant *authv1.ObserveSpaceAdmissionSubjectRequest
	if p.applicant != nil {
		applicant = proto.Clone(p.applicant).(*authv1.ObserveSpaceAdmissionSubjectRequest)
	}
	return applicant, proto.Clone(p.reviewer).(*authv1.ObserveSpaceAdmissionSubjectRequest), nil
}
func (p *JoinApprovalAliasProposal) Seal(applicant *admissionauthority.VerifiedSubject, permission *admissionauthority.VerifiedReviewerPermission) (*VerifiedJoinApprovalAlias, error) {
	d := &VerifiedJoinApprovalAlias{proposal: p, applicant: applicant, permission: permission}
	if err := d.check(); err != nil {
		return nil, err
	}
	return d, nil
}
func (d *VerifiedJoinApprovalAlias) check() error {
	if d == nil || d.proposal == nil || d.permission == nil {
		return ErrJoinAttemptUnavailable
	}
	a, r, err := d.proposal.OwnerRequests()
	if err != nil {
		return err
	}
	if _, err = d.permission.Transcript(r); err != nil {
		return err
	}
	if a != nil {
		if d.applicant == nil {
			return ErrJoinAttemptUnavailable
		}
		if _, err = d.applicant.Transcript(a); err != nil {
			return err
		}
	} else if d.applicant != nil {
		return ErrJoinAttemptUnavailable
	}
	return nil
}

// Alias records stay DECIDED/observational forever. Every read is freshly
// authorized; no alias can enter the original c7 owner-free completed branch.
func (s *SpaceStore) ReadJoinApprovalAlias(ctx context.Context, d *VerifiedJoinApprovalAlias) (*spacev1.ApproveJoinRequestResponse, *JoinSubmissionRecovery, error) {
	if s == nil || s.Pool == nil || s.tx != nil {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	if err := d.check(); err != nil {
		return nil, nil, err
	}
	p := d.proposal.operation
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, nil, err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	key := joinDigest("public-operation-lock", struct{ Actor, Operation uuid.UUID }{p.input.ActorProfileID, p.input.OperationID})
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, int32(binary.BigEndian.Uint32(key[:4])), int32(binary.BigEndian.Uint32(key[4:8]))); err != nil {
		return nil, nil, err
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{p.input.SpaceID}); err != nil {
		return nil, nil, err
	}
	local := *s
	local.tx = tx
	before, err := local.settingsMembership(ctx, p.input)
	if err != nil {
		return nil, nil, err
	}
	current, err := local.joinApprovalAliasSnapshot(ctx, p)
	if err != nil || current == nil || !bytes.Equal(current.basis, d.proposal.snapshot.basis) || current.attempt != d.proposal.snapshot.attempt || current.published != d.proposal.snapshot.published {
		return nil, nil, ErrJoinAttemptUnavailable
	}
	_, err = tx.Exec(ctx, `UPDATE space_join_operations SET state='DECIDED',observed_basis_sha256=$3 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='RESERVED' AND decision_basis IS NULL AND observed_basis_sha256 IS NULL AND publication_operation_id IS NULL`, p.input.ActorProfileID, p.input.OperationID, current.basis)
	if err != nil {
		return nil, nil, err
	}
	if err = local.assertJoinApprovalAliasObserved(ctx, p, current.basis); err != nil {
		return nil, nil, err
	}
	var response *spacev1.ApproveJoinRequestResponse
	var recovery *JoinSubmissionRecovery
	if current.published {
		replay, exists, e := local.LookupJoinOperationReplay(ctx, current.original)
		if e != nil || !exists || replay.Operation.State != "COMPLETED" || replay.Code == nil || *replay.Code != 0 {
			return nil, nil, ErrJoinAttemptUnavailable
		}
		response = &spacev1.ApproveJoinRequestResponse{}
		if proto.Unmarshal(replay.Proto, response) != nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Result == nil || len(response.Result.ProtoReflect().GetUnknown()) != 0 || response.Result.State != spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED || response.Result.RequestId != p.requestID.String() || response.Result.SpaceMembership == nil || len(response.Result.SpaceMembership.ProtoReflect().GetUnknown()) != 0 || response.Result.SpaceMembership.SpaceId != p.input.SpaceID.String() || response.Result.SpaceMembership.ProfileId != current.input.ProfileID.String() || response.Result.SpaceMembership.JoinedAt == nil || response.Result.SpaceMembership.JoinedAt.CheckValid() != nil {
			return nil, nil, ErrJoinAttemptUnavailable
		}
		if current.publication == nil {
			return nil, nil, ErrJoinAttemptUnavailable
		}
		var joined time.Time
		if err = tx.QueryRow(ctx, `SELECT published_at FROM space_member_admission_intents WHERE operation_id=$1 AND state='PUBLISHED'`, *current.publication).Scan(&joined); err != nil || !response.Result.SpaceMembership.JoinedAt.AsTime().Equal(joined) {
			return nil, nil, ErrJoinAttemptUnavailable
		}
		safe := &spacev1.ApproveJoinRequestResponse{Result: &spacev1.JoinAdmissionResult{State: spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED, RequestId: p.requestID.String(), SpaceMembership: &spacev1.SpaceMembership{SpaceId: p.input.SpaceID.String(), ProfileId: current.input.ProfileID.String(), JoinedAt: timestamppb.New(joined)}}}
		if !proto.Equal(response, safe) {
			return nil, nil, ErrJoinAttemptUnavailable
		}
		response = safe
	} else {
		recovery = &JoinSubmissionRecovery{AttemptID: current.attempt, DecisionBasisSHA256: slices.Clone(current.basis), PublicationOperationID: current.publication}
	}
	after, err := local.settingsMembership(ctx, p.input)
	if err != nil {
		return nil, nil, err
	}
	if len(before) != len(after) {
		return nil, nil, ErrJoinAttemptDenied
	}
	for i := range before {
		if !proto.Equal(before[i], after[i]) {
			return nil, nil, ErrJoinAttemptDenied
		}
	}
	if err = d.check(); err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	if err = d.check(); err != nil {
		return nil, nil, err
	}
	return response, recovery, nil
}

func (s *SpaceStore) assertJoinApprovalAliasObserved(ctx context.Context, p *JoinApprovalOperation, basis []byte) error {
	if s == nil || s.tx == nil {
		return ErrJoinAttemptUnavailable
	}
	var exact bool
	err := s.db().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 AND actor_account_id=$3 AND space_id=$4 AND request_method=$5 AND request_sha256=$6 AND state='DECIDED' AND observed_basis_sha256=$7 AND decision_basis IS NULL AND decision_basis_sha256 IS NULL AND publication_operation_id IS NULL AND publication_state IS NULL AND outcome_code IS NULL AND outcome_proto IS NULL)`, p.input.ActorProfileID, p.input.OperationID, p.input.ActorAccountID, p.input.SpaceID, p.input.Method, p.input.RequestSHA256, basis).Scan(&exact)
	if err != nil {
		return err
	}
	if !exact {
		return ErrJoinAttemptUnavailable
	}
	return nil
}

// FIRST-only additional restriction. The coordinator's public adapter must map
// any error here to a NONRETIRING rollback, retaining the typed public cause.
func (s *SpaceStore) MarkJoinApprovalAliasPublishedOutcome(ctx context.Context, d *VerifiedJoinApprovalAlias, native *MemberAdmissionIntent) error {
	if s == nil || s.tx == nil || native == nil || native.Input.JoinAttempt == nil || native.State != "PUBLISHED" {
		return ErrJoinAttemptUnavailable
	}
	if err := d.check(); err != nil {
		return fmt.Errorf("alias initial proof: %w", err)
	}
	p := d.proposal
	if p.snapshot.published || p.applicant == nil || !bytes.Equal(native.Input.DecisionBasisSHA256, p.snapshot.basis) || native.Input.JoinAttempt.AttemptID != p.snapshot.attempt {
		return ErrJoinAttemptUnavailable
	}
	if err := s.assertJoinApprovalAliasObserved(ctx, p.operation, p.snapshot.basis); err != nil {
		return fmt.Errorf("alias observed journal: %w", err)
	}
	before, err := s.settingsMembership(ctx, p.operation.input)
	if err != nil {
		return fmt.Errorf("alias initial membership: %w", err)
	}
	current, err := s.joinApprovalAliasSnapshot(ctx, p.operation)
	if err != nil {
		return fmt.Errorf("alias published snapshot: %w", err)
	}
	if err != nil || current == nil || !current.published || current.publication == nil || *current.publication != native.Input.OperationID || !bytes.Equal(current.basis, p.snapshot.basis) {
		return fmt.Errorf("alias publication tuple: %w", ErrJoinAttemptUnavailable)
	}
	op, q, _, err := s.joinApprovalOriginalOperation(ctx, native)
	if err != nil {
		return fmt.Errorf("alias original request: %w", err)
	}
	if q != p.operation.requestID || !sameJoinRequestOperation(op, current.original) {
		return ErrJoinAttemptUnavailable
	}
	if err = s.markJoinApprovalOriginalOutcome(ctx, op, q, native); err != nil {
		return fmt.Errorf("alias original acknowledgement: %w", err)
	}
	after, err := s.settingsMembership(ctx, p.operation.input)
	if err != nil {
		return fmt.Errorf("alias final membership: %w", err)
	}
	if len(before) != len(after) {
		return ErrJoinAttemptDenied
	}
	for i := range before {
		if !proto.Equal(before[i], after[i]) {
			return ErrJoinAttemptDenied
		}
	}
	if err = d.check(); err != nil {
		return fmt.Errorf("alias final proof: %w", err)
	}
	return nil
}
