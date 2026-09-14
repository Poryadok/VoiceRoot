package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	callsv1 "voice.app/voice/calls/v1"
)

type RedisCallStore struct {
	client        *redis.Client
	prefix        string
	moveTestHooks redisMoveTestHooks
}

// redisMoveTestHooks is intentionally instance-scoped and nil by default.
// Same-package integration tests use it only to make a second real Redis
// client invalidate WATCH immediately before EXEC.
type redisMoveTestHooks struct {
	beforeMoveExec func(ctx context.Context, attempt int, watchedKeys []string) error
}

const redisTransitionAttempts = 4

type redisVoiceRoomMoveLedger struct {
	Request VoiceRoomMoveRequest `json:"request"`
	Result  VoiceRoomMoveResult  `json:"result"`
}

func NewRedisCallStore(client *redis.Client, prefix string) *RedisCallStore {
	if prefix == "" {
		prefix = "voice:"
	}
	return &RedisCallStore{client: client, prefix: prefix}
}

func (s *RedisCallStore) CreateCall(ctx context.Context, call Call) (Call, error) {
	if call.States == nil {
		call.States = defaultStates(call)
	}
	keys := []string{s.callKey(call.RoomID)}
	if call.IsVoiceRoom() {
		keys = append(keys, s.activeVoiceRoomKey(call.VoiceRoomID))
	}
	if call.IsGroupVoice() {
		keys = append(keys, s.activeChatKey(call.ChatID))
	}
	for _, profileID := range call.ProfileIDs() {
		if profileID != "" {
			keys = append(keys, s.activeKey(profileID))
		}
	}
	for attempt := 0; attempt < redisTransitionAttempts; attempt++ {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			if exists, err := tx.Exists(ctx, s.callKey(call.RoomID)).Result(); err != nil {
				return err
			} else if exists != 0 {
				return ErrInvalidState
			}
			if call.IsVoiceRoom() {
				if _, err := tx.Get(ctx, s.activeVoiceRoomKey(call.VoiceRoomID)).Result(); err == nil {
					return ErrActiveCall
				} else if !errors.Is(err, redis.Nil) {
					return err
				}
			}
			for _, profileID := range call.ProfileIDs() {
				if profileID == "" {
					continue
				}
				if _, err := tx.Get(ctx, s.activeKey(profileID)).Result(); err == nil {
					return ErrActiveCall
				} else if !errors.Is(err, redis.Nil) {
					return err
				}
			}
			payload, err := json.Marshal(call)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { s.writeCallProjection(pipe, ctx, call, payload); return nil })
			return err
		}, keys...)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		if err != nil {
			return Call{}, err
		}
		return call, nil
	}
	return Call{}, ErrMoveContention
}

func (s *RedisCallStore) GetCall(ctx context.Context, roomID string) (Call, error) {
	b, err := s.client.Get(ctx, s.callKey(roomID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Call{}, ErrNotFound
	}
	if err != nil {
		return Call{}, err
	}
	var call Call
	if err := json.Unmarshal(b, &call); err != nil {
		return Call{}, err
	}
	return call, nil
}

func (s *RedisCallStore) GetActiveGroupCallForChat(ctx context.Context, chatID string) (Call, error) {
	roomID, err := s.client.Get(ctx, s.activeChatKey(chatID)).Result()
	if errors.Is(err, redis.Nil) {
		return Call{}, ErrNotFound
	}
	if err != nil {
		return Call{}, err
	}
	call, err := s.GetCall(ctx, roomID)
	if err != nil {
		return Call{}, err
	}
	if !call.IsGroupVoice() || call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
		_ = s.deleteIfValue(ctx, s.activeChatKey(chatID), roomID)
		return Call{}, ErrNotFound
	}
	return call, nil
}

func (s *RedisCallStore) GetActiveCall(ctx context.Context, profileID string) (Call, error) {
	roomID, err := s.client.Get(ctx, s.activeKey(profileID)).Result()
	if errors.Is(err, redis.Nil) {
		return Call{}, ErrNotFound
	}
	if err != nil {
		return Call{}, err
	}
	call, err := s.GetCall(ctx, roomID)
	if err != nil {
		return Call{}, err
	}
	if !call.IsActiveForProfile(profileID) {
		_ = s.deleteIfValue(ctx, s.activeKey(profileID), roomID)
		return Call{}, ErrNotFound
	}
	return call, nil
}

