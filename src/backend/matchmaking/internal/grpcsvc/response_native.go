package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/responseprincipal"
	"voice/backend/matchmaking/internal/store"
)

type ResponseForwardLifetime interface {
	EnterForward(context.Context, uuid.UUID, [32]byte) (func(), error)
}

func (s *MatchmakingGRPC) respondToNativeMatch(ctx context.Context, req *matchmakingv1.RespondToMatchRequest) (*matchmakingv1.RespondToMatchResponse, error) {
	subject, ok := responseprincipal.SubjectFromContext(ctx, req)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "verified match response required")
	}
	if s.Matches == nil || s.Matches.NativeAdmission == nil || s.Sessions == nil {
		return nil, status.Error(codes.Unavailable, "match response unavailable")
	}
	matchID, err := uuid.Parse(req.MatchId)
	if err != nil || matchID == uuid.Nil || matchID.String() != req.MatchId {
		return nil, status.Error(codes.InvalidArgument, "invalid match response")
	}
	params := store.ResponseParams{MatchID: matchID, AccountID: subject.AccountID, ProfileID: subject.ProfileID, OperationID: subject.OperationID, SessionEpoch: subject.SessionEpoch, RequestHash: subject.RequestHash, Accept: req.Accept}
	if result, found, err := s.Matches.ReadResponseReceipt(ctx, params); err != nil {
		return nil, nativeResponseError(err)
	} else if found {
		return s.nativeResponseReply(ctx, params, result)
	}
	candidate, err := s.Matches.CaptureResponseCandidate(ctx, params)
	if err != nil {
		return nil, nativeResponseError(err)
	}
	var proof store.ResponseValidationProof
	if params.Accept {
		if s.ResponseValidator == nil || s.ResponseLifetime == nil {
			return nil, status.Error(codes.Unavailable, "match response validation unavailable")
		}
		validation, err := s.ResponseValidator.ValidateResponseCandidate(ctx, candidate)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "match response validation unavailable")
		}
		if len(validation.ChangedPartyIDs) > 0 {
			if s.resetResponseParties(ctx, candidate, validation.ChangedPartyIDs) != nil {
				return nil, status.Error(codes.Unavailable, "match response validation unavailable")
			}
			return nil, status.Error(codes.FailedPrecondition, "party source changed")
		}
		if validation.Attempt == nil || !validation.Attempt.Matches(candidate) {
			return nil, status.Error(codes.Unavailable, "match response validation unavailable")
		}
		proof = validation.Attempt
		release, err := s.ResponseLifetime.EnterForward(ctx, uuid.New(), candidate.Hash())
		if err != nil || release == nil {
			if release != nil {
				release()
			}
			return nil, status.Error(codes.Unavailable, "match response admission unavailable")
		}
		defer release()
	}
	result, err := s.Matches.RecordValidatedResponse(ctx, candidate, proof)
	if err != nil {
		var denied *store.PlatformBanDenial
		if errors.As(err, &denied) {
			if s.resetResponseDeniedAccounts(ctx, candidate, denied.Accounts()) != nil {
				return nil, status.Error(codes.Unavailable, "match response validation unavailable")
			}
			return nil, status.Error(codes.PermissionDenied, "match response not permitted")
		}
		return nil, nativeResponseError(err)
	}
	if params.Accept && result.AllAccepted && !result.Replayed && !result.Expired {
		provider, ok := s.Squad.(ActivatedSquadProvisioner)
		if !ok {
			return nil, status.Error(codes.Unavailable, "match provisioning unavailable")
		}
		activated, err := provider.ProvisionAndActivate(ctx, params.MatchID, result.Match.ProfileIDs())
		if err != nil || activated.ID != params.MatchID || activated.Status != store.MatchStatusActive {
			return nil, status.Error(codes.Unavailable, "match provisioning unavailable")
		}
		// A bare provider return value is not authority for public resource IDs.
		// The actual durable activation must be visible in the native store.
		actual, err := s.Matches.Get(ctx, params.MatchID)
		if err != nil || actual.Status != store.MatchStatusActive || actual.ChatID == nil || actual.VoiceRoomID == nil {
			return nil, status.Error(codes.Unavailable, "match provisioning unavailable")
		}
		result.Match = actual
	}
	return s.nativeResponseReply(ctx, params, result)
}

func nativeResponseError(err error) error {
	if errors.Is(err, store.ErrMatchOperationConflict) {
		return status.Error(codes.AlreadyExists, "match response operation conflict")
	}
	return status.Error(codes.Unavailable, "match response unavailable")
}

func (s *MatchmakingGRPC) nativeResponseReply(ctx context.Context, params store.ResponseParams, result store.AcceptanceResult) (*matchmakingv1.RespondToMatchResponse, error) {
	if params.Accept && (result.Expired || result.Proposal.Response != store.ProposalResponseAccepted) {
		return nil, status.Error(codes.FailedPrecondition, "match response is terminal")
	}
	if !params.Accept && result.Proposal.Response != store.ProposalResponseDeclined {
		return nil, status.Error(codes.FailedPrecondition, "match already responded")
	}
	session, err := s.Sessions.Get(ctx, result.Proposal.SearchSessionID)
	if err != nil || session.ProfileID != params.ProfileID {
		return nil, status.Error(codes.Unavailable, "match response unavailable")
	}
	match := result.Match
	if match.Status != store.MatchStatusActive {
		match.ChatID = nil
		match.VoiceRoomID = nil
	}
	return &matchmakingv1.RespondToMatchResponse{Match: toProtoMatch(match), SearchSession: toProtoSession(session)}, nil
}

func (s *MatchmakingGRPC) resetResponseParties(ctx context.Context, candidate store.ResponseCandidate, ids []uuid.UUID) error {
	allowed := map[uuid.UUID]bool{}
	for _, party := range candidate.Parties() {
		allowed[party.ID] = true
	}
	seen := map[uuid.UUID]bool{}
	if len(ids) == 0 {
		return store.ErrSourcePartyUnavailable
	}
	for _, id := range ids {
		if !allowed[id] || seen[id] {
			return store.ErrSourcePartyUnavailable
		}
		seen[id] = true
	}
	_, err := s.Matches.ResetVoiceParties(ctx, append([]uuid.UUID(nil), ids...))
	return err
}

func (s *MatchmakingGRPC) resetResponseDeniedAccounts(ctx context.Context, candidate store.ResponseCandidate, denied []uuid.UUID) error {
	allowed := map[uuid.UUID]bool{}
	for _, id := range candidate.Accounts() {
		allowed[id] = true
	}
	if len(denied) == 0 {
		return store.ErrSourcePartyUnavailable
	}
	deny := map[uuid.UUID]bool{}
	for _, id := range denied {
		if !allowed[id] {
			return store.ErrSourcePartyUnavailable
		}
		deny[id] = true
	}
	var ids []uuid.UUID
	for _, party := range candidate.Parties() {
		for _, member := range party.MemberAuthorities {
			if deny[member.AccountID] {
				ids = append(ids, party.ID)
				break
			}
		}
	}
	return s.resetResponseParties(ctx, candidate, ids)
}
