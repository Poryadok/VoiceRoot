package main

import (
	"net/http"
	"strings"

	notificationv1 "voice.app/voice/notification/v1"
)

func (t *transcoder) serveNotifications(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodPost && rest == "register-device":
		req := &notificationv1.RegisterDeviceRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.notification.RegisterDevice(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "unregister-device":
		req := &notificationv1.UnregisterDeviceRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.notification.UnregisterDevice(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "settings":
		req := &notificationv1.GetNotificationSettingsRequest{}
		if scope := queryFirst(r, "scope_type"); scope != "" {
			req.ScopeType = &scope
		}
		if scopeID := queryFirst(r, "scope_id"); scopeID != "" {
			req.ScopeId = &scopeID
		}
		resp, err := t.clients.notification.GetNotificationSettings(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPut && rest == "settings":
		req := &notificationv1.UpdateNotificationSettingsRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.notification.UpdateNotificationSettings(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPut && rest == "quiet-hours":
		req := &notificationv1.SetQuietHoursRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.notification.SetQuietHours(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	default:
		if rest != "" && strings.Contains(rest, "/") {
			return false
		}
		return false
	}
}
