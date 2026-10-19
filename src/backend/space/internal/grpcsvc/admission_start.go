package grpcsvc

import (
	"context"
	"crypto/sha256"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func (s *admissionServer) StartJoinAttempt(ctx context.Context, original *spacev1.StartJoinAttemptRequest) (*spacev1.StartJoinAttemptResponse, error) {
	if original == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	request := proto.Clone(original).(*spacev1.StartJoinAttemptRequest)
	call, err := lifecycleprincipal.RequireAdmissionStart(ctx, request)
	if err != nil {
		return nil, err
	}
	actor, err := call.ActorFor(request)
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
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	digest := sha256.Sum256(raw)
	account, profile := uuid.MustParse(actor.AccountID), uuid.MustParse(actor.ProfileID)
	selection, exists, err := s.store.LookupJoinStartRequest(ctx, account, profile, operation, digest[:])
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if exists && selection.Operation.State == "COMPLETED" {
		response, err := store.ReplayJoinStartResponse(selection)
		if err != nil {
			return nil, mapAdmissionError(err)
		}
		if _, err = call.ActorFor(request); err != nil {
			return nil, err
		}
		return response, nil
	}
	if !exists {
		var space uuid.UUID
		var invite *uuid.UUID
		switch target := request.Target.(type) {
		case *spacev1.StartJoinAttemptRequest_SpaceId:
			space, err = admissionUUID(target.SpaceId)
			if err != nil {
				return nil, err
			}
		case *spacev1.StartJoinAttemptRequest_InviteCode:
			if target.InviteCode == "" {
				return nil, status.Error(codes.InvalidArgument, "invite code required")
			}
			inv, err := s.store.GetInviteByCode(ctx, target.InviteCode)
			if err != nil {
				return nil, mapAdmissionError(err)
			}
			if inv == nil {
				return nil, status.Error(codes.NotFound, "invite not found")
			}
			space = inv.SpaceID
			id := inv.ID
			invite = &id
		default:
			return nil, status.Error(codes.InvalidArgument, "join target required")
		}
		selection, err = s.store.ReserveJoinStartSelection(ctx, store.JoinOperationInput{SpaceID: space, ActorAccountID: account, ActorProfileID: profile, OperationID: operation, Method: store.JoinOperationStart, RequestSHA256: digest[:]}, invite)
		if err != nil {
			return nil, mapAdmissionError(err)
		}
		if selection.Operation.State == "COMPLETED" {
			return store.ReplayJoinStartResponse(selection)
		}
	}
	if s.deps.Subjects == nil {
		return nil, status.Error(codes.Unavailable, "admission owner transport unavailable")
	}
	proposal, err := s.store.PrepareJoinStartDecision(ctx, request, selection)
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
	response, err := s.store.CommitJoinStartDecision(ctx, decision)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	return response, nil
}
