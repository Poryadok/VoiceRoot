package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	callsv1 "voice.app/voice/calls/v1"
)

const expiredRingingScanCount = 128

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
	if call.IsGroupVoice() && !call.ManagedGameSession {
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

func (s *RedisCallStore) ListActiveSpaceVoiceCalls(ctx context.Context, spaceID string) ([]Call, error) {
	if strings.TrimSpace(spaceID) == "" {
		return nil, ErrInvalidState
	}
	roomIDs, err := s.client.SMembers(ctx, s.activeSpaceVoiceRoomsKey(spaceID)).Result()
	if err != nil {
		return nil, err
	}
	calls := make([]Call, 0, len(roomIDs))
	for _, roomID := range roomIDs {
		call, err := s.GetCall(ctx, roomID)
		if errors.Is(err, ErrNotFound) {
			_ = s.client.SRem(ctx, s.activeSpaceVoiceRoomsKey(spaceID), roomID).Err()
			continue
		}
		if err != nil {
			return nil, err
		}
		if call.IsVoiceRoom() && call.SpaceID == spaceID && call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
			calls = append(calls, call)
			continue
		}
		_ = s.client.SRem(ctx, s.activeSpaceVoiceRoomsKey(spaceID), roomID).Err()
	}
	return calls, nil
}

// ListActiveSpaceIDs enumerates only the durable Space room-index keys. Unlike
// ordinary session keys, these indexes have no expiry and survive Voice restart.
func (s *RedisCallStore) ListActiveSpaceIDs(ctx context.Context) ([]string, error) {
	prefix := s.prefix + "active_space_voice_rooms:"
	seen := make(map[string]struct{})
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, prefix+"*", expiredRingingScanCount).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			spaceID, ok := strings.CutPrefix(key, prefix)
			if !ok || spaceID == "" {
				continue
			}
			members, err := s.client.SCard(ctx, key).Result()
			if err != nil {
				return nil, err
			}
			if members > 0 {
				seen[spaceID] = struct{}{}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	spaces := make([]string, 0, len(seen))
	for id := range seen {
		spaces = append(spaces, id)
	}
	sort.Strings(spaces)
	return spaces, nil
}

func (s *RedisCallStore) RaiseSpaceMediaEpochFloor(ctx context.Context, spaceID string, kind SpaceMediaEpochKind, epoch uint64) (SpaceMediaEpochFloors, error) {
	if strings.TrimSpace(spaceID) == "" || epoch == 0 || (kind != SpaceAccessEpoch && kind != RolePolicyEpoch) {
		return SpaceMediaEpochFloors{}, ErrInvalidState
	}
	key := s.spaceMediaFloorKey(spaceID, kind)
	for attempt := 0; attempt < redisTransitionAttempts; attempt++ {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			current, err := tx.Get(ctx, key).Uint64()
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			if epoch <= current {
				return nil
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, strconv.FormatUint(epoch, 10), 0)
				return nil
			})
			return err
		}, key)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		if err != nil {
			return SpaceMediaEpochFloors{}, err
		}
		return s.GetSpaceMediaEpochFloors(ctx, spaceID)
	}
	return SpaceMediaEpochFloors{}, ErrMoveContention
}

func (s *RedisCallStore) GetSpaceMediaEpochFloors(ctx context.Context, spaceID string) (SpaceMediaEpochFloors, error) {
	var floors SpaceMediaEpochFloors
	for kind, target := range map[SpaceMediaEpochKind]*uint64{SpaceAccessEpoch: &floors.AccessEpoch, RolePolicyEpoch: &floors.PolicyEpoch} {
		value, err := s.client.Get(ctx, s.spaceMediaFloorKey(spaceID, kind)).Uint64()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return SpaceMediaEpochFloors{}, err
		}
		*target = value
	}
	return floors, nil
}

func (s *RedisCallStore) GetSpaceMediaEpochProgress(ctx context.Context, spaceID string) (SpaceMediaEpochProgress, error) {
	observed, err := s.GetSpaceMediaEpochFloors(ctx, spaceID)
	if err != nil {
		return SpaceMediaEpochProgress{}, err
	}
	var reconciled SpaceMediaEpochFloors
	for kind, target := range map[SpaceMediaEpochKind]*uint64{SpaceAccessEpoch: &reconciled.AccessEpoch, RolePolicyEpoch: &reconciled.PolicyEpoch} {
		value, err := s.client.Get(ctx, s.spaceMediaReconciledKey(spaceID, kind)).Uint64()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return SpaceMediaEpochProgress{}, err
		}
		*target = value
	}
	return SpaceMediaEpochProgress{Observed: observed, Reconciled: reconciled}, nil
}

