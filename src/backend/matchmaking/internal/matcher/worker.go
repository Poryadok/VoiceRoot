package matcher

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"
)

// MatchFoundEvent is published when a match proposal is created.
type MatchFoundEvent struct {
	MatchID    uuid.UUID
	GameID     uuid.UUID
	Mode       string
	Region     string
	ProfileIDs []uuid.UUID
	SessionIDs []uuid.UUID
}

// EventPublisher publishes match-found events.
type EventPublisher interface {
	PublishMatchFound(ctx context.Context, ev MatchFoundEvent) error
}

// PeerBanChecker reports whether two profiles must not be matched.
type PeerBanChecker interface {
	IsPairBanned(ctx context.Context, a, b uuid.UUID) (bool, error)
}

// Worker polls queues and creates match proposals.
type Worker struct {
	Queue    *queue.RedisQueue
	Sessions *store.SessionStore
	Matches  *store.MatchStore
	Games    *store.GameStore
	Bans     PeerBanChecker
	Events   EventPublisher
	Logger   *slog.Logger
}

// RunOnce attempts to form matches for all active games.
func (w *Worker) RunOnce(ctx context.Context) error {
	if w == nil || w.Games == nil || w.Queue == nil || w.Sessions == nil || w.Matches == nil {
		return nil
	}
	res, err := w.Games.List(ctx, store.ListGamesParams{PageSize: 100, Status: store.StatusActive})
	if err != nil {
		return err
	}
	for _, game := range res.Games {
		cfg, err := config.Parse(game.ConfigRaw)
		if err != nil {
			continue
		}
		for _, mode := range cfg.Modes {
			for _, region := range cfg.Regions {
				if err := w.tryMatchQueue(ctx, game.ID, mode, region, cfg); err != nil && w.Logger != nil {
					w.Logger.Warn("matcher queue pass failed",
						slog.String("game_id", game.ID.String()),
						slog.String("mode", mode.Name),
						slog.String("region", region),
						slog.Any("error", err))
				}
			}
		}
	}
	return nil
}

func (w *Worker) tryMatchQueue(ctx context.Context, gameID uuid.UUID, mode config.Mode, region string, cfg config.GameConfig) error {
	depth, err := w.Queue.QueueDepth(ctx, gameID, mode.Name, region)
	if err != nil || depth < int64(mode.Slots) {
		return err
	}

	ids, err := w.Queue.ListSessionIDs(ctx, gameID, mode.Name, region, 0)
	if err != nil {
		return err
	}
	sessions, err := w.Sessions.ListSearchingByIDs(ctx, ids)
	if err != nil {
		return err
	}
	if len(sessions) < mode.Slots {
		return nil
	}

	byID := make(map[uuid.UUID]store.SearchSession, len(sessions))
	for _, sess := range sessions {
		byID[sess.ID] = sess
	}

	ordered := make([]store.SearchSession, 0, len(sessions))
	for _, id := range ids {
		if sess, ok := byID[id]; ok {
			ordered = append(ordered, sess)
		}
	}

	for i := 0; i < len(ordered); i++ {
		anchor := ordered[i]
		anchorCrit, err := criteria.Parse(anchor.Criteria)
		if err != nil {
			continue
		}
		group := []store.SearchSession{anchor}
		used := map[uuid.UUID]bool{anchor.ID: true}

		for j := i + 1; j < len(ordered) && len(group) < mode.Slots; j++ {
			candidate := ordered[j]
			if used[candidate.ID] {
				continue
			}
			candCrit, err := criteria.Parse(candidate.Criteria)
			if err != nil {
				continue
			}
			if !criteria.Compatible(anchorCrit, candCrit, mode) {
				continue
			}
			if w.Bans != nil {
				banned, err := w.Bans.IsPairBanned(ctx, anchor.ProfileID, candidate.ProfileID)
				if err != nil {
					if w.Logger != nil {
						w.Logger.Warn("peer ban check failed; fail-open", slog.Any("error", err))
					}
				} else if banned {
					continue
				}
			}
			compatible := true
			for _, existing := range group {
				existCrit, err := criteria.Parse(existing.Criteria)
				if err != nil || !criteria.Compatible(existCrit, candCrit, mode) {
					compatible = false
					break
				}
				if w.Bans != nil {
					banned, err := w.Bans.IsPairBanned(ctx, existing.ProfileID, candidate.ProfileID)
					if err != nil {
						if w.Logger != nil {
							w.Logger.Warn("peer ban check failed; fail-open", slog.Any("error", err))
						}
					} else if banned {
						compatible = false
						break
					}
				}
			}
			if !compatible {
				continue
			}
			group = append(group, candidate)
			used[candidate.ID] = true
		}

		if len(group) < mode.Slots {
			continue
		}

		proposalSessions := make([]store.ProposalSession, len(group))
		sessionIDs := make([]uuid.UUID, len(group))
		profileIDs := make([]uuid.UUID, len(group))
		for k, sess := range group {
			proposalSessions[k] = store.ProposalSession{
				SessionID: sess.ID,
				ProfileID: sess.ProfileID,
				PartyID:   sess.PartyID,
			}
			sessionIDs[k] = sess.ID
			profileIDs[k] = sess.ProfileID
		}

		result, err := w.Matches.CreateProposal(ctx, store.CreateProposalParams{
			GameID:   gameID,
			Mode:     mode.Name,
			Region:   region,
			Sessions: proposalSessions,
		})
		if err != nil {
			return err
		}

		for _, sess := range group {
			_ = w.Queue.Dequeue(ctx, gameID, mode.Name, region, sess.ID)
		}

		if w.Events != nil {
			_ = w.Events.PublishMatchFound(ctx, MatchFoundEvent{
				MatchID:    result.Match.ID,
				GameID:     gameID,
				Mode:       mode.Name,
				Region:     region,
				ProfileIDs: profileIDs,
				SessionIDs: sessionIDs,
			})
		}
		return nil
	}
	return nil
}
