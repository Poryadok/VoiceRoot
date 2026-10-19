package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

func (t *transcoder) serveSpaceAudit(w http.ResponseWriter, r *http.Request, rest string) bool {
	parts := strings.Split(rest, "/")
	if r.Method != http.MethodGet || len(parts) != 2 || parts[1] != "audit-log" {
		return false
	}
	w = &noStoreResponseWriter{ResponseWriter: w}
	req, err := parseSpaceAuditQuery(parts[0], r.URL.RawQuery)
	if err != nil {
		writeSpaceLifecycleError(w, err)
		return true
	}
	ctx, err := t.lifecycleContext(r, req, spacev1.SpaceService_GetAuditLog_FullMethodName, uuid.NewString())
	if err != nil {
		writeSpaceLifecycleError(w, err)
		return true
	}
	resp, err := t.clients.spaceLifecycle.GetAuditLog(ctx, req)
	if err != nil {
		writeSpaceLifecycleError(w, err)
		return true
	}
	if resp.GetAuditLogList() == nil {
		writeSpaceLifecycleError(w, status.Error(codes.Unavailable, "invalid audit response"))
		return true
	}
	// The audit contract keeps an explicit empty entries array and terminal cursor.
	body, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(resp)
	if err != nil {
		writeSpaceLifecycleError(w, status.Error(codes.Unavailable, "invalid audit response"))
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
	return true
}

func parseSpaceAuditQuery(spaceID, raw string) (*spacev1.GetAuditLogRequest, error) {
	invalid := status.Error(codes.InvalidArgument, "invalid audit query")
	if !canonicalLifecycleUUID(spaceID) {
		return nil, invalid
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, invalid
	}
	for key, entries := range values {
		if len(entries) != 1 {
			return nil, invalid
		}
		switch key {
		case "actor_profile_id", "action", "from", "to", "cursor", "page_size":
		default:
			return nil, invalid
		}
	}
	req := &spacev1.GetAuditLogRequest{SpaceId: spaceID, Page: &commonv1.CursorPageRequest{PageSize: 50, Cursor: values.Get("cursor")}}
	if actor, present := values["actor_profile_id"]; present {
		if !canonicalLifecycleUUID(actor[0]) {
			return nil, invalid
		}
		req.ActorProfileId = &actor[0]
	}
	if action, present := values["action"]; present {
		if strings.TrimSpace(action[0]) == "" {
			return nil, invalid
		}
		req.Action = &action[0]
	}
	for key, target := range map[string]**timestamppb.Timestamp{"from": &req.From, "to": &req.To} {
		if value, present := values[key]; present {
			parsed, err := time.Parse(time.RFC3339Nano, value[0])
			if err != nil {
				return nil, invalid
			}
			stamp := timestamppb.New(parsed.UTC())
			if stamp.CheckValid() != nil {
				return nil, invalid
			}
			*target = stamp
		}
	}
	if req.From != nil && req.To != nil && !req.From.AsTime().Before(req.To.AsTime()) {
		return nil, invalid
	}
	if limit, present := values["page_size"]; present {
		parsed, err := strconv.ParseInt(limit[0], 10, 32)
		if err != nil || parsed < 1 || parsed > 100 {
			return nil, invalid
		}
		req.Page.PageSize = int32(parsed)
	}
	return req, nil
}