func (s *RedisCallStore) MarkSpaceMediaEpochReconciled(ctx context.Context, spaceID string, kind SpaceMediaEpochKind, epoch uint64) (SpaceMediaEpochProgress, error) {
	if strings.TrimSpace(spaceID) == "" || epoch == 0 || (kind != SpaceAccessEpoch && kind != RolePolicyEpoch) {
		return SpaceMediaEpochProgress{}, ErrInvalidState
	}
	observedKey := s.spaceMediaFloorKey(spaceID, kind)
	reconciledKey := s.spaceMediaReconciledKey(spaceID, kind)
	for attempt := 0; attempt < redisTransitionAttempts; attempt++ {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			observed, err := tx.Get(ctx, observedKey).Uint64()
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			if epoch > observed {
				return ErrSpaceMediaStaleGrant
			}
			current, err := tx.Get(ctx, reconciledKey).Uint64()
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			if epoch <= current {
				return nil
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, reconciledKey, strconv.FormatUint(epoch, 10), 0)
				return nil
			})
			return err
		}, observedKey, reconciledKey)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		if err != nil {
			return SpaceMediaEpochProgress{}, err
		}
		return s.GetSpaceMediaEpochProgress(ctx, spaceID)
	}
	return SpaceMediaEpochProgress{}, ErrMoveContention
}

func (s *RedisCallStore) AdmitSpaceMediaParticipant(ctx context.Context, roomID string, participant SpaceMediaParticipant, maxParticipants int) (Call, error) {
	if participant.ProfileID == "" || participant.Identity == "" || participant.Generation == "" ||
		participant.Issued.SessionEpoch == 0 || participant.Issued.AccessEpoch == 0 || participant.Issued.PolicyEpoch == 0 ||
		!participant.Issued.CanJoin || !participant.Issued.CanSubscribe {
		return Call{}, ErrInvalidState
	}
	initial, err := s.GetCall(ctx, roomID)
	if err != nil {
		return Call{}, err
	}
	if !initial.IsVoiceRoom() || initial.SpaceID == "" {
		return Call{}, ErrInvalidState
	}
	accessFloorKey := s.spaceMediaFloorKey(initial.SpaceID, SpaceAccessEpoch)
	policyFloorKey := s.spaceMediaFloorKey(initial.SpaceID, RolePolicyEpoch)
	profileKey := s.activeKey(participant.ProfileID)
	for attempt := 0; attempt < redisTransitionAttempts; attempt++ {
		var result Call
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			call, err := s.getCallTx(ctx, tx, roomID)
			if err != nil {
				return err
			}
			if !call.IsVoiceRoom() || call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE || call.SpaceID == "" || call.SpaceID != initial.SpaceID {
				return ErrInvalidState
			}
			accessFloor, err := tx.Get(ctx, s.spaceMediaFloorKey(call.SpaceID, SpaceAccessEpoch)).Uint64()
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			policyFloor, err := tx.Get(ctx, s.spaceMediaFloorKey(call.SpaceID, RolePolicyEpoch)).Uint64()
			if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			if participant.Issued.AccessEpoch < accessFloor || participant.Issued.PolicyEpoch < policyFloor {
				return ErrSpaceMediaStaleGrant
			}
			if current, ok := call.SpaceMedia[participant.ProfileID]; ok {
				if current.Generation == participant.Generation && current.Identity == participant.Identity && !current.Revoking {
					result = call
					return nil
				}
				return ErrSpaceMediaTransition
			}
			if activeRoomID, err := tx.Get(ctx, profileKey).Result(); err == nil && activeRoomID != roomID {
				return ErrActiveCall
			} else if err != nil && !errors.Is(err, redis.Nil) {
				return err
			}
			if len(call.States) >= maxParticipants {
				return ErrRoomFull
			}
			if call.States == nil {
				call.States = map[string]ParticipantState{}
			}
			if call.SpaceMedia == nil {
				call.SpaceMedia = map[string]SpaceMediaParticipant{}
			}
			call.States[participant.ProfileID] = ParticipantState{ProfileID: participant.ProfileID}
			participant.Reconciled = participant.Issued
			call.SpaceMedia[participant.ProfileID] = participant
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
		}, s.callKey(roomID), profileKey, accessFloorKey, policyFloorKey)
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

func (s *RedisCallStore) BeginSpaceMediaRevocation(ctx context.Context, roomID, profileID, identity, generation string) (Call, bool, error) {
	return s.mutateSpaceMediaParticipant(ctx, roomID, profileID, identity, generation, func(_ context.Context, _ *redis.Tx, call *Call, participant *SpaceMediaParticipant) (bool, error) {
		if participant.Revoking {
			return true, nil
		}
		participant.Revoking = true
		return true, nil
	})
}

