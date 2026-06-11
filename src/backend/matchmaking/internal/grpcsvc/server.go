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
	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// MatchmakingGRPC implements catalog RPCs for Phase 7.
type MatchmakingGRPC struct {
	matchmakingv1.UnimplementedMatchmakingServiceServer
	Games *store.GameStore
}

func (s *MatchmakingGRPC) ListGames(ctx context.Context, req *matchmakingv1.ListGamesRequest) (*matchmakingv1.ListGamesResponse, error) {
	if s.Games == nil {
		return nil, status.Error(codes.Unavailable, "game store unavailable")
	}
	pageSize := int32(50)
	if req.GetPage() != nil && req.GetPage().GetPageSize() > 0 {
		pageSize = req.GetPage().GetPageSize()
	}
	cursor := ""
	if req.GetPage() != nil {
		cursor = req.GetPage().GetCursor()
	}
	res, err := s.Games.List(ctx, store.ListGamesParams{
		Cursor:   cursor,
		PageSize: pageSize,
		Status:   store.StatusActive,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list games: %v", err)
	}
	return &matchmakingv1.ListGamesResponse{
		GameList: &matchmakingv1.GameList{
			Games:      toProtoGames(res.Games),
			NextCursor: res.NextCursor,
		},
	}, nil
}

func (s *MatchmakingGRPC) GetGame(ctx context.Context, req *matchmakingv1.GetGameRequest) (*matchmakingv1.GetGameResponse, error) {
	if s.Games == nil {
		return nil, status.Error(codes.Unavailable, "game store unavailable")
	}
	id, err := uuid.Parse(req.GetGameId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid game_id")
	}
	g, err := s.Games.Get(ctx, id)
	if errors.Is(err, store.ErrGameNotFound) {
		return nil, status.Error(codes.NotFound, "game not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get game: %v", err)
	}
	return &matchmakingv1.GetGameResponse{Game: toProtoGame(g)}, nil
}

func (s *MatchmakingGRPC) CreateGame(ctx context.Context, req *matchmakingv1.CreateGameRequest) (*matchmakingv1.CreateGameResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if strings.TrimSpace(req.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	cfg, err := config.Parse(req.GetConfigJson())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid config_json: %v", err)
	}
	if s.Games == nil {
		return nil, status.Error(codes.Unavailable, "game store unavailable")
	}
	// v1: any authenticated caller may create; platform moderator check deferred to Moderation Service.
	g, err := s.Games.Create(ctx, req.GetName(), cfg, profileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create game: %v", err)
	}
	return &matchmakingv1.CreateGameResponse{Game: toProtoGame(g)}, nil
}

func (s *MatchmakingGRPC) UpdateGame(ctx context.Context, req *matchmakingv1.UpdateGameRequest) (*matchmakingv1.UpdateGameResponse, error) {
	if _, ok := authctx.ProfileID(ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	id, err := uuid.Parse(req.GetGameId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid game_id")
	}
	params := store.UpdateGameParams{}
	if req.Name != nil {
		name := req.GetName()
		params.Name = &name
	}
	if req.ConfigJson != nil {
		cfg, err := config.Parse(req.GetConfigJson())
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid config_json: %v", err)
		}
		params.Config = &cfg
	}
	if req.Status != nil {
		st := req.GetStatus()
		params.Status = &st
	}
	if s.Games == nil {
		return nil, status.Error(codes.Unavailable, "game store unavailable")
	}
	g, err := s.Games.Update(ctx, id, params)
	if errors.Is(err, store.ErrGameNotFound) {
		return nil, status.Error(codes.NotFound, "game not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update game: %v", err)
	}
	return &matchmakingv1.UpdateGameResponse{Game: toProtoGame(g)}, nil
}

func (s *MatchmakingGRPC) SearchGames(ctx context.Context, req *matchmakingv1.SearchGamesRequest) (*matchmakingv1.SearchGamesResponse, error) {
	if s.Games == nil {
		return nil, status.Error(codes.Unavailable, "game store unavailable")
	}
	games, err := s.Games.Search(ctx, req.GetQuery(), 20)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search games: %v", err)
	}
	return &matchmakingv1.SearchGamesResponse{
		GameList: &matchmakingv1.GameList{Games: toProtoGames(games)},
	}, nil
}

func toProtoGames(games []store.Game) []*matchmakingv1.Game {
	out := make([]*matchmakingv1.Game, len(games))
	for i, g := range games {
		out[i] = toProtoGame(g)
	}
	return out
}

func toProtoGame(g store.Game) *matchmakingv1.Game {
	pg := &matchmakingv1.Game{
		Id:                 g.ID.String(),
		Name:               g.Name,
		ConfigJson:         g.ConfigRaw,
		Status:             g.Status,
		CreatedAt:          timestamppb.New(g.CreatedAt),
	}
	if g.IconURL != nil {
		pg.IconUrl = g.IconURL
	}
	if g.ExternalID != nil {
		pg.ExternalId = g.ExternalID
	}
	if g.CreatedBy != nil {
		pg.CreatedByProfileId = g.CreatedBy.String()
	}
	return pg
}
