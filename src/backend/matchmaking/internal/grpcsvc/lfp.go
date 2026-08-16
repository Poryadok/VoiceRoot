package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/authctx"
	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/runtimeconfig"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// lfpStoryCriteria is the Looking-for-party criteria_json shape from Story Service.
type lfpStoryCriteria struct {
	GameID string                 `json:"game_id"`
	Mode   string                 `json:"mode"`
	Region string                 `json:"region"`
	Self   criteria.SelfCriteria  `json:"self"`
	Sought criteria.SoughtCriteria `json:"sought"`
}

// DecideLfpRequest accepts or declines a JOIN/INVITE from an LFP story
// (docs/features/matchmaking.md Social Discovery; roadmap П.3).
func (s *MatchmakingGRPC) DecideLfpRequest(ctx context.Context, req *matchmakingv1.DecideLfpRequestRequest) (*matchmakingv1.DecideLfpRequestResponse, error) {
	authorID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Lfp == nil || s.Parties == nil || s.Sessions == nil || s.Games == nil {
		return nil, status.Error(codes.Unavailable, "lfp decision unavailable")
	}
	if s.Queue == nil {
		return nil, status.Error(codes.Unavailable, "queue unavailable")
	}
	if err := s.Queue.Ping(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "queue unavailable")
	}

	storyID, err := uuid.Parse(strings.TrimSpace(req.GetStoryId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid story_id")
	}
	responderID, err := uuid.Parse(strings.TrimSpace(req.GetResponderProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid responder_profile_id")
	}
	responseType := strings.ToUpper(strings.TrimSpace(req.GetResponseType()))
	if responseType != store.LfpResponseJoin && responseType != store.LfpResponseInvite {
		return nil, status.Error(codes.InvalidArgument, "response_type must be JOIN or INVITE")
	}
	decision := strings.ToUpper(strings.TrimSpace(req.GetDecision()))
	if decision != "ACCEPT" && decision != "DECLINE" {
		return nil, status.Error(codes.InvalidArgument, "decision must be ACCEPT or DECLINE")
	}

	pending, err := s.Lfp.GetPendingRequest(ctx, storyID, responderID, responseType)
	if errors.Is(err, store.ErrLfpRequestNotFound) {
		return nil, status.Error(codes.NotFound, "lfp request not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get lfp request: %v", err)
	}
	if pending.Status != store.LfpRequestPending {
		return nil, status.Error(codes.FailedPrecondition, "lfp request not pending")
	}
	if pending.AuthorProfileID != authorID {
		return nil, status.Error(codes.PermissionDenied, "only LFP author can decide")
	}

	listing, err := s.Lfp.GetListing(ctx, storyID)
	if errors.Is(err, store.ErrLfpListingNotFound) {
		return nil, status.Error(codes.FailedPrecondition, "lfp listing not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get lfp listing: %v", err)
	}
	if listing.InactiveAt != nil {
		return nil, status.Error(codes.FailedPrecondition, "lfp announcement no longer active")
	}

	if decision == "DECLINE" {
		decided, err := s.Lfp.DecideRequest(ctx, pending.ID, store.LfpRequestDeclined, nil)
		if errors.Is(err, store.ErrLfpRequestNotPending) {
			return nil, status.Error(codes.FailedPrecondition, "lfp request not pending")
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "decline lfp request: %v", err)
		}
		return &matchmakingv1.DecideLfpRequestResponse{Status: decided.Status}, nil
	}

	parsed, err := parseLfpStoryCriteria(listing.CriteriaJSON)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "invalid lfp criteria: %v", err)
	}
	gameID, err := uuid.Parse(strings.TrimSpace(parsed.GameID))
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "lfp criteria game_id must be a UUID")
	}
	modeName := strings.TrimSpace(parsed.Mode)
	if modeName == "" {
		return nil, status.Error(codes.FailedPrecondition, "lfp criteria mode required")
	}
	searchCrit := criteria.SearchCriteria{
		Region: strings.TrimSpace(parsed.Region),
		Self:   parsed.Self,
		Sought: parsed.Sought,
	}
	if searchCrit.Region == "" {
		searchCrit.Region = "eu"
	}

	game, err := s.Games.Get(ctx, gameID)
	if errors.Is(err, store.ErrGameNotFound) {
		return nil, status.Error(codes.NotFound, "game not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get game: %v", err)
	}
	if game.Status != store.StatusActive {
		return nil, status.Error(codes.FailedPrecondition, "game not active")
	}
	gameCfg, err := config.Parse(game.ConfigRaw)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid game config: %v", err)
	}

	var leader uuid.UUID
	var members []uuid.UUID
	switch responseType {
	case store.LfpResponseJoin:
		leader = authorID
		members = []uuid.UUID{authorID, responderID}
	case store.LfpResponseInvite:
		leader = responderID
		members = []uuid.UUID{responderID, authorID}
	}
	if _, err := criteria.Validate(searchCrit, gameCfg, modeName, len(members)); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "invalid criteria: %v", err)
	}
	canonical := criteria.MustMarshal(searchCrit)

	party, err := s.Parties.Create(ctx, store.CreatePartyParams{
		LeaderProfileID:  leader,
		MemberProfileIDs: members,
		GameID:           gameID,
		Mode:             modeName,
		Criteria:         canonical,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create party: %v", err)
	}

	sessions, err := s.enqueuePartySearch(ctx, party, members, gameID, modeName, searchCrit, canonical)
	if err != nil {
		return nil, err
	}

	decided, err := s.Lfp.DecideRequest(ctx, pending.ID, store.LfpRequestAccepted, &party.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "accept lfp request: %v", err)
	}
	_ = s.Lfp.MarkListingInactive(ctx, storyID)

	partyID := party.ID.String()
	out := &matchmakingv1.DecideLfpRequestResponse{
		Status: decided.Status,
		PartyId: &partyID,
	}
	for i := range sessions {
		out.SearchSessions = append(out.SearchSessions, toProtoSession(sessions[i]))
	}
	return out, nil
}

