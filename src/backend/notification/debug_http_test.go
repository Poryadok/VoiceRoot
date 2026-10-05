package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/store"
)

func TestNotificationHTTPHandlerDoesNotRegisterDebugRecorderByDefault(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			request := httptest.NewRequest(method, "/debug/recorded-pushes", nil)
			recorder := httptest.NewRecorder()
			notificationHTTPHandler(serviceName).ServeHTTP(recorder, request)
			require.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestFCMPushCaptureWrapperIsSelectedOnlyByDebugGate(t *testing.T) {
	inner := &fcm.NoopSender{}
	require.Same(t, inner, maybeRecordFCMSender(inner, false))
	require.IsType(t, &fcm.RecordSender{}, maybeRecordFCMSender(inner, true))
}

func TestLegacyRecordingFlagDoesNotCaptureOrRegisterDebugRouteWithoutDebugGate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	profileID := uuid.New()
	token := store.DeviceToken{Token: "synthetic-fixture-token", PushService: "fcm"}
	payload := fcm.PushPayload{Title: "fixture title", Body: "fixture body", Data: map[string]string{"type": "fixture"}}
	inner := &fcmSendCapture{
		expectedContext: ctx, expectedProfileID: profileID,
		expectedToken: token, expectedPayload: payload,
	}
	debugEnabled, err := parseDebugHTTPEnabled(func(string) (string, bool) { return "", false }, true)
	require.NoError(t, err)
	require.False(t, debugEnabled)

	wrapped := maybeRecordFCMSender(inner, debugEnabled)
	err = wrapped.Send(ctx, profileID, token, payload)
	require.NoError(t, err)
	require.Equal(t, 1, inner.calls)
	require.True(t, inner.contextPreserved)
	require.True(t, inner.profilePreserved)
	require.True(t, inner.tokenPreserved)
	require.True(t, inner.payloadPreserved)
	_, recorded := fcm.GlobalPushRecorder.LastForProfile(profileID)
	require.False(t, recorded)

	handler := notificationHTTPHandlerWithReadinessAndDebug(serviceName, nil, debugEnabled)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/debug/recorded-pushes?profile_id="+profileID.String(), nil))
	require.Equal(t, http.StatusNotFound, response.Code)
}

type fcmSendCapture struct {
	calls             int
	expectedContext   context.Context
	expectedProfileID uuid.UUID
	expectedToken     store.DeviceToken
	expectedPayload   fcm.PushPayload
	contextPreserved  bool
	profilePreserved  bool
	tokenPreserved    bool
	payloadPreserved  bool
}

func (s *fcmSendCapture) Send(ctx context.Context, profileID uuid.UUID, token store.DeviceToken, payload fcm.PushPayload) error {
	s.calls++
	s.contextPreserved = ctx == s.expectedContext
	s.profilePreserved = profileID == s.expectedProfileID
	s.tokenPreserved = reflect.DeepEqual(token, s.expectedToken)
	s.payloadPreserved = reflect.DeepEqual(payload, s.expectedPayload)
	return nil
}

func TestEnabledCaptureDelegatesExactlyOnceWithUnchangedArguments(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	profileID := uuid.New()
	token := store.DeviceToken{Token: "synthetic-fixture-token", PushService: "fcm"}
	payload := fcm.PushPayload{Title: "fixture title", Body: "fixture body", Data: map[string]string{"type": "fixture"}}
	inner := &fcmSendCapture{
		expectedContext: ctx, expectedProfileID: profileID,
		expectedToken: token, expectedPayload: payload,
	}
	wrapped := maybeRecordFCMSender(inner, true)

	err := wrapped.Send(ctx, profileID, token, payload)

	require.NoError(t, err)
	require.Equal(t, 1, inner.calls)
	require.True(t, inner.contextPreserved)
	require.True(t, inner.profilePreserved)
	require.True(t, inner.tokenPreserved)
	require.True(t, inner.payloadPreserved)
	_, recorded := fcm.GlobalPushRecorder.LastForProfile(profileID)
	require.True(t, recorded)
}

func TestNotificationHTTPHandlerRegistersDebugRecorderOnlyWhenEnabled(t *testing.T) {
	handler := notificationHTTPHandlerWithReadinessAndDebug(serviceName, nil, true)
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/debug/recorded-pushes", nil))
	require.Equal(t, http.StatusBadRequest, get.Code)
	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/debug/recorded-pushes", nil))
	require.Equal(t, http.StatusMethodNotAllowed, post.Code)
}

func TestParseDebugHTTPEnabledRequiresStrictBooleanAndRecorderOptIn(t *testing.T) {
	lookup := func(value string, present bool) func(string) (string, bool) {
		return func(string) (string, bool) { return value, present }
	}
	for _, tc := range []struct {
		name      string
		value     string
		present   bool
		recording bool
		want      bool
		wantErr   bool
	}{
		{name: "absent", recording: true},
		{name: "true normalized", value: " TRUE ", present: true, recording: true, want: true},
		{name: "false normalized", value: " False ", present: true, recording: true},
		{name: "debug requires recording", value: "true", present: true, wantErr: true},
		{name: "empty rejected", present: true, recording: true, wantErr: true},
		{name: "numeric rejected", value: "1", present: true, recording: true, wantErr: true},
		{name: "yes rejected", value: "yes", present: true, recording: true, wantErr: true},
		{name: "abbreviated true rejected", value: "t", present: true, recording: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDebugHTTPEnabled(lookup(tc.value, tc.present), tc.recording)
			require.Equal(t, tc.want, got)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDebugRecorderComposeOptInAndProductionConfigsStayDisabled(t *testing.T) {
	// Source-level contract: backend CI runs this test with the Notification module.
	compose, err := readNotificationContractFile("docker-compose.yml")
	require.NoError(t, err)
	service := composeServiceBlock(t, compose, "notification")
	require.Contains(t, service, "NOTIFICATION_RECORD_PUSHES: \"true\"")
	require.Contains(t, service, "NOTIFICATION_DEBUG_HTTP_ENABLED: \"true\"")

	for _, path := range []string{"deploy/staging/configmap-app.yaml", "deploy/prod/configmap-app.yaml"} {
		config, readErr := readNotificationContractFile(path)
		require.NoError(t, readErr)
		require.Contains(t, config, "NOTIFICATION_RECORD_PUSHES: \"true\"")
		require.NotContains(t, config, debugHTTPEnabledEnv)
	}
}

func composeServiceBlock(t *testing.T, compose, name string) string {
	t.Helper()
	lines := strings.Split(compose, "\n")
	start := -1
	for i, line := range lines {
		if line == "  "+name+":" {
			start = i
			break
		}
	}
	require.NotEqual(t, -1, start)
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "  ") && !strings.HasPrefix(lines[i], "   ") && strings.HasSuffix(lines[i], ":") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func readNotificationContractFile(path string) (string, error) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	contents, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path)))
	return string(contents), err
}
