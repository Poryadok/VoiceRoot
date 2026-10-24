package main

import (
	"errors"
	"os"
	"strconv"
	voicejwt "voice/backend/pkg/jwt"
	"voice/backend/pkg/releasepolicy"
)

func writeRealtimeReleaseSnapshot(runtime *releasepolicy.OwnerRuntime, hub *wsHub, updater presenceUpdater, config realtimeConfig) error {
	if runtime == nil || hub == nil {
		return errors.New("missing realtime release binding")
	}
	mode := "absent"
	if g, ok := updater.(*grpcPresenceUpdater); ok && g != nil && g.client != nil {
		mode = "grpc"
	}
	if runtime.Policy().Profile() == releasepolicy.Alpha && mode != "grpc" {
		return errors.New("unbound realtime User updater")
	}
	authMode := "absent"
	if validator, ok := config.tokenValidator.(*voicejwt.Validator); ok && validator != nil {
		authMode = "jwks"
	}
	if runtime.Policy().Profile() == releasepolicy.Alpha && (authMode != "jwks" || !config.sessionEpochStrict || config.sessionEpochFloor == nil) {
		return errors.New("realtime alpha requires JWT and strict session epoch enforcement")
	}
	instance, err := os.Hostname()
	if err != nil {
		return err
	}
	routes := []string{"http:GET /health", "http:GET /ready", "http:GET /ws", "ws:heartbeat", "ws:resume", "ws:subscribe", "ws:unsubscribe", "ws:typing_start", "ws:typing_stop", "ws:mark_read", "ws:delivery_ack", "ws:presence_update"}
	return runtime.WriteSnapshot("realtime", instance, hub.releasePolicy, []string{"realtime.presence_update_custom_status.v1"}, routes, map[string]string{"user_presence": mode, "authentication": authMode, "session_epoch_strict": strconv.FormatBool(config.sessionEpochStrict)})
}
