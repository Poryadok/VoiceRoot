package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	callsv1 "voice.app/voice/calls/v1"
)

// OrdinaryReplacementAuthority proves from the native journal that an older
// conflicting room has completed cleanup. It runs outside database transactions
// and must reject unknown/legacy or foreign ownership.
type OrdinaryReplacementAuthority func(context.Context, string) error

type OrdinaryProjectionStore interface {
	ApplyOrdinaryProjection(context.Context, Call, OrdinaryReplacementAuthority) (Call, error)
}

func validateOrdinaryProjection(c Call) error {
	room, err := uuid.Parse(c.RoomID)
	if err != nil || room == uuid.Nil || room.String() != c.RoomID {
		return ErrInvalidState
	}
	op, err := uuid.Parse(c.OrdinaryOperationID)
	if err != nil || op == uuid.Nil || op.String() != c.OrdinaryOperationID {
		return ErrInvalidState
	}
	if (c.SessionKind != callsv1.VoiceSessionKind_VOICE_SESSION_KIND_CALL && c.SessionKind != callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE) ||
		c.OrdinaryVersion == 0 || c.OrdinaryMediaIdentities == nil || c.States == nil || c.ManagedGameSession ||
		c.MatchSquadMatchID != "" || c.SpaceID != "" || c.VoiceRoomID != "" || c.ApplicationID != "" || c.EnvironmentID != "" || c.SessionID != "" || c.IsVoiceRoom() {
		return ErrInvalidState
	}
	for profile := range c.States {
		if c.OrdinaryMediaIdentities[profile] == "" {
			return ErrInvalidState
		}
	}
	return nil
}

func mergeOrdinaryProjection(prior, incoming Call) (Call, error) {
	if prior.RoomID != "" {
		if prior.OrdinaryOperationID != incoming.OrdinaryOperationID || prior.ManagedGameSession || prior.MatchSquadMatchID != "" ||
			prior.SpaceID != "" || prior.VoiceRoomID != "" || prior.ChatID != incoming.ChatID || prior.SessionKind != incoming.SessionKind {
			return Call{}, ErrInvalidState
		}
		if prior.OrdinaryVersion >= incoming.OrdinaryVersion {
			return prior, nil
		}
	}
	// The caller-owned maps must never alias mutable projection state.
	identities := make(map[string]string, len(incoming.OrdinaryMediaIdentities))
	for profile, identity := range incoming.OrdinaryMediaIdentities {
		identities[profile] = identity
	}
	incoming.OrdinaryMediaIdentities = identities
	states := make(map[string]ParticipantState, len(incoming.States))
	for profile, state := range incoming.States {
		if old, exists := prior.States[profile]; exists && prior.OrdinaryMediaIdentities[profile] == identities[profile] {
			state = old
		}
		states[profile] = state
	}
	incoming.States = states
	// Screen shares belong to a committed generation, like participant flags.
	incoming.ScreenShares = nil
	for _, share := range prior.ScreenShares {
		if _, exists := states[share.ProfileID]; exists && prior.OrdinaryMediaIdentities[share.ProfileID] == identities[share.ProfileID] {
			incoming.ScreenShares = append(incoming.ScreenShares, share)
		}
	}
	return incoming, nil
}

func (s *MemoryCallStore) ApplyOrdinaryProjection(ctx context.Context, call Call, authorize OrdinaryReplacementAuthority) (Call, error) {
	if err := validateOrdinaryProjection(call); err != nil {
		return Call{}, err
	}
	// Never call PostgreSQL under the in-memory mutex. A conflicting owner can
	// change while checking; loop and validate the same snapshot before mutation.
	for attempt := 0; attempt < redisTransitionAttempts; attempt++ {
		s.mu.Lock()
		prior := s.calls[call.RoomID]
		merged, err := mergeOrdinaryProjection(prior, call)
		if err != nil {
			s.mu.Unlock()
			return Call{}, err
		}
		conflicts := map[string]bool{}
		for room, other := range s.calls {
			if room == call.RoomID {
				continue
			}
			for _, profile := range merged.ProfileIDs() {
				if merged.IsActiveForProfile(profile) && other.IsActiveForProfile(profile) {
					conflicts[room] = true
				}
			}
			if merged.IsGroupVoice() && merged.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE && other.IsGroupVoice() && merged.ChatID == other.ChatID && other.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
				conflicts[room] = true
			}
		}
		if len(conflicts) == 0 {
			s.calls[call.RoomID] = merged
			s.mu.Unlock()
			return merged, nil
		}
		s.mu.Unlock()
		if authorize == nil {
			return Call{}, ErrActiveCall
		}
		for room := range conflicts {
			if err := authorize(ctx, room); err != nil {
				return Call{}, err
			}
		}
		s.mu.Lock()
		// Confirm unchanged owner snapshots before replacing stale projections.
		for room := range conflicts {
			other := s.calls[room]
			if other.OrdinaryOperationID == "" || other.ManagedGameSession || other.MatchSquadMatchID != "" || other.SpaceID != "" {
				s.mu.Unlock()
				return Call{}, ErrInvalidState
			}
		}
		// Native closed rooms cannot become active again; recording their
		// terminal projection here prevents Memory lookup's map-order ambiguity.
		for room := range conflicts {
			other := s.calls[room]
			other.Status = callsv1.CallStatus_CALL_STATUS_ENDED
			other.States = map[string]ParticipantState{}
			s.calls[room] = other
		}
		s.mu.Unlock()
	}
	return Call{}, ErrMoveContention
}

