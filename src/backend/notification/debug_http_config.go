package main

import (
	"fmt"
	"strings"

	"voice/backend/notification/internal/fcm"
)

const debugHTTPEnabledEnv = "NOTIFICATION_DEBUG_HTTP_ENABLED"

func parseDebugHTTPEnabled(lookup func(string) (string, bool), recordPushes bool) (bool, error) {
	raw, present := lookup(debugHTTPEnabledEnv)
	if !present {
		return false, nil
	}

	var enabled bool
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true":
		enabled = true
	case "false":
		enabled = false
	default:
		return false, fmt.Errorf("%s must be true or false when set", debugHTTPEnabledEnv)
	}
	if enabled && !recordPushes {
		return false, fmt.Errorf("%s requires NOTIFICATION_RECORD_PUSHES=true", debugHTTPEnabledEnv)
	}
	return enabled, nil
}

func maybeRecordFCMSender(sender fcm.Sender, debugRecorderEnabled bool) fcm.Sender {
	if !debugRecorderEnabled {
		return sender
	}
	return &fcm.RecordSender{Inner: sender}
}
