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
	"voice/backend/matchmaking/internal/profile"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func (s *MatchmakingGRPC) GetMyPlayerProfile(ctx context.Context, _ *matchmakingv1.GetMyPlayerProfileRequest) (*matchmakingv1.GetMyPlayerProfileResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.ProfileGames == nil {
		return nil, status.Error(codes.Unavailable, "profile store unavailable")
	}
	entries, err := s.ProfileGames.ListByProfile(ctx, profileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list profile: %v", err)
	}
	return &matchmakingv1.GetMyPlayerProfileResponse{Entries: toProtoProfileEntries(entries)}, nil
}

func (s *MatchmakingGRPC) GetPlayerProfile(ctx context.Context, req *matchmakingv1.GetPlayerProfileRequest) (*matchmakingv1.GetPlayerProfileResponse, error) {
	if _, ok := authctx.ProfileID(ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil || profileID == uuid.Nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	if s.ProfileGames == nil {
		return nil, status.Error(codes.Unavailable, "profile store unavailable")
	}
	entries, err := s.ProfileGames.ListByProfile(ctx, profileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list profile: %v", err)
	}
	return &matchmakingv1.GetPlayerProfileResponse{Entries: toProtoProfileEntries(entries)}, nil
}

func (s *MatchmakingGRPC) UpsertPlayerGameEntry(ctx context.Context, req *matchmakingv1.UpsertPlayerGameEntryRequest) (*matchmakingv1.UpsertPlayerGameEntryResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	gameID, err := uuid.Parse(strings.TrimSpace(req.GetGameId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid game_id")
	}
	if s.Games == nil || s.ProfileGames == nil {
		return nil, status.Error(codes.Unavailable, "store unavailable")
	}
	g, err := s.Games.Get(ctx, gameID)
	if errors.Is(err, store.ErrGameNotFound) {
		return nil, status.Error(codes.NotFound, "game not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get game: %v", err)
	}
	if g.Status != store.StatusActive {
		return nil, status.Error(codes.NotFound, "game not found")
	}
	in := profile.EntryInput{Region: req.GetRegion()}
	if req.Role != nil {
		role := req.GetRole()
		in.Role = &role
	}
	if req.Rank != nil {
		rank := req.GetRank()
		in.Rank = &rank
	}
	if err := profile.ValidateEntry(g.Config, in); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	entry, err := s.ProfileGames.Upsert(ctx, store.UpsertProfileGameParams{
		ProfileID: profileID,
		GameID:    gameID,
		Region:    in.Region,
		Role:      in.Role,
		Rank:      in.Rank,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert profile entry: %v", err)
	}
	return &matchmakingv1.UpsertPlayerGameEntryResponse{Entry: toProtoProfileEntry(entry)}, nil
}

func (s *MatchmakingGRPC) DeletePlayerGameEntry(ctx context.Context, req *matchmakingv1.DeletePlayerGameEntryRequest) (*matchmakingv1.DeletePlayerGameEntryResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	gameID, err := uuid.Parse(strings.TrimSpace(req.GetGameId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid game_id")
	}
	if s.ProfileGames == nil {
		return nil, status.Error(codes.Unavailable, "profile store unavailable")
	}
	if err := s.ProfileGames.Delete(ctx, profileID, gameID); errors.Is(err, store.ErrProfileGameEntryNotFound) {
		return nil, status.Error(codes.NotFound, "entry not found")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "delete profile entry: %v", err)
	}
	return &matchmakingv1.DeletePlayerGameEntryResponse{}, nil
}

func toProtoProfileEntries(entries []store.ProfileGameEntry) []*matchmakingv1.PlayerGameEntry {
	out := make([]*matchmakingv1.PlayerGameEntry, len(entries))
	for i, e := range entries {
		out[i] = toProtoProfileEntry(e)
	}
	return out
}

func toProtoProfileEntry(e store.ProfileGameEntry) *matchmakingv1.PlayerGameEntry {
	pg := &matchmakingv1.PlayerGameEntry{
		GameId:    e.GameID.String(),
		Region:    e.Region,
		UpdatedAt: timestamppb.New(e.UpdatedAt),
	}
	if e.Role != nil {
		pg.Role = e.Role
	}
	if e.Rank != nil {
		pg.Rank = e.Rank
	}
	return pg
}
