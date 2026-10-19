package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/notification/internal/apns"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/grpcsvc"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/releasepolicy"
)

var buildSourceSHA string

type notificationReleaseConfig struct {
	policy                             *releasepolicy.Policy
	recordPushes, debugRecorderEnabled bool
	snapshotPath                       string
}
type notificationDeliveryRuntime struct {
	Pusher  *dispatch.PushDispatcher
	Service *grpcsvc.NotificationGRPC
	config  notificationReleaseConfig
}

var errReleaseConfig = errors.New("invalid Notification release configuration")

func loadNotificationReleaseConfig(lookup func(string) (string, bool), source string, readFile func(string) ([]byte, error)) (*notificationReleaseConfig, error) {
	policy, err := releasepolicy.Load(lookup, source, readFile)
	if err != nil {
		return nil, errReleaseConfig
	}
	recordRaw, recordSet := lookup("NOTIFICATION_RECORD_PUSHES")
	record := strings.EqualFold(strings.TrimSpace(recordRaw), "true")
	if policy.Profile() == releasepolicy.Alpha && recordSet && !strings.EqualFold(strings.TrimSpace(recordRaw), "false") {
		return nil, errReleaseConfig
	}
	debug, err := parseDebugHTTPEnabled(lookup, record)
	if err != nil {
		return nil, errReleaseConfig
	}
	if policy.Profile() == releasepolicy.Alpha && (record || debug) {
		return nil, errReleaseConfig
	}
	snapshotPath, snapshotSet := lookup("VOICE_RELEASE_SNAPSHOT_FILE")
	if policy.Profile() == releasepolicy.Alpha && (!snapshotSet || !filepath.IsAbs(snapshotPath)) {
		return nil, errReleaseConfig
	}
	if policy.Profile() == releasepolicy.Development && snapshotSet {
		return nil, errReleaseConfig
	}
	return &notificationReleaseConfig{policy: policy, recordPushes: record, debugRecorderEnabled: debug, snapshotPath: snapshotPath}, nil
}

func newNotificationDeliveryRuntime(config *notificationReleaseConfig, tokens *store.DeviceTokenStore, settings *store.SettingsStore, fcmSender fcm.Sender, apnsSender apns.Sender, voipSender apns.VoIPSender) (*notificationDeliveryRuntime, error) {
	if config == nil || config.policy == nil || (config.policy.Profile() != releasepolicy.Alpha && config.policy.Profile() != releasepolicy.Development) {
		return nil, errReleaseConfig
	}
	if config.policy.Profile() == releasepolicy.Alpha {
		if config.recordPushes || config.debugRecorderEnabled {
			return nil, errReleaseConfig
		}
		if _, noop := fcmSender.(*fcm.NoopSender); noop {
			fcmSender = nil
		}
		apnsSender = nil
		voipSender = nil
	}
	pusher := &dispatch.PushDispatcher{ReleasePolicy: config.policy, FCM: fcmSender, APNs: apnsSender, VoIP: voipSender}
	service := &grpcsvc.NotificationGRPC{ReleasePolicy: config.policy, Tokens: tokens, Settings: settings, Pusher: pusher}
	if settings != nil {
		pusher.LifecycleDelivery = settings
		service.SpaceLifecycle = settings
	}
	return &notificationDeliveryRuntime{Pusher: pusher, Service: service, config: *config}, nil
}

func (runtime *notificationDeliveryRuntime) Snapshot(instance string) ([]byte, error) {
	if runtime == nil || runtime.config.policy == nil || runtime.config.policy.Profile() != releasepolicy.Alpha || runtime.Pusher == nil || runtime.Service == nil || runtime.Pusher.ReleasePolicy != runtime.config.policy || runtime.Service.ReleasePolicy != runtime.config.policy || runtime.Service.Pusher != runtime.Pusher || !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`).MatchString(instance) {
		return nil, errReleaseConfig
	}
	fcmMode := "absent"
	if runtime.Pusher.FCM != nil {
		sender, real := runtime.Pusher.FCM.(*fcm.HTTPSender)
		if !real || sender == nil {
			return nil, errReleaseConfig
		}
		fcmMode = "real"
	}
	adapters := map[string]string{"fcm": fcmMode, "apns": "disabled", "voip_apns": "disabled"}
	projection := struct {
		RecordPushes  bool              `json:"record_pushes"`
		DebugRecorder bool              `json:"debug_recorder"`
		Adapters      map[string]string `json:"adapter_modes"`
	}{runtime.config.recordPushes, runtime.config.debugRecorderEnabled, adapters}
	configBytes, err := json.Marshal(projection)
	if err != nil {
		return nil, errReleaseConfig
	}
	routes := []string{"dispatch:PushDispatcher.Send"}
	for _, method := range notificationv1.NotificationService_ServiceDesc.Methods {
		routes = append(routes, "grpc:/"+notificationv1.NotificationService_ServiceDesc.ServiceName+"/"+method.MethodName)
	}
	sort.Strings(routes)
	routeBytes, err := json.Marshal(routes)
	if err != nil {
		return nil, errReleaseConfig
	}
	digest := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	snapshot := struct {
		SchemaVersion          int               `json:"schema_version"`
		ServiceID              string            `json:"service_id"`
		InstanceID             string            `json:"instance_id"`
		BuildSourceSHA         string            `json:"build_source_sha"`
		PolicySHA256           string            `json:"policy_sha256"`
		Profile                string            `json:"profile"`
		ConfigProjectionSHA256 string            `json:"config_projection_sha256"`
		CapabilityStates       map[string]string `json:"capability_states"`
		GuardIDs               []string          `json:"guard_ids"`
		RouteInventorySHA256   string            `json:"route_inventory_sha256"`
		AdapterModes           map[string]string `json:"adapter_modes"`
	}{2, "notification", instance, runtime.config.policy.SourceSHA(), runtime.config.policy.SHA256(), releasepolicy.Alpha, digest(configBytes), runtime.config.policy.CapabilityStates(), []string{"notification.register_mobile_device.v1", "notification.dispatch_mobile_push.v1"}, digest(routeBytes), adapters}
	return json.Marshal(snapshot)
}

func writeNotificationReleaseSnapshot(path string, data []byte) error {
	if !filepath.IsAbs(path) || len(data) == 0 {
		return errReleaseConfig
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errReleaseConfig
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return errReleaseConfig
	}
	return nil
}
