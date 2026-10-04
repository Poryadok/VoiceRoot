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

// SquadCleanup releases temporary squad resources after a match's durable
// active-to-completed transition. It is intentionally an internal provider
// seam: A2 roster/session events and concrete Chat/Voice cleanup wiring remain
// outside this fixture-only A3 contract slice.
type SquadCleanup interface {
	Cleanup(ctx context.Context, matchID uuid.UUID) error
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
		// A final accept can be retried after the other participant's response
		// activated the match.  Preserve that caller's successful outcome when
		// the durable proposal proves it was this exact accepted response.
		if req.GetAccept() && match.Status == store.MatchStatusActive {
			proposal, proposalErr := s.Matches.GetProposalForProfile(ctx, matchID, profileID)
			if proposalErr == nil && proposal.Response == store.ProposalResponseAccepted {
				sess, sessionErr := s.Sessions.Get(ctx, proposal.SearchSessionID)
				if sessionErr == nil {
					return &matchmakingv1.RespondToMatchResponse{
						Match:         toProtoMatch(match),
						SearchSession: toProtoSession(sess),
					}, nil
				}
			}
		}
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
		decision, err := s.Matches.RecordDecline(ctx, matchID, profileID)
		if errors.Is(err, store.ErrProposalNotFound) {
			return nil, status.Error(codes.PermissionDenied, "not a match participant")
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "record match decline: %v", err)
		}
		if decision.Expired {
			if err := s.projectDeadlineRecovery(ctx, decision.ChangedSessions); err != nil {
				return nil, err
			}
		}
		sess, err := s.Sessions.Get(ctx, decision.Proposal.SearchSessionID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get declined session: %v", err)
		}
		return &matchmakingv1.RespondToMatchResponse{
			Match:         toProtoMatch(decision.Match),
			SearchSession: toProtoSession(sess),
		}, nil
	}

	decision, err := s.Matches.RecordAcceptance(ctx, matchID, profileID)
	if errors.Is(err, store.ErrProposalNotFound) {
		return nil, status.Error(codes.PermissionDenied, "not a match participant")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "record match acceptance: %v", err)
	}
	match, proposal = decision.Match, decision.Proposal
	if decision.Expired {
		if err := s.projectDeadlineRecovery(ctx, decision.ChangedSessions); err != nil {
			return nil, err
		}
		if decision.Replayed {
			sess, sessionErr := s.Sessions.Get(ctx, proposal.SearchSessionID)
			if sessionErr == nil {
				return &matchmakingv1.RespondToMatchResponse{Match: toProtoMatch(match), SearchSession: toProtoSession(sess)}, nil
			}
		}
		return nil, status.Error(codes.FailedPrecondition, "match acceptance deadline passed")
	}
	if proposal.Response != store.ProposalResponseAccepted {
		return nil, status.Error(codes.FailedPrecondition, "already responded")
	}
	if match.Status != store.MatchStatusPendingAccept {
		if decision.Replayed && (match.Status == store.MatchStatusActive || match.Status == store.MatchStatusAbandoned) {
			sess, sessionErr := s.Sessions.Get(ctx, proposal.SearchSessionID)
			if sessionErr == nil {
				return &matchmakingv1.RespondToMatchResponse{Match: toProtoMatch(match), SearchSession: toProtoSession(sess)}, nil
			}
		}
		return nil, status.Error(codes.FailedPrecondition, "match not awaiting response")
	}
	if !decision.AllAccepted {
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
			return nil, status.Errorf(codes.Unavailable, "squad provisioning unavailable: %v", err)
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

func (s *MatchmakingGRPC) projectDeadlineRecovery(ctx context.Context, sessions []store.SearchSession) error {
	if s.Queue == nil {
		return nil
	}
	for _, sess := range sessions {
		if sess.Status == store.SessionStatusSearching {
			crit, err := criteria.Parse(sess.Criteria)
			if err != nil {
				return status.Errorf(codes.Internal, "recover session criteria: %v", err)
			}
			if err := s.Queue.EnqueueScoped(ctx, sess.SpaceID, sess.GameID, sess.Mode, crit.Region, sess.ID, sess.CreatedAt); err != nil {
				return status.Errorf(codes.Unavailable, "recover search queue: %v", err)
			}
			continue
		}
		if err := s.Queue.ReleaseLock(ctx, sess.ProfileID, sess.ID); err != nil {
			return status.Errorf(codes.Unavailable, "release declined party search lock: %v", err)
		}
	}
	return nil
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
