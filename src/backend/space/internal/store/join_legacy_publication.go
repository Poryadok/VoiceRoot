package store

import (
	"bytes"
	"context"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
)

// MarkLegacyJoinPublishedOutcome is a scoped FIRST-publication POST hook. It
// saves only the original automatic legacy result. An original pending reply
// belongs to its own completed operation and is never replaced by approval.
// The native public coordinator must additionally enforce its final initiating
// call check after every SQL operation following this hook.
func (s *SpaceStore) MarkLegacyJoinPublishedOutcome(ctx context.Context, p *JoinLegacyStartProposal, native *MemberAdmissionIntent) error {
	if ctx == nil || s == nil || s.tx == nil || p == nil || p.call == nil || p.selection == nil || native == nil || native.State != "PUBLISHED" || native.Input.JoinAttempt == nil {
		return ErrJoinAttemptUnavailable
	}
	if _, err := p.call.ActorFor(p.request); err != nil {
		return err
	}
	op := p.selection.Context.Operation
	saved, err := s.loadLegacyJoinSelection(ctx, op, p.selection.Context.Origin)
	if err != nil {
		return err
	}
	actual, err := saved.Context.SHA256()
	expected, expectedErr := p.selection.Context.SHA256()
	if err != nil || expectedErr != nil || actual != expected || saved.Operation.State != "DECIDED" || saved.Operation.AttemptID == nil || *saved.Operation.AttemptID != native.Input.JoinAttempt.AttemptID || native.Input.SpaceID != op.SpaceID || native.Input.AccountID != op.ActorAccountID || native.Input.ProfileID != op.ActorProfileID {
		return ErrJoinAttemptUnavailable
	}
	history, err := s.loadLegacyJoinDecision(ctx, native.Input.DecisionBasisSHA256, true)
	if err != nil {
		return err
	}
	c := history.basis.Context
	if c.Kind != JoinLegacyReady || c.OperationOrigin != saved.Context.Origin || c.SpaceID != op.SpaceID || c.AttemptID != *saved.Operation.AttemptID || c.ActorAccountID != op.ActorAccountID || c.ActorProfileID != op.ActorProfileID || c.OperationID != op.OperationID || c.RequestMethod != op.Method || !bytes.Equal(c.RequestSHA256, op.RequestSHA256) || !bytes.Equal(c.ReadyEvidenceSHA256, native.Input.JoinAttempt.EvidenceSHA256) || history.publication == nil || *history.publication != native.Input.OperationID || history.publicationState == nil || *history.publicationState != "PUBLISHED" {
		return ErrJoinAttemptUnavailable
	}
	a, err := scanJoinAttempt(s.db().QueryRow(ctx, `SELECT `+joinAttemptColumns+` FROM space_join_attempts WHERE id=$1`, c.AttemptID))
	if err != nil || a.State != "PUBLISHED" || a.Input.SpaceID != op.SpaceID || a.Input.AccountID != op.ActorAccountID || a.Input.ProfileID != op.ActorProfileID || a.publication == nil || *a.publication != native.Input.OperationID || a.PolicyVersion != saved.Context.PolicyVersion || native.PolicyVersion != a.PolicyVersion || native.Generation <= 0 || !bytes.Equal(a.policyDigest, saved.Context.PolicySHA256) || !bytes.Equal(a.binding, c.AttemptBindingSHA256) || !bytes.Equal(a.submission, c.SubmissionSHA256) || !bytes.Equal(a.EvidenceSHA256, c.ReadyEvidenceSHA256) || a.Input.Guest != native.Input.Guest || !sameOptionalJoinAttempt(a.Input.InviteID, native.Input.InviteID) || !sameOptionalJoinAttempt(a.Input.InviteID, saved.Context.InviteID) {
		return ErrJoinAttemptUnavailable
	}
	var joined time.Time
	requestSHA := admissionRequestHash(native.Input)
	if err = s.db().QueryRow(ctx, `SELECT published_at FROM space_member_admission_intents WHERE operation_id=$1 AND state='PUBLISHED' AND request_sha256=$2 AND space_id=$3 AND profile_id=$4 AND account_id=$5 AND join_attempt_id=$6 AND join_attempt_evidence_sha256=$7 AND decision_basis_sha256=$8 AND generation=$9 AND policy_version=$10`, native.Input.OperationID, requestSHA[:], op.SpaceID, op.ActorProfileID, op.ActorAccountID, c.AttemptID, c.ReadyEvidenceSHA256, native.Input.DecisionBasisSHA256, native.Generation, native.PolicyVersion).Scan(&joined); err != nil {
		return ErrJoinAttemptUnavailable
	}
	response := &spacev1.SpaceMembership{SpaceId: op.SpaceID.String(), ProfileId: op.ActorProfileID.String(), JoinedAt: timestamppb.New(joined)}
	if !response.JoinedAt.IsValid() {
		return ErrJoinAttemptUnavailable
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return err
	}
	tag, err := s.db().Exec(ctx, `UPDATE space_join_operations SET state='COMPLETED',outcome_code=0,outcome_proto=$3 WHERE actor_profile_id=$1 AND operation_id=$2 AND state='DECIDED' AND basis_version=2 AND selected_attempt_id=$4 AND request_sha256=$5 AND decision_basis_sha256=$6 AND observed_basis_sha256 IS NULL AND publication_operation_id=$7 AND publication_state='PUBLISHED'`, op.ActorProfileID, op.OperationID, raw, c.AttemptID, op.RequestSHA256, native.Input.DecisionBasisSHA256, native.Input.OperationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrJoinAttemptConflict
	}
	_, err = p.call.ActorFor(p.request)
	return err
}
