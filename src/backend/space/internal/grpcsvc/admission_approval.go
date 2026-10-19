package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func (s *admissionServer) ApproveJoinRequest(ctx context.Context, original *spacev1.ApproveJoinRequestRequest) (*spacev1.ApproveJoinRequestResponse, error) {
	if original == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	request := proto.Clone(original).(*spacev1.ApproveJoinRequestRequest)
	call, err := lifecycleprincipal.RequireAdmissionApproval(ctx, request)
	if err != nil {
		return nil, err
	}
	operation, err := store.PrepareJoinApprovalOperation(ctx, request)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if s == nil || s.store == nil {
		return nil, status.Error(codes.Unavailable, "approval storage unavailable")
	}
	saved, recovery, err := s.store.ReadJoinApprovalOperation(ctx, operation)
	var readErr error
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if saved != nil {
		return saved, nil
	}
	if recovery == nil {
		proposal, err := s.store.PrepareJoinApprovalDecision(ctx, request)
		if err != nil {
			// A concurrent exact original winner may have committed while this caller
			// waited. Read its immutable decision; never reselect another A or family.
			saved, recovery, readErr = s.store.ReadJoinApprovalOperation(ctx, operation)
			if readErr != nil {
				return nil, mapAdmissionError(readErr)
			}
			if saved != nil {
				return saved, nil
			}
			if recovery == nil {
				return nil, mapAdmissionError(err)
			}
		} else {
			if s.deps.Subjects == nil || s.deps.Reviewers == nil {
				return nil, status.Error(codes.Unavailable, "approval authority unavailable")
			}
			applicantRequest, reviewerRequest, err := proposal.OwnerRequests()
			if err != nil {
				return nil, mapAdmissionError(err)
			}
			applicant, err := s.deps.Subjects.Observe(ctx, applicantRequest)
			if err != nil {
				return nil, mapAdmissionError(err)
			}
			reviewer, err := s.deps.Subjects.Observe(ctx, reviewerRequest)
			if err != nil {
				return nil, mapAdmissionError(err)
			}
			permission, err := s.deps.Reviewers.Observe(ctx, reviewer, reviewerRequest)
			if err != nil {
				return nil, mapAdmissionError(err)
			}
			decision, err := proposal.Seal(applicant, permission)
			if err != nil {
				return nil, mapAdmissionError(err)
			}
			_, err = s.store.CommitJoinApprovalDecision(ctx, decision)
			if err != nil {
				saved, recovery, readErr = s.store.ReadJoinApprovalOperation(ctx, operation)
				if readErr != nil {
					return nil, mapAdmissionError(readErr)
				}
				if saved != nil {
					return saved, nil
				}
				if recovery == nil {
					return nil, mapAdmissionError(err)
				}
			} else {
				_, recovery, err = s.store.ReadJoinApprovalOperation(ctx, operation)
				if err != nil {
					return nil, mapAdmissionError(err)
				}
			}
		}
	}
	if recovery == nil {
		return nil, status.Error(codes.Unavailable, "approval decision unavailable")
	}
	if s.deps.Roles == nil || s.deps.Subjects == nil || s.deps.Reviewers == nil {
		return nil, status.Error(codes.Unavailable, "native approval authority unavailable")
	}
	intent, err := s.store.PrepareJoinDecisionAdmission(ctx, recovery.DecisionBasisSHA256)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if intent == nil {
		return nil, status.Error(codes.Unavailable, "approval publication unavailable")
	}
	if recovery.PublicationOperationID != nil && *recovery.PublicationOperationID != intent.Input.OperationID {
		return nil, status.Error(codes.Unavailable, "approval publication binding unavailable")
	}
	_, err = s.store.CompleteInteractiveMemberAdmissionWithPublicationCall(ctx, intent.Input.OperationID, s.deps.Roles, s.deps.Subjects, s.deps.Reviewers, nil, func(ctx context.Context, local *store.SpaceStore, native *store.MemberAdmissionIntent, _ *store.MembershipRow) error {
		return local.MarkJoinApprovalPublishedOutcome(ctx, operation, native)
	}, call, request)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	saved, _, err = s.store.ReadJoinApprovalOperation(ctx, operation)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if saved == nil {
		return nil, status.Error(codes.Unavailable, "approval outcome unavailable")
	}
	return saved, nil
}
