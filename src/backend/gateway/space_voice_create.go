package main

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"strings"
	spacev1 "voice.app/voice/space/v1"
)

func isSpaceVoiceCreateRequest(r *http.Request) bool {
	if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/api/v1/spaces/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/spaces/"), "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] == "voice-rooms"
}

func (t *transcoder) serveSpaceVoiceCreate(w http.ResponseWriter, r *http.Request) {
	w = &noStoreResponseWriter{ResponseWriter: w}
	space := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/spaces/"), "/voice-rooms")
	if !canonicalLifecycleUUID(space) || r.URL.RawQuery != "" {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid voice room request"))
		return
	}
	req := &spacev1.CreateVoiceRoomRequest{}
	if err := readLifecycleProtoJSON(r, req); err != nil {
		writeGRPCError(w, err)
		return
	}
	if req.SpaceId != "" && req.SpaceId != space || strings.TrimSpace(req.Name) == "" {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid voice room request"))
		return
	}
	req.SpaceId = space
	// The shared producer consumes only Gateway's verified claims and emits the
	// exact typed body/account type; incoming identity headers are not signed.
	ctx, err := t.spaceJoinContext(r, req, spacev1.SpaceService_CreateVoiceRoom_FullMethodName)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	response, err := t.clients.spaceLifecycle.CreateVoiceRoom(ctx, req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProtoJSON(w, http.StatusOK, response)
}
