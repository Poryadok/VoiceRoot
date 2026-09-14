package main

import (
	"net/http"
	"strings"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

// serveSpacesVoiceRooms keeps the Space identifier path-bound.  Voice receives
// the delegated principal only through Gateway metadata; neither request body
// nor path can assert the actor.
func (t *transcoder) serveSpacesVoiceRooms(w http.ResponseWriter, r *http.Request, rest string) bool {
	if t == nil || t.clients.voice == nil {
		return false
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 4 || parts[1] != "voice-rooms" || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[2]) == "" {
		return false
	}
	spaceID, fromVoiceRoomID := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[2])
	ctx := withGRPCMetadata(r.Context(), r)
	if r.Method == http.MethodPost && len(parts) == 4 && parts[3] == "move" {
		req, err := readVoiceRoomMoveJSON(r)
		if err != nil {
			writeVoiceRoomMoveError(w, err)
			return true
		}
		resp, err := t.clients.voice.MoveToVoiceRoom(ctx, &callsv1.MoveToVoiceRoomRequest{
			FromVoiceRoomId: fromVoiceRoomID,
			ToVoiceRoomId:   req.GetToVoiceRoomId(),
			Space:           &spacev1.SpaceRef{Id: spaceID},
			OperationId:     req.GetOperationId(),
		})
		if err != nil {
			writeVoiceRoomMoveError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	}
	if r.Method == http.MethodPost && len(parts) == 6 && parts[3] == "participants" && strings.TrimSpace(parts[4]) != "" && parts[5] == "move" {
		req, err := readVoiceRoomMoveJSON(r)
		if err != nil {
			writeVoiceRoomMoveError(w, err)
			return true
		}
		req.FromVoiceRoomId = fromVoiceRoomID
		req.ParticipantProfileId = strings.TrimSpace(parts[4])
		req.Space = &spacev1.SpaceRef{Id: spaceID}
		resp, err := t.clients.voice.MoveVoiceRoomParticipant(ctx, req)
		if err != nil {
			writeVoiceRoomMoveError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	}
	return false
}
