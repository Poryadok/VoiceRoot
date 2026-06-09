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
	client *redis.Client
	prefix string
}

func NewRedisCallStore(client *redis.Client, prefix string) *RedisCallStore {
	if prefix == "" {
		prefix = "voice:"
	}
	return &RedisCallStore{client: client, prefix: prefix}
}

func (s *RedisCallStore) CreateCall(ctx context.Context, call Call) (Call, error) {
	active, err := s.GetActiveCall(ctx, call.InitiatorProfileID)
	if err == nil && active.RoomID != "" {
		return Call{}, ErrActiveCall
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Call{}, err
	}
	if !call.IsGroupVoice() && call.CalleeProfileID != "" {
		active, err = s.GetActiveCall(ctx, call.CalleeProfileID)
		if err == nil && active.RoomID != "" {
			return Call{}, ErrActiveCall
		}
		if err != nil && !errors.Is(err, ErrNotFound) {
			return Call{}, err
		}
	}
	if call.States == nil {
		call.States = defaultStates(call)
	}
	if err := s.save(ctx, call); err != nil {
		return Call{}, err
	}
	return call, nil
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
		_ = s.client.Del(ctx, s.activeChatKey(chatID)).Err()
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
		_ = s.client.Del(ctx, s.activeKey(profileID)).Err()
		return Call{}, ErrNotFound
	}
	return call, nil
}

func (s *RedisCallStore) AddParticipant(ctx context.Context, roomID, profileID string, maxParticipants int) (Call, error) {
	call, err := s.GetCall(ctx, roomID)
	if err != nil {
		return Call{}, err
	}
	if !call.IsGroupVoice() || call.Status != callsv1.CallStatus_CALL_STATUS_ACTIVE {
		return Call{}, ErrInvalidState
	}
	if call.IsParticipant(profileID) {
		return call, nil
	}
	active, err := s.GetActiveCall(ctx, profileID)
	if err == nil && active.RoomID != "" && active.RoomID != roomID {
		return Call{}, ErrActiveCall
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Call{}, err
	}
	if len(call.States) >= maxParticipants {
		return Call{}, ErrRoomFull
	}
	if call.States == nil {
		call.States = map[string]ParticipantState{}
	}
	call.States[profileID] = ParticipantState{
		ProfileID: profileID,
		IsVideoOn: call.MediaKind == callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO,
	}
	if err := s.save(ctx, call); err != nil {
		return Call{}, err
	}
	return call, nil
}

func (s *RedisCallStore) SetStatus(ctx context.Context, roomID string, status callsv1.CallStatus, endedAt time.Time) (Call, error) {
	call, err := s.GetCall(ctx, roomID)
	if err != nil {
		return Call{}, err
	}
	call.Status = status
	call.EndedAt = endedAt
	if err := s.save(ctx, call); err != nil {
		return Call{}, err
	}
	return call, nil
}

func (s *RedisCallStore) UpdateVoiceState(ctx context.Context, roomID, profileID string, patch VoiceStatePatch) (Call, ParticipantState, error) {
	call, err := s.GetCall(ctx, roomID)
	if err != nil {
		return Call{}, ParticipantState{}, err
	}
	if !call.IsParticipant(profileID) {
		return Call{}, ParticipantState{}, ErrNotParticipant
	}
	if call.States == nil {
		call.States = defaultStates(call)
	}
	state := call.States[profileID]
	if patch.IsMuted != nil {
		state.IsMuted = *patch.IsMuted
	}
	if patch.IsDeafened != nil {
		state.IsDeafened = *patch.IsDeafened
	}
	if patch.IsVideoOn != nil {
		state.IsVideoOn = *patch.IsVideoOn
	}
	call.States[profileID] = state
	if err := s.save(ctx, call); err != nil {
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

func (s *RedisCallStore) save(ctx context.Context, call Call) error {
	b, err := json.Marshal(call)
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.callKey(call.RoomID), b, 24*time.Hour)
	if call.IsGroupVoice() {
		if call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE && call.ChatID != "" {
			pipe.Set(ctx, s.activeChatKey(call.ChatID), call.RoomID, 24*time.Hour)
		} else if call.ChatID != "" {
			pipe.Del(ctx, s.activeChatKey(call.ChatID))
		}
	}
	activeProfiles := call.ProfileIDs()
	if call.Status == callsv1.CallStatus_CALL_STATUS_RINGING || call.Status == callsv1.CallStatus_CALL_STATUS_ACTIVE {
		for _, profileID := range activeProfiles {
			if profileID != "" {
				pipe.Set(ctx, s.activeKey(profileID), call.RoomID, 24*time.Hour)
			}
		}
	} else {
		keys := make([]string, 0, len(activeProfiles))
		for _, profileID := range activeProfiles {
			if profileID != "" {
				keys = append(keys, s.activeKey(profileID))
			}
		}
		if len(keys) > 0 {
			pipe.Del(ctx, keys...)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
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