func (s *RedisCallStore) CompleteSpaceMediaRevocation(ctx context.Context, roomID, profileID, identity, generation string) (Call, bool, error) {
	return s.mutateSpaceMediaParticipant(ctx, roomID, profileID, identity, generation, func(_ context.Context, _ *redis.Tx, call *Call, participant *SpaceMediaParticipant) (bool, error) {
		if !participant.Revoking {
			return false, ErrSpaceMediaTransition
		}
		delete(call.SpaceMedia, profileID)
		delete(call.States, profileID)
		*call = removeScreenSharesForProfile(*call, profileID)
		if len(call.States) == 0 && !call.ManagedGameSession {
			call.Status, call.EndedAt = callsv1.CallStatus_CALL_STATUS_ENDED, time.Now().UTC()
		}
		return true, nil
	})
}

func (s *RedisCallStore) ReconcileSpaceMediaParticipant(ctx context.Context, roomID, profileID, identity, generation string, grant SpaceMediaGrant) (Call, bool, error) {
	return s.mutateSpaceMediaParticipant(ctx, roomID, profileID, identity, generation, func(ctx context.Context, tx *redis.Tx, call *Call, participant *SpaceMediaParticipant) (bool, error) {
		if participant.Revoking {
			return false, nil
		}
		accessFloor, err := tx.Get(ctx, s.spaceMediaFloorKey(call.SpaceID, SpaceAccessEpoch)).Uint64()
		if err != nil && !errors.Is(err, redis.Nil) {
			return false, err
		}
		policyFloor, err := tx.Get(ctx, s.spaceMediaFloorKey(call.SpaceID, RolePolicyEpoch)).Uint64()
		if err != nil && !errors.Is(err, redis.Nil) {
			return false, err
		}
		if grant.AccessEpoch < accessFloor || grant.PolicyEpoch < policyFloor {
			return false, ErrSpaceMediaStaleGrant
		}
		participant.Reconciled = grant
		call.SpaceMedia[profileID] = *participant
		return true, nil
	})
}

