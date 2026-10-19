package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/notification/internal/authctx"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

func releaseConfigForTest(t *testing.T) *notificationReleaseConfig {
	t.Helper()
	data, err := os.ReadFile("../pkg/releasepolicy/testdata/free-invite-alpha.json")
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	values := map[string]string{"VOICE_RELEASE_PROFILE": "free-invite-alpha", "VOICE_RELEASE_POLICY_FILE": filepath.Join(t.TempDir(), "policy.json"), "VOICE_RELEASE_POLICY_SHA256": hex.EncodeToString(sum[:]), "VOICE_RELEASE_SNAPSHOT_FILE": filepath.Join(t.TempDir(), "effective.json")}
	cfg, err := loadNotificationReleaseConfig(func(key string) (string, bool) { v, ok := values[key]; return v, ok }, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", func(string) ([]byte, error) { return data, nil })
	require.NoError(t, err)
	return cfg
}

type releaseFCMSpy struct{ calls int }

func (s *releaseFCMSpy) Send(context.Context, uuid.UUID, store.DeviceToken, fcm.PushPayload) error {
	s.calls++
	return nil
}

func TestReleaseRuntimeOneAuthorityCoversRegistrationAndFinalDispatch(t *testing.T) {
	cfg := releaseConfigForTest(t)
	spy := &releaseFCMSpy{}
	runtime, err := newNotificationDeliveryRuntime(cfg, nil, nil, spy, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, runtime.Pusher)
	require.NotNil(t, runtime.Service)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, uuid.NewString()))
	_, err = runtime.Service.RegisterDevice(ctx, &notificationv1.RegisterDeviceRequest{Platform: "android", Token: "otherwise-valid"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.NoError(t, runtime.Pusher.Send(ctx, uuid.New(), store.DeviceToken{Platform: "android", PushService: "fcm", Token: "old"}, push.Payload{}))
	require.Zero(t, spy.calls)
	require.NoError(t, runtime.Pusher.Send(ctx, uuid.New(), store.DeviceToken{Platform: "web", PushService: "fcm", Token: "web"}, push.Payload{}))
	require.Equal(t, 1, spy.calls)
	_, err = runtime.Snapshot("replica-1")
	require.Error(t, err, "a test observer cannot be represented as a real release adapter")
}

func TestReleaseSnapshotPrivateBoundAndExclusive(t *testing.T) {
	runtime, err := newNotificationDeliveryRuntime(releaseConfigForTest(t), nil, nil, nil, nil, nil)
	require.NoError(t, err)
	data, err := runtime.Snapshot("replica-1")
	require.NoError(t, err)
	var observed map[string]any
	require.NoError(t, json.Unmarshal(data, &observed))
	require.Equal(t, "notification", observed["service_id"])
	require.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", observed["build_source_sha"])
	require.Len(t, observed["policy_sha256"], 64)
	require.Len(t, observed["config_projection_sha256"], 64)
	require.Len(t, observed["route_inventory_sha256"], 64)
	require.Equal(t, "absent", observed["adapter_modes"].(map[string]any)["fcm"])
	require.Equal(t, "disabled", observed["adapter_modes"].(map[string]any)["apns"])
	path := filepath.Join(t.TempDir(), "effective.json")
	require.NoError(t, writeNotificationReleaseSnapshot(path, data))
	require.Error(t, writeNotificationReleaseSnapshot(path, []byte("replacement")), "stale evidence cannot be overwritten")
	actual, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data, actual)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestReleaseBootstrapRejectsBeforeListener(t *testing.T) {
	if os.Getenv("VOICE_TEST_RELEASE_BOOTSTRAP_CHILD") == "1" {
		main()
		return
	}
	for _, profile := range []string{"", "free-invite-alpha"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestReleaseBootstrapRejectsBeforeListener$")
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "VOICE_TEST_RELEASE_BOOTSTRAP_CHILD=1", "VOICE_RELEASE_PROFILE=" + profile, "LISTEN_ADDR=invalid-address", "DATABASE_URL="}
		out, err := cmd.CombinedOutput()
		require.Error(t, err)
		require.Contains(t, string(out), "notification release policy configuration", "must reject before DB/provider/listener work")
		require.False(t, strings.Contains(string(out), "listening"))
	}
}

func TestReleaseBootstrapCannotRecordOrFakeOfflinePresence(t *testing.T) {
	for _, key := range []string{"NOTIFICATION_RECORD_PUSHES", "NOTIFICATION_DEBUG_HTTP_ENABLED"} {
		base := releaseConfigForTest(t)
		values := map[string]string{"VOICE_RELEASE_PROFILE": "development", key: "true"}
		_, err := loadNotificationReleaseConfig(func(key string) (string, bool) { v, ok := values[key]; return v, ok }, "", nil)
		if key == "NOTIFICATION_RECORD_PUSHES" {
			require.NoError(t, err)
		}
		// Validate after actual config resolution, not from policy's desired bits.
		if key == "NOTIFICATION_RECORD_PUSHES" {
			base.recordPushes = true
		} else {
			base.debugRecorderEnabled = true
		}
		_, err = newNotificationDeliveryRuntime(base, nil, nil, nil, nil, nil)
		require.Error(t, err)
	}
}
