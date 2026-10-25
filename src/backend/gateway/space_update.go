package main

import (
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
)

func isSpaceUpdateRequest(r *http.Request) bool {
	if r.Method != http.MethodPatch || !strings.HasPrefix(r.URL.Path, "/api/v1/spaces/") {
		return false
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/spaces/")
	id = strings.TrimSuffix(id, "/matchmaking/config")
	return id != "" && !strings.Contains(id, "/")
}

func (t *transcoder) serveSpaceUpdate(w http.ResponseWriter, r *http.Request) {
	w = &noStoreResponseWriter{ResponseWriter: w}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/spaces/")
	mm := strings.HasSuffix(id, "/matchmaking/config")
	if mm {
		id = strings.TrimSuffix(id, "/matchmaking/config")
	}
	if !canonicalLifecycleUUID(id) || r.URL.RawQuery != "" {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid Space update request"))
		return
	}
	if mm {
		t.serveSpaceMMUpdate(w, r, id)
		return
	}
	req := &spacev1.UpdateSpaceRequest{}
	if err := readLifecycleProtoJSON(r, req); err != nil {
		writeGRPCError(w, err)
		return
	}
	if req.SpaceId != "" && req.SpaceId != id {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "Space update target mismatch"))
		return
	}
	req.SpaceId = id
	ctx, err := t.spaceJoinContext(r, req, spacev1.SpaceService_UpdateSpace_FullMethodName)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	response, err := t.clients.spaceLifecycle.UpdateSpace(ctx, req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProtoJSON(w, http.StatusOK, response)
}

func (t *transcoder) serveSpaceMMUpdate(w http.ResponseWriter, r *http.Request, id string) {
	req := &spacev1.UpdateSpaceMmConfigRequest{}
	if err := readLifecycleProtoJSON(r, req); err != nil {
		writeGRPCError(w, err)
		return
	}
	if req.SpaceId != "" && req.SpaceId != id {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "Space update target mismatch"))
		return
	}
	req.SpaceId = id
	ctx, err := t.spaceJoinContext(r, req, spacev1.SpaceService_UpdateSpaceMmConfig_FullMethodName)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	response, err := t.clients.spaceLifecycle.UpdateSpaceMmConfig(ctx, req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProtoJSON(w, http.StatusOK, response)
}