func (s *RedisCallStore) GetCallByVoiceRoomID(ctx context.Context, voiceRoomID string) (Call, error) {
	roomID, err := s.client.Get(ctx, s.activeVoiceRoomKey(voiceRoomID)).Result()
	if errors.Is(err, redis.Nil) {
		return Call{}, ErrNotFound
	}
	if err != nil {
		return Call{}, err
	}
	call, err := s.GetCall(ctx, roomID)
	if err != nil {
		return Call{}, err
	}
	if !call.IsVoiceRoom() || call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
		_ = s.deleteIfValue(ctx, s.activeVoiceRoomKey(voiceRoomID), roomID)
		return Call{}, ErrNotFound
	}
	return call, nil
}

func (s *RedisCallStore) RemoveParticipant(ctx context.Context, roomID, profileID string) (Call, error) {
	return s.transitionParticipant(ctx, roomID, profileID, 0, true)
}

func (s *RedisCallStore) AddParticipant(ctx context.Context, roomID, profileID string, maxParticipants int) (Call, error) {
	return s.transitionParticipant(ctx, roomID, profileID, maxParticipants, false)
}

// transitionParticipant is the shared CAS boundary for public join/leave and
// the move path.  It watches the roster document and the subject session key,
// so neither a stale add nor a stale remove can restore a move's old roster.
func (s *RedisCallStore) transitionParticipant(ctx context.Context, roomID, profileID string, maxParticipants int, remove bool) (Call, error) {
	keys := []string{s.callKey(roomID), s.activeKey(profileID)}
	for attempt := 0; attempt < redisTransitionAttempts; attempt++ {
		var result Call
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			call, err := s.getCallTx(ctx, tx, roomID)
			if err != nil {
				return err
			}
			if !call.isOpenVoiceSession() || call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
				return ErrInvalidState
			}
			if remove {
				if !call.IsParticipant(profileID) {
					return ErrNotParticipant
				}
				delete(call.States, profileID)
				call = removeScreenSharesForProfile(call, profileID)
				if len(call.States) == 0 {
					call.Status, call.EndedAt = callsv1.CallStatus_CALL_STATUS_ENDED, time.Now().UTC()
				}
			} else {
				if call.IsParticipant(profileID) {
					result = call
					return nil
				}
				activeRoomID, activeErr := tx.Get(ctx, s.activeKey(profileID)).Result()
				if activeErr == nil && activeRoomID != "" && activeRoomID != roomID {
					return ErrActiveCall
				}
				if activeErr != nil && !errors.Is(activeErr, redis.Nil) {
					return activeErr
				}
				if len(call.States) >= maxParticipants {
					return ErrRoomFull
				}
				if call.States == nil {
					call.States = map[string]ParticipantState{}
				}
				call.States[profileID] = ParticipantState{ProfileID: profileID, IsVideoOn: call.MediaKind == callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO}
			}
			payload, err := json.Marshal(call)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, s.callKey(call.RoomID), payload, 24*time.Hour)
				if call.IsVoiceRoom() {
					if call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
						pipe.Set(ctx, s.activeVoiceRoomKey(call.VoiceRoomID), call.RoomID, 24*time.Hour)
					} else {
						pipe.Del(ctx, s.activeVoiceRoomKey(call.VoiceRoomID))
					}
				}
				if remove {
					pipe.Del(ctx, s.activeKey(profileID))
				} else {
					pipe.Set(ctx, s.activeKey(profileID), call.RoomID, 24*time.Hour)
				}
				return nil
			})
			if err == nil {
				result = call
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

func (s *RedisCallStore) FindVoiceRoomMove(ctx context.Context, req VoiceRoomMoveRequest) (VoiceRoomMoveResult, bool, error) {
	b, err := s.client.Get(ctx, s.moveOperationKey(req.ActorProfileID, req.OperationID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return VoiceRoomMoveResult{}, false, nil
	}
	if err != nil {
		return VoiceRoomMoveResult{}, false, err
	}
	var prior redisVoiceRoomMoveLedger
	if err := json.Unmarshal(b, &prior); err != nil {
		return VoiceRoomMoveResult{}, false, err
	}
	if !sameVoiceRoomMoveRequest(prior.Request, req) {
		return VoiceRoomMoveResult{}, false, ErrOperationConflict
	}
	prior.Result.Replayed = true
	return prior.Result, true, nil
}

// MoveVoiceRoomParticipant uses WATCH/EXEC over both room projections, the
// subject session and the operation ledger.  A concurrent join/leave retries
// instead of exposing an intermediate roster where the participant is absent
// from both rooms.
func (s *RedisCallStore) MoveVoiceRoomParticipant(ctx context.Context, req VoiceRoomMoveRequest) (VoiceRoomMoveResult, error) {
	var result VoiceRoomMoveResult
	for attempt := 0; attempt < redisTransitionAttempts; attempt++ {
		keys := []string{s.moveOperationKey(req.ActorProfileID, req.OperationID), s.activeVoiceRoomKey(req.FromVoiceRoomID), s.activeVoiceRoomKey(req.ToVoiceRoomID), s.activeKey(req.ParticipantProfileID)}
		for _, voiceRoomID := range []string{req.FromVoiceRoomID, req.ToVoiceRoomID} {
			roomID, err := s.client.Get(ctx, s.activeVoiceRoomKey(voiceRoomID)).Result()
			if err == nil && roomID != "" {
				keys = append(keys, s.callKey(roomID))
			} else if err != nil && !errors.Is(err, redis.Nil) {
				return VoiceRoomMoveResult{}, err
			}
		}
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			ledger, err := tx.Get(ctx, s.moveOperationKey(req.ActorProfileID, req.OperationID)).Bytes()
			if err == nil {
				var prior redisVoiceRoomMoveLedger
				if err := json.Unmarshal(ledger, &prior); err != nil {
					return err
				}
				if !sameVoiceRoomMoveRequest(prior.Request, req) {
					return ErrOperationConflict
				}
				prior.Result.Replayed = true
				result = prior.Result
				return nil
			}
			if !errors.Is(err, redis.Nil) {
				return err
			}
			source, err := s.getVoiceRoomCallTx(ctx, tx, req.FromVoiceRoomID)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return ErrInvalidState
				}
				return err
			}
			if !source.IsParticipant(req.ParticipantProfileID) || source.SpaceID != req.SpaceID {
				return ErrInvalidState
			}
			destination, err := s.getVoiceRoomCallTx(ctx, tx, req.ToVoiceRoomID)
			destinationExists := err == nil
			if err != nil && !errors.Is(err, ErrNotFound) {
				return err
			}
			if destinationExists {
				if destination.SpaceID != req.SpaceID || !destination.IsVoiceRoom() || destination.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
					return ErrInvalidState
				}
				if !destination.IsParticipant(req.ParticipantProfileID) && len(destination.States) >= req.MaxParticipants {
					return ErrRoomFull
				}
			} else {
				destination = Call{RoomID: req.DestinationRoomID, LivekitRoomName: "voice-room-" + req.ToVoiceRoomID, VoiceRoomID: req.ToVoiceRoomID, SpaceID: req.SpaceID, SessionKind: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, InitiatorProfileID: req.ParticipantProfileID, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE, States: map[string]ParticipantState{req.ParticipantProfileID: {ProfileID: req.ParticipantProfileID}}}
			}
			delete(source.States, req.ParticipantProfileID)
			source = removeScreenSharesForProfile(source, req.ParticipantProfileID)
			if len(source.States) == 0 {
				source.Status, source.EndedAt = callsv1.CallStatus_CALL_STATUS_ENDED, req.Now
			}
			if destinationExists && !destination.IsParticipant(req.ParticipantProfileID) {
				destination.States[req.ParticipantProfileID] = ParticipantState{ProfileID: req.ParticipantProfileID}
			}
			result = VoiceRoomMoveResult{Source: source, Destination: destination}
			ledgerJSON, err := json.Marshal(redisVoiceRoomMoveLedger{Request: req, Result: result})
			if err != nil {
				return err
			}
			sourceJSON, err := json.Marshal(source)
			if err != nil {
				return err
			}
			destinationJSON, err := json.Marshal(destination)
			if err != nil {
				return err
			}
			if hook := s.moveTestHooks.beforeMoveExec; hook != nil {
				if err := hook(ctx, attempt, append([]string(nil), keys...)); err != nil {
					return err
				}
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, s.callKey(source.RoomID), sourceJSON, 24*time.Hour)
				pipe.Set(ctx, s.callKey(destination.RoomID), destinationJSON, 24*time.Hour)
				if source.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
					pipe.Set(ctx, s.activeVoiceRoomKey(source.VoiceRoomID), source.RoomID, 24*time.Hour)
				} else {
					pipe.Del(ctx, s.activeVoiceRoomKey(source.VoiceRoomID))
				}
				pipe.Set(ctx, s.activeVoiceRoomKey(destination.VoiceRoomID), destination.RoomID, 24*time.Hour)
				pipe.Set(ctx, s.activeKey(req.ParticipantProfileID), destination.RoomID, 24*time.Hour)
				pipe.Set(ctx, s.moveOperationKey(req.ActorProfileID, req.OperationID), ledgerJSON, 24*time.Hour)
				return nil
			})
			return err
		}, keys...)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		if err != nil {
			return VoiceRoomMoveResult{}, err
		}
		return result, nil
	}
	return VoiceRoomMoveResult{}, ErrMoveContention
}

