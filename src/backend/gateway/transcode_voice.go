package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
)

func (t *transcoder) serveVoice(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodPost && rest == "calls":
		req, err := readStartCallJSON(r)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.voice.StartCall(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "calls/active":
		resp, err := t.clients.voice.GetActiveCall(ctx, &callsv1.GetActiveCallRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case strings.HasPrefix(rest, "calls/"):
		parts := strings.Split(strings.TrimPrefix(rest, "calls/"), "/")
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return false
		}
		roomID := strings.TrimSpace(parts[0])
		action := parts[1]
		switch {
		case r.Method == http.MethodPost && action == "accept":
			resp, err := t.clients.voice.AcceptCall(ctx, &callsv1.AcceptCallRequest{RoomId: roomID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		case r.Method == http.MethodPost && action == "decline":
			resp, err := t.clients.voice.DeclineCall(ctx, &callsv1.DeclineCallRequest{RoomId: roomID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		case r.Method == http.MethodPost && action == "join":
			resp, err := t.clients.voice.JoinCall(ctx, &callsv1.JoinCallRequest{RoomId: roomID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		case r.Method == http.MethodPost && action == "leave":
			_, err := t.clients.voice.LeaveCall(ctx, &callsv1.LeaveCallRequest{RoomId: roomID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true
		case r.Method == http.MethodPost && action == "end":
			_, err := t.clients.voice.EndCall(ctx, &callsv1.EndCallRequest{RoomId: roomID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true
		case r.Method == http.MethodGet && action == "token":
			resp, err := t.clients.voice.GetJoinToken(ctx, &callsv1.GetJoinTokenRequest{RoomId: roomID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		case r.Method == http.MethodPatch && action == "state":
			req := &callsv1.UpdateVoiceStateRequest{RoomId: roomID}
			if err := readProtoJSON(r, req); err != nil {
				writeGRPCError(w, err)
				return true
			}
			req.RoomId = roomID
			resp, err := t.clients.voice.UpdateVoiceState(ctx, req)
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		case r.Method == http.MethodGet && action == "states":
			resp, err := t.clients.voice.GetVoiceStates(ctx, &callsv1.GetVoiceStatesRequest{RoomId: roomID})
			if err != nil {
				writeGRPCError(w, err)
				return true
			}
			writeProtoJSON(w, http.StatusOK, resp)
			return true
		}

	}

	return false
}

func readStartCallJSON(r *http.Request) (*callsv1.StartCallRequest, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid_body")
	}
	if len(body) == 0 {
		return &callsv1.StartCallRequest{}, nil
	}

	var req callsv1.StartCallRequest
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid_json")
	}
	if mediaRaw, ok := raw["media_kind"]; ok {
		var media string
		if err := json.Unmarshal(mediaRaw, &media); err == nil {
			switch strings.ToLower(strings.TrimSpace(media)) {
			case "audio":
				raw["media_kind"] = json.RawMessage(`"CALL_MEDIA_KIND_AUDIO"`)
			case "video":
				raw["media_kind"] = json.RawMessage(`"CALL_MEDIA_KIND_VIDEO"`)
			}
		}
	}
	normalized, err := json.Marshal(raw)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid_json")
	}
	if err := protoJSONUnmarshal.Unmarshal(normalized, &req); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid_json")
	}
	return &req, nil
}
