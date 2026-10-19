package grpcsvc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/runtimeconfig"
	"voice/backend/matchmaking/internal/searchprincipal"
	"voice/backend/matchmaking/internal/store"
)

func (s *MatchmakingGRPC) startProtectedSearch(ctx context.Context, req *matchmakingv1.StartSearchRequest) (store.SearchSession, error) {
	verified, ok := searchprincipal.SubjectFromContext(ctx, req)
	if !ok {
		return store.SearchSession{}, status.Error(codes.Unauthenticated, "invalid search principal")
	}
	if req.PartyId != nil {
		return store.SearchSession{}, status.Error(codes.InvalidArgument, "party_id is not accepted for public search")
	}
	if s == nil || s.Games == nil || s.Parties == nil || s.PartySnapshots == nil || s.Queue == nil {
		return store.SearchSession{}, status.Error(codes.Unavailable, "search authority unavailable")
	}
	if s.Bans == nil {
		return store.SearchSession{}, status.Error(codes.FailedPrecondition, "ban store not configured")
	}
	if err := s.Queue.Ping(ctx); err != nil {
		return store.SearchSession{}, status.Error(codes.Unavailable, "queue unavailable")
	}
	gameID, err := uuid.Parse(strings.TrimSpace(req.GetGameId()))
	if err != nil || gameID == uuid.Nil {
		return store.SearchSession{}, status.Error(codes.InvalidArgument, "invalid game_id")
	}
	modeName := strings.TrimSpace(req.GetMode())
	if modeName == "" {
		return store.SearchSession{}, status.Error(codes.InvalidArgument, "mode required")
	}
	crit, err := criteria.Parse(req.GetCriteriaJson())
	if err != nil {
		return store.SearchSession{}, status.Error(codes.InvalidArgument, "invalid search criteria")
	}
	game, err := s.Games.Get(ctx, gameID)
	if errors.Is(err, store.ErrGameNotFound) {
		return store.SearchSession{}, status.Error(codes.NotFound, "game not found")
	}
	if err != nil {
		return store.SearchSession{}, status.Error(codes.Unavailable, "game unavailable")
	}
	if game.Status != store.StatusActive {
		return store.SearchSession{}, status.Error(codes.FailedPrecondition, "game not active")
	}
	cfg, err := config.Parse(game.ConfigRaw)
	if err != nil {
		return store.SearchSession{}, status.Error(codes.Unavailable, "game configuration unavailable")
	}
	subject := partysource.Subject{SearchOperationID: verified.SearchOperationID, AccountID: verified.AccountID, ProfileID: verified.ProfileID, SessionEpoch: verified.SessionEpoch}
	snapshot, err := s.PartySnapshots.Snapshot(ctx, subject)
	if errors.Is(err, partysource.ErrIneligible) {
		return store.SearchSession{}, status.Error(codes.FailedPrecondition, "party unavailable")
	}
	if err != nil || partysource.ValidateSearchSnapshot(snapshot, subject) != nil {
		return store.SearchSession{}, status.Error(codes.Unavailable, "party authority unavailable")
	}
	if _, err := criteria.Validate(crit, cfg, modeName, len(snapshot.MemberProfileIDs)); err != nil {
		return store.SearchSession{}, status.Error(codes.InvalidArgument, "invalid search criteria or party size")
	}
	created, err := s.Parties.CreateSourceParty(ctx, store.CreateSourcePartyParams{
		SearchOperationID: verified.SearchOperationID, SearchRequestHash: verified.SearchRequestHash,
		InitiatorAccountID: verified.AccountID, InitiatorProfileID: verified.ProfileID, InitiatorSessionEpoch: verified.SessionEpoch,
		ProtocolVersion: snapshot.ProtocolVersion, SourceKind: snapshot.Kind, SourceRoomID: snapshot.RoomID,
		SourceRosterVersion: snapshot.RosterVersion, MemberProfileIDs: snapshot.MemberProfileIDs,
		MemberAuthorityVersion: snapshot.MemberAuthorityVersion, MemberAuthorities: snapshot.MemberAuthorities, SourceAuthorityHash: snapshot.SourceAuthorityHash,
		GameID: gameID, Mode: modeName, Criteria: criteria.MustMarshal(crit), TimeoutAt: time.Now().UTC().Add(runtimeconfig.LoadSearchTiming().Timeout),
	})
	switch {
	case errors.Is(err, store.ErrPlatformBanDenied):
		return store.SearchSession{}, status.Error(codes.PermissionDenied, "party matchmaking unavailable")
	case errors.Is(err, store.ErrActiveSearchExists), errors.Is(err, store.ErrSearchOperationConflict), errors.Is(err, store.ErrSourcePartyChanged):
		return store.SearchSession{}, status.Error(codes.FailedPrecondition, "party search unavailable")
	case err != nil:
		return store.SearchSession{}, status.Error(codes.Unavailable, "search unavailable")
	}
	// Redis is recovered only from the committed durable intent by Projector.
	// A lost commit acknowledgment can replay this operation without new sessions.
	if !created.Replay && s.Events != nil {
		for _, session := range created.Sessions {
			if err := s.Events.PublishSearchStarted(ctx, session.ID.String(), session.ProfileID.String(), gameID.String(), modeName, crit.Region); err != nil && s.Logger != nil {
				s.Logger.Warn("party search-started event unavailable")
			}
		}
	}
	for _, session := range created.Sessions {
		if session.ProfileID == verified.ProfileID {
			return session, nil
		}
	}
	return store.SearchSession{}, status.Error(codes.Unavailable, "search unavailable")
}
