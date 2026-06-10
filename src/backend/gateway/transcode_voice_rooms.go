package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

func withVoiceRoomIDMetadata(ctx context.Context, voiceRoomID string) context.Context {
	if voiceRoomID == "" {
		return ctx
	}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		out := md.Copy()
		out.Set("x-voice-room-id", voiceRoomID)
		return metadata.NewOutgoingContext(ctx, out)
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-room-id", voiceRoomID))
}

func (t *transcoder) serveVoice(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	if strings.HasPrefix(rest, "rooms/") {
		parts := strings.Split(strings.TrimPrefix(rest, "rooms/"), "/")
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return false
		}
		voiceRoomID := strings.TrimSpace(parts[0])
		action := parts[1]
		switch {
		case r.Method == http.MethodPost && action == "join":
			resp, err := t.clients.voice.JoinVoiceRoom(ctx, &callsv1.JoinVoiceRoomRequest{
				VoiceRoomId: voiceRoomID,
				Space:       readSpaceRefJSON(r),
			})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		case r.Method == http.MethodPost && action == "leave":
			_, err := t.clients.voice.LeaveVoiceRoom(ctx, &callsv1.LeaveVoiceRoomRequest{
				VoiceRoomId: voiceRoomID,
			})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true
		case r.Method == http.MethodGet && action == "states":
			statesCtx := withVoiceRoomIDMetadata(ctx, voiceRoomID)
			resp, err := t.clients.voice.GetVoiceStates(statesCtx, &callsv1.GetVoiceStatesRequest{
				VoiceRoomId: &voiceRoomID,
			})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		}
	}

	return t.serveVoiceCalls(w, r, rest, ctx)
}

func readSpaceRefJSON(r *http.Request) *spacev1.SpaceRef {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		return &spacev1.SpaceRef{}
	}
	var payload struct {
		Space struct {
			ID string `json:"id"`
		} `json:"space"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return &spacev1.SpaceRef{}
	}
	return &spacev1.SpaceRef{Id: strings.TrimSpace(payload.Space.ID)}
}
