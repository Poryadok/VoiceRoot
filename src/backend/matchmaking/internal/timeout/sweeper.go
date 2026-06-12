package timeout

import (
	"context"
	"log/slog"
	"time"

	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/runtimeconfig"
	"voice/backend/matchmaking/internal/store"
)

// Sweeper enforces search nudge and timeout policies.
type Sweeper struct {
	Sessions *store.SessionStore
	Queue    *queue.RedisQueue
	Events   mmevents.Publisher
	Timing   runtimeconfig.SearchTiming
	Now      func() time.Time
	Logger   *slog.Logger
}

// RunOnce processes due nudges and expired searching sessions.
func (s *Sweeper) RunOnce(ctx context.Context) error {
	if s == nil || s.Sessions == nil {
		return nil
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	timing := s.Timing
	if timing.NudgeAfter <= 0 {
		timing = runtimeconfig.LoadSearchTiming()
	}

	nudgeCutoff := now.Add(-timing.NudgeAfter)
	sessions, err := s.Sessions.ListSearchingNeedingNudge(ctx, nudgeCutoff, 100)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		if s.Events != nil {
			if err := s.Events.PublishSearchNudge(ctx, sess.ID.String(), sess.ProfileID.String(), sess.GameID.String(), sess.Mode); err != nil {
				if s.Logger != nil {
					s.Logger.Warn("mm.search_nudge publish failed",
						slog.String("session_id", sess.ID.String()),
						slog.Any("error", err))
				}
				continue
			}
		}
		if _, err := s.Sessions.MarkNudged(ctx, sess.ID); err != nil && s.Logger != nil {
			s.Logger.Warn("mark nudged failed",
				slog.String("session_id", sess.ID.String()),
				slog.Any("error", err))
		}
	}

	expired, err := s.Sessions.ListSearchingExpired(ctx, now, 100)
	if err != nil {
		return err
	}
	for _, sess := range expired {
		if err := s.cleanupQueue(ctx, sess); err != nil {
			if s.Logger != nil {
				s.Logger.Warn("expire queue cleanup failed",
					slog.String("session_id", sess.ID.String()),
					slog.Any("error", err))
			}
			continue
		}
		if _, err := s.Sessions.ExpireSearching(ctx, sess.ID); err != nil {
			if s.Logger != nil {
				s.Logger.Warn("expire session failed",
					slog.String("session_id", sess.ID.String()),
					slog.Any("error", err))
			}
			continue
		}
		if s.Events != nil {
			if err := s.Events.PublishSearchTimeout(ctx, sess.ID.String(), sess.ProfileID.String(), sess.GameID.String(), sess.Mode); err != nil && s.Logger != nil {
				s.Logger.Warn("mm.search_timeout publish failed",
					slog.String("session_id", sess.ID.String()),
					slog.Any("error", err))
			}
		}
	}
	return nil
}

func (s *Sweeper) cleanupQueue(ctx context.Context, sess store.SearchSession) error {
	if s.Queue == nil {
		return nil
	}
	crit, err := criteria.Parse(sess.Criteria)
	if err != nil {
		return err
	}
	if err := s.Queue.Dequeue(ctx, sess.GameID, sess.Mode, crit.Region, sess.ID); err != nil {
		return err
	}
	return s.Queue.ReleaseLock(ctx, sess.ProfileID, sess.ID)
}