func (s *RedisCallStore) getCallTx(ctx context.Context, tx *redis.Tx, roomID string) (Call, error) {
	b, err := tx.Get(ctx, s.callKey(roomID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Call{}, ErrNotFound
	}
	if err != nil {
		return Call{}, err
	}
	var call Call
	if err := json.Unmarshal(b, &call); err != nil {
		return Call{}, err
	}
	return call, nil
}

// deleteIfValue repairs a stale projection without deleting a newer writer's
// replacement index between the read validation and the cleanup.
func (s *RedisCallStore) deleteIfValue(ctx context.Context, key, expected string) error {
	return s.client.Watch(ctx, func(tx *redis.Tx) error {
		actual, err := tx.Get(ctx, key).Result()
		if errors.Is(err, redis.Nil) {
			return nil
		}
		if err != nil {
			return err
		}
		if actual != expected {
			return nil
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { pipe.Del(ctx, key); return nil })
		return err
	}, key)
}

// mutateCall serializes every call-document writer with roster moves.  The
// watched call key fences a stale state/screen/status writer; profile and room
// indexes are included because the commit updates the full projection.
func (s *RedisCallStore) mutateCall(ctx context.Context, roomID string, mutate func(*Call) error) (Call, error) {
	for attempt := 0; attempt < redisTransitionAttempts; attempt++ {
		before, err := s.GetCall(ctx, roomID)
		if err != nil {
			return Call{}, err
		}
		keys := []string{s.callKey(roomID)}
		if before.IsVoiceRoom() {
			keys = append(keys, s.activeVoiceRoomKey(before.VoiceRoomID))
		}
		if before.IsGroupVoice() {
			keys = append(keys, s.activeChatKey(before.ChatID))
		}
		for _, profileID := range before.ProfileIDs() {
			if profileID != "" {
				keys = append(keys, s.activeKey(profileID))
			}
		}
		var result Call
		err = s.client.Watch(ctx, func(tx *redis.Tx) error {
			call, err := s.getCallTx(ctx, tx, roomID)
			if err != nil {
				return err
			}
			if err := mutate(&call); err != nil {
				return err
			}
			payload, err := json.Marshal(call)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				s.writeCallProjection(pipe, ctx, call, payload)
				return nil
			})
			if err == nil {
				result = call
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

func (s *RedisCallStore) writeCallProjection(pipe redis.Pipeliner, ctx context.Context, call Call, payload []byte) {
	pipe.Set(ctx, s.callKey(call.RoomID), payload, 24*time.Hour)
	if call.IsGroupVoice() && call.ChatID != "" {
		if call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
			pipe.Set(ctx, s.activeChatKey(call.ChatID), call.RoomID, 24*time.Hour)
		} else {
			pipe.Del(ctx, s.activeChatKey(call.ChatID))
		}
	}
	if call.IsVoiceRoom() && call.VoiceRoomID != "" {
		if call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
			pipe.Set(ctx, s.activeVoiceRoomKey(call.VoiceRoomID), call.RoomID, 24*time.Hour)
		} else {
			pipe.Del(ctx, s.activeVoiceRoomKey(call.VoiceRoomID))
		}
	}
	if call.Status == callsv1.CallStatus_CALL_STATUS_RINGING || call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
		for _, profileID := range call.ProfileIDs() {
			if profileID != "" {
				pipe.Set(ctx, s.activeKey(profileID), call.RoomID, 24*time.Hour)
			}
		}
	} else {
		for _, profileID := range call.ProfileIDs() {
			if profileID != "" {
				pipe.Del(ctx, s.activeKey(profileID))
			}
		}
	}
}

func (s *RedisCallStore) getVoiceRoomCallTx(ctx context.Context, tx *redis.Tx, voiceRoomID string) (Call, error) {
	roomID, err := tx.Get(ctx, s.activeVoiceRoomKey(voiceRoomID)).Result()
	if errors.Is(err, redis.Nil) {
		return Call{}, ErrNotFound
	}
	if err != nil {
		return Call{}, err
	}
	b, err := tx.Get(ctx, s.callKey(roomID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Call{}, ErrNotFound
	}
	if err != nil {
		return Call{}, err
	}
	var call Call
	if err := json.Unmarshal(b, &call); err != nil {
		return Call{}, err
	}
	if !call.IsVoiceRoom() || call.VoiceRoomID != voiceRoomID || call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
		return Call{}, ErrNotFound
	}
	return call, nil
}

func (s *RedisCallStore) SetStatus(ctx context.Context, roomID string, status callsv1.CallStatus, endedAt time.Time) (Call, error) {
	return s.mutateCall(ctx, roomID, func(call *Call) error {
		call.Status, call.EndedAt = status, endedAt
		return nil
	})
}

func (s *RedisCallStore) UpdateVoiceState(ctx context.Context, roomID, profileID string, patch VoiceStatePatch) (Call, ParticipantState, error) {
	var state ParticipantState
	call, err := s.mutateCall(ctx, roomID, func(call *Call) error {
		if !call.IsParticipant(profileID) {
			return ErrNotParticipant
		}
		if call.States == nil {
			call.States = defaultStates(*call)
		}
		state = call.States[profileID]
		if patch.IsMuted != nil {
			state.IsMuted = *patch.IsMuted
		}
		if patch.IsDeafened != nil {
			state.IsDeafened = *patch.IsDeafened
		}
		if patch.IsVideoOn != nil {
			state.IsVideoOn = *patch.IsVideoOn
		}
		if patch.IsCommander != nil {
			state.IsCommander = *patch.IsCommander
			if !*patch.IsCommander {
				state.IsBroadcasting = false
			}
		}
		if patch.HandRaised != nil {
			state.HandRaised = *patch.HandRaised
		}
		if patch.HasFloor != nil {
			state.HasFloor = *patch.HasFloor
		}
		if patch.IsBroadcasting != nil {
			state.IsBroadcasting = *patch.IsBroadcasting
		}
		call.States[profileID] = state
		return nil
	})
	if err != nil {
		return Call{}, ParticipantState{}, err
	}
	return call, state, nil
}

func (s *RedisCallStore) ListExpiredRinging(ctx context.Context, now time.Time) ([]Call, error) {
	keys, err := s.client.Keys(ctx, s.prefix+"call:*").Result()
	if err != nil {
		return nil, err
	}
	var out []Call
	for _, key := range keys {
		b, err := s.client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}
		var call Call
		if err := json.Unmarshal(b, &call); err != nil {
			continue
		}
		if call.Status == callsv1.CallStatus_CALL_STATUS_RINGING && !call.ExpiresAt.IsZero() && now.After(call.ExpiresAt) {
			out = append(out, call)
		}
	}
	return out, nil
}

