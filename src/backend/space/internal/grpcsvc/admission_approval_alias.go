package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

// Aliases observe the original decision. Their operation never becomes another
// winner or an owner-free saved completion. Each result requires a fresh READ.
func (s *admissionServer) approveJoinAlias(ctx context.Context, operation *store.JoinApprovalOperation, call *lifecycleprincipal.VerifiedAdmissionCall, request *spacev1.ApproveJoinRequestRequest) (*spacev1.ApproveJoinRequestResponse, bool, error) {
	for pass := 0; pass < 2; pass++ {
		proposal, err := s.store.PrepareJoinApprovalAlias(ctx, operation)
		if err != nil {
			return nil, true, mapAdmissionError(err)
		}
		if proposal == nil {
			return nil, false, nil
		}
		if s.deps.Subjects == nil || s.deps.Reviewers == nil {
			return nil, true, status.Error(codes.Unavailable, "approval authority unavailable")
		}
		a, r, err := proposal.OwnerRequests()
		if err != nil {
			return nil, true, mapAdmissionError(err)
		}
		var applicant *admissionauthority.VerifiedSubject
		if a != nil {
			applicant, err = s.deps.Subjects.Observe(ctx, a)
			if err != nil {
				return nil, true, mapAdmissionError(err)
			}
		}
		reviewer, err := s.deps.Subjects.Observe(ctx, r)
		if err != nil {
			return nil, true, mapAdmissionError(err)
		}
		permission, err := s.deps.Reviewers.Observe(ctx, reviewer, r)
		if err != nil {
			return nil, true, mapAdmissionError(err)
		}
		decision, err := proposal.Seal(applicant, permission)
		if err != nil {
			return nil, true, mapAdmissionError(err)
		}
		response, recovery, err := s.store.ReadJoinApprovalAlias(ctx, decision)
		if err != nil {
			return nil, true, mapAdmissionError(err)
		}
		if response != nil {
			return response, true, nil
		}
		if pass != 0 || recovery == nil || s.deps.Roles == nil {
			return nil, true, status.Error(codes.Unavailable, "approval continuation unavailable")
		}
		intent, err := s.store.PrepareJoinDecisionAdmission(ctx, recovery.DecisionBasisSHA256)
		if err != nil {
			return nil, true, mapAdmissionError(err)
		}
		if intent == nil || (recovery.PublicationOperationID != nil && *recovery.PublicationOperationID != intent.Input.OperationID) {
			return nil, true, status.Error(codes.Unavailable, "approval publication unavailable")
		}
		var restrictionErr error
		_, err = s.store.CompleteInteractiveMemberAdmissionWithPublicationCall(ctx, intent.Input.OperationID, s.deps.Roles, s.deps.Subjects, s.deps.Reviewers, nil, func(ctx context.Context, local *store.SpaceStore, native *store.MemberAdmissionIntent, _ *store.MembershipRow) error {
			restrictionErr = local.MarkJoinApprovalAliasPublishedOutcome(ctx, decision, native)
			if restrictionErr != nil {
				// Alias ACL loss is not a denial of the original candidate. The
				// native coordinator sees unknown and preserves PREPARING/floors.
				return store.ErrJoinAttemptUnavailable
			}
			return nil
		}, call, request)
		if restrictionErr != nil {
			return nil, true, mapAdmissionError(restrictionErr)
		}
		if err != nil {
			return nil, true, mapAdmissionError(err)
		}
		// A new READ capability is acquired on the next iteration. APPROVE
		// proof never authorizes a historical result after publication.
	}
	return nil, true, status.Error(codes.Unavailable, "approval outcome unavailable")
}
