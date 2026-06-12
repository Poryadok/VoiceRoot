package grpcsvc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/matchmaking/internal/authctx"
	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/runtimeconfig"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// SearchDeps wires queue search RPC dependencies.
type SearchDeps struct {
	Sessions *store.SessionStore
	Games    *store.GameStore
	Queue    *queue.RedisQueue
	Events   mmevents.Publisher
	Logger   *slog.Logger
}

func (s *MatchmakingGRPC) searchDeps() SearchDeps {
	return SearchDeps{
		Sessions: s.Sessions,
		Games:    s.Games,
		Queue:    s.Queue,
		Events:   s.Events,
		Logger:   s.Logger,
	}
}

func (s *MatchmakingGRPC) StartSearch(ctx context.Context, req *matchmakingv1.StartSearchRequest) (*matchmakingv1.StartSearchResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	deps := s.searchDeps()
	if deps.Sessions == nil || deps.Games == nil {
		return nil, status.Error(codes.Unavailable, "search unavailable")
	}
	if deps.Queue == nil {
		return nil, status.Error(codes.Unavailable, "queue unavailable")
	}
	if err := deps.Queue.Ping(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "queue unavailable")
	}

	gameID, err := uuid.Parse(strings.TrimSpace(req.GetGameId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid game_id")
	}
	modeName := strings.TrimSpace(req.GetMode())
	if modeName == "" {
		return nil, status.Error(codes.InvalidArgument, "mode required")
	}

	crit, err := criteria.Parse(req.GetCriteriaJson())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid criteria_json: %v", err)
	}

	game, err := deps.Games.Get(ctx, gameID)
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
	if _, err := criteria.Validate(crit, gameCfg, modeName, 1); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid criteria: %v", err)
	}

	if _, err := deps.Sessions.GetActiveSearching(ctx, profileID); err == nil {
		return nil, status.Error(codes.FailedPrecondition, "active search already exists")
	} else if !errors.Is(err, store.ErrSessionNotFound) {
		return nil, status.Errorf(codes.Internal, "check active search: %v", err)
	}

	canonical := criteria.MustMarshal(crit)
	searchTiming := runtimeconfig.LoadSearchTiming()
	timeoutAt := time.Now().UTC().Add(searchTiming.Timeout)
	sess, err := deps.Sessions.Create(ctx, store.CreateSessionParams{
		ProfileID: profileID,
		GameID:    gameID,
		Mode:      modeName,
		Criteria:  canonical,
		TimeoutAt: timeoutAt,
	})
	if errors.Is(err, store.ErrActiveSearchExists) {
		return nil, status.Error(codes.FailedPrecondition, "active search already exists")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create session: %v", err)
	}

	if err := deps.Queue.AcquireLock(ctx, profileID, sess.ID); err != nil {
		_, _ = deps.Sessions.Cancel(ctx, sess.ID)
		if errors.Is(err, queue.ErrLockHeld) {
			return nil, status.Error(codes.FailedPrecondition, "active search already exists")
		}
		return nil, status.Error(codes.Unavailable, "queue unavailable")
	}

	if err := deps.Queue.Enqueue(ctx, gameID, modeName, crit.Region, sess.ID, sess.CreatedAt); err != nil {
		_ = deps.Queue.ReleaseLock(ctx, profileID, sess.ID)
		_, _ = deps.Sessions.Cancel(ctx, sess.ID)
		return nil, status.Error(codes.Unavailable, "queue unavailable")
	}

	if deps.Events != nil {
		if pubErr := deps.Events.PublishSearchStarted(ctx, sess.ID.String(), profileID.String(), gameID.String(), modeName, crit.Region); pubErr != nil {
			if deps.Logger != nil {
				deps.Logger.Warn("mm.search_started publish failed", slog.Any("error", pubErr))
			}
		}
	}

	return &matchmakingv1.StartSearchResponse{SearchSession: toProtoSession(sess)}, nil
}

func (s *MatchmakingGRPC) CancelSearch(ctx context.Context, req *matchmakingv1.CancelSearchRequest) (*matchmakingv1.CancelSearchResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	deps := s.searchDeps()
	if deps.Sessions == nil {
		return nil, status.Error(codes.Unavailable, "search unavailable")
	}

	sessionID, err := uuid.Parse(strings.TrimSpace(req.GetSessionId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id")
	}

	sess, err := deps.Sessions.Get(ctx, sessionID)
	if errors.Is(err, store.ErrSessionNotFound) {
		return nil, status.Error(codes.NotFound, "session not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get session: %v", err)
	}
	if sess.ProfileID != profileID {
		return nil, status.Error(codes.PermissionDenied, "not session owner")
	}
	if sess.Status != store.SessionStatusSearching {
		return nil, status.Error(codes.FailedPrecondition, "session not searching")
	}

	crit, err := criteria.Parse(sess.Criteria)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid stored criteria: %v", err)
	}

	if deps.Queue != nil {
		_ = deps.Queue.Dequeue(ctx, sess.GameID, sess.Mode, crit.Region, sess.ID)
		_ = deps.Queue.ReleaseLock(ctx, profileID, sess.ID)
	}

	if _, err := deps.Sessions.Cancel(ctx, sess.ID); errors.Is(err, store.ErrSessionNotSearchable) {
		return nil, status.Error(codes.FailedPrecondition, "session not searching")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "cancel session: %v", err)
	}

	if deps.Events != nil {
		if pubErr := deps.Events.PublishSearchCancelled(ctx, sess.ID.String(), profileID.String()); pubErr != nil && deps.Logger != nil {
			deps.Logger.Warn("mm.search_cancelled publish failed", slog.Any("error", pubErr))
		}
	}

	return &matchmakingv1.CancelSearchResponse{}, nil
}

func (s *MatchmakingGRPC) GetSearchStatus(ctx context.Context, req *matchmakingv1.GetSearchStatusRequest) (*matchmakingv1.GetSearchStatusResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Sessions == nil {
		return nil, status.Error(codes.Unavailable, "search unavailable")
	}

	sessionID, err := uuid.Parse(strings.TrimSpace(req.GetSessionId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session_id")
	}

	sess, err := s.Sessions.Get(ctx, sessionID)
	if errors.Is(err, store.ErrSessionNotFound) {
		return nil, status.Error(codes.NotFound, "session not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get session: %v", err)
	}
	if sess.ProfileID != profileID {
		return nil, status.Error(codes.PermissionDenied, "not session owner")
	}

	return &matchmakingv1.GetSearchStatusResponse{SearchSession: toProtoSession(sess)}, nil
}

func toProtoSession(sess store.SearchSession) *matchmakingv1.SearchSession {
	out := &matchmakingv1.SearchSession{
		Id:           sess.ID.String(),
		ProfileId:    sess.ProfileID.String(),
		GameId:       sess.GameID.String(),
		Mode:         sess.Mode,
		CriteriaJson: sess.Criteria,
		Status:       sess.Status,
	}
	if sess.PartyID != nil {
		partyID := sess.PartyID.String()
		out.PartyId = &partyID
	}
	if sess.TimeoutAt != nil {
		out.TimeoutAt = timestamppb.New(*sess.TimeoutAt)
	}
	if sess.MatchedAt != nil {
		out.MatchedAt = timestamppb.New(*sess.MatchedAt)
	}
	if sess.MatchID != nil {
		matchID := sess.MatchID.String()
		out.MatchId = &matchID
	}
	return out
}