func (s *RedisCallStore) StartScreenShare(ctx context.Context, roomID, profileID, streamID string) (Call, ScreenShareEntry, error) {
	var entry ScreenShareEntry
	call, err := s.mutateCall(ctx, roomID, func(call *Call) error {
		if !call.IsParticipant(profileID) {
			return ErrNotParticipant
		}
		if call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
			return ErrInvalidState
		}
		var err error
		*call, entry, err = startScreenShareLocked(*call, profileID, streamID)
		return err
	})
	if err != nil {
		return Call{}, ScreenShareEntry{}, err
	}
	return call, entry, nil
}

func (s *RedisCallStore) StopScreenShare(ctx context.Context, roomID, profileID, streamID string) (Call, error) {
	return s.mutateCall(ctx, roomID, func(call *Call) error {
		if !call.IsParticipant(profileID) {
			return ErrNotParticipant
		}
		updated, err := stopScreenShareLocked(*call, profileID, streamID)
		if err == nil {
			*call = updated
		}
		return err
	})
}

func (s *RedisCallStore) StopScreenSharesForProfile(ctx context.Context, roomID, profileID string) (Call, error) {
	return s.mutateCall(ctx, roomID, func(call *Call) error {
		*call = removeScreenSharesForProfile(*call, profileID)
		return nil
	})
}

func (s *RedisCallStore) callKey(roomID string) string {
	return s.prefix + "call:" + roomID
}

func (s *RedisCallStore) activeKey(profileID string) string {
	return s.prefix + "session:" + profileID
}

func (s *RedisCallStore) activeChatKey(chatID string) string {
	return s.prefix + "active_chat:" + chatID
}

func (s *RedisCallStore) activeVoiceRoomKey(voiceRoomID string) string {
	return s.prefix + "active_voice_room:" + voiceRoomID
}

func (s *RedisCallStore) moveOperationKey(actorProfileID, operationID string) string {
	return s.prefix + "voice_move_operation:" + actorProfileID + ":" + operationID
}
