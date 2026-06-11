package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/authctx"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// CompleteMatch records squad leave for the authenticated participant.
func (s *MatchmakingGRPC) CompleteMatch(ctx context.Context, req *matchmakingv1.CompleteMatchRequest) (*matchmakingv1.CompleteMatchResponse, error) {
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
	before, err := s.Matches.Get(ctx, matchID)
	if errors.Is(err, store.ErrMatchNotFound) {
		return nil, status.Error(codes.NotFound, "match not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get match: %v", err)
	}
	if !matchHasProfile(before, profileID) {
		return nil, status.Error(codes.PermissionDenied, "not a match participant")
	}

	updated, err := s.Matches.CompleteMatchLeave(ctx, matchID, profileID)
	if errors.Is(err, store.ErrNotMatchParticipant) {
		return nil, status.Error(codes.PermissionDenied, "not a match participant")
	}
	if errors.Is(err, store.ErrMatchNotFound) {
		return nil, status.Error(codes.NotFound, "match not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "complete match: %v", err)
	}

	if updated.Status == store.MatchStatusCompleted && before.Status != store.MatchStatusCompleted && s.Events != nil {
		duration := int64(0)
		if updated.CompletedAt != nil {
			duration = int64(updated.CompletedAt.Sub(updated.CreatedAt).Seconds())
		}
		profileIDs := make([]string, 0, len(updated.Participants))
		for _, p := range updated.Participants {
			profileIDs = append(profileIDs, p.ProfileID)
		}
		_ = s.Events.PublishMatchCompleted(ctx, mmevents.MatchCompletedEvent{
			MatchID:         updated.ID.String(),
			DurationSeconds: duration,
			ProfileIDs:      profileIDs,
		})
	}

	return &matchmakingv1.CompleteMatchResponse{Match: toProtoMatch(updated)}, nil
}

// RateMatch submits a peer rating for a completed match.
func (s *MatchmakingGRPC) RateMatch(ctx context.Context, req *matchmakingv1.RateMatchRequest) (*matchmakingv1.RateMatchResponse, error) {
	raterID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Matches == nil || s.Ratings == nil {
		return nil, status.Error(codes.Unavailable, "rating unavailable")
	}
	matchID, err := uuid.Parse(strings.TrimSpace(req.GetMatchId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid match_id")
	}
	ratedID, err := uuid.Parse(strings.TrimSpace(req.GetRatedProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid rated_profile_id")
	}
	if raterID == ratedID {
		return nil, status.Error(codes.InvalidArgument, "cannot rate self")
	}
	stars := int(req.GetStars())

	match, err := s.Matches.Get(ctx, matchID)
	if errors.Is(err, store.ErrMatchNotFound) {
		return nil, status.Error(codes.NotFound, "match not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get match: %v", err)
	}
	if !matchHasProfile(match, raterID) {
		return nil, status.Error(codes.PermissionDenied, "not a match participant")
	}
	if !matchHasProfile(match, ratedID) {
		return nil, status.Error(codes.InvalidArgument, "rated user not in match")
	}
	if match.Status != store.MatchStatusCompleted {
		return nil, status.Error(codes.FailedPrecondition, "match not completed")
	}

	err = s.Ratings.InsertMatchRating(ctx, store.InsertMatchRatingParams{
		MatchID:        matchID,
		RaterProfileID: raterID,
		RatedProfileID: ratedID,
		Stars:          stars,
	})
	if errors.Is(err, store.ErrDuplicateMatchRating) {
		return nil, status.Error(codes.AlreadyExists, "already rated")
	}
	if errors.Is(err, store.ErrInvalidRatingStars) {
		return nil, status.Error(codes.InvalidArgument, "invalid stars")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "insert rating: %v", err)
	}

	if _, err := s.Ratings.UpsertPlayerRating(ctx, ratedID, match.GameID, stars); err != nil {
		return nil, status.Errorf(codes.Internal, "update aggregate: %v", err)
	}

	if s.Events != nil {
		_ = s.Events.PublishRatingSubmitted(ctx, mmevents.RatingSubmittedEvent{
			MatchID:         matchID.String(),
			RaterProfileID:  raterID.String(),
			RatedProfileID:  ratedID.String(),
			Stars:           int32(stars),
		})
	}

	return &matchmakingv1.RateMatchResponse{}, nil
}

// GetPlayerRating returns aggregate MM rating for a profile and game.
func (s *MatchmakingGRPC) GetPlayerRating(ctx context.Context, req *matchmakingv1.GetPlayerRatingRequest) (*matchmakingv1.GetPlayerRatingResponse, error) {
	if s.Ratings == nil {
		return nil, status.Error(codes.Unavailable, "rating unavailable")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	gameID, err := uuid.Parse(strings.TrimSpace(req.GetGameId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid game_id")
	}
	pr, err := s.Ratings.GetPlayerRating(ctx, profileID, gameID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get player rating: %v", err)
	}
	return &matchmakingv1.GetPlayerRatingResponse{
		PlayerRating: &matchmakingv1.PlayerRating{
			ProfileId:    pr.ProfileID.String(),
			GameId:       pr.GameID.String(),
			RatingValue:  pr.RatingValue,
			GamesPlayed:  pr.GamesPlayed,
		},
	}, nil
}

// BanFromMM creates a peer MM ban from the authenticated profile.
func (s *MatchmakingGRPC) BanFromMM(ctx context.Context, req *matchmakingv1.BanFromMMRequest) (*matchmakingv1.BanFromMMResponse, error) {
	bannerID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Bans == nil {
		return nil, status.Error(codes.Unavailable, "ban unavailable")
	}
	targetID, err := uuid.Parse(strings.TrimSpace(req.GetTargetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid target_profile_id")
	}
	if bannerID == targetID {
		return nil, status.Error(codes.InvalidArgument, "cannot ban self")
	}
	reason := ""
	if req.Reason != nil {
		reason = strings.TrimSpace(req.GetReason())
	}
	if err := s.Bans.InsertMMPeerBan(ctx, store.InsertMMPeerBanParams{
		BannerProfileID: bannerID,
		TargetProfileID: targetID,
		Reason:          reason,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "ban: %v", err)
	}
	return &matchmakingv1.BanFromMMResponse{}, nil
}

// GetMMBanStatus reports whether the authenticated profile has banned target.
func (s *MatchmakingGRPC) GetMMBanStatus(ctx context.Context, req *matchmakingv1.GetMMBanStatusRequest) (*matchmakingv1.GetMMBanStatusResponse, error) {
	bannerID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Bans == nil {
		return nil, status.Error(codes.Unavailable, "ban unavailable")
	}
	targetID, err := uuid.Parse(strings.TrimSpace(req.GetTargetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid target_profile_id")
	}
	banned, err := s.Bans.IsPeerBanned(ctx, bannerID, targetID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ban status: %v", err)
	}
	return &matchmakingv1.GetMMBanStatusResponse{
		MmBanStatus: &matchmakingv1.MMBanStatus{Banned: banned},
	}, nil
}

// UnbanFromMM removes a peer MM ban.
func (s *MatchmakingGRPC) UnbanFromMM(ctx context.Context, req *matchmakingv1.UnbanFromMMRequest) (*matchmakingv1.UnbanFromMMResponse, error) {
	bannerID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Bans == nil {
		return nil, status.Error(codes.Unavailable, "ban unavailable")
	}
	targetID, err := uuid.Parse(strings.TrimSpace(req.GetTargetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid target_profile_id")
	}
	if err := s.Bans.RemoveMMPeerBan(ctx, bannerID, targetID); err != nil {
		return nil, status.Errorf(codes.Internal, "unban: %v", err)
	}
	return &matchmakingv1.UnbanFromMMResponse{}, nil
}
