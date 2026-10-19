package matcher

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"voice/backend/matchmaking/internal/config"
	"voice/backend/matchmaking/internal/criteria"
	"voice/backend/matchmaking/internal/store"
)

// queuedPartyBundles is only local selection metadata. It does not adopt a
// source identity or replace the locked complete-party/protected source checks.
func queuedPartyBundles(ctx context.Context, ordered []store.SearchSession, parties *store.PartyStore) ([][]store.SearchSession, error) {
	bundles := make([][]store.SearchSession, 0, len(ordered))
	positions := make(map[uuid.UUID]int)
	seen := make(map[uuid.UUID]bool)
	for _, session := range ordered {
		if seen[session.ID] {
			continue
		}
		seen[session.ID] = true
		if session.PartyID == nil {
			bundles = append(bundles, []store.SearchSession{session})
			continue
		}
		position, exists := positions[*session.PartyID]
		if !exists {
			position = len(bundles)
			positions[*session.PartyID] = position
			bundles = append(bundles, nil)
		}
		bundles[position] = append(bundles[position], session)
	}
	complete := make([][]store.SearchSession, 0, len(bundles))
	for _, bundle := range bundles {
		if bundle[0].PartyID == nil {
			complete = append(complete, bundle)
			continue
		}
		party, err := parties.Get(ctx, *bundle[0].PartyID)
		if errors.Is(err, store.ErrPartyNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if party.DisbandedAt != nil || len(party.MemberProfileIDs) != len(bundle) || len(bundle) == 0 {
			continue
		}
		members := make(map[uuid.UUID]bool, len(bundle))
		valid := true
		for _, profile := range party.MemberProfileIDs {
			if profile == uuid.Nil || members[profile] {
				valid = false
			}
			members[profile] = true
		}
		for _, session := range bundle {
			if !members[session.ProfileID] || session.GameID != party.GameID || session.Mode != party.Mode {
				valid = false
			}
			delete(members, session.ProfileID)
		}
		if valid && len(members) == 0 {
			complete = append(complete, bundle)
		}
	}
	return complete, nil
}

func selectWholePartyGroup(ctx context.Context, bundles [][]store.SearchSession, mode config.Mode, bans PeerBanChecker) ([]store.SearchSession, error) {
	compatible := func(group, addition []store.SearchSession) (bool, error) {
		all := append(append([]store.SearchSession(nil), group...), addition...)
		for i, session := range all {
			current, err := criteria.Parse(session.Criteria)
			if err != nil {
				return false, nil
			}
			for j := 0; j < i; j++ {
				previous, err := criteria.Parse(all[j].Criteria)
				if err != nil || !criteria.Compatible(previous, current, mode) {
					return false, nil
				}
				denied, err := bans.IsPairBanned(ctx, all[j].ProfileID, session.ProfileID)
				if err != nil {
					return false, err
				}
				if denied {
					return false, nil
				}
			}
		}
		return true, nil
	}
	for i, anchor := range bundles {
		if len(anchor) > mode.Slots {
			continue
		}
		ok, err := compatible(nil, anchor)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		group := append([]store.SearchSession(nil), anchor...)
		for _, candidate := range bundles[i+1:] {
			if len(group)+len(candidate) > mode.Slots {
				continue
			}
			ok, err := compatible(group, candidate)
			if err != nil {
				return nil, err
			}
			if ok {
				group = append(group, candidate...)
			}
			if len(group) == mode.Slots {
				break
			}
		}
		if len(group) == mode.Slots {
			return group, nil
		}
	}
	return nil, nil
}