func parseLfpStoryCriteria(raw string) (lfpStoryCriteria, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return lfpStoryCriteria{}, errors.New("empty criteria")
	}
	var parsed lfpStoryCriteria
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return lfpStoryCriteria{}, err
	}
	return parsed, nil
}

func (s *MatchmakingGRPC) enqueuePartySearch(
	ctx context.Context,
	party store.Party,
	members []uuid.UUID,
	gameID uuid.UUID,
	modeName string,
	crit criteria.SearchCriteria,
	canonical string,
) ([]store.SearchSession, error) {
	timeoutAt := time.Now().UTC().Add(runtimeconfig.LoadSearchTiming().Timeout)
	out := make([]store.SearchSession, 0, len(members))
	for _, profileID := range members {
		if err := s.cancelActiveSearchIfAny(ctx, profileID); err != nil {
			return nil, err
		}
		partyID := party.ID
		sess, err := s.Sessions.Create(ctx, store.CreateSessionParams{
			ProfileID: profileID,
			PartyID:   &partyID,
			GameID:    gameID,
			Mode:      modeName,
			Criteria:  canonical,
			TimeoutAt: timeoutAt,
		})
		if errors.Is(err, store.ErrActiveSearchExists) {
			return nil, status.Error(codes.FailedPrecondition, "active search already exists")
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "create search session: %v", err)
		}
		if err := s.Queue.AcquireLock(ctx, profileID, sess.ID); err != nil {
			_, _ = s.Sessions.Cancel(ctx, sess.ID)
			if errors.Is(err, queue.ErrLockHeld) {
				return nil, status.Error(codes.FailedPrecondition, "active search already exists")
			}
			return nil, status.Error(codes.Unavailable, "queue unavailable")
		}
		if err := s.Queue.EnqueueScoped(ctx, nil, gameID, modeName, crit.Region, sess.ID, sess.CreatedAt); err != nil {
			_ = s.Queue.ReleaseLock(ctx, profileID, sess.ID)
			_, _ = s.Sessions.Cancel(ctx, sess.ID)
			return nil, status.Error(codes.Unavailable, "queue unavailable")
		}
		if s.Events != nil {
			_ = s.Events.PublishSearchStarted(ctx, sess.ID.String(), profileID.String(), gameID.String(), modeName, crit.Region)
		}
		out = append(out, sess)
	}
	return out, nil
}

func (s *MatchmakingGRPC) cancelActiveSearchIfAny(ctx context.Context, profileID uuid.UUID) error {
	sess, err := s.Sessions.GetActiveSearching(ctx, profileID)
	if errors.Is(err, store.ErrSessionNotFound) {
		return nil
	}
	if err != nil {
		return status.Errorf(codes.Internal, "check active search: %v", err)
	}
	if sess.Status == store.SessionStatusPendingAccept {
		return status.Error(codes.FailedPrecondition, "active match pending accept")
	}
	if sess.Status != store.SessionStatusSearching {
		return nil
	}
	parsed, err := criteria.Parse(sess.Criteria)
	if err == nil && s.Queue != nil {
		_ = s.Queue.Dequeue(ctx, sess.GameID, sess.Mode, parsed.Region, sess.ID)
		_ = s.Queue.ReleaseLock(ctx, profileID, sess.ID)
	}
	if _, err := s.Sessions.Cancel(ctx, sess.ID); err != nil && !errors.Is(err, store.ErrSessionNotSearchable) {
		return status.Errorf(codes.Internal, "cancel search: %v", err)
	}
	return nil
}
