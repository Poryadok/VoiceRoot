package grpcsvc

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func replaySubmitResponse(saved *store.JoinOperationReplay, space, profile uuid.UUID) (*spacev1.SubmitJoinAttemptResponse, error) {
	if saved == nil || saved.Operation.State != "COMPLETED" || saved.Code == nil {
		return nil, status.Error(codes.Unavailable, "submission outcome unavailable")
	}
	var response spacev1.SubmitJoinAttemptResponse
	if proto.Unmarshal(saved.Proto, &response) != nil || response.Result == nil {
		return nil, status.Error(codes.Unavailable, "submission outcome unavailable")
	}
	r := response.Result
	if *saved.Code == int32(codes.PermissionDenied) && r.State == spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_FAILED && r.RequestId == "" && r.SpaceMembership == nil {
		return nil, status.Error(codes.PermissionDenied, "admission denied")
	}
	if *saved.Code != 0 {
		return nil, status.Error(codes.Unavailable, "submission outcome unavailable")
	}
	switch r.State {
	case spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING:
		if _, err := admissionUUID(r.RequestId); err != nil || r.SpaceMembership != nil {
			return nil, status.Error(codes.Unavailable, "submission outcome unavailable")
		}
	case spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED:
		if r.SpaceMembership == nil || r.SpaceMembership.SpaceId != space.String() || r.SpaceMembership.ProfileId != profile.String() || r.SpaceMembership.JoinedAt == nil || !r.SpaceMembership.JoinedAt.IsValid() {
			return nil, status.Error(codes.Unavailable, "submission outcome unavailable")
		}
	default:
		return nil, status.Error(codes.Unavailable, "submission outcome unavailable")
	}
	return &response, nil
}

func (s *admissionServer) SubmitJoinAttempt(ctx context.Context, original *spacev1.SubmitJoinAttemptRequest) (*spacev1.SubmitJoinAttemptResponse, error) {
	if original == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	request := proto.Clone(original).(*spacev1.SubmitJoinAttemptRequest)
	call, err := lifecycleprincipal.RequireAdmissionSubmit(ctx, request)
	if err != nil {
		return nil, err
	}
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	space, err := admissionUUID(request.SpaceId)
	if err != nil {
		return nil, err
	}
	attempt, err := admissionUUID(request.AttemptId)
	if err != nil {
		return nil, err
	}
	operation, err := admissionUUID(request.OperationId)
	if err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return nil, status.Error(codes.Unavailable, "admission storage unavailable")
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid submission")
	}
	sha := sha256.Sum256(raw)
	profile := uuid.MustParse(actor.ProfileID)
	op := store.JoinOperationInput{SpaceID: space, ActorAccountID: uuid.MustParse(actor.AccountID), ActorProfileID: profile, OperationID: operation, Method: store.JoinOperationSubmit, RequestSHA256: sha[:]}
	saved, exists, err := s.store.LookupJoinOperationReplay(ctx, op)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if exists && saved.Operation.AttemptID != nil && *saved.Operation.AttemptID != attempt {
		return nil, status.Error(codes.AlreadyExists, "submission operation conflict")
	}
	if exists && saved.Operation.State == "COMPLETED" {
		response, replayErr := replaySubmitResponse(saved, space, profile)
		if _, err = call.ActorFor(request); err != nil {
			return nil, err
		}
		return response, replayErr
	}
	if exists && saved.Operation.State == "DECIDED" {
		return s.resumeJoinSubmission(ctx, request, call, op)
	}
	if s.deps.Subjects == nil {
		return nil, status.Error(codes.Unavailable, "admission owner transport unavailable")
	}
	proposal, err := s.store.PrepareJoinSubmissionDecision(ctx, request)
	if errors.Is(err, entrypolicy.ErrDenied) {
		rejected, err := s.store.PrepareJoinRejectedSubmission(ctx, request)
		if err != nil {
			return nil, mapAdmissionError(err)
		}
		expected, err := rejected.OwnerRequest()
		if err != nil {
			return nil, mapAdmissionError(err)
		}
		subject, err := s.deps.Subjects.Observe(ctx, expected)
		if err != nil {
			return nil, mapAdmissionError(err)
		}
		decision, err := rejected.Seal(subject)
		if err != nil {
			return nil, mapAdmissionError(err)
		}
		if err = s.store.CommitJoinRejectedSubmission(ctx, decision); err != nil {
			return nil, mapAdmissionError(err)
		}
		if _, err = call.ActorFor(request); err != nil {
			return nil, err
		}
		return nil, status.Error(codes.PermissionDenied, "admission denied")
	}
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	expected, err := proposal.OwnerRequest()
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	subject, err := s.deps.Subjects.Observe(ctx, expected)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	decision, err := proposal.Seal(subject)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if _, err = s.store.CommitJoinSubmissionDecision(ctx, decision); err != nil {
		return nil, mapAdmissionError(err)
	}
	return s.resumeJoinSubmission(ctx, request, call, op)
}

func (s *admissionServer) resumeJoinSubmission(ctx context.Context, request *spacev1.SubmitJoinAttemptRequest, call *lifecycleprincipal.VerifiedAdmissionCall, op store.JoinOperationInput) (*spacev1.SubmitJoinAttemptResponse, error) {
	history, err := s.store.RecoverJoinSubmissionDecision(ctx, request)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if history.DecisionKind == store.JoinDecisionSubmitPending {
		response, err := s.store.CompletePendingJoinSubmissionOutcome(ctx, request)
		if err != nil {
			return nil, mapAdmissionError(err)
		}
		if _, err = call.ActorFor(request); err != nil {
			return nil, err
		}
		return response, nil
	}
	if history.DecisionKind != store.JoinDecisionSubmitReady || s.deps.Roles == nil || s.deps.Subjects == nil {
		return nil, status.Error(codes.Unavailable, "native admission completion unavailable")
	}
	intent, err := s.store.PrepareJoinDecisionAdmission(ctx, history.DecisionBasisSHA256)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	_, err = s.store.CompleteInteractiveMemberAdmissionWithPublicationCall(ctx, intent.Input.OperationID, s.deps.Roles, s.deps.Subjects, s.deps.Reviewers, nil, func(ctx context.Context, local *store.SpaceStore, native *store.MemberAdmissionIntent, _ *store.MembershipRow) error {
		return local.MarkJoinSubmissionPublishedOutcome(ctx, request, native)
	}, call, request)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	saved, exists, err := s.store.LookupJoinOperationReplay(ctx, op)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if !exists {
		return nil, status.Error(codes.Unavailable, "submission outcome unavailable")
	}
	response, replayErr := replaySubmitResponse(saved, op.SpaceID, op.ActorProfileID)
	if _, err = call.ActorFor(request); err != nil {
		return nil, err
	}
	return response, replayErr
}
