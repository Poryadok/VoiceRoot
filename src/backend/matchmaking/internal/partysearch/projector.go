// Package partysearch projects durable whole-party queue intents. Redis is not
// authority for source identity or eligibility at any forward boundary.
package partysearch

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"
)

type ProjectionQueue interface {
	RecoverParty(context.Context, []queue.SearchRecovery) error
	RecoverRelease(context.Context, queue.SearchRecovery) error
}

type Projector struct {
	Store           *store.PartyStore
	Queue           ProjectionQueue
	Validator       partysource.ProjectionValidator
	ForwardLifetime ProjectionLifetime
	Resetter        ProjectionResetter
}

func (p *Projector) RunOnce(ctx context.Context) (bool, error) {
	if p == nil || p.Store == nil || p.Queue == nil {
		return false, store.ErrSourcePartyUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	lease, err := p.Store.ClaimQueueIntent(ctx, 30*time.Second)
	if err != nil || lease == nil {
		return false, err
	}
	cut, workErr := p.Store.ReadQueueIntent(ctx, *lease)
	finishLease := *lease
	if workErr == nil && cut.Enqueue && p.Store.NativeAdmission == nil {
		var release func()
		release, workErr = p.Store.EnterPrecutQueueProjection(ctx)
		if release != nil {
			defer release()
		}
	}
	if workErr == nil {
		if cut.Enqueue && p.Store.NativeAdmission != nil {
			var release func()
			release, workErr = p.authorizeNativeProjection(ctx, &cut)
			finishLease = cut.Lease
			if release != nil {
				defer release()
			}
		}
	}
	if workErr == nil {
		finishLease = cut.Lease
	}
	if workErr == nil {
		var sessions []queue.SearchRecovery
		sessions, workErr = queueRecoveries(cut.Sessions)
		if workErr == nil {
			if cut.Enqueue {
				workErr = p.Queue.RecoverParty(ctx, sessions)
			} else {
				// Cleanup is idempotent and must attempt every retained session,
				// even if one independent release fails temporarily.
				for _, session := range sessions {
					workErr = errors.Join(workErr, p.Queue.RecoverRelease(ctx, session))
				}
			}
		}
	}
	delay := 10 * time.Second
	if workErr != nil {
		delay = time.Second
	}
	cleanup, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
	defer cleanupCancel()
	finishErr := p.Store.FinishQueueIntent(cleanup, finishLease, delay, workErr == nil)
	return true, errors.Join(workErr, finishErr)
}

func queueRecoveries(sessions []store.SearchSession) ([]queue.SearchRecovery, error) {
	if len(sessions) == 0 {
		return nil, store.ErrSourcePartyUnavailable
	}
	out := make([]queue.SearchRecovery, 0, len(sessions))
	for _, s := range sessions {
		var criteria struct {
			Region string `json:"region"`
		}
		if s.ID == uuid.Nil || s.ProfileID == uuid.Nil || s.GameID == uuid.Nil || s.Mode == "" || s.CreatedAt.IsZero() || s.RecoveryGeneration < 0 || json.Unmarshal([]byte(s.Criteria), &criteria) != nil || criteria.Region == "" || strings.TrimSpace(criteria.Region) != criteria.Region {
			return nil, store.ErrSourcePartyUnavailable
		}
		out = append(out, queue.SearchRecovery{SpaceID: s.SpaceID, GameID: s.GameID, Mode: s.Mode, Region: criteria.Region, SessionID: s.ID, ProfileID: s.ProfileID, CreatedAt: s.CreatedAt, Generation: s.RecoveryGeneration})
	}
	return out, nil
}
