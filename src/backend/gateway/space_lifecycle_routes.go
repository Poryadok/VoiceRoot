package main

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
)

func (t *transcoder) serveSpaceLifecycle(w http.ResponseWriter, r *http.Request, rest string) bool {
	parts := strings.Split(rest, "/")
	isDelete := r.Method == http.MethodDelete && len(parts) == 1 && parts[0] != ""
	isRestore := r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "restore"
	isRead := r.Method == http.MethodGet && len(parts) == 1 && parts[0] != "" && t.clients.spaceLifecycle != nil
	if !isDelete && !isRestore && !isRead {
		return false
	}
	w = &noStoreResponseWriter{ResponseWriter: w}
	bad := func() { writeSpaceLifecycleError(w, status.Error(codes.InvalidArgument, "invalid lifecycle request")) }
	if !canonicalLifecycleUUID(parts[0]) {
		bad()
		return true
	}
	if isRead {
		if r.URL.RawQuery != "" {
			bad()
			return true
		}
		req := &spacev1.GetSpaceRequest{SpaceId: parts[0]}
		ctx, err := t.lifecycleContext(r, req, spacev1.SpaceService_GetSpace_FullMethodName, uuid.NewString())
		if err != nil {
			writeSpaceLifecycleError(w, err)
			return true
		}
		resp, err := t.clients.spaceLifecycle.GetSpace(ctx, req)
		if err != nil {
			writeSpaceLifecycleError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	}
	if isDelete {
		req := &spacev1.DeleteSpaceRequest{}
		if err := readLifecycleProtoJSON(r, req); err != nil {
			writeSpaceLifecycleError(w, err)
			return true
		}
		if req.SpaceId != "" && req.SpaceId != parts[0] || !canonicalLifecycleUUID(req.OperationId) || req.ConfirmationName == "" || req.Proof == "" {
			bad()
			return true
		}
		req.SpaceId = parts[0]
		ctx, err := t.lifecycleContext(r, req, spacev1.SpaceService_DeleteSpace_FullMethodName, req.OperationId)
		if err != nil {
			writeSpaceLifecycleError(w, err)
			return true
		}
		_, err = t.clients.spaceLifecycle.DeleteSpace(ctx, req)
		if err != nil {
			writeSpaceLifecycleError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	req := &spacev1.RestoreSpaceRequest{}
	if err := readLifecycleProtoJSON(r, req); err != nil {
		writeSpaceLifecycleError(w, err)
		return true
	}
	if req.SpaceId != "" && req.SpaceId != parts[0] || !canonicalLifecycleUUID(req.OperationId) {
		bad()
		return true
	}
	req.SpaceId = parts[0]
	ctx, err := t.lifecycleContext(r, req, spacev1.SpaceService_RestoreSpace_FullMethodName, req.OperationId)
	if err != nil {
		writeSpaceLifecycleError(w, err)
		return true
	}
	resp, err := t.clients.spaceLifecycle.RestoreSpace(ctx, req)
	if err != nil {
		writeSpaceLifecycleError(w, err)
		return true
	}
	writeProtoJSON(w, http.StatusOK, resp)
	return true
}