func (s *RedisCallStore) mutateSpaceMediaParticipant(ctx context.Context, roomID, profileID, identity, generation string, mutate func(context.Context, *redis.Tx, *Call, *SpaceMediaParticipant) (bool, error)) (Call, bool, error) {
	initial, err := s.GetCall(ctx, roomID)
	if err != nil {
		return Call{}, false, err
	}
	keys := []string{s.callKey(roomID), s.activeKey(profileID)}
	if initial.SpaceID != "" {
		keys = append(keys, s.spaceMediaFloorKey(initial.SpaceID, SpaceAccessEpoch), s.spaceMediaFloorKey(initial.SpaceID, RolePolicyEpoch))
	}
	for attempt := 0; attempt < redisTransitionAttempts; attempt++ {
		var result Call
		var changed bool
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			call, err := s.getCallTx(ctx, tx, roomID)
			if err != nil {
				return err
			}
			participant, ok := call.SpaceMedia[profileID]
			if !ok || participant.Identity != identity || participant.Generation != generation {
				result = call
				return nil
			}
			changed, err = mutate(ctx, tx, &call, &participant)
			if err != nil || !changed {
				result = call
				return err
			}
			if _, exists := call.SpaceMedia[profileID]; exists {
				call.SpaceMedia[profileID] = participant
			}
			payload, err := json.Marshal(call)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				s.writeCallProjection(pipe, ctx, call, payload)
				if _, exists := call.SpaceMedia[profileID]; !exists {
					pipe.Del(ctx, s.activeKey(profileID))
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
			return Call{}, false, err
		}
		return result, changed, nil
	}
	return Call{}, false, ErrMoveContention
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
				delete(call.SpaceMedia, profileID)
				call = removeScreenSharesForProfile(call, profileID)
				if len(call.States) == 0 && !call.ManagedGameSession {
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
				pipe.Set(ctx, s.callKey(call.RoomID), payload, callTTL(call))
				if call.IsVoiceRoom() {
					if call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
						pipe.Set(ctx, s.activeVoiceRoomKey(call.VoiceRoomID), call.RoomID, callTTL(call))
						if call.SpaceID != "" {
							pipe.SAdd(ctx, s.activeSpaceVoiceRoomsKey(call.SpaceID), call.RoomID)
						}
					} else {
						pipe.Del(ctx, s.activeVoiceRoomKey(call.VoiceRoomID))
						if call.SpaceID != "" {
							pipe.SRem(ctx, s.activeSpaceVoiceRoomsKey(call.SpaceID), call.RoomID)
						}
					}
				}
				if remove {
					pipe.Del(ctx, s.activeKey(profileID))
				} else {
					pipe.Set(ctx, s.activeKey(profileID), call.RoomID, callTTL(call))
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
				pipe.Set(ctx, s.callKey(source.RoomID), sourceJSON, callTTL(source))
				pipe.Set(ctx, s.callKey(destination.RoomID), destinationJSON, callTTL(destination))
				if source.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
					pipe.Set(ctx, s.activeVoiceRoomKey(source.VoiceRoomID), source.RoomID, callTTL(source))
				} else {
					pipe.Del(ctx, s.activeVoiceRoomKey(source.VoiceRoomID))
				}
				if source.SpaceID != "" {
					if source.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
						pipe.SAdd(ctx, s.activeSpaceVoiceRoomsKey(source.SpaceID), source.RoomID)
					} else {
						pipe.SRem(ctx, s.activeSpaceVoiceRoomsKey(source.SpaceID), source.RoomID)
					}
				}
				pipe.Set(ctx, s.activeVoiceRoomKey(destination.VoiceRoomID), destination.RoomID, callTTL(destination))
				if destination.SpaceID != "" {
					pipe.SAdd(ctx, s.activeSpaceVoiceRoomsKey(destination.SpaceID), destination.RoomID)
				}
				pipe.Set(ctx, s.activeKey(req.ParticipantProfileID), destination.RoomID, callTTL(destination))
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
	pipe.Set(ctx, s.callKey(call.RoomID), payload, callTTL(call))
	if call.IsGroupVoice() && !call.ManagedGameSession && call.ChatID != "" {
		if call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
			pipe.Set(ctx, s.activeChatKey(call.ChatID), call.RoomID, 24*time.Hour)
		} else {
			pipe.Del(ctx, s.activeChatKey(call.ChatID))
		}
	}
	if call.IsVoiceRoom() && call.VoiceRoomID != "" {
		if call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
			pipe.Set(ctx, s.activeVoiceRoomKey(call.VoiceRoomID), call.RoomID, callTTL(call))
			if call.SpaceID != "" {
				pipe.SAdd(ctx, s.activeSpaceVoiceRoomsKey(call.SpaceID), call.RoomID)
			}
		} else {
			pipe.Del(ctx, s.activeVoiceRoomKey(call.VoiceRoomID))
			if call.SpaceID != "" {
				pipe.SRem(ctx, s.activeSpaceVoiceRoomsKey(call.SpaceID), call.RoomID)
			}
		}
	}
	if call.Status == callsv1.CallStatus_CALL_STATUS_RINGING || call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
		for _, profileID := range call.ProfileIDs() {
			if profileID != "" {
				pipe.Set(ctx, s.activeKey(profileID), call.RoomID, callTTL(call))
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
	var out []Call
	callKeyPrefix := s.callKey("")
	seen := make(map[string]struct{})
	var cursor uint64
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, callKeyPrefix+"*", expiredRingingScanCount).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			roomID, ok := strings.CutPrefix(key, callKeyPrefix)
			if !ok || roomID == "" {
				continue
			}
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}

			b, err := s.client.Get(ctx, key).Bytes()
			if err != nil {
				continue
			}
			var call Call
			if err := json.Unmarshal(b, &call); err != nil || call.RoomID != roomID {
				continue
			}
			if call.Status == callsv1.CallStatus_CALL_STATUS_RINGING && !call.ExpiresAt.IsZero() && now.After(call.ExpiresAt) {
				out = append(out, call)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
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

func (s *RedisCallStore) activeSpaceVoiceRoomsKey(spaceID string) string {
	return s.prefix + "active_space_voice_rooms:" + spaceID
}

func (s *RedisCallStore) spaceMediaFloorKey(spaceID string, kind SpaceMediaEpochKind) string {
	name := "access"
	if kind == RolePolicyEpoch {
		name = "role_policy"
	}
	return s.prefix + "space_media_floor:" + spaceID + ":" + name
}

func (s *RedisCallStore) spaceMediaReconciledKey(spaceID string, kind SpaceMediaEpochKind) string {
	name := "access"
	if kind == RolePolicyEpoch {
		name = "role_policy"
	}
	return s.prefix + "space_media_reconciled:" + spaceID + ":" + name
}

func callTTL(call Call) time.Duration {
	if call.IsVoiceRoom() && call.SpaceID != "" {
		return 0
	}
	return 24 * time.Hour
}

func (s *RedisCallStore) moveOperationKey(actorProfileID, operationID string) string {
	return s.prefix + "voice_move_operation:" + actorProfileID + ":" + operationID
}
