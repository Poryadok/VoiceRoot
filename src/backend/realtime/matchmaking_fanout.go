package main

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

func matchmakingFanouts(data []byte) ([]profileFanout, bool) {
	var e eventsv1.MatchmakingStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return nil, false
	}
	switch payload := e.GetPayload().(type) {
	case *eventsv1.MatchmakingStreamEvent_MatchFound:
		return matchFoundFanouts(payload.MatchFound)
	case *eventsv1.MatchmakingStreamEvent_MatchCompleted:
		return matchCompletedFanouts(payload.MatchCompleted)
	case *eventsv1.MatchmakingStreamEvent_SearchNudge:
		return searchNudgeFanouts(payload.SearchNudge)
	case *eventsv1.MatchmakingStreamEvent_MatchTimeout:
		return searchTimeoutFanouts(payload.MatchTimeout)
	default:
		return nil, false
	}
}

func matchFoundFanouts(ev *eventsv1.MatchFound) ([]profileFanout, bool) {
	if ev == nil || ev.GetMatchId() == "" {
		return nil, false
	}
	payload := map[string]string{
		"type":     "match_found",
		"match_id": ev.GetMatchId(),
		"game_id":  ev.GetGameId(),
		"mode":     ev.GetMode(),
		"region":   ev.GetRegion(),
	}
	if ev.ChatId != nil {
		payload["chat_id"] = ev.GetChatId()
	}
	if ev.VoiceRoomId != nil {
		payload["voice_room_id"] = ev.GetVoiceRoomId()
	}
	if len(ev.GetSessionIds()) > 0 {
		payload["session_id"] = ev.GetSessionIds()[0]
	}
	return fanoutsForProfiles(ev.GetProfileIds(), "match_found", payload)
}

func searchNudgeFanouts(ev *eventsv1.SearchNudge) ([]profileFanout, bool) {
	if ev == nil || ev.GetSessionId() == "" || ev.GetProfileId() == "" {
		return nil, false
	}
	payload := map[string]string{
		"type":       "search_nudge",
		"session_id": ev.GetSessionId(),
		"game_id":    ev.GetGameId(),
		"mode":       ev.GetMode(),
	}
	return fanoutsForProfiles([]string{ev.GetProfileId()}, "search_nudge", payload)
}

func searchTimeoutFanouts(ev *eventsv1.MatchTimeout) ([]profileFanout, bool) {
	if ev == nil || ev.GetSessionId() == "" || ev.GetProfileId() == "" {
		return nil, false
	}
	payload := map[string]string{
		"type":       "search_timeout",
		"session_id": ev.GetSessionId(),
		"game_id":    ev.GetGameId(),
		"mode":       ev.GetMode(),
	}
	return fanoutsForProfiles([]string{ev.GetProfileId()}, "search_timeout", payload)
}

func matchCompletedFanouts(ev *eventsv1.MatchCompleted) ([]profileFanout, bool) {
	if ev == nil || ev.GetMatchId() == "" {
		return nil, false
	}
	payload := map[string]string{
		"type":              "match_completed",
		"match_id":          ev.GetMatchId(),
		"duration_seconds":  fmt.Sprintf("%d", ev.GetDurationSeconds()),
	}
	return fanoutsForProfiles(ev.GetProfileIds(), "match_completed", payload)
}

func fanoutsForProfiles(profileIDs []string, op string, payload map[string]string) ([]profileFanout, bool) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	env := fanoutEnvelope{Op: op, D: b}
	var fanouts []profileFanout
	for _, profileID := range profileIDs {
		if profileID == "" {
			continue
		}
		fanouts = append(fanouts, profileFanout{ProfileID: profileID, Envelope: env})
	}
	if len(fanouts) == 0 {
		return nil, false
	}
	return fanouts, true
}

func dispatchMatchmakingStreamEvent(hub *wsHub, data []byte) {
	fanouts, ok := matchmakingFanouts(data)
	if !ok {
		return
	}
	for _, f := range fanouts {
		hub.broadcastToProfile(f.ProfileID, f.Envelope, nil, "")
	}
}
