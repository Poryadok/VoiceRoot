package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/matchmaking/internal/authctx"
	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// SquadProvisioner creates voice+chat resources for an active match squad.
type SquadProvisioner interface {
	Provision(ctx context.Context, matchID uuid.UUID, profileIDs []uuid.UUID) (voiceRoomID, chatID string, err error)
}

func (s *MatchmakingGRPC) GetMatch(ctx context.Context, req *matchmakingv1.GetMatchRequest) (*matchmakingv1.GetMatchResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Matches == nil {
		return nil, status.Error(codes.Unavailable, "match unavailable")
	}
	matchID, err := uuid.Parse(strings.TrimSpace(req.GetMatchId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid match_id")
	}
	match, err := s.Matches.Get(ctx, matchID)
	if errors.Is(err, store.ErrMatchNotFound) {
		return nil, status.Error(codes.NotFound, "match not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get match: %v", err)
	}
	if !matchHasProfile(match, profileID) {
		return nil, status.Error(codes.PermissionDenied, "not a match participant")
	}
	return &matchmakingv1.GetMatchResponse{Match: toProtoMatch(match)}, nil
}

func (s *MatchmakingGRPC) RespondToMatch(ctx context.Context, req *matchmakingv1.RespondToMatchRequest) (*matchmakingv1.RespondToMatchResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Matches == nil || s.Sessions == nil {
		return nil, status.Error(codes.Unavailable, "match unavailable")
	}
	matchID, err := uuid.Parse(strings.TrimSpace(req.GetMatchId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid match_id")
	}
	match, err := s.Matches.Get(ctx, matchID)
	if errors.Is(err, store.ErrMatchNotFound) {
		return nil, status.Error(codes.NotFound, "match not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get match: %v", err)
	}
	if match.Status != store.MatchStatusPendingAccept {
		return nil, status.Error(codes.FailedPrecondition, "match not awaiting response")
	}

	proposal, err := s.Matches.GetProposalForProfile(ctx, matchID, profileID)
	if errors.Is(err, store.ErrProposalNotFound) {
		return nil, status.Error(codes.PermissionDenied, "not a match participant")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get proposal: %v", err)
	}

	if !req.GetAccept() {
		return s.handleMatchDecline(ctx, match, proposal)
	}

	if _, err := s.Matches.SetProposalResponse(ctx, matchID, profileID, store.ProposalResponseAccepted); err != nil {
		if errors.Is(err, store.ErrProposalNotFound) {
			return nil, status.Error(codes.FailedPrecondition, "already responded")
		}
		return nil, status.Errorf(codes.Internal, "accept match: %v", err)
	}

	allAccepted, err := s.Matches.AllProposalsAccepted(ctx, matchID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "check proposals: %v", err)
	}
	if !allAccepted {
		match, _ = s.Matches.Get(ctx, matchID)
		sess, _ := s.Sessions.Get(ctx, proposal.SearchSessionID)
		return &matchmakingv1.RespondToMatchResponse{
			Match:         toProtoMatch(match),
			SearchSession: toProtoSession(sess),
		}, nil
	}

	profileIDs := match.ProfileIDs()
	var voiceRoomID, chatID string
	if s.Squad != nil {
		voiceRoomID, chatID, err = s.Squad.Provision(ctx, matchID, profileIDs)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "squad provisioning unavailable")
		}
	}
	match, err = s.Matches.ActivateMatch(ctx, matchID, voiceRoomID, chatID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "activate match: %v", err)
	}
	sess, err := s.Sessions.Get(ctx, proposal.SearchSessionID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get session: %v", err)
	}
	return &matchmakingv1.RespondToMatchResponse{
		Match:         toProtoMatch(match),
		SearchSession: toProtoSession(sess),
	}, nil
}

func (s *MatchmakingGRPC) handleMatchDecline(ctx context.Context, match store.Match, proposal store.MatchProposal) (*matchmakingv1.RespondToMatchResponse, error) {
	if _, err := s.Matches.SetProposalResponse(ctx, match.ID, proposal.ProfileID, store.ProposalResponseDeclined); err != nil {
		return nil, status.Errorf(codes.Internal, "decline match: %v", err)
	}
	_ = s.Matches.AbandonMatch(ctx, match.ID)

	proposals, err := s.Matches.ListProposals(ctx, match.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list proposals: %v", err)
	}

	var ownSession store.SearchSession
	for _, p := range proposals {
		sess, err := s.Sessions.ResetToSearching(ctx, p.SearchSessionID)
		if err != nil {
			continue
		}
		if p.ProfileID == proposal.ProfileID {
			ownSession = sess
		}
		if s.Queue != nil {
			crit, err := criteria.Parse(sess.Criteria)
			if err == nil {
				_ = s.Queue.Enqueue(ctx, sess.GameID, sess.Mode, crit.Region, sess.ID, sess.CreatedAt)
			}
		}
	}
	if ownSession.ID == uuid.Nil {
		ownSession, _ = s.Sessions.Get(ctx, proposal.SearchSessionID)
	}
	match, _ = s.Matches.Get(ctx, match.ID)
	return &matchmakingv1.RespondToMatchResponse{
		Match:         toProtoMatch(match),
		SearchSession: toProtoSession(ownSession),
	}, nil
}

func matchHasProfile(match store.Match, profileID uuid.UUID) bool {
	for _, id := range match.ProfileIDs() {
		if id == profileID {
			return true
		}
	}
	return false
}

func toProtoMatch(m store.Match) *matchmakingv1.Match {
	out := &matchmakingv1.Match{
		Id:         m.ID.String(),
		GameId:     m.GameID.String(),
		Mode:       m.Mode,
		Region:     m.Region,
		Status:     m.Status,
		CreatedAt:  timestamppb.New(m.CreatedAt),
		ProfileIds: make([]string, 0, len(m.Participants)),
	}
	for _, p := range m.Participants {
		out.ProfileIds = append(out.ProfileIds, p.ProfileID)
	}
	if m.VoiceRoomID != nil {
		out.VoiceRoomId = m.VoiceRoomID
	}
	if m.ChatID != nil {
		out.ChatId = m.ChatID
	}
	return out
}