func (s *RedisCallStore) ApplyOrdinaryProjection(ctx context.Context, call Call, authorize OrdinaryReplacementAuthority) (Call, error) {
	if err := validateOrdinaryProjection(call); err != nil {
		return Call{}, err
	}
	keys := []string{s.callKey(call.RoomID)}
	profiles := map[string]bool{}
	for profile := range call.OrdinaryMediaIdentities {
		profiles[profile] = true
	}
	for _, profile := range call.ProfileIDs() {
		profiles[profile] = true
	}
	for profile := range profiles {
		keys = append(keys, s.activeKey(profile))
	}
	if call.IsGroupVoice() {
		keys = append(keys, s.activeChatKey(call.ChatID))
	}
	var result Call
	for attempt := 0; attempt < redisTransitionAttempts; attempt++ {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			var prior Call
			body, err := tx.Get(ctx, s.callKey(call.RoomID)).Bytes()
			if err == nil {
				if err = json.Unmarshal(body, &prior); err != nil {
					return err
				}
			} else if !errors.Is(err, redis.Nil) {
				return err
			}
			for profile := range prior.OrdinaryMediaIdentities {
				if !profiles[profile] {
					return ErrInvalidState
				}
			}
			merged, err := mergeOrdinaryProjection(prior, call)
			if err != nil {
				return err
			}
			if prior.OrdinaryVersion >= call.OrdinaryVersion {
				result = prior
				return nil
			}
			owners := map[string]string{}
			for profile := range profiles {
				key := s.activeKey(profile)
				owner, err := tx.Get(ctx, key).Result()
				if err != nil && !errors.Is(err, redis.Nil) {
					return err
				}
				owners[key] = owner
				if merged.IsActiveForProfile(profile) && owner != "" && owner != call.RoomID {
					if authorize == nil {
						return ErrActiveCall
					}
					if err = authorize(ctx, owner); err != nil {
						return err
					}
				}
			}
			if call.IsGroupVoice() {
				key := s.activeChatKey(call.ChatID)
				owner, err := tx.Get(ctx, key).Result()
				if err != nil && !errors.Is(err, redis.Nil) {
					return err
				}
				owners[key] = owner
				if merged.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE && len(merged.States) > 0 && owner != "" && owner != call.RoomID {
					if authorize == nil {
						return ErrActiveCall
					}
					if err = authorize(ctx, owner); err != nil {
						return err
					}
				}
			}
			payload, err := json.Marshal(merged)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, s.callKey(call.RoomID), payload, callTTL(merged))
				for profile := range profiles {
					key := s.activeKey(profile)
					if merged.IsActiveForProfile(profile) {
						pipe.Set(ctx, key, call.RoomID, callTTL(merged))
					} else if owners[key] == call.RoomID {
						pipe.Del(ctx, key)
					}
				}
				if merged.IsGroupVoice() {
					key := s.activeChatKey(call.ChatID)
					if len(merged.States) > 0 && merged.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
						pipe.Set(ctx, key, call.RoomID, callTTL(merged))
					} else if owners[key] == call.RoomID {
						pipe.Del(ctx, key)
					}
				}
				return nil
			})
			if err == nil {
				result = merged
			}
			return err
		}, keys...)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		if err != nil {
			return Call{}, err
		}
		return result, nil
	}
	return Call{}, ErrMoveContention
}
