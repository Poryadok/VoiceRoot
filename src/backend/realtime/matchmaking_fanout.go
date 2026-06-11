package main

import (
	"encoding/json"

	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

func matchmakingFanouts(data []byte) ([]profileFanout, bool) {
	var e eventsv1.MatchmakingStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return nil, false
	}
	mf, ok := e.GetPayload().(*eventsv1.MatchmakingStreamEvent_MatchFound)
	if !ok || mf.MatchFound == nil || mf.MatchFound.GetMatchId() == "" {
		return nil, false
	}
	ev := mf.MatchFound
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
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	env := fanoutEnvelope{Op: "match_found", D: b}
	var fanouts []profileFanout
	for _, profileID := range ev.GetProfileIds() {
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
